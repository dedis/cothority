package conode

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/proto/sign"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"errors"
	"strconv"
	"net"
	"strings"
	"github.com/dedis/crypto/abstract"
)


// Make connections and run server.go
func RunServer(address string, conf *app.ConfigConode, cb sign.Callbacks) {
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
	// Add our private key to the tree (compatiblity issues again with graphs/
	// lib)
	addPrivateKey(suite, address, conf)
	// load the configuration
	//dbg.Lvl3("loading configuration")
	var hc *graphs.HostConfig
	opts := graphs.ConfigOptions{ConnType: "tcp", Host: address, Suite: suite}

	hc, err = graphs.LoadConfig(conf.Hosts, conf.Tree, suite, opts)
	if err != nil {
		dbg.Fatal(err)
	}

	// Listen to stamp-requests on port 2001
	stampers, err := RunPeer(hc, 0, cb, address)
	if err != nil {
		dbg.Fatal(err)
	}

	// Start the cothority-listener on port 2000
	err = hc.Run(true, sign.MerkleTree, address)
	if err != nil {
		dbg.Fatal(err)
	}

	defer func(sn *sign.Node) {
		dbg.Lvl2("Program timestamper has terminated:", address)
		sn.Close()
	}(hc.SNodes[0])

	for _, s := range stampers {
		// only listen if this is the hostname specified
		if s.Name() == address {
			s.Hostname = address
			s.App = "stamp"
			if s.IsRoot(0) {
				dbg.Lvl2("Root timestamper at:", address)
				s.Run("root")

			} else {
				dbg.Lvl2("Running regular timestamper on:", address)
				s.Run("regular")
			}
		}
	}
}

// run each host in hostnameSlice with the number of clients given
func RunPeer(hc *graphs.HostConfig, nclients int, cb sign.Callbacks, hostname string) ([]*sign.Peer, error) {
	dbg.Lvl3("RunTimestamper on", hc.Hosts)
	hostnames := make(map[string]*sign.Node)
	// make a list of hostnames we want to run
	if hostname == "" {
		hostnames = hc.Hosts
	} else {
		sn, ok := hc.Hosts[hostname]
		if !ok {
			return nil, errors.New("hostname given not in config file:" + hostname)
		}
		hostnames[hostname] = sn
	}
	// for each client in
	peers := make([]*sign.Peer, 0, len(hostnames))
	for _, sn := range hc.SNodes {
		if _, ok := hostnames[sn.Name()]; !ok {
			dbg.Lvl1("signing node not in hostnmaes")
			continue
		}
		peers = append(peers, sign.NewPeer(sn, cb))
		if hc.Dir == nil {
			dbg.Lvl3(hc.Hosts, "listening for clients")
			peers[len(peers) - 1].Setup()
		}
	}
	dbg.Lvl3("peers:", peers)
	for _, s := range peers[1:] {

		_, p, err := net.SplitHostPort(s.Name())
		if err != nil {
			dbg.Fatal("RunTimestamper: bad Tcp host")
		}
		pn, err := strconv.Atoi(p)
		if hc.Dir != nil {
			pn = 0
		} else if err != nil {
			dbg.Fatal("port ", pn, "is not valid integer")
		}
		//dbg.Lvl4("client connecting to:", hp)

	}

	return peers, nil
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
