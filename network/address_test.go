package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnType(t *testing.T) {
	var tests = []struct {
		Value    string
		Expected ConnType
	}{
		{"tcp", PlainTCP},
		{"tls", TLS},
		{"purb", PURB},
		{"tcp4", UnvalidConnType},
		{"_tls", UnvalidConnType},
	}

	for _, str := range tests {
		if connType(str.Value) != str.Expected {
			t.Error("Wrong ConnType for " + str.Value)
		}
	}
}

func TestAddressValid(t *testing.T) {
	var tests = []struct {
		Value    string
		Expected bool
	}{
		{"tls:10.0.0.4:2000", true},
		{"tcp:10.0.0.4:2000", true},
		{"purb:10.0.0.4:2000", true},
		{"tls4:10.0.0.4:2000", false},
		{"tls:1000.0.0.4:2000", false},
		{"tlsx10.0.0.4:2000", false},
		{"tls:10.0.0.4x2000", false},
		{"tlsx10.0.0.4x2000", false},
	}

	for i, str := range tests {
		add := Address(str.Value)
		assert.Equal(t, str.Expected, add.Valid(), "Address (%d) %s", i, str.Value)
	}
}
