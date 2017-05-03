package chainiac

import (
	"testing"

	"bytes"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateRootControl(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	c := NewClient()
	_, _, cerr := c.CreateRootControl(roster, roster, nil, 0, 0, 0)
	require.NotNil(t, cerr)
}

func TestClient_CreateRootControl2(t *testing.T) {
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := NewClient()
	root, inter, cerr := c.CreateRootControl(el, el, nil, 1, 1, 1)
	log.ErrFatal(cerr)
	if root == nil || inter == nil {
		t.Fatal("Pointers are nil")
	}
	log.ErrFatal(root.VerifyForwardSignatures(),
		"Root signature invalid:")
	log.ErrFatal(inter.VerifyForwardSignatures(),
		"Root signature invalid:")
	update, cerr := c.GetUpdateChain(root.Roster, root.Hash)
	log.ErrFatal(cerr)
	root = update.Reply[0]
	require.True(t, root.ChildSL[0].Equal(inter.Hash), "Root doesn't point to intermediate")
	if !bytes.Equal(inter.ParentBlockID, root.Hash) {
		t.Fatal("Intermediate doesn't point to root")
	}
}
