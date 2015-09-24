package main

import (
	"bytes"
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
	"github.com/dedis/protobuf"
	"log"
)

// XXX should be config items
const thresT = 3
const thresR = 3
const thresN = 5

func pickInsurers(suite abstract.Suite, group []abstract.Point,
	Rc, Rs []byte) []int {

	// Seed the PRNG for insurer selection
	var key []byte
	key = append(key, Rc...)
	key = append(key, Rs...)
	prng := suite.Cipher(key)

	ntrustees := thresN
	nservers := len(group)
	sel := make([]int, ntrustees)
	for i := 0; i < ntrustees; i++ {
		sel[i] = int(random.Uint64(prng) % uint64(nservers))
	}
	return sel
}

type Server struct {

	// Network interface
	host Host

	// XXX use more generic pub/private keypair infrastructure
	// to support PGP, SSH, etc keys?
	suite   abstract.Suite
	rand    abstract.Cipher
	keysize int
	keypair config.KeyPair

	// XXX servers shouldn't really need to know everyone else
	group []abstract.Point
	self  int // our server index
}

func (s *Server) init(host Host, suite abstract.Suite,
	group []abstract.Point, self int) {
	s.host = host
	s.suite = suite
	s.rand = suite.Cipher(abstract.RandomKey)
	s.keysize = s.rand.KeySize()
	s.keypair.Gen(suite, s.rand)
	s.group = group
	s.self = self
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
	sel := pickInsurers(s.suite, s.group, Rc, Rs)
	selkeys := make([]abstract.Point, len(sel))
	for i := range sel {
		selkeys[i] = s.group[sel[i]]
	}
	deal := &poly.Promise{}
	deal.ConstructPromise(secPair, &s.keypair, thresT, thresR, selkeys)
	dealb, err := deal.MarshalBinary()
	if err != nil {
		return
	}

	// Send our R2
	r2 := R2{Rs: Rs, Deal: dealb}
	r2b, err := protobuf.Encode(&r2)
	if err != nil {
		return
	}
	if err = conn.Send(r2b); err != nil {
		return
	}

	// Receive client's I3
	var i3 I3
	if msg, err = conn.Recv(); err != nil {
		return
	}
	if err = protobuf.Decode(msg, &i3); err != nil {
		return
	}

	// Decrypt and validate all the shares we've been dealt.
	nsrv := len(s.group)
	if len(i3.R2s) != nsrv {
		return errors.New("wrong-length R2 array in I3 message")
	}
	shares := []R4Share{}
	r3resps := []R3Resp{}
	for i := 0; i < nsrv; i++ {
		r2i := R2{}
		r2ib := i3.R2s[i]
		if len(r2ib) == 0 {
			continue // Missing R2 - that's OK, just skip
		}
		if err = protobuf.Decode(r2ib, &r2i); err != nil {
			return
		}
		// XXX equivocation-check other servers' responses

		// Unmarshal and validate server i's Deal
		deal := &poly.Promise{}
		deal.UnmarshalInit(thresT, thresR, thresN, s.suite)
		if err = deal.UnmarshalBinary(r2i.Deal); err != nil {
			return
		}

		// Which insurers did server i deal its secret to?
		sel := pickInsurers(s.suite, s.group, Rc, r2i.Rs)
		for k := range sel {
			if sel[k] != s.self {
				continue // share dealt to someone else
			}

			// Decrypt and validate the specific share we were dealt
			// XXX produce response rather than returning if invalid
			share, resp, err := deal.ProduceResponse(
				k, &s.keypair)
			if err != nil {
				return err
			}

			// Marshal the response to return to the client
			var r3resp R3Resp
			r3resp.Dealer = i
			r3resp.Index = k
			r3resp.Resp, err = resp.MarshalBinary()
			if err != nil {
				return err
			}
			r3resps = append(r3resps, r3resp)

			// Save the revealed share for later
			shares = append(shares, R4Share{i, k, share})

			log.Printf("server %d dealt server %d share %d",
				i, s.self, k)
		}
	}

	// Send our R3
	r3 := R3{Resp: r3resps}
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

	// Validate the R4, mainly just making sure it's a subset of the R3 set
	if len(i4.R2s) != nsrv {
		return errors.New("wrong-length R2 array in I4 message")
	}
	for i := 0; i < nsrv; i++ {
		r2ib := i4.R2s[i]
		if len(r2ib) != 0 && !bytes.Equal(r2ib, i3.R2s[i]) {
			return errors.New("R2 set in I4 not a subset of I3")
		}
	}

	// Send our R4
	// XXX but only if our deal is still included?
	r4 := R4{Shares: shares}
	r4b, err := protobuf.Encode(&r4)
	if err != nil {
		return
	}
	if err = conn.Send(r4b); err != nil {
		return
	}

	return
}
