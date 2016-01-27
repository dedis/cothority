package main

import (
	"bytes"
	"crypto/cipher"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/sig"
	"github.com/dedis/protobuf"
	"reflect"
)

// XXX should be config items
const thresT = 3
const thresR = 3
const thresN = 5

func pickInsurers(suite abstract.Suite, srvpub []sig.SchnorrPublicKey,
	Rc, Rs []byte) []int {

	// Seed the PRNG for insurer selection
	var key []byte
	key = append(key, Rc...)
	key = append(key, Rs...)
	prng := suite.Cipher(key)

	ntrustees := thresN
	nservers := len(srvpub)
	sel := make([]int, ntrustees)
	for i := 0; i < ntrustees; i++ {
		sel[i] = int(random.Uint64(prng) % uint64(nservers))
	}
	return sel
}

func sigEncode(suite abstract.Suite, seckey sig.SecretKey, rand cipher.Stream,
	obj interface{}) (msg []byte, err error) {

	// Encode message
	enc, err := protobuf.Encode(obj)
	if err != nil {
		return nil, err
	}

	// Create signature
	buf := &bytes.Buffer{}
	wr := sig.Writer(buf, seckey, rand)
	if _, err = wr.Write(enc); err != nil {
		return nil, err
	}
	if err = wr.Close(); err != nil {
		return nil, err
	}

	msg = buf.Bytes()
	return msg, nil
}

func sigDecode(suite abstract.Suite, pubkey sig.PublicKey,
	msg []byte, obj interface{}) (err error) {

	// Check signature
	var n int = 0
	rd := sig.Reader(bytes.NewReader(msg), pubkey)
	if n, err = rd.Read(msg); err != nil {
		return err
	}

	// Decode message
	var cons = make(protobuf.Constructors)
	var secret abstract.Secret
	cons[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Secret() }
	return protobuf.DecodeWithConstructors(msg[:n], obj, cons)

}
