package chainiac_test

import (
	"testing"

	"bytes"

	"github.com/dedis/cothority/chainiac"
	_ "github.com/dedis/cothority/chainiac/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestClient_CreateRootControl(t *testing.T) {
	t.Skip("Not yet implemented")
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := chainiac.NewClient()
	_, _, cerr := c.CreateRootControl(roster, roster, nil, 0, 0, 0)
	require.NotNil(t, cerr)
}

func TestClient_CreateRootControl2(t *testing.T) {
	t.Skip("Not yet implemented")
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := chainiac.NewClient()
	root, inter, cerr := c.CreateRootControl(el, el, nil, 1, 1, 1)
	log.ErrFatal(cerr)
	if root == nil || inter == nil {
		t.Fatal("Pointers are nil")
	}
	log.ErrFatal(root.VerifyForwardSignatures(),
		"Root signature invalid:")
	log.ErrFatal(inter.VerifyForwardSignatures(),
		"Root signature invalid:")
	update, cerr := skipchain.NewClient().GetUpdateChain(root.Roster, root.Hash)
	log.ErrFatal(cerr)
	root = update[0]
	require.True(t, root.ChildSL[0].Equal(inter.Hash), "Root doesn't point to intermediate")
	if !bytes.Equal(inter.ParentBlockID, root.Hash) {
		t.Fatal("Intermediate doesn't point to root")
	}
}
