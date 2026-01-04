package models

import (
	"net/netip"
)

type WGEServer struct {
	Ttl               int32          `toml:"Ttl"`
	IntrfcName        string         `toml:"InterfaceName"`
	WireguardEndpoint netip.AddrPort `toml:"WireguardEndpoint"`
	WireguardDns      []netip.Addr   `toml:"WireguardDNS"`
}

type WgClient struct {
	Name       string `toml:"Name"`
	GenerateQR bool   `toml:"GenerateQR"`
}

type WGEClient struct {
	Clients   []WgClient `toml:"WgClients"`
	KeepAlive int8       `toml:"PersistentKeepAlive"`
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
