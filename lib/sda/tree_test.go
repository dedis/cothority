package sda_test

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/satori/go.uuid"
)

var tSuite = ed25519.NewAES128SHA256Ed25519(false)
var prefix = "localhost:"

// genLocalhostPeerNames will generate n localhost names with port indices starting from p
func genLocalhostPeerNames(n, p int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = prefix + strconv.Itoa(p+i)
	}
	return names
}

// GenIdentityList generate a IdentityList out of names
func GenIdentityList(suite abstract.Suite, names []string) *sda.IdentityList {
	var ids []*network.Identity
	for _, n := range names {
		kp := cliutils.KeyPair(suite)
		ids = append(ids, network.NewIdentity(kp.Public, []string{n}))
	}
	return sda.NewIdentityList(ids)
}

// test the ID generation
func TestTreeId(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	idsList := GenIdentityList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromIdentityList(idsList)
	tree := sda.Tree{IdList: idsList, Root: root}
	var h bytes.Buffer
	h.Write(idsList.Id().Bytes())
	h.Write(root.Id().Bytes())
	genId := uuid.NewV5(uuid.NamespaceURL, h.String())
	if !uuid.Equal(genId, tree.Id()) {
		t.Fatal("Id generated is wrong")
	}
}

// Test if topology correctly handles the "virtual" connections in the topology
func TestTreeConnectedTo(t *testing.T) {

	names := genLocalhostPeerNames(3, 2000)
	peerList := GenIdentityList(tSuite, names)
	// Generate two example topology
	root, _ := ExampleGenerateTreeFromIdentityList(peerList)
	// Generate the network
	if !root.IsConnectedTo("localhost:2001") {
		t.Fatal("Root should be connection to localhost:2001")
	}

}

func ExampleGenerateTreeFromIdentityList(pl *sda.IdentityList) (*sda.TreeNode, []*sda.TreeNode) {
	var nodes []*sda.TreeNode
	var root *sda.TreeNode
	for i, id := range pl.List {
		node := sda.NewTreeNode(fmt.Sprintf("%s%d", prefix, 2000+i), id)
		nodes = append(nodes, node)
		if i == 0 {
			root = node
		}
	}
	// Very simplistic depth-2 tree
	for i := 1; i < len(nodes); i++ {
		root.AddChild(nodes[i])
	}
	return root, nodes
}

// Test initialisation of new peer-list
func TestIdentityListNew(t *testing.T) {
	adresses := []string{"localhost:1010", "localhost:1012"}
	pl := GenIdentityList(tSuite, adresses)
	if len(pl.List) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.List))
	}
	if pl.Id() == uuid.Nil {
		t.Fatal("PeerList without ID is not allowed")
	}
}

// Test initialisation of new peer-list from config-file
func TestInitPeerListFromConfigFile(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	idsList := GenIdentityList(tSuite, names)
	// write it
	app.WriteTomlConfig(idsList.Toml(tSuite), "identities.toml", "testdata")
	// decode it
	var decoded sda.IdentityListToml
	if err := app.ReadTomlConfig(&decoded, "identities.toml", "testdata"); err != nil {
		t.Fatal("COuld not read from file the identityList")
	}
	decodedList := decoded.IdentityList(tSuite)
	if len(decodedList.List) != 3 {
		t.Fatalf("Expected two identities in IdentityList. Instead got %d", len(decodedList.List))
	}
	if decodedList.Id() == uuid.Nil {
		t.Fatal("PeerList without ID is not allowed")
	}
}

// Test initialisation of new random tree from a peer-list

// Test initialisation of new graph from config-file using a peer-list
// XXX again this test might be obsolete/does more harm then it is useful:
// It forces every field to be exported/made public
// and we want to get away from config files (or not?)

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to graph:
// - parent
// - children
// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph
