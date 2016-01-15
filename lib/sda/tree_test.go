package sda_test

import (
	"strconv"
	"testing"

	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/satori/go.uuid"
)

var tSuite = ed25519.NewAES128SHA256Ed25519(false)
var prefix = "localhost:"

// test the ID generation
func TestTreeId(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	idsList := GenEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := idsList.GenerateBinaryTree()
	/*
			TODO: re-calculate the uuid
		root, _ := ExampleGenerateTreeFromEntityList(idsList)
		tree := sda.Tree{IdList: idsList, Root: root}
		var h bytes.Buffer
		h.Write(idsList.Id().Bytes())
		h.Write(root.Id().Bytes())
		genId := uuid.NewV5(uuid.NamespaceURL, h.String())
		if !uuid.Equal(genId, tree.Id()) {
				t.Fatal("Id generated is wrong")
			}
	*/
	if len(tree.Id.String()) != 36 {
		t.Fatal("Id generated is wrong")
	}
}

// Test if topology correctly handles the "virtual" connections in the topology
func TestTreeConnectedTo(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	peerList := GenEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := peerList.GenerateBinaryTree()
	// Generate the network
	if !tree.Root.IsConnectedTo(peerList.List[1]) {
		t.Fatal("Root should be connected to localhost:2001")
	}
}

// Test initialisation of new peer-list
func TestEntityListNew(t *testing.T) {
	adresses := []string{"localhost:1010", "localhost:1012"}
	pl := GenEntityList(tSuite, adresses)
	if len(pl.List) != 2 {
		t.Fatalf("Expected two peers in PeerList. Instead got %d", len(pl.List))
	}
	if pl.Id == uuid.Nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(pl.Id.String()) != 36 {
		t.Fatal("PeerList ID does not seem to be a UUID.")
	}
}

// Test initialisation of new peer-list from config-file
func TestInitPeerListFromConfigFile(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	idsList := GenEntityList(tSuite, names)
	// write it
	app.WriteTomlConfig(idsList.Toml(tSuite), "identities.toml", "testdata")
	// decode it
	var decoded sda.EntityListToml
	if err := app.ReadTomlConfig(&decoded, "identities.toml", "testdata"); err != nil {
		t.Fatal("COuld not read from file the entityList")
	}
	decodedList := decoded.EntityList(tSuite)
	if len(decodedList.List) != 3 {
		t.Fatalf("Expected two identities in EntityList. Instead got %d", len(decodedList.List))
	}
	if decodedList.Id == uuid.Nil {
		t.Fatal("PeerList without ID is not allowed")
	}
	if len(decodedList.Id.String()) != 36 {
		t.Fatal("PeerList ID does not seem to be a UUID hash.")
	}
}

// Test initialisation of new random tree from a peer-list

// Test initialisation of new graph from config-file using a peer-list
// XXX again this test might be obsolete/does more harm then it is useful:
// It forces every field to be exported/made public
// and we want to get away from config files (or not?)

// Test initialisation of new graph when one peer is represented more than
// once

// Test access to tree:
// - parent
func TestTreeParent(t *testing.T) {
	names := genLocalhostPeerNames(3, 2000)
	peerList := GenEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := peerList.GenerateBinaryTree()
	child := tree.Root.Children[0]
	if child.Parent.Id != tree.Root.Id {
		t.Fatal("Parent of child of root is not the root...")
	}
}

// - children
func TestTreeChildren(t *testing.T) {
	names := genLocalhostPeerNames(2, 2000)
	peerList := GenEntityList(tSuite, names)
	// Generate two example topology
	tree, nodes := peerList.GenerateBinaryTree()
	child := tree.Root.Children[0]
	if child.Id != nodes[1].Id {
		t.Fatal("Parent of child of root is not the root...")
	}
}

// Test marshal/unmarshaling of trees
func TestUnMarshalTree(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	names := genLocalhostPeerNames(10, 2000)
	peerList := GenEntityList(tSuite, names)
	// Generate two example topology
	tree, _ := peerList.GenerateBinaryTree()
	tree_binary, err := tree.Marshal()

	if err != nil {
		t.Fatal("Error while marshaling:", err)
	}
	if len(tree_binary) == 0 {
		t.Fatal("Marshaled tree is empty")
	}

	tree2, err := sda.NewTreeFromMarshal(tree_binary, peerList)
	if err != nil {
		t.Fatal("Error while unmarshaling:", err)
	}
	if !tree.Equal(tree2) {
		dbg.Lvl3(tree, "\n", tree2)
		t.Fatal("Tree and Tree2 are not identical")
	}
}

// - public keys
// - corner-case: accessing parent/children with multiple instances of the same peer
// in the graph

// genLocalhostPeerNames will generate n localhost names with port indices starting from p
func genLocalhostPeerNames(n, p int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = prefix + strconv.Itoa(p+i)
	}
	return names
}

// GenEntityList generate a EntityList out of names
func GenEntityList(suite abstract.Suite, names []string) *sda.EntityList {
	var ids []*network.Entity
	for _, n := range names {
		kp := cliutils.KeyPair(suite)
		ids = append(ids, network.NewEntity(kp.Public, n))
	}
	return sda.NewEntityList(ids)
}
