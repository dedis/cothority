package sign_test

import (
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/cothority/coconet"
	"github.com/dedis/cothority/lib/oldconfig"
	"github.com/dedis/cothority/sign"
)

// NOTE: when announcing must provide round numbers

// Testing suite for signing
// NOTE: when testing if we can gracefully accommodate failures we must:
// 1. Wrap our hosts in FaultyHosts (ex: via field passed in LoadConfig)
// 2. Set out Nodes TesingFailures field to true
// 3. We can Choose at which stage our nodes fail by using SetDeadFor
//    or we can choose to take them off completely via SetDead

//       0
//      /
//     1
//    / \
//   2   3
func TestStaticMerkle(t *testing.T) {
	if err := runStaticTest(sign.MerkleTree, 100); err != nil {
		t.Fatal(err)
	}
}

func TestStaticPubKey(t *testing.T) {
	if err := runStaticTest(sign.PubKey, 100); err != nil {
		t.Fatal(err)
	}
}

func TestStaticFaulty(t *testing.T) {
	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 1)

	if err := runStaticTest(sign.PubKey, 100, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

var DefaultView = 0

func runStaticTest(signType sign.Type, RoundsPerView int, faultyNodes ...int) error {
	// Crypto setup
	suite := edwards.NewAES128SHA256Ed25519(true)
	//suite := nist.NewAES128SHA256P256()
	rand := suite.Cipher([]byte("example"))

	// number of nodes for the test
	nNodes := 4
	// create new directory for communication between peers
	dir := coconet.NewGoDirectory()
	// Create Hosts and Peers
	h := make([]coconet.Host, nNodes)

	for i := 0; i < nNodes; i++ {
		hostName := "host" + strconv.Itoa(i)

		if len(faultyNodes) > 0 {
			h[i] = &coconet.FaultyHost{}
			gohost := coconet.NewGoHost(hostName, dir)
			h[i] = coconet.NewFaultyHost(gohost)
		} else {
			h[i] = coconet.NewGoHost(hostName, dir)
		}

	}

	for _, fh := range faultyNodes {
		h[fh].(*coconet.FaultyHost).SetDeadFor("response", true)
	}

	// Create Signing Nodes out of the hosts
	nodes := make([]*sign.Node, nNodes)
	for i := 0; i < nNodes; i++ {
		nodes[i] = sign.NewNode(h[i], suite, rand)
		nodes[i].Type = signType
		nodes[i].GenSetPool()
		nodes[i].RoundsPerView = RoundsPerView
		defer nodes[i].Close()

		h[i].SetPubKey(nodes[i].PubKey)
		// To test the already keyed signing node, uncomment
		// PrivKey := suite.Secret().Pick(rand)
		// nodes[i] = NewKeyedNode(h[i], suite, PrivKey)
	}
	nodes[0].Height = 2
	nodes[1].Height = 1
	nodes[2].Height = 0
	nodes[3].Height = 0
	// Add edges to parents
	h[1].AddParent(DefaultView, h[0].Name())
	h[2].AddParent(DefaultView, h[1].Name())
	h[3].AddParent(DefaultView, h[1].Name())
	// Add edges to children, listen to children
	h[0].AddChildren(DefaultView, h[1].Name())
	h[1].AddChildren(DefaultView, h[2].Name(), h[3].Name())

	for _, host := range h {
		host.Listen()
		host.Connect(0)
	}

	for i := 0; i < nNodes; i++ {
		if len(faultyNodes) > 0 {
			nodes[i].FailureRate = 1
		}

		go func(i int) {
			// start listening for messages from within the tree
			nodes[i].Listen()
		}(i)
	}

	// Have root node initiate the signing protocol
	// via a simple annoucement
	nodes[0].LogTest = []byte("Hello World")
	// return nodes[0].Announce(DefaultView, &coll_sign.AnnouncementMessage{LogTest: nodes[0].LogTest, Round: 1})
	return nodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: nodes[0].LogTest, Round: 1})
}

// Configuration file data/exconf.json
//       0
//      / \
//     1   4
//    / \   \
//   2   3   5
func TestSmallConfigHealthy(t *testing.T) {
	suite := nist.NewAES128SHA256P256()
	RoundsPerView := 100
	if err := runTreeSmallConfig(sign.MerkleTree, RoundsPerView, suite, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSmallConfigHealthyNistQR512(t *testing.T) {
	suite := nist.NewAES128SHA256QR512()
	RoundsPerView := 100
	if err := runTreeSmallConfig(sign.MerkleTree, RoundsPerView, suite, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSmallConfigHealthyEd25519(t *testing.T) {
	suite := ed25519.NewAES128SHA256Ed25519(true)
	RoundsPerView := 100
	if err := runTreeSmallConfig(sign.MerkleTree, RoundsPerView, suite, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSmallConfigFaulty(t *testing.T) {
	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 2, 5)
	suite := nist.NewAES128SHA256P256()
	RoundsPerView := 100
	if err := runTreeSmallConfig(sign.MerkleTree, RoundsPerView, suite, 1, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

func TestSmallConfigFaulty2(t *testing.T) {
	failureRate := 15
	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 1, 2, 3, 4, 5)
	RoundsPerView := 100
	suite := nist.NewAES128SHA256P256()
	if err := runTreeSmallConfig(sign.MerkleTree, RoundsPerView, suite, failureRate, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

func runTreeSmallConfig(signType sign.Type, RoundsPerView int, suite abstract.Suite, failureRate int, faultyNodes ...int) error {
	var hc *oldconfig.HostConfig
	var err error
	opts := oldconfig.ConfigOptions{Suite: suite}

	if len(faultyNodes) > 0 {
		opts.Faulty = true
	}
	hc, err = oldconfig.LoadConfig("../test/data/exconf.json", opts)
	if err != nil {
		return err
	}

	for _, fh := range faultyNodes {
		fmt.Println("Setting", hc.SNodes[fh].Name(), "as faulty")
		if failureRate == 100 {
			hc.SNodes[fh].Host.(*coconet.FaultyHost).SetDeadFor("commit", true)

		}
		// hc.SNodes[fh].Host.(*coconet.FaultyHost).Die()
	}

	if len(faultyNodes) > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailureRate = failureRate
		}
	}
	for _, sn := range hc.SNodes {
		sn.RoundsPerView = RoundsPerView
	}

	err = hc.Run(false, signType)
	if err != nil {
		return err
	}

	for _, sn := range hc.SNodes {
		defer sn.Close()
	}
	// Have root node initiate the signing protocol via a simple annoucement
	hc.SNodes[0].LogTest = []byte("Hello World")
	hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: 1})

	return nil
}

func TestTreeFromBigConfig(t *testing.T) {
	// this test configuration HostList is incorrect -- duplicates are present
	return

	// not mixing view changes in
	RoundsPerView := 100

	hc, err := oldconfig.LoadConfig("../test/data/exwax.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, sn := range hc.SNodes {
		sn.RoundsPerView = RoundsPerView
	}
	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()

	// give it some time to set up
	time.Sleep(2 * time.Second)

	hc.SNodes[0].LogTest = []byte("hello world")
	err = hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: 1})
	if err != nil {
		t.Error(err)
	}
}

// tree from configuration file data/exconf.json
func TestMultipleRounds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	// not mixing view changes in
	RoundsPerView := 100
	hc, err := oldconfig.LoadConfig("../test/data/exconf.json")
	if err != nil {
		t.Fatal(err)
	}
	N := 5
	for _, sn := range hc.SNodes {
		sn.RoundsPerView = RoundsPerView
	}
	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()

	// give it some time to set up
	time.Sleep(1 * time.Second)

	// Have root node initiate the signing protocol
	// via a simple annoucement
	for i := 1; i <= N; i++ {
		hc.SNodes[0].LogTest = []byte("Hello World" + strconv.Itoa(i))
		err = hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: i})
		if err != nil {
			t.Error(err)
		}
	}
}

func TestTCPStaticConfig(t *testing.T) {
	// not mixing view changes in
	RoundsPerView := 100
	time.Sleep(5 * time.Second)
	hc, err := oldconfig.LoadConfig("../test/data/extcpconf.json", oldconfig.ConfigOptions{ConnType: "tcp", GenHosts: true})
	if err != nil {
		t.Error(err)
	}
	for _, n := range hc.SNodes {
		n.RoundsPerView = RoundsPerView
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()

	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal(err)
	}

	// give it some time to set up
	time.Sleep(2 * time.Second)

	hc.SNodes[0].LogTest = []byte("hello world")
	hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: 1})
	log.Println("Test Done")
}

func TestTCPStaticConfigRounds(t *testing.T) {
	// not mixing view changes in
	RoundsPerView := 100
	time.Sleep(5 * time.Second)
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	hc, err := oldconfig.LoadConfig("../test/data/extcpconf.json", oldconfig.ConfigOptions{ConnType: "tcp", GenHosts: true})
	if err != nil {
		t.Fatal("error loading configuration: ", err)
	}

	for _, n := range hc.SNodes {
		n.RoundsPerView = RoundsPerView
	}

	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()
	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal("error running:", err)
	}
	// give it some time to set up
	time.Sleep(2 * time.Second)

	N := 5
	for i := 1; i <= N; i++ {
		hc.SNodes[0].LogTest = []byte("hello world")
		hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: i})
	}
}

// Tests the integration of View Change with Signer (ability to reach consensus on a view change)
// After achieving consensus, View is not actually changed, because of Signer test framework limitations
// See tests in stamp/ for the actual view change ocurring
// Go channels, static configuration, multiple rounds
func TestViewChangeChan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	hc, err := oldconfig.LoadConfig("../test/data/exconf.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()

	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal(err)
	}
	// give it some time to set up
	time.Sleep(2 * time.Second)

	// Have root node initiate the signing protocol
	// via a simple annoucement
	N := 6
	for i := 1; i <= N; i++ {
		hc.SNodes[0].LogTest = []byte("Hello World" + strconv.Itoa(i))
		err = hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: i})
		if err == sign.ChangingViewError {
			log.Println("Attempted round", i, "but received view change. waiting then retrying")
			time.Sleep(3 * time.Second)
			i--
			continue
		}

		if err != nil {
			t.Error(err)
		}
	}
}

// Tests the integration of View Change with Signer (ability to reach consensus on a view change)
// After achieving consensus, View is not actually changed, because of Signer test framework limitations
// See tests in stamp/ for the actual view change ocurring
func TestViewChangeTCP(t *testing.T) {
	time.Sleep(5 * time.Second)
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	hc, err := oldconfig.LoadConfig("../test/data/extcpconf.json", oldconfig.ConfigOptions{ConnType: "tcp", GenHosts: true})
	if err != nil {
		t.Fatal("error loading configuration: ", err)
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()
	err = hc.Run(false, sign.MerkleTree)
	if err != nil {
		t.Fatal("error running:", err)
	}
	// give it some time to set up
	time.Sleep(2 * time.Second)

	N := 6
	for i := 1; i <= N; i++ {
		hc.SNodes[0].LogTest = []byte("hello world")
		hc.SNodes[0].StartAnnouncement(&sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: i})
	}
}

// func TestTreeBigConfigTCP(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("skipping test in short mode.")
// 	}
// 	hc, err := LoadConfig("data/wax.json", ConfigOptions{ConnType: "tcp", GenHosts: true})
// 	if err != nil {
// 		t.Error()
// 	}
// 	err = hc.Run(false, coll_sign.MerkleTree)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	hc.SNodes[0].LogTest = []byte("hello world")
// 	err = hc.SNodes[0].Announce(&AnnouncementMessage{hc.SNodes[0].LogTest})
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	for _, n := range hc.SNodes {
// 		n.Close()
// 	}
// }

/*func BenchmarkTreeBigConfigTCP(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping test in short mode.")
	}
	hc, err := LoadConfig("data/wax.json", "tcp")
	if err != nil {
		b.Error()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.SNodes[0].LogTest = []byte("hello world")
		hc.SNodes[0].Announce(&AnnouncementMessage{hc.SNodes[0].LogTest})
	}
}*/
