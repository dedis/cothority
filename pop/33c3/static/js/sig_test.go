package main

import (
	"bytes"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/random"
	"github.com/stretchr/testify/assert"
)

func TestSig(t *testing.T) {
	suite := ed25519.NewAES128SHA256Ed25519(false)
	private := suite.NewKey(random.Stream)
	privMarsh, _ := private.MarshalBinary()
	public := suite.Point().Mul(nil, private)
	marsh1, _ := public.MarshalBinary()

	pr2 := suite.NewKey(random.Stream)
	pub2 := suite.Point().Mul(nil, pr2)
	marsh2, _ := pub2.MarshalBinary()

	var cont struct {
		Attendees [][]byte
		Nonce     string
		Context   string
	}

	cont.Attendees = make([][]byte, 2)
	cont.Attendees[0] = marsh1
	cont.Attendees[1] = marsh2
	cont.Nonce = "hello"
	cont.Context = "TestContext"

	var b bytes.Buffer
	err := toml.NewEncoder(&b).Encode(cont)
	assert.Nil(t, err)

	_, errStr := Sign(string(privMarsh), b.String())
	assert.Empty(t, errStr)
}
