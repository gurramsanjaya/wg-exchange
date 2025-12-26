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
	"os"
	"path"

	"wg-exchange/cmd"
	"wg-exchange/models"

	"github.com/BurntSushi/toml"
)

const (
	uriFormat  = "http://%s/"
	confFormat = "%s.conf"
)

type clientProcessor struct {
	uri string
	// tlsCertPath      string
	defaultInterface models.Interface

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

	if resp, err := http.DefaultClient.Post(c.uri, "application/octet-stream", r); err != nil {
		log.Fatalln("http failure")
		return err
	} else {

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
		} else {

			clientConf := &models.ClientConfig{}
			if err := gob.NewDecoder(resp.Body).Decode(&clientConf); err != nil {
				return err
			} else {
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
			}
		}
	}
	return nil
}

func (c *clientProcessor) encode(w io.WriteCloser, val *models.Credentials) {
	defer w.Close()
	gob.NewEncoder(w).Encode(val)
}

func main() {
	version := flag.Bool("version", false, "version")
	configFile := flag.String("conf", cmd.DefaultClientTomlName, "config file name")
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

	if !wgeConf.Client.Endpoint.IsValid() {
		log.Println("invalid endpoint")
		return
	}

	if len(wgeConf.Client.IntrfcNames) == 0 {
		log.Println("no client interfaces found")
		return
	}

	proc := &clientProcessor{
		uri:              fmt.Sprintf(uriFormat, wgeConf.Client.Endpoint),
		defaultInterface: wgeConf.WgInterface,
		keepAlive:        wgeConf.Client.KeepAlive,
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
