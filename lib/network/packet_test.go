package network

import (
	"bytes"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"reflect"
	"testing"
)

var s abstract.Suite = edwards.NewAES128SHA256Ed25519(false)
var key1 config.KeyPair = cliutils.KeyPair(s)
var key2 config.KeyPair = cliutils.KeyPair(s)

func TestListBasicSignatureMarshaling(t *testing.T) {
	Suite = s
	bs := BasicSignature{
		Pub:   key1.Public,
		Chall: key1.Secret,
		Resp:  key1.Secret,
	}
	var length int = 10
	sigs := make([]BasicSignature, length)
	for i := 0; i < length; i++ {
		sigs[i] = bs
	}
	lbs := ListBasicSignature{
		Length: length,
		Sigs:   sigs,
	}
	var buf bytes.Buffer
	err := s.Write(&buf, &lbs)
	if err != nil {
		t.Error("Marshaling BasicSiganture should not throw error")
	}
	bytesBuffer := buf.Bytes()

	bbs := &ListBasicSignature{}
	err = bbs.UnmarshalBinary(bytesBuffer)
	if err != nil {
		t.Error("Unmarshaling BasicSignature should not throw an error")
	}

	if bbs.Length != lbs.Length {
		t.Error("Unmarshaling did not give the same ListBasicSIganture")
	}
	for i := 0; i < length; i++ {
		if !reflect.DeepEqual(bbs.Sigs[i], lbs.Sigs[i]) {
			t.Error("Unmarshaling did not give the same ListBasicSignature")
		}
	}
}
