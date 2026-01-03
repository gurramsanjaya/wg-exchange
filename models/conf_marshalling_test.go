package models

import (
	"fmt"
	"testing"
)

const testConfVal = `[Interface]
Address = 1.1.1.1
Address = 1:1::1
FwMark = 51820
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

[Peer]
Endpoint = test
AllowedIPs = 2.2.2.2/24, 2:2:2::2/120
PublicKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
PresharedKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

[Peer]
Endpoint = testing
PersistentKeepAlive = 12
PublicKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
PresharedKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

`

func TestBasicConfMarshalling(t *testing.T) {

	testConf := ClientConfig{
		Intrfc: Interface{
			Address: []string{"1.1.1.1", "1:1::1"},
			FwMark:  0x0000ca6c,
			Priv:    make([]byte, 32),
		},
		Config: Config{
			Peer: []Peer{
				{
					Endpoint: "test",
					Ips:      []string{"2.2.2.2/24", "2:2:2::2/120"},
					Credentials: Credentials{
						Pub: make([]byte, 32),
						Psk: make([]byte, 32),
					},
				},
				{
					Endpoint:  "testing",
					KeepAlive: 12,
					Credentials: Credentials{
						Pub: make([]byte, 32),
						Psk: make([]byte, 32),
					},
				},
			},
		},
	}
	val, err := testConf.MarshalText()

	if err != nil {
		t.Error("error:", err)
	}

	if string(val) != testConfVal {
		fmt.Println(string(val))
		t.Fatal("marshal mismatch")
	}

}
