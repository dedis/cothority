package main

import (
	"bytes"
	"crypto/cipher"
	"errors"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
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

	nsrv  int
	srv   []Conn           // Connections to communicate with each server
	group []abstract.Point // Public keys of all servers in group

	Transcript                  // Third-party verifiable message transcript
	Rc         []byte           // Client's trustee-selection random value
	Rs         [][]byte         // Servers' trustee-selection random values
	deals      []*poly.Promise  // Unmarshaled deals from servers
	shares     []poly.PriShares // Revealed shares
}

func (c *Client) init(host Host, suite abstract.Suite, rand cipher.Stream,
	srvname []string, srvpub []abstract.Point) {
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
	c.group = srvpub
}

func (c *Client) run() error {

	keysize := c.suite.Cipher(abstract.NoKey).KeySize()

	// Choose client's trustee-selection randomness
	Rc := make([]byte, keysize)
	c.rand.XORKeyStream(Rc, Rc)
	c.Rc = Rc

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
				log.Printf("recvR1 server %d: %s", i, err)
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
	c.Rs = make([][]byte, c.nsrv)
	c.deals = make([]*poly.Promise, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR2(i, &r1[i], &r2[i]); err != nil {
				log.Printf("recvR2 server %d: %s", i, err)
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
			if err := c.recvR3(i, &r3[i], c.deals); err != nil {
				log.Printf("recvR3 server %d: %s", i, err)
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
	c.shares = make([]poly.PriShares, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.R2[i] != nil {
			c.shares[i].Empty(c.suite, thresT, thresN)
		}
	}
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] != nil {
			if err := c.recvR4(i, &r4[i]); err != nil {
				log.Printf("recvR4 server %d: %s", i, err)
				c.srv[i] = nil
			}
		}
	}

	// Reconstruct the final secret
	output := c.suite.Secret().Zero()
	for i := range c.shares {
		if c.R2[i] != nil {
			log.Printf("reconstruct secret %d from %d shares\n",
				i, c.shares[i].NumShares())
			// XXX handle not-enough-shares gracefully
			secret := c.shares[i].Secret()
			output.Add(output, secret)
		}
	}

	log.Printf("Output value: %v", output)

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

	// Unmarshal and validate the Deal
	deal := &poly.Promise{}
	deal.UnmarshalInit(thresT, thresR, thresN, c.suite)
	if err = deal.UnmarshalBinary(r2.Deal); err != nil {
		return
	}
	c.Rs[i] = r2.Rs
	c.deals[i] = deal

	return
}

func (c *Client) recvR3(i int, r3 *R3, deals []*poly.Promise) (err error) {

	if c.R3[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(c.R3[i], r3); err != nil {
		return
	}

	// Validate the R3 responses and use them to eliminate bad shares
	for _, r3resp := range r3.Resp {
		j := r3resp.Dealer
		//idx := r3resp.Index	// XXX
		resp := &poly.Response{}
		resp.UnmarshalInit(c.suite)
		if err = resp.UnmarshalBinary(r3resp.Resp); err != nil {
			return
		}
		// XXX check that resp is securely bound to promise!!! FIX
		if !resp.Good() {
			log.Printf("server %d dealt bad promise to %d",
				j, i)
			c.R2[j] = nil
			c.deals[j] = nil
		}
	}

	return
}

func (c *Client) recvR4(i int, r4 *R4) (err error) {

	if c.R4[i], err = c.srv[i].Recv(); err != nil {
		return
	}
	e := protobuf.Encoding{Constructor: c.suite}
	if err = e.Decode(c.R4[i], r4); err != nil {
		return
	}

	// Validate the R4 response and all the revealed shares
	Rc := c.Rc
	for _, r4share := range r4.Shares {
		j := r4share.Dealer
		idx := r4share.Index
		share := r4share.Share
		if j < 0 || j >= len(c.group) || c.R2[j] == nil {
			log.Printf("discarded share from %d", j)
			continue
		}

		// Verify that the share really was assigned to server i
		sel := pickInsurers(c.suite, c.group, Rc, c.Rs[j])
		if sel[idx] != i {
			return errors.New(fmt.Sprintf(
				"server %d claimed share it wasn't dealt",
				i))
		}

		// Verify the share
		err = c.deals[j].VerifyRevealedShare(idx, share)
		if err != nil {
			return
		}

		// Stash it for reconstruction
		c.shares[j].SetShare(idx, share)
		log.Printf("dealer %d share %d for server %d received\n",
			j, idx, i)
	}

	return
}
