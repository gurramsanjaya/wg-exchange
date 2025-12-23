package models

type Key []byte

type Credentials struct {
	Pub Key `toml:"PublicKey"`
	Psk Key `toml:"PrivateKey"`
}

type Peer struct {
	Endpoint  string   `toml:"Endpoint"`
	Ips       []string `toml:"AllowedIPs" singleline:"true"`
	KeepAlive int8     `toml:"PersistentKeepAlive"`
	Credentials
}

type Interface struct {
	Address  []string `toml:"Address"`
	Dns      []string `toml:"DNS"`
	FwMark   int32    `toml:"FwMark"`
	PreUp    []string `toml:"PreUp"`
	PostUp   []string `toml:"PostUp"`
	PreDown  []string `toml:"PreDown"`
	PostDown []string `toml:"PostDown"`
	Priv     Key      `toml:"PrivateKey"`
}

type ServerInterface struct {
	ListenPort int32 `toml:"ListenPort"`
	Interface
}

type Config struct {
	Peer []Peer `toml:"Peer"`
}

type ServerConfig struct {
	Intrfc ServerInterface `toml:"Interface"`
	Config
}

type ClientConfig struct {
	Intrfc Interface `toml:"Interface"`
	Config
}
