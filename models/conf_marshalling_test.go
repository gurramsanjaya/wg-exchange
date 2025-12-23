package models

import (
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
PersistentKeepAlive = 0
PublicKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

[Peer]
Endpoint = testing
PersistentKeepAlive = 0
PublicKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=

`

func TestBasicConfMarshalling(t *testing.T) {

	testConf := ClientConfig{
		Intrfc: Interface{
			Address: []string{"1.1.1.1", "1:1::1"},
			FwMark:  0x0000ca6c,
		},
		Config: Config{
			Peer: []Peer{
				{Endpoint: "test", Ips: []string{"2.2.2.2/24", "2:2:2::2/120"}},
				{Endpoint: "testing"},
			},
		},
	}
	val, err := testConf.MarshalText()

	if err != nil {
		t.Error("error:", err)
	}

	if string(val) != testConfVal {
		t.Fatal("marshal mismatch")
	}

}
