package skipchain

import (
	"testing"

	"bytes"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func TestClientGenesis(t *testing.T) {
	l := sda.NewLocalTest()
	l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	c.ProposeRoster(nil, nil)
}

func TestClient_ProposeSkipBlock(t *testing.T) {

}

func TestClient_GetUpdateChain(t *testing.T) {

}

func TestClient_CreateRootInterm(t *testing.T) {
	t.Skip("TODO")
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	root, interm, err := c.CreateRootInterm(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	if root == nil || interm == nil {
		t.Fatal("Pointers are nil")
	}
	if err = root.VerifySignatures(); err != nil {
		t.Fatal("Root signature invalid:", err)
	}
	if err = interm.VerifySignatures(); err != nil {
		t.Fatal("Root signature invalid:", err)
	}
	if !bytes.Equal(root.ChildSL.Hash, interm.Hash) {
		t.Fatal("Root doesn't point to intermediate")
	}
	if !bytes.Equal(interm.ParentBlock, root.Hash) {
		t.Fatal("Intermediate doesn't point to root")
	}
}

func TestClient_CreateData(t *testing.T) {
	t.Skip("To be implemented")
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, interm, err := c.CreateRootInterm(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	td := &testData{1, "data-sc"}
	data, err := c.CreateData(interm, 4, td, VerifyNone)
	if err = data.VerifySignatures(); err != nil {
		t.Fatal("Couldn't verify data-signature:", err)
	}
	if !bytes.Equal(data.ParentBlock, interm.Hash) {
		t.Fatal("Data-chain doesn't point to intermediate-chain")
	}
	if !bytes.Equal(interm.ChildSL.Hash, data.Hash) {
		t.Fatal("Intermediate chain doesn't point to data-chain")
	}
	_, td1, err := network.UnmarshalRegisteredType(data.Data, network.DefaultConstructors(network.Suite))
	dbg.ErrFatal(err)
	if *td != td1.(testData) {
		t.Fatal("Stored data is not the same as initial data")
	}
}

func TestClient_ProposeData(t *testing.T) {
	t.Skip("To be implemented")
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, interm, err := c.CreateRootInterm(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	td := &testData{1, "data-sc"}
	data1, err := c.CreateData(interm, 4, td, VerifyNone)
	dbg.ErrFatal(err)
	_, err = c.ProposeData(interm.Hash, td)
	if err == nil {
		t.Fatal("Shouldn't be able to add data-SkipBlock to Intermediate")
	}
	td.A++
	data2, err := c.ProposeData(data1.Hash, td)
	dbg.ErrFatal(err)
	data_last, err := c.GetUpdateChain(data1.Hash)
	dbg.ErrFatal(err)
	if len(data_last.Update) != 2 {
		t.Fatal("Should have two SkipBlocks for update-chain")
	}
	if !data_last.Update[1].Equal(data2.Latest) {
		t.Fatal("Newest SkipBlock should be stored")
	}
}

func TestClient_ProposeRoster(t *testing.T) {
	t.Skip("To be implemented")
	nbrHosts := 5
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(nbrHosts, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, interm, err := c.CreateRootInterm(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	el.List = el.List[:nbrHosts-1]
	reply1, err := c.ProposeRoster(interm.Hash, el)
	dbg.ErrFatal(err)
	_, err = c.ProposeRoster(interm.Hash, el)
	if err == nil {
		t.Fatal("Appending two Blocks to the same last block should fail")
	}
	reply2, err := c.ProposeRoster(reply1.Latest.GetHash(), el)
	dbg.ErrFatal(err)
	if !bytes.Equal(reply1.Latest.GetCommon().ForwardLink[0].Hash,
		reply2.Latest.GetHash()) {
		t.Fatal("second should point to third SkipBlock")
	}

	updates, err := c.GetUpdateChain(interm.GetHash())
	if len(updates.Update) != 3 {
		t.Fatal("Should now have three Blocks to go from Genesis to current")
	}
	if !updates.Update[2].Equal(reply2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
}

type testData struct {
	A int
	B string
}
