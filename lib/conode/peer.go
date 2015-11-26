package conode

import (
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"

	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/crypto/abstract"
	"strings"
)

/*
This will run rounds with RoundCosiStamper while listening for
incoming requests through StampListener.
 */

type Peer struct {
	*sign.Node

	conf      *app.ConfigConode

	RLock     sync.Mutex
	CloseChan chan bool
	Closed    bool

	Logger    string
	Hostname  string
}

// NewPeer returns a peer that can be used to set up
// connections.
func NewPeer(address string, conf *app.ConfigConode) *Peer {
	suite := app.GetSuite(conf.Suite)

	var err error
	// make sure address has a port or insert default one
	address, err = cliutils.VerifyPort(address, DefaultPort)
	if err != nil {
		dbg.Fatal(err)
	}

	// For retro compatibility issues, convert the base64 encoded key into hex
	// encoded keys....
	convertTree(suite, conf.Tree)
	// Add our private key to the tree (compatibility issues again with graphs/
	// lib)
	addPrivateKey(suite, address, conf)
	// load the configuration
	dbg.Lvl3("loading configuration")
	var hc *graphs.HostConfig
	opts := graphs.ConfigOptions{ConnType: "tcp", Host: address, Suite: suite}

	hc, err = graphs.LoadConfig(conf.Hosts, conf.Tree, suite, opts)
	if err != nil {
		dbg.Fatal(err)
	}

	// Listen to stamp-requests on port 2001
	node := hc.Hosts[address]
	peer := &Peer{
		conf: conf,
		Node: node,
		RLock: sync.Mutex{},
		CloseChan: make(chan bool, 5),
		Hostname: address,
	}

	// Start the cothority-listener on port 2000
	err = hc.Run(true, sign.MerkleTree, address)
	if err != nil {
		dbg.Fatal(err)
	}

	go func() {
		err := peer.Node.Listen()
		dbg.Lvl3("Node.listen quits with status", err)
		peer.CloseChan <- true
		peer.Close()
	}()
	return peer
}

// LoopRounds starts the system by sending a round of type
// 'roundType' every second for number of 'rounds'.
// If 'rounds' < 0, it loops forever, or until you call
// peer.Close().
func (peer *Peer) LoopRounds(roundType string, rounds int) {
	dbg.Lvl3("Stamp-server", peer.Node.Name(), "starting with IsRoot=", peer.IsRoot(peer.ViewNo))
	ticker := time.NewTicker(sign.ROUND_TIME)

	for {
		select {
		case nextRole := <-peer.ViewChangeCh():
			dbg.Lvl2(peer.Name(), "assuming next role is", nextRole)
		case <-peer.CloseChan:
			dbg.Lvl3("Server-peer", peer.Name(), "has closed the connection")
			return
		case <-ticker.C:
			if peer.IsRoot(peer.ViewNo) {
				dbg.Lvl3(peer.Name(), "Stamp server in round", peer.LastRound() + 1, "of", rounds)
				round, err := sign.NewRoundFromType(roundType, peer.Node)
				if err != nil {
					dbg.Fatal("Couldn't create", roundType, err)
				}
				err = peer.StartAnnouncement(round)
				if err != nil {
					dbg.Lvl3(err)
					time.Sleep(1 * time.Second)
					break
				}
			} else {
				dbg.Lvl3(peer.Name(), "running as regular")
			}
		}

		if peer.LastRound() >= rounds && rounds >= 0 {
			dbg.Lvl3(peer.Name(), "reports exceeded the max round: terminating",
				peer.LastRound(), ">=", rounds)
			ticker.Stop()
			peer.SendCloseAll()
			return
		}
	}
}

// Sends the 'CloseAll' to everybody
func (peer *Peer)SendCloseAll(){
	peer.Node.CloseAll(peer.Node.ViewNo)
}

// Closes the channel
func (peer *Peer) Close() {
	if peer.Closed {
		dbg.Lvl1("Peer", peer.Name(), "Already closed!")
		return
	} else {
		peer.Closed = true
	}
	peer.CloseChan <- true
	peer.Node.Close()
	StampListenersClose()
	dbg.Lvlf3("Closing of peer: %s finished", peer.Name())
}

// Simple ephemeral helper for compatibility issues
// From base64 => hexadecimal
func convertTree(suite abstract.Suite, t *graphs.Tree) {
	if t.PubKey != "" {
		point, err := cliutils.ReadPub64(suite, strings.NewReader(t.PubKey))
		if err != nil {
			dbg.Fatal("Could not decode base64 public key")
		}

		str, err := cliutils.PubHex(suite, point)
		if err != nil {
			dbg.Fatal("Could not encode point to hexadecimal ")
		}
		t.PubKey = str
	}
	for _, c := range t.Children {
		convertTree(suite, c)
	}
}

// Add our own private key in the tree. This function exists because of
// compatibility issues with the graphs/lib.
func addPrivateKey(suite abstract.Suite, address string, conf *app.ConfigConode) {
	fn := func(t *graphs.Tree) {
		// this is our node in the tree
		if t.Name == address {
			if conf.Secret != nil {
				// convert to hexa
				s, err := cliutils.SecretHex(suite, conf.Secret)
				if err != nil {
					dbg.Fatal("Error converting our secret key to hexadecimal")
				}
				// adds it
				t.PriKey = s
			}
		}
	}
	conf.Tree.TraverseTree(fn)
}
