package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
)

const (
	DefaultClientTomlName = "client.toml"
	DefaultServerTomlName = "server.toml"
	DefaultFWMark         = 51820
	AddPeerPath           = "/"
)

var (
	AppVersion     = "n/a"
	CommitHash     = "n/a"
	BuildTimestamp = "n/a"
)

func BuildVersionOutput(appName string) string {
	return fmt.Sprintf("%s %s (%s) built on %s\n", appName, AppVersion, CommitHash, BuildTimestamp)
}

// func logOnHandshake(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
// 	for _, val := range rawCerts {
// 		cert, err := x509.ParseCertificate(val)
// 		if err != nil {
// 			continue
// 		}
// 		log.Println("Cert log:", cert.IsCA, cert.SubjectKeyId, cert.Issuer, cert.Subject)
// 	}
// 	return nil

// }

// Adding the common cert pulling stuff here
func GetCommonTlsConfig(certPath string, keyPath string) (config *tls.Config, err error) {
	// this is only the key mapped cert, should be the first one in the bundle otherwise it will fail
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	config = &tls.Config{
		Certificates: []tls.Certificate{cert},
		// VerifyPeerCertificate: logOnHandshake,
	}
	return
}

func GetCACertPool(certPath string) (certPool *x509.CertPool, err error) {
	buf, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	// By default pull from systemCertPool, if there are any CAs in the bundle, add them too
	certPool, err = x509.SystemCertPool()
	if err != nil {
		log.Println("can't access system cert pool, continuing anyway...")
		certPool = x509.NewCertPool()
	}
	// modified the exisiting code for CertPool.AppendCertsFromPEM with just an extra cert.IsCA check
	for len(buf) > 0 {
		var block *pem.Block
		block, buf = pem.Decode(buf)

		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, err
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		if cert.IsCA {
			certPool.AddCert(cert)
		}
	}
	return
}

func GetClientConfig(certPath string, keyPath string) (config *tls.Config, err error) {
	config, err = GetCommonTlsConfig(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	certPool, err := GetCACertPool(certPath)
	if err != nil {
		return nil, err
	}

	// this will force matching of the serverName with the DNS or IP entry in your cert
	config.InsecureSkipVerify = false
	config.RootCAs = certPool
	config.ClientSessionCache = tls.NewLRUClientSessionCache(1)
	return config, nil
}

func GetServerConfig(certPath string, keyPath string) (config *tls.Config, err error) {
	config, err = GetCommonTlsConfig(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	certPool, err := GetCACertPool(certPath)
	if err != nil {
		return nil, err
	}

	config.ClientCAs = certPool
	config.ClientAuth = tls.RequireAndVerifyClientCert

	return config, nil
}

func GetHttpProtocolsConfig() (protocols *http.Protocols) {
	protocols = &http.Protocols{}
	protocols.SetHTTP2(true)
	protocols.SetHTTP1(false)
	protocols.SetUnencryptedHTTP2(false)
	return
}
