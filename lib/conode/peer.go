package conode

import (
	"bytes"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"

	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/abstract"
)

/*
This will run rounds with RoundCosiStamper while listening for
incoming requests through StampListener.
*/

type Peer struct {
	*sign.Node

	conf *app.ConfigConode

	RLock     sync.Mutex
	CloseChan chan bool
	Closed    bool

	Logger   string
	Hostname string
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

	// Add our private key to the tree (compatibility issues again with graphs/
	// lib)
	addPrivateKey(suite, address, conf)
	// Create the first view for the node
	firstView := tree.NewViewFromConfigTree(suite, conf.Tree, address)
	// Create the global views
	views := tree.NewViews()
	views.AddView(0, firstView)
	// create the TCP Host
	net := network.NewTcpHost(address, network.DefaultConstructors(suite))
	// finally create the sign.Node
	node := sign.NewKeyedNode(suite, net, views)

	peer := &Peer{
		conf:      conf,
		Node:      node,
		RLock:     sync.Mutex{},
		CloseChan: make(chan bool, 5),
		Hostname:  address,
	}

	// Connect to the parent if we are not root
	if !node.Root(0) {
		if err := node.ConnectParent(0); err != nil {
			dbg.Fatal(address, err, "ABORT")
		}
	}
	// Then listen + process messages
	go func() {
		err := node.Listen(address)
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
	dbg.Lvl3("Stamp-server", peer.Node.Name(), "starting with IsRoot=", peer.Root(peer.ViewNo))
	ticker := time.NewTicker(sign.ROUND_TIME)
	firstRound := peer.Node.LastRound()
	if !peer.Root(peer.ViewNo) {
		// Children don't need to tick, only the root.
		ticker.Stop()
	}

	for {
		select {
		case nextRole := <-peer.ViewChangeCh():
			dbg.Lvl2(peer.Name(), "assuming next role is", nextRole)
		case <-peer.CloseChan:
			dbg.Lvl3("Server-peer", peer.Name(), "has closed the connection")
			return
		case <-ticker.C:
			dbg.Lvl3("Ticker is firing in", peer.Hostname)
			roundNbr := peer.LastRound() - firstRound
			if roundNbr >= rounds && rounds >= 0 {
				dbg.Lvl3(peer.Name(), "reached max round: closing",
					roundNbr, ">=", rounds)
				ticker.Stop()
				if peer.Root(peer.ViewNo) {
					dbg.Lvl3(peer.Name(), "is root, asking everybody to terminate")
					peer.SendCloseAll()
				}
			} else {
				if peer.Root(peer.ViewNo) {
					dbg.Lvl2(peer.Name(), "Stamp server in round",
						roundNbr+1, "of", rounds)
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
		}
	}
}

// Sends the 'CloseAll' to everybody
func (peer *Peer) SendCloseAll() {
	peer.Node.CloseAll(peer.Node.ViewNo)
	peer.Node.Close()
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
	// XXX TODO This has nothing to do here
	StampListenersClose()
	dbg.Lvlf3("Closing of peer: %s finished", peer.Name())
}

// Add our own private key in the tree.  This is an old way of doing it that
// came from using the graphs lib. TODO we should find a way to not having to
// iterate in the tree one more time just to put the private key inside. For the
// moment it is needed because, we convert the ConfigTree to a tree.Node so we
// create the peers each time
func addPrivateKey(suite abstract.Suite, address string, conf *app.ConfigConode) {
	queue := make([]*tree.ConfigTree, 0)
	queue = append(queue, conf.Tree)
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		// this is our node in the tree
		if t.Name == address {
			if conf.Secret != nil {
				// convert to hexa
				var b bytes.Buffer
				err := cliutils.WriteSecret64(suite, &b, conf.Secret)
				if err != nil {
					dbg.Fatal("Error converting our secret key to hexadecimal")
				}
				// adds it
				t.PriKey = b.String()
				return
			}
		}
		queue = append(queue, t.Children...)
	}
}
