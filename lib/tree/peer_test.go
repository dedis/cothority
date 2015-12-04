package tree_test

import (
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"strconv"
	"testing"
)

var DefaultPort int = 2000

func TestNewPeerListLocal(t *testing.T) {
	_, pl := createLocalPeerlist(11)
	if len(pl.Peers) != 11 {
		t.Fatal("Did not get 11 peers")
	}
	for i, p := range pl.Peers {
		if p.Name != "localhost:"+strconv.Itoa(DefaultPort+i) {
			t.Fatal("Peer", i, "is not from localhost")
		}
	}
}

/*
func TestMarshalling(t *testing.T) {
	_, pl := createLocalPeerlist(11)
	b, err := pl.MarshalJSON()
	if err != nil {
		t.Fatal("Couldn't marshal:", err)
	}

	pln := &tree.PeerList{}
	err = pln.UnmarshalJSON(b)
	if err != nil {
		t.Fatal("Couldn't unmarshal:", err)
	}
	bn, err := pln.MarshalJSON()
	if err != nil {
		t.Fatal("Couldn't marshal new PeerList")
	}

	if bytes.Compare(b, bn) != 0 {
		t.Fatal("Both marshalled PeerLists are not the same")
	}
}
*/

func createLocalPeerlist(nbr int) (abstract.Suite, *tree.PeerList) {
	s := edwards.NewAES128SHA256Ed25519(false)
	return s, tree.NewPeerList(s, []string{"localhost"}, nbr, DefaultPort)
}
