package main

import (
	"bytes"
	"crypto/cipher"
	"errors"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/sig"
	"github.com/dedis/protobuf"
	"log"
	"reflect"
)

type Client struct {

	// Network interface
	host Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite             abstract.Suite
	rand              cipher.Stream
	keysize, hashsize int
	seckey            sig.SecretKey

	session *Session // Unique session identifier tuple
	group   *Group   // Group parameter block

	nsrv   int
	srv    []Conn                 // Connections to communicate with each server
	srvpub []sig.SchnorrPublicKey // Public keys of all servers in group

	t Transcript // Third-party verifiable message transcript

	r1 []R1 // Decoded R1 messages
	r2 []R2 // Decoded R2 messages
	r3 []R3 // Decoded R3 messages
	r4 []R4 // Decoded R4 messages

	Rc     []byte           // Client's trustee-selection random value
	Rs     [][]byte         // Servers' trustee-selection random values
	deals  []poly.Promise   // Unmarshaled deals from servers
	shares []poly.PriShares // Revealed shares
}

func (c *Client) init(host Host, suite abstract.Suite, rand cipher.Stream,
	session *Session, group *Group,
	clisec sig.SecretKey, srvname []string, srvpub []sig.SchnorrPublicKey) {

	c.host = host
	c.suite = suite
	c.rand = rand

	c.session = session
	c.group = group

	cipher := c.suite.Cipher(abstract.NoKey)
	c.keysize = cipher.KeySize()
	c.hashsize = cipher.HashSize()
	c.seckey = clisec

	c.nsrv = len(srvname)
	c.srv = make([]Conn, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		c.srv[i] = host.Open(srvname[i])
	}
	c.srvpub = srvpub
}

func (c *Client) run() (err error) {

	// Choose client's trustee-selection randomness
	Rc := make([]byte, c.keysize)
	c.rand.XORKeyStream(Rc, Rc)
	c.Rc = Rc

	// Compute Session ID
	sessionb, err := protobuf.Encode(c.session)
	if err != nil {
		return err
	}
	sid := abstract.Sum(c.suite, sessionb)

	// Compute Group ID
	groupb, err := protobuf.Encode(c.group)
	if err != nil {
		panic(err)
	}
	gid := abstract.Sum(c.suite, groupb)

	// Phase 1: Send client's I1 message
	//log.Printf("Client: I1")
	i1 := I1{SID: sid, GID: gid, HRc: abstract.Sum(c.suite, Rc),
		S: sessionb, G: groupb}
	if c.t.I1, err = c.send(&i1); err != nil {
		return err
	}

	// Receive servers' R1 messages
	c.r1 = make([]R1, c.nsrv)
	c.t.R1 = make([][]byte, c.nsrv)
	c.recv(c.t.R1, c.r1, c.processR1)

	// Phase 2
	//log.Printf("Client: I2")
	i2 := I2{SID: sid, Rc: Rc}
	if c.t.I2, err = c.send(&i2); err != nil {
		return err
	}

	c.r2 = make([]R2, c.nsrv)
	c.t.R2 = make([][]byte, c.nsrv)
	c.deals = make([]poly.Promise, c.nsrv)
	c.recv(c.t.R2, c.r2, c.processR2)

	// Phase 3
	//log.Printf("Client: I3")
	i3 := I3{SID: sid, R2s: c.t.R2}
	if c.t.I3, err = c.send(&i3); err != nil {
		return err
	}

	c.r3 = make([]R3, c.nsrv)
	c.t.R3 = make([][]byte, c.nsrv)
	c.recv(c.t.R3, c.r3, c.processR3)

	// Phase 4
	//log.Printf("Client: I4")
	i4 := I4{SID: sid, R2s: c.t.R2}
	if c.t.I4, err = c.send(&i4); err != nil {
		return err
	}

	c.r4 = make([]R4, c.nsrv)
	c.t.R4 = make([][]byte, c.nsrv)
	c.shares = make([]poly.PriShares, c.nsrv)
	for i := 0; i < c.nsrv; i++ {
		if c.t.R2[i] != nil {
			c.shares[i].Empty(c.suite, thresT, thresN)
		}
	}
	c.recv(c.t.R4, c.r4, c.processR4)

	// Reconstruct the final secret
	output := c.suite.Secret().Zero()
	for i := range c.shares {
		if c.t.R2[i] != nil {
			log.Printf("reconstruct secret %d from %d shares\n",
				i, c.shares[i].NumShares())
			secret, err := c.shares[i].Secret()
			if err != nil { // not enough shares!?
				return err
			}
			output.Add(output, secret)
		}
	}

	log.Printf("Output value: %v", output)
	return nil
}

// Protobufs encode, sign, and send a message to all the servers.
func (c *Client) send(obj interface{}) (msg []byte, err error) {

	// Encode and sign the client's message.
	if msg, err = sigEncode(c.suite, c.seckey, c.rand, obj); err != nil {
		return
	}

	// Send it to all servers.
	for i := 0; i < c.nsrv; i++ {
		if c.srv[i] == nil {
			continue
		} // Server failed previously
		if err = c.srv[i].Send(msg); err != nil {
			c.fail(i, err)
		}
	}
	return
}

// Receive and decode messages from each of the operational servers
func (c *Client) recv(msgs [][]byte, objs interface{},
	process func(i int) error) {
	for i := 0; i < c.nsrv; i++ {
		c.recvFrom(i, msgs, objs, process)
	}
}

func (c *Client) recvFrom(i int, msgs [][]byte, objs interface{},
	process func(i int) error) {

	var err error
	defer func() {
		if err != nil {
			c.fail(i, err)
		}
	}()

	if c.srv[i] == nil {
		return
	} // Server failed previously

	// Receive message from server i
	if msgs[i], err = c.srv[i].Recv(); err != nil {
		return
	}

	// Decode the message into the appropriate object:
	// objs should be an array of the appropriate message type.
	objsv := reflect.ValueOf(objs)
	objp := objsv.Index(i).Addr().Interface()
	if err = sigDecode(c.suite, &c.srvpub[i], msgs[i], objp); err != nil {
		return
	}

	// Process the message
	if err = process(i); err != nil {
		return
	}
}

// Handle a server failure
func (c *Client) fail(i int, err error) {
	c.srv[i].Close()
	c.srv[i] = nil
	log.Printf("server %d failed: %s", i, err)
}

func (c *Client) processR1(i int) (err error) {

	// Validate the R1 response
	if !bytes.Equal(c.r1[i].HI1, abstract.Sum(c.suite, c.t.I1)) {
		return errors.New("server responded to wrong I1")
	}
	HRs := c.r1[i].HRs
	if len(HRs) != c.hashsize {
		return errors.New("HRs wrong length")
	}

	return nil
}

func (c *Client) processR2(i int) (err error) {

	// Validate the R2 response
	if !bytes.Equal(c.r2[i].HI2, abstract.Sum(c.suite, c.t.I2)) {
		return errors.New("server responded to wrong I2")
	}
	Rs := c.r2[i].Rs
	if len(Rs) != c.keysize {
		return errors.New("Rs wrong length")
	}
	HRs := abstract.Sum(c.suite, Rs)
	if !bytes.Equal(HRs, c.r1[i].HRs) {
		return errors.New("server random hash mismatch")
	}

	// Unmarshal and validate the Deal
	deal := &c.deals[i]
	deal.UnmarshalInit(thresT, thresR, thresN, c.suite)
	if err = deal.UnmarshalBinary(c.r2[i].Deal); err != nil {
		return
	}

	return
}

func (c *Client) processR3(i int) (err error) {

	// Validate the R3 responses and use them to eliminate bad shares
	if !bytes.Equal(c.r3[i].HI3, abstract.Sum(c.suite, c.t.I3)) {
		return errors.New("server responded to wrong I3")
	}
	for _, r3resp := range c.r3[i].Resp {
		j := r3resp.Dealer
		if j < 0 || j >= c.nsrv {
			return errors.New(fmt.Sprintf(
				"bad dealer %d in R3Resp", j))
		}
		if c.t.R2[j] == nil {
			log.Printf("discarding share from failed dealer %d", j)
			continue
		}

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
			c.t.R2[j] = nil
		}
	}

	return
}

func (c *Client) processR4(i int) (err error) {

	// Validate the R4 response and all the revealed shares
	if !bytes.Equal(c.r4[i].HI4, abstract.Sum(c.suite, c.t.I4)) {
		return errors.New("server responded to wrong I3")
	}
	Rc := c.Rc
	for _, r4share := range c.r4[i].Shares {
		j := r4share.Dealer
		idx := r4share.Index
		share := r4share.Share
		if j < 0 || j >= len(c.srvpub) {
			return errors.New(fmt.Sprintf(
				"bad dealer number %d in R3Resp", j))
		}
		if c.t.R2[j] == nil {
			log.Printf("discarding share from failed dealer %d", j)
			continue
		}

		// Verify that the share really was assigned to server i
		sel := pickInsurers(c.suite, c.srvpub, Rc, c.r2[j].Rs)
		if sel[idx] != i {
			return errors.New(fmt.Sprintf(
				"server %d claimed share it wasn't dealt", i))
		}

		// Verify the share
		err = c.deals[j].VerifyRevealedShare(idx, share)
		if err != nil {
			return
		}

		// Stash it for reconstruction
		c.shares[j].SetShare(idx, share)
		//log.Printf("dealer %d share %d for server %d received\n",
		//	j, idx, i)
	}

	return
}
