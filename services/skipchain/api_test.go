package skipchain

import (
	"testing"

	"bytes"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	network.RegisterMessageType(&testData{})
}

func TestClient_ProposeSkipBlock(t *testing.T) {

}

func TestClient_GetUpdateChain(t *testing.T) {

}

func TestClient_CreateRootInter(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	root, inter, err := c.CreateRootInter(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	if root == nil || inter == nil {
		t.Fatal("Pointers are nil")
	}
	if err = root.VerifySignatures(); err != nil {
		t.Fatal("Root signature invalid:", err)
	}
	if err = inter.VerifySignatures(); err != nil {
		t.Fatal("Root signature invalid:", err)
	}
	if !bytes.Equal(root.ChildSL.Hash, inter.Hash) {
		t.Fatal("Root doesn't point to intermediate")
	}
	if !bytes.Equal(inter.ParentBlockID, root.Hash) {
		t.Fatal("Intermediate doesn't point to root")
	}
}

func TestClient_CreateData(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(2, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, inter, err := c.CreateRootInter(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	td := &testData{1, "data-sc"}
	inter, data, err := c.CreateData(inter, 4, VerifyNone, td)
	dbg.ErrFatal(err)
	if err = data.VerifySignatures(); err != nil {
		t.Fatal("Couldn't verify data-signature:", err)
	}
	if !bytes.Equal(data.ParentBlockID, inter.Hash) {
		t.Fatal("Data-chain doesn't point to intermediate-chain")
	}
	if !bytes.Equal(inter.ChildSL.Hash, data.Hash) {
		t.Fatal("Intermediate chain doesn't point to data-chain")
	}
	_, td1, err := network.UnmarshalRegisteredType(data.Data, network.DefaultConstructors(network.Suite))
	dbg.ErrFatal(err)
	if *td != td1.(testData) {
		t.Fatal("Stored data is not the same as initial data")
	}
}

func TestClient_ProposeData(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, inter, err := c.CreateRootInter(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	td := &testData{1, "data-sc"}
	inter, data1, err := c.CreateData(inter, 4, VerifyNone, td)
	dbg.ErrFatal(err)
	td.A++
	data2, err := c.ProposeData(inter, data1, td)
	dbg.ErrFatal(err)
	dataLast, err := c.GetUpdateChain(inter, data1.Hash)
	dbg.ErrFatal(err)
	if len(dataLast.Update) != 2 {
		t.Fatal("Should have two SkipBlocks for update-chain")
	}
	if !dataLast.Update[1].Equal(data2.Latest) {
		t.Fatal("Newest SkipBlock should be stored")
	}
}

func TestClient_ProposeRoster(t *testing.T) {
	nbrHosts := 5
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(nbrHosts, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	_, inter, err := c.CreateRootInter(el, el, 1, 1, VerifyNone)
	dbg.ErrFatal(err)
	el.List = el.List[:nbrHosts-1]
	sb1, err := c.ProposeRoster(inter, el)
	dbg.ErrFatal(err)
	_, err = c.ProposeRoster(inter, el)
	if err == nil {
		t.Fatal("Appending two Blocks to the same last block should fail")
	}
	sb2, err := c.ProposeRoster(sb1.Latest, el)
	dbg.ErrFatal(err)
	if !sb2.Previous.Equal(sb1.Latest) {
		t.Fatal("New previous should be previous latest")
	}
	if !bytes.Equal(sb2.Previous.ForwardLink[0].Hash,
		sb2.Latest.Hash) {
		t.Fatal("second should point to third SkipBlock")
	}

	updates, err := c.GetUpdateChain(inter, inter.Hash)
	if len(updates.Update) != 3 {
		t.Fatal("Should now have three Blocks to go from Genesis to current")
	}
	if !updates.Update[2].Equal(sb2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
}

type testData struct {
	A int
	B string
}
