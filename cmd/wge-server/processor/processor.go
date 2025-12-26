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

	"wg-exchange/cmd"
	dbusclient "wg-exchange/cmd/wge-server/dbus_client"
	"wg-exchange/cmd/wge-server/terminator"
	"wg-exchange/models"

	"github.com/gofrs/flock"
)

const (
	wireguardPath = "/etc/wireguard/"
	maxIPIncr     = (1 << 8) - 1 // just the last byte of the address
	ipv6PeerMask  = 128
	ipv4PeerMask  = 32
)

var (
	DefaultAllowedIps = [...]string{"0.0.0.0/0", "::/0"}
)

type procEntry struct {
	creds models.Credentials
	ips   []string
}

// For quickly checking and dispatching clientconf back in response
type Store struct {
	sync.Mutex
	pubKeys []*ecdh.PublicKey

	dns       []string
	netIps    []netip.Prefix
	pub       *ecdh.PublicKey
	endpoint  string
	processor *Processor
}

// For slower addition to the server conf
type Processor struct {
	ch             chan procEntry
	intrfc         string
	path           string
	fLock          *flock.Flock
	systemdManager *dbusclient.SystemdManager
	servConf       models.ServerConfig
}

/** --- Store --- */

func cmp(a *ecdh.PublicKey, b *ecdh.PublicKey) int {
	tmpA := a.Bytes()
	tmpB := b.Bytes()
	// TODO: make this quicker
	for i := range len(tmpA) {
		if tmpA[i] < tmpB[i] {
			return -1
		} else if tmpA[i] > tmpB[i] {
			return 1
		}
	}
	return 0
}

func (s *Store) getNextIps(incr int) (clientAddress []string, serverPeerIps []string, err error) {
	if incr > maxIPIncr {
		return nil, nil, errors.New("max peers reached")
	}

	for _, val := range s.netIps {
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

func (s *Store) AddKey(creds models.Credentials) (*models.ClientConfig, error) {
	s.Lock()
	defer s.Unlock()

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
	if _, ok := slices.BinarySearchFunc(s.pubKeys, pub, cmp); ok {
		return nil, errors.New("rejected")
	}

	// Assign ips, incr = 1(server ip) + previous peers
	cIps, sIps, err := s.getNextIps(len(s.pubKeys) + 1)
	if err != nil {
		return nil, err
	}

	// conf to send to client
	c := &models.ClientConfig{
		Intrfc: models.Interface{
			Dns:     s.dns,
			Address: cIps,
			FwMark:  cmd.DefaultFWMark,
		},
		Config: models.Config{
			Peer: []models.Peer{
				{
					Endpoint:    s.endpoint,
					Ips:         DefaultAllowedIps[:],
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
	case s.processor.ch <- p:
	default:
		return nil, errors.New("buffer full")
	}

	s.pubKeys = append(s.pubKeys, pub)
	slices.SortFunc(s.pubKeys, cmp)

	return c, nil
}

func NewStore(servConf models.WGEServerConf) (store *Store, err error) {

	store = &Store{
		pubKeys: make([]*ecdh.PublicKey, 0, 20),
		dns:     make([]string, 0, 2),
		netIps:  make([]netip.Prefix, 0, 2),
		processor: &Processor{
			ch:             make(chan procEntry, 20),
			systemdManager: dbusclient.DefaultSystemdManager,
		},
	}

	proc := store.processor

	if servConf.Server.IntrfcName == "" {
		return nil, errors.New("invalid device name")
	}
	proc.intrfc = servConf.Server.IntrfcName
	proc.path = path.Join(wireguardPath, fmt.Sprintf("%s.conf", proc.intrfc))
	log.Println("server conf path:", proc.path)

	// file lock
	proc.fLock = flock.New(path.Join(os.TempDir(), fmt.Sprintf(".wge-%s", proc.intrfc)))

	// set endpoint into store
	if !servConf.Server.WireguardEndpoint.IsValid() {
		return nil, errors.New("invalid wireguard endpoint")
	}
	store.endpoint = servConf.Server.WireguardEndpoint.String()

	// set dns into store
	if len(servConf.Server.WireguardDns) == 0 {
		return nil, errors.New("dns is null")
	}
	for _, val := range servConf.Server.WireguardDns {
		if !val.IsValid() {
			return nil, errors.New("invalid dns")
		}
		store.dns = append(store.dns, val.String())
	}

	// set interface ips into store
	if len(servConf.WgInterface.Address) == 0 {
		return nil, errors.New("device ips is null")
	}
	for _, val := range servConf.WgInterface.Address {
		if tmp, err := netip.ParsePrefix(val); err != nil || !tmp.IsValid() {
			return nil, errors.New("device ips invalid")
		} else {
			store.netIps = append(store.netIps, tmp)
		}
	}

	// private key generation
	var privTemp *ecdh.PrivateKey
	if privTemp, err = ecdh.X25519().GenerateKey(rand.Reader); err != nil {
		return
	}
	store.pub = privTemp.PublicKey()

	// set private to conf
	servConf.WgInterface.Priv = privTemp.Bytes()

	// store server conf for now
	proc.servConf = models.ServerConfig{
		Intrfc: servConf.WgInterface,
	}
	proc.servConf.Intrfc.ListenPort = int32(servConf.Server.WireguardEndpoint.Port())

	terminator.HookInto(store.processor.RunProcessor)
	return store, nil
}

/** --- Processor --- */

func (p *Processor) initializeServerConf() error {
	buf, err := p.servConf.MarshalText()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()

	// check fstat once, just to be sure
	if fstat, err := f.Stat(); err != nil {
		return err
	} else if fstat.Mode()&0o640 == 0 {
		// might happen if the file is already present but with different mode
		return errors.New("device file mode mismatch")
	}

	if _, err := fmt.Fprint(f, string(buf)); err != nil {
		return err
	}
	return nil
}

func (p *Processor) processEntry(entry procEntry) error {
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
		f, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o640)
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

func (p *Processor) RunProcessor(ctx context.Context, cancel context.CancelFunc) {
	if ok, err := p.fLock.TryLock(); err != nil || !ok {
		// manually trigger cancel
		log.Println("another instance is currently running...", err)
		cancel()
		return
	}
	defer p.fLock.Unlock()

	// write the initial interface to conf file, we already have fLock
	if err := p.initializeServerConf(); err != nil {
		log.Println("failure initializing server conf file with interface...", err)
		cancel()
		return
	}

	// try enabling the service
	if err := p.systemdManager.EnableAndStartService(p.intrfc); err != nil {
		log.Println("failure enabling service", err)
		cancel()
		return
	}

	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	unRefreshed := 0
	prevTime := time.Now()

	for range tick.C {
		select {
		case <-ctx.Done():
			// cleanup
			if unRefreshed > 0 {
				if err := p.systemdManager.RestartService(p.intrfc); err != nil {
					log.Println("failure restarting service...", err)
				}
			}
			return
		case entry := <-p.ch:
			if err := p.processEntry(entry); err != nil {
				log.Println("failure to add entry...", err)
				// disable server and stop service on error
				if err := p.systemdManager.DisableAndStopService(p.intrfc); err != nil {
					log.Println("failure disabling service", err)
				}
				cancel()
			}
			unRefreshed += 1
		default:
		}

		if unRefreshed > 0 && time.Since(prevTime) > time.Minute {
			if err := p.systemdManager.RestartService(p.intrfc); err != nil {
				log.Println("failure restarting service...", err)
				cancel()
			}
			prevTime = time.Now()
			unRefreshed = 0
		}
	}
}
