package main

import (
	"bytes"
	"crypto/cipher"
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/protobuf"
	"log"
)

type Client struct {

	// Network interface
	host Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite abstract.Suite
	rand  cipher.Stream
	//	pubKey	abstract.Point
	//	priKey	abstract.Secret

	nsrv int
	srv  []Conn

	Transcript
}

func (c *Client) init(host Host, suite abstract.Suite, rand cipher.Stream,
	srvname []string) {
	c.host = host

	c.suite = suite
	c.rand = rand
	//	c.priKey = priKey
	//	c.pubKey = suite.Point().Mul(nil, priKey)

	c.nsrv = len(srvname)
	c.srv = make([]Conn, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		c.srv[i] = host.Open(srvname[i])
	}
}

func (c *Client) run() error {

	keysize := c.suite.Cipher(abstract.NoKey).KeySize()

	// Choose client's trustee-selection randomness
	Rc := make([]byte, keysize)
	c.rand.XORKeyStream(Rc, Rc)

	var err error
	var i1 I1
	i1.HRc = abstract.Sum(c.suite, Rc)
	if c.I1, err = protobuf.Encode(&i1); err != nil {
		return err
	}
	for i := 0; i < c.nsrv; i++ {
		if err = c.srv[i].Send(c.I1); err != nil {
			return err
		}
	}
	r1 := make([]R1, c.nsrv)
	c.R1 = make([][]byte, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR1(i, &r1[i]); err != nil {
				log.Printf("Server %d failed: %s", i, err)
				c.srv[i] = nil
			}
		}
	}

	// Phase 2
	i2 := I2{Rc: Rc}
	if c.I2, err = protobuf.Encode(&i2); err != nil {
		return err
	}
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err = c.srv[i].Send(c.I2); err != nil {
				return err
			}
		}
	}
	r2 := make([]R2, c.nsrv)
	c.R2 = make([][]byte, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR2(i, &r1[i], &r2[i]); err != nil {
				log.Printf("Server %d failed: %s", i, err)
				c.srv[i] = nil
			}
		}
	}

	// Phase 3
	i3 := I3{R2s: c.R2}
	if c.I3, err = protobuf.Encode(&i3); err != nil {
		return err
	}
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err = c.srv[i].Send(c.I3); err != nil {
				return err
			}
		}
	}
	r3 := make([]R3, c.nsrv)
	c.R3 = make([][]byte, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR3(i, &r3[i]); err != nil {
				log.Printf("Server %d failed: %s", i, err)
				c.srv[i] = nil
			}
		}
	}

	// Phase 4
	i4 := I4{R2s: c.R2}
	if c.I4, err = protobuf.Encode(&i4); err != nil {
		return err
	}
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err = c.srv[i].Send(c.I4); err != nil {
				return err
			}
		}
	}
	r4 := make([]R4, c.nsrv)
	c.R4 = make([][]byte, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR4(i, &r4[i]); err != nil {
				log.Printf("Server %d failed: %s", i, err)
				c.srv[i] = nil
			}
		}
	}

	// XXX reconstruct random value

	println("done")
	return nil
}

func (c *Client) recvR1(i int, r1 *R1) (err error) {

	if c.R1[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(c.R1[i], r1); err != nil {
		return
	}
	return
}

func (c *Client) recvR2(i int, r1 *R1, r2 *R2) (err error) {

	if c.R2[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(c.R2[i], r2); err != nil {
		return
	}

	// Validate the R2 response
	HRs := abstract.Sum(c.suite, r2.Rs)
	if !bytes.Equal(HRs, r1.HRs) {
		err = errors.New("server random hash mismatch")
		return err
	}
	// XXX check the Deal's basic validity

	return
}

func (c *Client) recvR3(i int, r3 *R3) (err error) {

	if c.R3[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(c.R3[i], r3); err != nil {
		return
	}

	// Validate the R3 response
	// XXX

	return
}

func (c *Client) recvR4(i int, r4 *R4) (err error) {

	if c.R4[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(c.R4[i], r4); err != nil {
		return
	}

	// Validate the R4 response
	// XXX check the returned shares

	return
}
