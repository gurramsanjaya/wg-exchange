package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"wg-exchange/cmd"
	"wg-exchange/models"

	"github.com/BurntSushi/toml"
)

const (
	confFormat = "%s.conf"
)

var (
	configFile = flag.String("conf", cmd.DefaultClientTomlName, "config file name")
	certPath   = flag.String("cert", "client.pem", "tls client cert bundle file")
	keyPath    = flag.String("key", "client.key", "tls client key file")
	endpoint   = flag.String("endpoint", "https://127.0.0.1:7777", "server endpoint")
	version    = flag.Bool("version", false, "version")
)

type clientProcessor struct {
	url              *url.URL
	defaultInterface models.Interface
	client           *http.Client

	keepAlive int8
}

func (c *clientProcessor) createClient(intrfcNm string) error {
	// open the file before hand
	if err := os.Mkdir(intrfcNm, 0o740); !errors.Is(err, os.ErrExist) {
		return err
	}
	fPath := path.Join(intrfcNm, fmt.Sprintf(confFormat, intrfcNm))
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()

	// Key generation
	priv, err1 := ecdh.X25519().GenerateKey(rand.Reader)
	psk, err2 := ecdh.P256().GenerateKey(rand.Reader)
	if err1 != nil || err2 != nil {
		return errors.Join(err1, err2)
	}
	pub := priv.PublicKey()

	r, w := io.Pipe()
	val := models.Credentials{
		Pub: pub.Bytes(),
		Psk: psk.Bytes(),
	}

	go c.encode(w, &val)

	addPeerURI := c.url
	addPeerURI.Path = cmd.AddPeerPath

	resp, err := c.client.Post(addPeerURI.String(), "application/octet-stream", r)
	if err != nil {
		log.Println("http failure")
		return err
	}
	defer resp.Body.Close()
	log.Println(resp.Status)
	if resp.StatusCode != http.StatusOK {

		var buf []byte
		if _, err := resp.Body.Read(buf); err != nil {
			return err
		} else {
			log.Println("body:", string(buf))
			return errors.New("status code other than 200")
		}
	}

	clientConf := &models.ClientConfig{}
	err = gob.NewDecoder(resp.Body).Decode(&clientConf)
	if err != nil {
		return err
	}

	if len(clientConf.Peer) != 1 {
		return errors.New("client has no peer")
	}
	clientConf.Intrfc.Priv = priv.Bytes()
	// Set defaults as needed
	clientConf.Intrfc.FwMark = c.defaultInterface.FwMark
	clientConf.Intrfc.PreUp = c.defaultInterface.PreUp
	clientConf.Intrfc.PreDown = c.defaultInterface.PreDown
	clientConf.Intrfc.PostUp = c.defaultInterface.PostUp
	clientConf.Intrfc.PostDown = c.defaultInterface.PostDown
	clientConf.Peer[0].KeepAlive = c.keepAlive

	if buf, err := clientConf.MarshalText(); err != nil {
		return err
	} else if _, err := fmt.Fprint(f, string(buf)); err != nil {
		return err
	}
	return nil
}

func (c *clientProcessor) encode(w io.WriteCloser, val *models.Credentials) {
	defer w.Close()
	gob.NewEncoder(w).Encode(val)
}

func validateEndpoint(endpoint string) (url *url.URL, err error) {
	// this validation is iffy...
	// TODO: see if https://github.com/davidmytton/url-verifier/ is feasible
	url, err = url.Parse(endpoint)
	if err != nil {
		return nil, err
	} else if url.Scheme != "https" || url.Opaque != "" {
		return nil, errors.New("not https scheme")
	}
	url.Path = ""
	url.Fragment = ""
	url.RawQuery = ""
	log.Println("using endpoint:", url)
	return
}

func main() {
	flag.Parse()

	if *version {
		fmt.Fprint(os.Stderr, cmd.BuildVersionOutput("WG-Exchange Client"))
		return
	}
	var wgeConf models.WGEClientConf

	if _, err := toml.DecodeFile(*configFile, &wgeConf); err != nil {
		log.Println("invalid toml conf file", err)
		return
	}

	if len(wgeConf.Client.IntrfcNames) == 0 {
		log.Println("no client interfaces found")
		return
	}

	url, err := validateEndpoint(*endpoint)
	if err != nil {
		log.Println("endpoint invalid...", err)
		return
	}

	config, err := cmd.GetClientConfig(*certPath, *keyPath)
	if err != nil {
		log.Println("cert,key issues...", err)
		return
	}

	proc := &clientProcessor{
		url:              url,
		defaultInterface: wgeConf.WgInterface,
		keepAlive:        wgeConf.Client.KeepAlive,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: config,
				Protocols:       cmd.GetHttpProtocolsConfig(),
			},
		},
	}

	// each should a different name so they don't overwrite
	for _, val := range wgeConf.Client.IntrfcNames {
		log.Println("trying client -", val)
		if err := proc.createClient(val); err != nil {
			log.Println(err)
		} else {
			log.Println("successfully created client -", val)
		}

	}
}
