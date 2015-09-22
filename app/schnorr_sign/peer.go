package schnorr_sign

import (
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	conf "github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
	"net"
	"strings"
	"time"
)

// How many times a peer tries to connect to another until it works
var ConnRetry = 5

// How many time do you wait before trying to connect again
var ConnWaitRetry = time.Second * 2

type Peer struct {
	// simple int representing its index in the matrix of peers
	Id int
	// its own IP addr : Port
	Name string

	// connections mapped by their ID
	conns map[int]net.Conn

	// wether it is a "passive" peer or a "root" peer (i.e. actively starting signatures etc)
	role string

	// N, R, T parameters + suite used throughout the process
	info poly.PolyInfo

	// its own private / public key pair
	key conf.KeyPair

	// public key list
	pubKeys []*abstract.Point

	// Its receiver part of the shared secret setup
	receiver *poly.Receiver

	// Its Dealer part of the shared secret setup
	dealer *poly.Dealer
}

// NewPeer returns a new peer with its id and the number of peers in the schnorr signature algo
// TODO verification of string addr:port
func NewPeer(id int, name string, p poly.PolyInfo) *Peer {
	if id >= p.N {
		log.Fatal("Error while NewPeer : gien ", id, " as id whereas polyinfo.N = ", p.N)

	}
	// Setup of the private / public pair
	key := conf.KeyPair{}
	key.Gen(p.Suite, random.Stream)

	// setup of the public list of key
	pubKeys := make([]*abstract.Point, p.N)
	pubKeys[id] = &key.Public

	// setup of the receiver part
	// Dealer will be instantiated later when all the public keys are known
	receiver := poly.NewReceiver(p, &key)

	return &Peer{
		Id:       id,
		conns:    make(map[int]net.Conn, p.N),
		role:     "peer",
		Name:     name,
		info:     p,
		key:      key,
		pubKeys:  pubKeys,
		receiver: receiver,
	}
}

func (p *Peer) Listen() error {
	results := strings.Split(p.Name, ":")
	port := ":" + results[1]
	ln, err := net.Listen("tcp", port)
	if err != nil {
		dbg.Lvl1(p.Name, ": Error while listening on port ", port, "ABORT  => ", err)
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg.Lvl1(p.Name, ": Error while listening on port ", port, " => ", err)
			continue
		}
		go p.HandleConnection(conn)
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
				dbg.Lvl1(p.Name, "could not connect to", host, " ", ConnRetry, "times. Abort.")
				tick.Stop()
				return err
				// let's try again one more time
			} else {
				dbg.Lvl1(p.Name, "could not connect to", host, ". Retry in ", ConnWaitRetry.String())
				count += 1
			}
		}
		// handle successful connection
		dbg.Lvl2(p.Name, "has connected with peer ", host)
		go p.HandleConnection(conn)
	}
	return nil
}

// HandleConnection is the main logic of the signing algo
func (p *Peer) HandleConnection(conn net.Conn) {
	pid := p.synWithPeer(conn)
	dbg.Lvl2(p.Name, "(id ", p.Id, ") has SYN'd with peer ", conn.RemoteAddr().String(), " (id ", pid, ")")

}

// SynWithPeer will receive and send the public keys between the peer
// Returns the ID of the peer connected to
func (p *Peer) synWithPeer(conn net.Conn) int {
	// First we need to SYN mutually
	s := Syn{
		Id:     p.Id,
		Public: p.key.Public,
	}
	err := p.info.Suite.Write(conn, &s)
	if err != nil {
		log.Fatal(p.Name, "could not send SYN to ", conn.RemoteAddr().String())
	}
	// Receive the other SYN
	err = p.info.Suite.Read(conn, &s)
	if err != nil {
		log.Fatal(p.Name, "could not receive SYN from ", conn.RemoteAddr().String())
	}
	if s.Id < 0 || s.Id >= p.info.N {
		log.Fatal(p.Name, "received wrong SYN info from ", conn.RemoteAddr().String())
	}
	if p.pubKeys[s.Id] != nil {
		log.Fatal(p.Name, "already received a SYN for this index ")
	}
	p.pubKeys[s.Id] = &s.Public
	return s.Id
}
