package main

import (
	"crypto/cipher"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
)

type Client struct {

	// Network interface
	host	Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite	abstract.Suite
	rand	cipher.Stream
//	pubKey	abstract.Point
//	priKey	abstract.Secret

	nsrv	int
	srv	[]Peer
}

func (c *Client) init(host Host, suite abstract.Suite, rand cipher.Stream,
			srvname []string) {
	c.host = host

	c.suite = suite
	c.rand = rand
//	c.priKey = priKey
//	c.pubKey = suite.Point().Mul(nil, priKey)

	c.nsrv = len(srvname)
	c.srv = make([]Peer, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		c.srv[i] = host.Find(srvname[i])
	}
}

func (c *Client) run() error {

	keysize := c.suite.Cipher(abstract.NoKey).KeySize()

	// Choose client's trustee-selection randomness
	Rc := make([]byte, keysize)
	c.rand.XORKeyStream(Rc, Rc)

	var i1 I1
	i1.HRc = abstract.Sum(c.suite, Rc)
	i1b, err := protobuf.Encode(&i1)
	if err != nil {
		return err
	}
	for i := 0; i < c.nsrv; i++ {
		if err = c.srv[i].Send(i1b); err != nil {
			return err
		}
	}

	println("done")
	return nil
}

