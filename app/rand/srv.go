package main

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
)

type Server struct {

	// Network interface
	host	Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite	abstract.Suite
	rand	abstract.Cipher
	keysize	int
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

func (s *Server) serve() error {

	msg, cli := s.host.Recv()
	var i1 I1
	if err := protobuf.Decode(msg, &i1); err != nil {
		return err
	}

	// Choose server's trustee-selection randomness
	Rs := make([]byte, s.keysize)
	s.rand.XORKeyStream(Rs, Rs)

	var r1 R1
	r1.HRs = abstract.Sum(s.suite, Rs)
	r1b, err := protobuf.Encode(&r1)
	if err != nil {
		return err
	}
	if err := cli.Send(r1b); err != nil {
		return err
	}

	return nil
}

func (s *Server) run() {

	if err := s.serve(); err != nil {
		panic(err)
	}

	println(s.host.Name() + " done")
}

