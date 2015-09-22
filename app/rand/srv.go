package main

import (
	"fmt"
	"bytes"
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/protobuf"
)

// XXX should be config items
const thresT = 2
const thresR = 2
const thresN = 3

func pickInsurers(suite abstract.Suite, group []Server,
		Rc, Rs []byte) ([]int, []abstract.Point) {

	// Seed the PRNG for insurer selection
	var key []byte
	key = append(key, Rc...)
	key = append(key, Rs...)
	prng := suite.Cipher(key)

	ntrustees := thresN
	nservers := len(group)
	sel := make([]int, ntrustees)
	pub := make([]abstract.Point, ntrustees)
	for i := 0; i < ntrustees; i++ {
		sel[i] = int(random.Uint64(prng) % uint64(nservers))
		pub[i] = group[sel[i]].keypair.Public
	}
	return sel, pub
}

type Server struct {

	// Network interface
	host Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite   abstract.Suite
	rand    abstract.Cipher
	keysize int
	keypair	config.KeyPair

	// XXX servers shouldn't really need to know everyone else
	group	[]Server
}

func (s *Server) init(host Host, suite abstract.Suite, group []Server) {
	s.host = host
	s.suite = suite
	s.rand = suite.Cipher(abstract.RandomKey)
	s.keysize = s.rand.KeySize()
	s.keypair.Gen(suite, s.rand)
	s.group = group
}

func (s *Server) serve(conn Conn) (err error) {

	// Receive client's I1
	var msg []byte
	if msg, err = conn.Recv(); err != nil {
		return
	}
	var i1 I1
	if err = protobuf.Decode(msg, &i1); err != nil {
		return
	}

	// Choose server's trustee-selection randomness
	Rs := make([]byte, s.keysize)
	s.rand.XORKeyStream(Rs, Rs)

	// Send our R1
	var r1 R1
	r1.HRs = abstract.Sum(s.suite, Rs)
	r1b, err := protobuf.Encode(&r1)
	if err != nil {
		return err
	}
	if err = conn.Send(r1b); err != nil {
		return err
	}

	// Receive client's I2
	if msg, err = conn.Recv(); err != nil {
		return
	}
	var i2 I2
	if err = protobuf.Decode(msg, &i2); err != nil {
		return
	}
	Rc := i2.Rc
	HRc := abstract.Sum(s.suite, Rc)
	if !bytes.Equal(HRc, i1.HRc) {
		err = errors.New("client random hash mismatch")
		return
	}

	// Construct our Deal
	secPair := &config.KeyPair{}
	secPair.Gen(s.suite, random.Stream)
	_, inspub := pickInsurers(s.suite, s.group, Rc, Rs)
	deal := &poly.Promise{}
	deal.ConstructPromise(secPair, &s.keypair, thresT, thresR, inspub)
	dealb, err := deal.MarshalBinary()
	if err != nil {
		return
	}

	// Send our R2
	r2 := R2{ Rs: Rs, Deal: dealb }
	r2b, err := protobuf.Encode(&r2)
	if err != nil {
		return
	}
	if err = conn.Send(r2b); err != nil {
		return
	}

	// Receive client's I3
	if msg, err = conn.Recv(); err != nil {
		return
	}
	var i3 I3
	if err = protobuf.Decode(msg, &i3); err != nil {
		return
	}

	// XXX cross-check Deals

	// Send our R3
	var r3 R3
	r3b, err := protobuf.Encode(&r3)
	if err != nil {
		return err
	}
	if err = conn.Send(r3b); err != nil {
		return err
	}

	// Receive client's I4
	if msg, err = conn.Recv(); err != nil {
		return
	}
	var i4 I4
	if err = protobuf.Decode(msg, &i4); err != nil {
		return
	}

	// XXX validate

	// Send our R4
	var r4 R4
	r4b, err := protobuf.Encode(&r4)
	if err != nil {
		return
	}
	if err = conn.Send(r4b); err != nil {
		return
	}

	return
}

