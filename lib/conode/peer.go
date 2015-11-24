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
	*StampListener

	RLock     sync.Mutex
	MaxRounds int
	CloseChan chan bool
	Closed    bool

	Logger    string
	Hostname  string
	App       string
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

	peer := &Peer{}

	peer.Node = node
	peer.RLock = sync.Mutex{}

	peer.CloseChan = make(chan bool, 5)
	peer.StampListener = NewStampListener(peer.Node.Name())
	peer.Hostname = address
	peer.App = "stamp"

	// Start the cothority-listener on port 2000
	err = hc.Run(true, sign.MerkleTree, address)
	if err != nil {
		dbg.Fatal(err)
	}

	go func() {
		err := peer.Node.Listen()
		dbg.Lvl2("Node.listen quits with status", err)
		peer.CloseChan <- true
	}()
	return peer
}

func (peer *Peer) LoopRounds() {
	// only listen if this is the hostname specified
	if peer.IsRoot(0) {
		dbg.Lvl3("Root timestamper at:", peer.Host)
		peer.Run("root")

	} else {
		dbg.Lvl3("Running regular timestamper on:", peer.Hostname)
		peer.Run("regular")
	}
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (peer *Peer) Run(role string) {
	dbg.Lvl3("Stamp-server", peer.Node.Name(), "starting with ", role)

	peer.RLock.Lock()

	// TODO: remove this hack
	peer.MaxRounds = -1
	peer.RLock.Unlock()

	var nextRole string // next role when view changes
	for {
		switch role {

		case "root":
			dbg.Lvl3(peer.Name(), "running as root")
			nextRole = peer.runAsRoot(peer.MaxRounds)
		case "regular":
			dbg.Lvl3(peer.Name(), "running as regular")
			nextRole = peer.runAsRegular()
		case "close":
			dbg.Lvl3(peer.Name(), "closing")
			return
		default:
			dbg.Fatal(peer.Name(), "Unable to run as anything")
			return
		}

		dbg.Lvl2(peer.Name(), "Role now:", role, "nextRole:", nextRole)
		if nextRole == "close" {
			peer.Close()
			return
		}
		role = nextRole
	}

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
	peer.StampListener.Close()
	dbg.Lvlf3("Closing of peer: %s finished", peer.Name())
}

// This node is the root-node - still possible to change
// the role
func (peer *Peer) runAsRoot(nRounds int) string {
	// every 5 seconds start a new round
	ticker := time.Tick(sign.ROUND_TIME)
	if peer.LastRound() + 1 > nRounds && nRounds >= 0 {
		dbg.Lvl1(peer.Name(), "runAsRoot called with too large round number")
		return "close"
	}

	dbg.Lvl3(peer.Name(), "running as root", peer.LastRound(), int64(nRounds))
	for {
		select {
		case nextRole := <-peer.ViewChangeCh():
			dbg.Lvl4(peer.Name(), "assuming next role is", nextRole)
			return nextRole
		// s.reRunWith(nextRole, nRounds, true)
		case <-ticker:

			dbg.Lvl4(peer.Name(), "Stamp server in round", peer.LastRound() + 1, "of", nRounds)

			round, err := sign.NewRoundFromType("cosistamper", peer.Node)
			if err != nil {
				dbg.Fatal("Couldn't create cosistamp", err)
			}
			err = peer.StartAnnouncement(round)
			if err != nil {
				dbg.Lvl3(err)
				time.Sleep(1 * time.Second)
				break
			}

			if peer.LastRound() + 1 >= nRounds && nRounds >= 0 {
				dbg.Lvl2(peer.Name(), "reports exceeded the max round: terminating", peer.LastRound() + 1, ">=", nRounds)
				return "close"
			}
		case <-peer.CloseChan:
			dbg.Lvl3("Server-peer", peer.Name(), "has closed the connection")
			return "close"
		}
	}
	dbg.Lvl3("Finished runAsRoot")
	return "close"
}

// This node is a child of the root-node
func (peer *Peer) runAsRegular() string {
	select {
	case <-peer.CloseChan:
		dbg.Lvl3("Regular-peer", peer.Name(), "has closed the connection")
		return "close"

	case nextRole := <-peer.ViewChangeCh():
		return nextRole
	}
}

// Simple ephemereal helper for comptability issues
// From base64 => hexadecimal
func convertTree(suite abstract.Suite, t *graphs.Tree) {
	point, err := cliutils.ReadPub64(suite, strings.NewReader(t.PubKey))
	if err != nil {
		dbg.Fatal("Could not decode base64 public key")
	}

	str, err := cliutils.PubHex(suite, point)
	if err != nil {
		dbg.Fatal("Could not encode point to hexadecimal ")
	}
	t.PubKey = str
	for _, c := range t.Children {
		convertTree(suite, c)
	}
}

// Add our own private key in the tree. This function exists because of
// compatilibty issues with the graphs/ lib.
func addPrivateKey(suite abstract.Suite, address string, conf *app.ConfigConode) {
	fn := func(t *graphs.Tree) {
		// this is our node in the tree
		if t.Name == address {
			// convert to hexa
			s, err := cliutils.SecretHex(suite, conf.Secret)
			if err != nil {
				dbg.Fatal("Error converting our secret key to hexadecimal")
			}
			// adds it
			t.PriKey = s
		}
	}
	conf.Tree.TraverseTree(fn)
}
