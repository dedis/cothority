package main

import (
	"bytes"
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
)

type Server struct {

	// Network interface
	host Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite   abstract.Suite
	rand    abstract.Cipher
	keysize int
	//	pubKey	abstract.Point
	//	priKey	abstract.Secret
}

func (s *Server) init(host Host, suite abstract.Suite) {
	s.host = host
	s.suite = suite
	s.rand = suite.Cipher(abstract.RandomKey)
	s.keysize = s.rand.KeySize()
	//	s.priKey = priKey
	//	s.pubKey = suite.Point().Mul(nil, priKey)
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
	HRc := abstract.Sum(s.suite, i2.Rc)
	if !bytes.Equal(HRc, i1.HRc) {
		err = errors.New("client random hash mismatch")
		return
	}

	// Construct our Deal
	// XXX

	// Send our R2
	var r2 R2
	r2.Rs = Rs
	r2b, err := protobuf.Encode(&r2)
	if err != nil {
		return err
	}
	if err = conn.Send(r2b); err != nil {
		return err
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

