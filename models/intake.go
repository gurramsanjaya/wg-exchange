package models

import (
	"net/netip"
)

type WGEServer struct {
	Ttl               int32          `toml:"Ttl"`
	ListenAddress     netip.AddrPort `toml:"ExchangeListenAddress"`
	IntrfcName        string         `toml:"InterfaceName"`
	TlsCert           string         `toml:"TlsCertPath"`
	TlsKey            string         `toml:"TlsKeyPath"`
	WireguardEndpoint netip.AddrPort `toml:"WireguardEndpoint"`
	WireguardDns      []netip.Addr   `toml:"WireguardDNS"`
}

type WGEClient struct {
	Endpoint    netip.AddrPort `toml:"ExchangeEndpoint"`
	IntrfcNames []string       `toml:"InterfaceNames"`
	KeepAlive   int8           `toml:"PersistentKeepAlive"`
}

// Skipping peer stuff here
type WGEServerConf struct {
	Server      WGEServer       `toml:"Server"`
	WgInterface ServerInterface `toml:"Interface"`
}

type WGEClientConf struct {
	Client      WGEClient `toml:"Client"`
	WgInterface Interface `toml:"Interface"`
}
