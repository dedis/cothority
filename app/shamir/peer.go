package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	conf "github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

// How many times a peer tries to connect to another until it works
var ConnRetry = 5

// How many time do you wait before trying to connect again
var ConnWaitRetry = time.Second * 2

type RemotePeer struct {
	// its connection
	Conn net.Conn
	// its name
	Hostname string
	// its id
	Id int
}

func (r *RemotePeer) String() string {
	return fmt.Sprintf("RemotePeer: %s (id: %d)", r.Hostname, r.Id)
}

func (r *RemotePeer) IsRoot() bool {
	return r.Id == 0
}

type Finish struct {
	Id int
}
type Peer struct {
	// simple int representing its index in the matrix of peers
	Id int
	// its own IP addr : Port
	Name string

	// the slice of peers connected to it
	remote map[int]RemotePeer

	// wether it is a "passive" peer or a "root" peer (i.e. actively starting signatures etc)
	root bool

	// N, R, T parameters + suite used throughout the process
	info poly.Threshold

	// suite used
	suite abstract.Suite

	// its own private / public key pair
	key conf.KeyPair

	// public key list
	pubKeys []abstract.Point

	// its Schnorr struct so every peer is able to gen a signature
	schnorr *poly.Schnorr

	// channel that handles the synchronization of the SYN between each peers
	synChan chan Syn
	// channel that handles the synchronization of ACK between  the peers
	ackChan chan Ack
	// channel that handles the synchronization of the END of the algorithm between the peers
	wgFin sync.WaitGroup
}

// NewPeer returns a new peer with its id and the number of peers in the schnorr signature algo
// TODO verification of string addr:port
func NewPeer(id int, name string, suite abstract.Suite, p poly.Threshold, isRoot bool) *Peer {

	if id >= p.N {
		log.Fatal("Error while NewPeer: gien", id, "as id whereas polyinfo.N =", p.N)

	}
	// Setup of the private / public pair
	key := cliutils.KeyPair(suite)
	// setup of the public list of key
	pubKeys := make([]abstract.Point, p.N)
	pubKeys[id] = key.Public
	dbg.Lvl3(name, "(id", id, ") has created its private/public key: public =>", key.Public)

	return &Peer{
		Id:      id,
		remote:  make(map[int]RemotePeer),
		root:    isRoot,
		Name:    name,
		info:    p,
		suite:   suite,
		key:     key,
		pubKeys: pubKeys,
		schnorr: new(poly.Schnorr),
		synChan: make(chan Syn),
		ackChan: make(chan Ack),
	}
}
func (p *Peer) IsRoot() bool {
	return p.root
}
func (p *Peer) Listen() {
	results := strings.Split(p.Name, ":")
	port := ":" + results[1]
	ln, err := net.Listen("tcp", port)
	if err != nil {
		dbg.Fatal(p.Name, ": Error while listening on port", port, "ABORT  =>", err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg.Fatal(p.Name, ": Error while listening on port", port, "=>", err)
		}
		go p.synWithPeer(conn)
	}
}

// ConnectTo will connect to the given host and start the SYN exchange (public key + id)
func (p *Peer) ConnectTo(host string) error {
	tick := time.NewTicker(ConnWaitRetry)
	count := 0
	for range tick.C {
		// connect
		conn, err := net.Dial("tcp", host)
		if err != nil {
			// we have tried too many times => abort
			if count == ConnRetry {
				tick.Stop()
				dbg.Fatal(p.Name, "could not connect to", host, "", ConnRetry, "times. Abort.")
				// let's try again one more time
			} else {
				dbg.Lvl2(p.Name, "could not connect to", host, ". Retry in", ConnWaitRetry.String())
				count += 1
			}
		}
		// handle successful connection
		dbg.Lvl3(p.Name, "has connected with peer", host)
		tick.Stop()
		// start to syn with the respective peer
		go p.synWithPeer(conn)
		break
	}
	return nil
}

// ForRemotePeers will launch the given function over a go routine
// for each remotepeer this peer has
func (p *Peer) ForRemotePeers(fn func(RemotePeer)) {
	for i, _ := range p.remote {
		go fn(p.remote[i])
	}
}

// WaitSYNs will wait until every peers has syn'd with this one
func (p *Peer) WaitSYNs() {
	for {
		s := <-p.synChan
		dbg.Lvl3(p.Name, "synChan received Syn id", s.Id)
		_, ok := p.remote[s.Id]
		if !ok {
			dbg.Fatal(p.Name, "received syn'd notification of an unknown peer... ABORT")
		}
		if len(p.remote) == p.info.N-1 {
			dbg.Lvl3(p.Name, "is SYN'd with every one")
			break
		}
	}
}

// SendACKS will send an ACK to everyone
func (p *Peer) SendACKs() {
	a := Ack{
		Id:    p.Id,
		Valid: true,
	}
	err := p.SendToAll(&a)
	if err != nil {
		dbg.Fatal(p.Name, "could not sent its ACKs to every one:", err)
	}
}

// WaitAcks will make  a peer  waits for all others peers to send an ACK to it
func (p *Peer) WaitACKs() {
	var wg sync.WaitGroup
	fn := func(rp RemotePeer) {
		a := Ack{}
		err := p.suite.Read(rp.Conn, &a)
		if err != nil {
			dbg.Fatal(p.Name, "could not receive an ACK from", rp.String(), "(err", err, ")")
		}
		//p.ackChan <- a
		wg.Done()
	}
	wg.Add(len(p.remote))
	p.ForRemotePeers(fn)

	dbg.Lvl3(p.Name, "is waiting for acks ...")
	wg.Wait()
	dbg.Lvl3(p.String(), "received all ACKs")
}

// Wait for the end of the alo so we can close connection nicely
func (p *Peer) WaitFins() {
	p.wgFin.Add(len(p.remote))
	fn := func(rp RemotePeer) {
		f := Finish{p.Id}
		err := p.suite.Write(rp.Conn, &f)
		if err != nil {
			dbg.Fatal(p.String(), "could not send FIN to", rp.String())
		}
		p.wgFin.Done()
	}
	p.ForRemotePeers(fn)
	dbg.Lvl3(p.String(), "waiting to send all FIN's packets")
	p.wgFin.Wait()
	// close all connections
	for _, rp := range p.remote {
		rp.Conn.Close()
	}
	dbg.Lvl3(p.String(), "close every connections")
}

// Peer logic after it has syn'd with another peer
func (p *Peer) SendAcks(rp RemotePeer) {
	// we have received every peer's public key
	if len(p.remote) == p.info.N-1 {
		// send an ACK to the root
		a := Ack{
			Id:    p.Id,
			Valid: true,
		}
		// everyone sends their ack to everyone
		p.SendToAll(&a)
	}
}

// Helpers to send any aribtrary data to the n-peer
func (p *Peer) SendToPeer(i int, data interface{}) error {
	return p.suite.Write(p.nConn(i), data)
}
func (p *Peer) SendToRoot(data interface{}) error {
	return p.SendToPeer(0, data)
}
func (p *Peer) SendToAll(data interface{}) error {
	for _, rp := range p.remote {
		if err := p.SendToPeer(rp.Id, data); err != nil {
			return err
		}
	}
	return nil
}

// Helper that returns the connection to peer i
func (p *Peer) nConn(i int) net.Conn {
	if _, ok := p.remote[i]; ok {
		return p.remote[i].Conn
	}
	return nil
}

// Helper to return the connection to the root
func (p *Peer) rootConn() net.Conn {
	return p.nConn(0)
}

// SynWithPeer will receive and send the public keys between the peer
// If all goes well, it will add the peer to the remotePeer array
// and notify to the channel synChan
func (p *Peer) synWithPeer(conn net.Conn) {
	if conn == nil {
		dbg.Fatal("Connection of", p.String(), "is nil")
	}
	// First we need to SYN mutually
	s := Syn{
		Id:     p.Id,
		Public: p.key.Public,
	}
	err := p.suite.Write(conn, &s)
	if err != nil {
		dbg.Fatal(p.Name, "could not send SYN to", conn.RemoteAddr().String())
	}
	// Receive the other SYN
	s2 := Syn{}
	err = p.suite.Read(conn, &s2)
	if err != nil {
		dbg.Fatal(p.Name, "could not receive SYN from", conn.RemoteAddr().String())
	}
	if s2.Id < 0 || s2.Id >= p.info.N {
		dbg.Fatal(p.Name, "received wrong SYN info from", conn.RemoteAddr().String())
	}
	if p.pubKeys[s2.Id] != nil {
		dbg.Fatal(p.Name, "already received a SYN for this index ")
	}
	p.pubKeys[s2.Id] = s2.Public
	rp := RemotePeer{Conn: conn, Id: s2.Id, Hostname: conn.RemoteAddr().String()}
	p.remote[s2.Id] = rp
	dbg.Lvl3(p.String(), "has SYN'd with peer", rp.String())
	p.synChan <- s2
}

func (p *Peer) String() string {
	role := ""
	if p.root {
		role = "Root: "
	} else {
		role = "Peer: "
	}
	return fmt.Sprintf("%s: %s (%d):", role, p.Name, p.Id)
}

// ComputeSharedSecret will make the exchange of dealers between
// the peers and will compute the sharedsecret at the end
func (p *Peer) ComputeSharedSecret() *poly.SharedSecret {
	// Construct the dealer
	dealerKey := cliutils.KeyPair(p.suite)
	dealer := new(poly.Deal).ConstructDeal(&dealerKey, &p.key, p.info.T, p.info.R, p.pubKeys)
	// Construct the receiver
	receiver := poly.NewReceiver(p.suite, p.info, &p.key)
	// add already its own dealer
	_, err := receiver.AddDeal(p.Id, dealer)
	if err != nil {
		dbg.Fatal(p.String(), "could not add its own dealer >< ABORT")
	}

	// Send the dealer struct TO every one
	err = p.SendToAll(dealer)
	dbg.Lvl3(p.Name, "sent its dealer to every peers. (err =", err, ")")
	// Receive the dealer struct FROM every one
	// wait with a chan to get ALL dealers
	dealChan := make(chan *poly.Deal)
	for _, rp := range p.remote {
		go func(rp RemotePeer) {
			d := new(poly.Deal).UnmarshalInit(p.info.T, p.info.R, p.info.N, p.suite)
			err := p.suite.Read(rp.Conn, d)
			if err != nil {
				dbg.Fatal(p.Name, "received a strange dealer from", rp.String(), ":", err)
			}
			dealChan <- d
		}(rp)
	}

	// wait to get all dealers
	dbg.Lvl3(p.Name, "wait to receive every other peer's dealer...")
	n := 0
	for {
		// get the dealer and add it
		d := <-dealChan
		dbg.Lvl3(p.Name, "collected one more dealer (count =", n, ")")
		// TODO: get the response back to the dealer
		_, err := receiver.AddDeal(p.Id, d)
		if err != nil {
			dbg.Fatal(p.Name, "has error when adding the dealer:", err)
		}
		n += 1
		// we get enough dealers to compute the shared secret
		if n == p.info.T-1 {
			dbg.Lvl3(p.Name, "received every Dealers")
			break
		}
	}

	sh, err := receiver.ProduceSharedSecret()
	if err != nil {
		dbg.Fatal(p.Name, "could not produce shared secret. Abort. (err", err, ")")
	}
	dbg.Lvl3(p.Name, "produced shared secret !")
	return sh
}

// SetupDistributedSchnorr will compute a shared secret in order
// to be able to use the schnorr t-n distributed algo
func (p *Peer) SetupDistributedSchnorr() {
	// first, we have to get the long term shared secret
	long := p.ComputeSharedSecret()
	// Then instantiate the Schnoor struct
	p.schnorr = p.schnorr.Init(p.suite, p.info, long)
}

// SchnorrSigRoot will first generate a
// random shared secret, then start a new round
// It will wait for the partial sig of the peers
// to finally render a SchnorrSig struct
func (p *Peer) SchnorrSigRoot(msg []byte) *poly.SchnorrSig {
	// First, gen. a random secret
	random := p.ComputeSharedSecret()
	// gen the hash out of the msg
	h := p.suite.Hash()
	h.Write(msg)
	// launch the new round
	err := p.schnorr.NewRound(random, h)
	if err != nil {
		dbg.Fatal(p.String(), "could not make a new round:", err)
	}

	// compute its own share of the signature
	ps := p.schnorr.RevealPartialSig()
	// add its own
	p.schnorr.AddPartialSig(ps)

	// no need to send to all if you are the root
	//	p.SendToAll(ps)
	// then receive every partial sig
	sigChan := make(chan *poly.SchnorrPartialSig)
	fn := func(rp RemotePeer) {
		psig := new(poly.SchnorrPartialSig)
		err := p.suite.Read(rp.Conn, psig)
		if err != nil {
			dbg.Fatal(p.String(), "could not decode PartialSig of", rp.String())
		}
		sigChan <- psig
	}
	p.ForRemotePeers(fn)

	// wait for all partial sig to be received
	n := 0
	for {
		psig := <-sigChan
		err := p.schnorr.AddPartialSig(psig)
		if err != nil {
			dbg.Fatal(p.String(), "could not add the partial signature received:", err)
		}
		n += 1
		if n == p.info.N-1 {
			dbg.Lvl3(p.String(), "received every other partial sig.")
			break
		}
	}

	sign, err := p.schnorr.Sig()
	if err != nil {
		dbg.Fatal(p.String(), "could not generate the global SchnorrSig", err)
	}
	return sign
}

func (p *Peer) SchnorrSigPeer(msg []byte) {
	// First, gen. a random secret
	random := p.ComputeSharedSecret()
	// launch the new round
	h := p.suite.Hash()
	h.Write(msg)
	err := p.schnorr.NewRound(random, h)
	if err != nil {
		dbg.Fatal(p.String(), "could not make a new round:", err)
	}

	// compute its own share of the signature
	ps := p.schnorr.RevealPartialSig()
	// then send it to root only
	p.SendToRoot(ps)
}

// VerifySchnorrSig will basically verify the validity of the issued signature
func (p *Peer) VerifySchnorrSig(ps *poly.SchnorrSig, msg []byte) error {
	h := p.suite.Hash()
	h.Write(msg)
	return p.schnorr.VerifySchnorrSig(ps, h)
}

// BroadcastSIgnature will broadcast the given signature to every other peer
// AND will retrieve the signature of every other peer also !
func (p *Peer) BroadcastSignature(s *poly.SchnorrSig) []*poly.SchnorrSig {
	arr := make([]*poly.SchnorrSig, 0, p.info.N)
	arr = append(arr, s)
	err := p.SendToAll(s)
	if err != nil {
		dbg.Fatal(p.String(), "could not sent to everyone its schnorr sig")
	}

	sigChan := make(chan *poly.SchnorrSig)
	fn := func(rp RemotePeer) {
		sch := new(poly.SchnorrSig).Init(p.suite, p.info)
		err := p.suite.Read(rp.Conn, sch)
		if err != nil {
			dbg.Fatal(p.String(), "could not decode schnorr sig from", rp.String())
		}
		sigChan <- sch
	}
	// wait for every peers's schnorr sig
	p.ForRemotePeers(fn)
	n := 0
	for {
		sig := <-sigChan
		arr = append(arr, sig)
		n += 1
		if n == p.info.N-1 {
			dbg.Lvl3(p.String(), "received every other schnorr sig.")
			break
		}
	}

	return arr
}
