package skipchain

import (
	"testing"

	"bytes"

	"sync"

	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(&testData{})
}

func TestClient_ProposeSkipBlock(t *testing.T) {

}

func TestClient_GetUpdateChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Long run not good for Travis")
	}
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	clients := make(map[int]*Client)
	for i := range [8]byte{} {
		clients[i] = NewTestClient(l)
	}
	_, inter, cerr := clients[0].CreateRootControl(el, el, 1, 1, 1, VerifyNone)
	log.ErrFatal(cerr)

	wg := sync.WaitGroup{}
	for i := range [128]byte{} {
		wg.Add(1)
		go func(i int) {
			_, cerr := clients[i%8].GetUpdateChain(inter, inter.Hash)
			log.ErrFatal(cerr)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func NewTestClient(l *onet.LocalTest) *Client {
	c := NewClient()
	c.Client = l.NewClient("Skipchain")
	return c
}

func TestClient_CreateRootInter(t *testing.T) {
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := NewTestClient(l)
	root, inter, cerr := c.CreateRootControl(el, el, 1, 1, 1, VerifyNone)
	log.ErrFatal(cerr)
	if root == nil || inter == nil {
		t.Fatal("Pointers are nil")
	}
	if err := root.VerifySignatures(); err != nil {
		t.Fatal("Root signature invalid:", err)
	}
	if err := inter.VerifySignatures(); err != nil {
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
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(2, true)
	defer l.CloseAll()

	c := NewTestClient(l)
	_, inter, cerr := c.CreateRootControl(el, el, 1, 1, 1, VerifyNone)
	log.ErrFatal(cerr)
	td := &testData{1, "data-sc"}
	inter, data, cerr := c.CreateData(inter, 1, 1, VerifyNone, td)
	log.ErrFatal(cerr)
	if err := data.VerifySignatures(); err != nil {
		t.Fatal("Couldn't verify data-signature:", err)
	}
	if !bytes.Equal(data.ParentBlockID, inter.Hash) {
		t.Fatal("Data-chain doesn't point to intermediate-chain")
	}
	if !bytes.Equal(inter.ChildSL.Hash, data.Hash) {
		t.Fatal("Intermediate chain doesn't point to data-chain")
	}
	_, td1, err := network.Unmarshal(data.Data)
	log.ErrFatal(err)
	reconstructed := td1.(*testData)
	if *td != *reconstructed {
		t.Fatalf("Stored data is not the same as initial data %+v vs %+v", td, td1.(*testData))
	}
}

func TestClient_ProposeData(t *testing.T) {
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()

	c := NewTestClient(l)
	log.Lvl1("Creating root and control chain")
	_, inter, cerr := c.CreateRootControl(el, el, 1, 1, 1, VerifyNone)
	log.ErrFatal(cerr)
	td := &testData{1, "data-sc"}
	log.Lvl1("Creating data chain")
	var data1 *SkipBlock
	inter, data1, cerr = c.CreateData(inter, 1, 1, VerifyNone, td)
	log.ErrFatal(cerr)
	td.A++
	log.Lvl1("Proposing data on intermediate chain")
	data2, cerr := c.ProposeData(inter, data1, td)
	log.ErrFatal(cerr)
	dataLast, cerr := c.GetUpdateChain(inter, data1.Hash)
	log.ErrFatal(cerr)
	if len(dataLast.Update) != 2 {
		t.Fatal("Should have two SkipBlocks for update-chain", len(dataLast.Update))
	}
	if !dataLast.Update[1].Equal(data2.Latest) {
		t.Fatal("Newest SkipBlock should be stored")
	}
	c.Close()
}

func TestClient_ProposeRoster(t *testing.T) {
	t.Skip("See https://github.com/dedis/cothority/issues/733")
	nbrHosts := 5
	l := onet.NewTCPTest()
	_, el, _ := l.GenTree(nbrHosts, true)
	defer l.CloseAll()

	c := NewTestClient(l)
	log.Lvl1("Creating root and control chain")
	_, inter, cerr := c.CreateRootControl(el, el, 1, 1, 1, VerifyNone)
	log.ErrFatal(cerr)
	el2 := onet.NewRoster(el.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", el2)
	var sb1 *ProposedSkipBlockReply
	sb1, cerr = c.ProposeRoster(inter, el2)
	log.ErrFatal(cerr)
	log.Lvl1("Proposing same roster again")
	_, cerr = c.ProposeRoster(inter, el2)
	if cerr == nil {
		t.Fatal("Appending two Blocks to the same last block should fail")
	}
	log.Lvl1("Proposing following roster")
	sb2, cerr := c.ProposeRoster(sb1.Latest, el2)
	log.ErrFatal(cerr)
	if !sb2.Previous.Equal(sb1.Latest) {
		t.Fatal("New previous should be previous latest")
	}
	if !bytes.Equal(sb2.Previous.ForwardLink[0].Hash,
		sb2.Latest.Hash) {
		t.Fatal("second should point to third SkipBlock")
	}

	updates, cerr := c.GetUpdateChain(inter, inter.Hash)
	if len(updates.Update) != 3 {
		t.Fatal("Should now have three Blocks to go from Genesis to current, but have", len(updates.Update), inter, sb2)
	}
	if !updates.Update[2].Equal(sb2.Latest) {
		t.Fatal("Last block in update-chain should be last block added")
	}
	c.Close()
}

type testData struct {
	A int
	B string
}
