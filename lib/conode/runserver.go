package conode

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
	"strings"
	"github.com/dedis/crypto/abstract"
)

/*
Loads the configuration and initialises the structures with
the private and public keys.
 */

// Make connections and run server.go
func RunServer(address string, conf *app.ConfigConode) {
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

	// for each client in
	peer := NewPeer(node)
	peer.ListenRequests()
	dbg.Lvl3("peer:", peer)

	// Start the cothority-listener on port 2000
	err = hc.Run(true, sign.MerkleTree, address)
	if err != nil {
		dbg.Fatal(err)
	}

	defer func(sn *sign.Node) {
		dbg.Lvl2("Program timestamper has terminated:", address)
		sn.Close()
	}(hc.SNodes[0])

	// only listen if this is the hostname specified
	if peer.Name() == address {
		peer.Hostname = address
		peer.App = "stamp"
		if peer.IsRoot(0) {
			dbg.Lvl3("Root timestamper at:", address)
			peer.Run("root")

		} else {
			dbg.Lvl3("Running regular timestamper on:", address)
			peer.Run("regular")
		}
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
