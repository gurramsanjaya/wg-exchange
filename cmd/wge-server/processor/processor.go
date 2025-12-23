package processor

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	dbusclient "wg-exchange/cmd/wge-server/dbus_client"
	"wg-exchange/cmd/wge-server/terminator"
	"wg-exchange/models"

	"github.com/gofrs/flock"
)

const (
	wireguardPath = "/etc/wireguard/"
	maxIncr       = (1 << 8) - 1
	ipv6PeerMask  = 128
	ipv4PeerMask  = 32
)

var (
	initErr error
	once    sync.Once

	store *peerStore
	proc  *processor

	allDefaultAllowedIps = [...]string{"0.0.0.0/0", "::/0"}
)

type procEntry struct {
	creds models.Credentials
	ips   []string
}

// For quickly checking and dispatching back clientconf
type peerStore struct {
	sync.Mutex
	pubKeys []*ecdh.PublicKey

	dns      []string
	netIps   []netip.Prefix
	pub      *ecdh.PublicKey
	endpoint string
}

// For slower addition to the server conf
type processor struct {
	ch             chan procEntry
	intrfc         string
	path           string
	fLock          *flock.Flock
	systemdManager *dbusclient.SystemdManager
}

func init() {
	store = &peerStore{
		pubKeys: make([]*ecdh.PublicKey, 0, 20),
		dns:     make([]string, 0, 2),
		netIps:  make([]netip.Prefix, 0, 2),
	}
	proc = &processor{
		ch: make(chan procEntry, 20),
	}

}

func cmp(a *ecdh.PublicKey, b *ecdh.PublicKey) int {
	tmpA := a.Bytes()
	tmpB := b.Bytes()
	for i := range len(tmpA) {
		if tmpA[i] < tmpB[i] {
			return -1
		} else if tmpA[i] > tmpB[i] {
			return 1
		}
	}
	return 0
}

func getNextIps(incr int) (clientAddress []string, serverPeerIps []string, err error) {
	if incr > maxIncr {
		return nil, nil, errors.New("max peers reached")
	}

	for _, val := range store.netIps {
		tmp := val.Addr().AsSlice()

		// increment
		tmp[len(tmp)-1] += byte(incr)
		tmpAddr, ok := netip.AddrFromSlice(tmp)

		if ok && val.Contains(tmpAddr) {
			c := netip.PrefixFrom(tmpAddr, val.Bits())

			var s netip.Prefix
			if val.Addr().Is4() {
				s, err = tmpAddr.Prefix(ipv4PeerMask)
			} else {
				s, err = tmpAddr.Prefix(ipv6PeerMask)
			}
			if err != nil {
				return nil, nil, errors.New("error deducing new address")
			}

			clientAddress = append(clientAddress, c.String())
			serverPeerIps = append(serverPeerIps, s.String())
		} else {
			return nil, nil, errors.New("network filled, no more peers can be added")
		}
	}
	return clientAddress, serverPeerIps, nil
}

func processEntry(entry procEntry) error {
	peerConf := &models.Config{
		Peer: []models.Peer{
			{
				Ips:         entry.ips,
				Credentials: entry.creds,
			},
		},
	}
	if buf, err := peerConf.MarshalText(); err != nil {
		return err
	} else {
		f, err := os.OpenFile(proc.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o640)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(buf); err != nil {
			return err
		}
	}
	return nil

}

func process(ctx context.Context, cancel context.CancelFunc) {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	if ok, err := proc.fLock.TryLock(); err != nil || !ok {
		// manually trigger cancel
		log.Println("couldn't acquire file lock, stopping...")
		cancel()
		return
	}
	defer proc.fLock.Unlock()

	unRefreshed := 0
	prevTime := time.Now()

	for range tick.C {
		select {
		case <-ctx.Done():
			// cleanup
			if unRefreshed > 0 {
				if err := proc.systemdManager.RestartService(proc.intrfc); err != nil {
					log.Println("failure restarting service...", err)
				}
			}
			return
		case entry := <-proc.ch:
			if err := processEntry(entry); err != nil {
				log.Println("failure to add entry...", err)
				cancel()
			}
			unRefreshed += 1
		default:
		}

		if unRefreshed > 0 && time.Since(prevTime) > time.Minute {
			if err := proc.systemdManager.RestartService(proc.intrfc); err != nil {
				log.Println("failure restarting service...", err)
				cancel()
			}
			prevTime = time.Now()
			unRefreshed = 0
		}
	}

}

func AddKey(creds models.Credentials) (*models.ClientConfig, error) {
	store.Lock()
	defer store.Unlock()

	// Verify keys
	// psk
	if creds.Psk != nil {
		if _, err := ecdh.X25519().NewPrivateKey(creds.Psk); err != nil {
			return nil, errors.New("invalid preshared key")
		}
	}
	// pub
	pub, err := ecdh.X25519().NewPublicKey(creds.Pub)
	if err != nil {
		return nil, errors.New("invalid public key")
	}

	// check if its previously sent
	if _, ok := slices.BinarySearchFunc(store.pubKeys, pub, cmp); ok {
		return nil, errors.New("rejected")
	}

	// Assign ips, incr = 1(server ip) + previous peers
	cIps, sIps, err := getNextIps(len(store.pubKeys) + 1)
	if err != nil {
		return nil, err
	}

	// conf to send to client
	c := &models.ClientConfig{
		Intrfc: models.Interface{
			Dns:     store.dns,
			Address: cIps,
		},
		Config: models.Config{
			Peer: []models.Peer{
				{
					Endpoint:    store.endpoint,
					Ips:         allDefaultAllowedIps[:],
					Credentials: creds,
				},
			},
		},
	}
	// entry to process
	p := procEntry{
		ips:   sIps,
		creds: creds,
	}

	select {
	case proc.ch <- p:
	default:
		return nil, errors.New("buffer full")
	}

	store.pubKeys = append(store.pubKeys, pub)
	slices.SortFunc(store.pubKeys, cmp)

	return c, nil
}

func InitProcessor(s models.WGEServerConf) error {
	once.Do(func() {
		if s.Server.IntrfcName == "" {
			initErr = errors.New("invalid device name")
			return
		}
		proc.intrfc = s.Server.IntrfcName
		proc.path = path.Join(wireguardPath, fmt.Sprintf("%s.conf", proc.intrfc))
		log.Println("path:", proc.path)

		// file lock
		proc.fLock = flock.New(path.Join(os.TempDir(), fmt.Sprintf(".wge-%s", proc.intrfc)))
		if l, err := proc.fLock.TryLock(); err != nil {
			initErr = err
			return
		} else if !l {
			initErr = errors.New("another instance is currently running")
			return
		}
		defer proc.fLock.Unlock()

		// set endpoint
		if !s.Server.WireguardEndpoint.IsValid() {
			initErr = errors.New("invalid wireguard endpoint")
			return
		}
		store.endpoint = s.Server.WireguardEndpoint.String()

		// set dns
		if len(s.Server.WireguardDns) == 0 {
			initErr = errors.New("dns is null")
			return
		}
		for _, val := range s.Server.WireguardDns {
			if !val.IsValid() {
				initErr = errors.New("invalid dns")
				return
			}
			store.dns = append(store.dns, val.String())
		}

		// set interface ips
		if len(s.WgInterface.Address) == 0 {
			initErr = errors.New("device ips is null")
			return
		}
		for _, val := range s.WgInterface.Address {
			if tmp, err := netip.ParsePrefix(val); err != nil || !tmp.IsValid() {
				initErr = errors.New("device ips invalid")
			} else {
				store.netIps = append(store.netIps, tmp)
			}
		}

		// private key generation
		var privTemp *ecdh.PrivateKey
		var err error
		if privTemp, err = ecdh.X25519().GenerateKey(rand.Reader); err != nil {
			initErr = err
			return
		}
		store.pub = privTemp.PublicKey()

		// set private to conf
		s.WgInterface.Priv = privTemp.Bytes()

		// write server conf to file
		tmp := models.ServerConfig{
			Intrfc: s.WgInterface,
		}
		tmp.Intrfc.ListenPort = int32(s.Server.WireguardEndpoint.Port())
		if buf, err := tmp.MarshalText(); err != nil {
			initErr = err
			return
		} else {
			f, err := os.OpenFile(proc.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o640)
			if err != nil {
				initErr = err
				return
			}
			defer f.Close()

			// check fstat once, just to be sure
			if fstat, err := f.Stat(); err != nil {
				initErr = err
				return
			} else if fstat.Mode()&0o640 == 0 {
				// might happen if the file is already present but with different mode
				initErr = errors.New("device file mode mismatch")
				return
			}

			if _, err := f.Write(buf); err != nil {
				initErr = err
				return
			}
		}

		// try enabling the service
		proc.systemdManager = dbusclient.GetManager()
		if err := proc.systemdManager.EnableAndStartService(proc.intrfc); err != nil {
			initErr = err
			return
		}

		terminator.HookInto(process)

	})
	return initErr
}
