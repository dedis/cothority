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
	c.ProposeSkipBlock(nil, nil)
}

func TestClient_ProposeSkipBlock(t *testing.T) {

}

func TestClient_GetUpdateChain(t *testing.T) {

}

func TestClient_CreateRootInterm(t *testing.T) {
	t.Skip("To be implemented")
	l := sda.NewLocalTest()
	l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	root, interm, err := c.CreateRootInterm(4, 4)
	dbg.ErrFatal(err)
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
	l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, interm, err := c.CreateRootInterm(4, 4)
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
	l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, interm, err := c.CreateRootInterm(4, 4)
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
	if !data_last.Update[1].Equal(data2) {
		t.Fatal("Newest SkipBlock should be stored")
	}
}

type testData struct {
	A int
	B string
}
