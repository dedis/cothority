package skipchain

import (
	"testing"

	"bytes"

	"strconv"

	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestSkipBlock_Hash1(t *testing.T) {
	sbd1 := NewSkipBlock()
	sbd1.Data = []byte("1")
	sbd1.Height = 4
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := NewSkipBlock()
	sbd2.Data = []byte("2")
	sbd1.Height = 2
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_Hash2(t *testing.T) {
	local := sda.NewLocalTest()
	hosts, el, _ := local.GenTree(2, false, false, false)
	defer local.CloseAll()
	sbd1 := NewSkipBlock()
	sbd1.EntityList = el
	sbd1.Height = 1
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := NewSkipBlock()
	sbd2.EntityList = local.GenEntityListFromHost(hosts[0])
	sbd2.Height = 1
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlockInterface(t *testing.T) {
	// Tests the different accessors
}

func TestService_ProposeSkipBlock(t *testing.T) {
	// First create a roster to attach the data to it
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, service := makeHELS(local, 5)

	// Setting up root roster
	sbRoot := makeGenesisRoster(service, el)

	// send a ProposeBlock
	genesis := NewSkipBlock()
	genesis.Data = []byte("In the beginning God created the heaven and the earth.")
	genesis.MaximumHeight = 2
	genesis.ParentBlockId = sbRoot.Hash
	genesis.EntityList = sbRoot.EntityList
	blockCount := uint32(0)
	psbrMsg, err := service.ProposeSkipBlock(nil, &ProposeSkipBlock{nil, genesis})
	assert.Nil(t, err)
	psbr := psbrMsg.(*ProposedSkipBlockReply)
	latest := psbr.Latest
	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount, latest.Index)
	// the genesis block has a random back-link:
	assert.Equal(t, 1, len(latest.BackLinkIds))
	assert.NotEqual(t, 0, latest.BackLinkIds)

	next := NewSkipBlock()
	next.Data = []byte("And the earth was without form, and void; " +
		"and darkness was upon the face of the deep. " +
		"And the Spirit of God moved upon the face of the waters.")
	next.MaximumHeight = 2
	next.ParentBlockId = sbRoot.Hash
	next.EntityList = sbRoot.EntityList
	id := psbr.Latest.Hash
	psbrMsg, err = service.ProposeSkipBlock(nil, &ProposeSkipBlock{id, genesis})
	assert.Nil(t, err)
	psbr2 := psbrMsg.(*ProposedSkipBlockReply)
	dbg.Lvl2(psbr2)
	if psbr2 == nil {
		t.Fatal("Didn't get anything in return")
	}
	assert.NotNil(t, psbr2)
	assert.NotNil(t, psbr2.Latest)
	latest2 := psbr2.Latest
	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount, latest2.Index)
	assert.Equal(t, 1, len(latest2.BackLinkIds))
	assert.NotEqual(t, 0, latest2.BackLinkIds)

	// We've added 2 blocks, + root block = 3
	assert.Equal(t, 3, len(service.SkipBlocks))
}

func TestService_GetUpdateChain(t *testing.T) {
	// Create a small chain and test whether we can get from one element
	// of the chain to the last element with a valid slice of SkipBlocks
	local := sda.NewLocalTest()
	defer local.CloseAll()
	sbLength := 3
	_, el, s := makeHELS(local, sbLength)
	sbs := make([]*SkipBlock, sbLength)
	sbs[0] = makeGenesisRoster(s, el)
	// init skipchain
	for i := 1; i < sbLength; i++ {
		el.List = el.List[0 : sbLength-(i+1)]
		newSB := NewSkipBlock()
		newSB.EntityList = el
		psbrMsg, err := s.ProposeSkipBlock(nil,
			&ProposeSkipBlock{sbs[i-1].Hash, newSB})
		assert.Nil(t, err)
		reply := psbrMsg.(*ProposedSkipBlockReply)
		sbs[i] = reply.Latest
	}

	for i := 0; i < sbLength; i++ {
		m, err := s.GetUpdateChain(nil, &GetUpdateChain{sbs[i].Hash})
		sbc := m.(*GetUpdateChainReply)
		dbg.ErrFatal(err)
		if !sbc.Update[0].Equal(sbs[i]) {
			t.Fatal("First hash is not from our SkipBlock")
		}
		if !sbc.Update[len(sbc.Update)-1].Equal(sbs[sbLength-1]) {
			dbg.Lvl2(sbc.Update[len(sbc.Update)-1].Hash)
			dbg.Lvl2(sbs[sbLength-1].Hash)
			t.Fatal("Last Hash is not equal to last SkipBlock for", i)
		}
		for up, sb1 := range sbc.Update {
			dbg.ErrFatal(sb1.VerifySignatures())
			if up < len(sbc.Update)-1 {
				sb2 := sbc.Update[up+1]
				h1 := sb1.Height
				h2 := sb2.Height
				dbg.Lvl2("sbc1.Height=", sb1.Height)
				dbg.Lvl2("sbc2.Height=", sb2.Height)
				// height := min(len(sb1.ForwardLink), h2)
				height := h1
				if h2 < height {
					height = h2
				}
				if !bytes.Equal(sb1.ForwardLink[height-1].Hash,
					sb2.Hash) {
					t.Fatal("Forward-pointer of", up,
						"is different of hash in", up+1)
				}
			}
		}
	}
}

func TestService_SetChildrenSkipBlock(t *testing.T) {
	// How many nodes in Root
	nodesRoot := 3

	local := sda.NewLocalTest()
	defer local.CloseAll()
	hosts, el, service := makeHELS(local, nodesRoot)

	// Setting up two chains and linking one to the other
	sbRoot := makeGenesisRoster(service, el)
	sbInter := makeGenesisRosterArgs(service, el, sbRoot.Hash, VerifyShard)
	scsb := &SetChildrenSkipBlock{sbRoot.Hash, sbInter.Hash}
	service.SetChildrenSkipBlock(nil, scsb)
	// Wait for block-propagation
	time.Sleep(time.Millisecond * 100)
	// Verifying other nodes also got the updated chains
	// Check for the root-chain
	for i, h := range hosts {
		dbg.Lvl2(skipchainSID)
		s := local.Services[h.Entity.ID][skipchainSID].(*Service)
		m, err := s.GetUpdateChain(h.Entity, &GetUpdateChain{sbRoot.Hash})
		dbg.ErrFatal(err, "Failed in iteration="+strconv.Itoa(i)+":")
		sb := m.(*GetUpdateChainReply)
		dbg.Lvl2(s.Context)
		if len(sb.Update) != 1 { // we expect only the first block
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		link := sb.Update[0].ChildSL
		if !bytes.Equal(link.Hash, sbInter.Hash) {
			t.Fatal("The child-link doesn't point to our intermediate SkipBlock", i)
		}
		// We need to verify the signature on the child-link, too. This
		// has to be signed by the collective signature of sbRoot.
		if err = sbRoot.VerifySignatures(); err != nil {
			t.Fatal("Signature on child-link is not valid")
		}
	}

	// And check for the intermediate-chain to be updated
	for _, h := range hosts {
		s := local.Services[h.Entity.ID][skipchainSID].(*Service)

		m, err := s.GetUpdateChain(h.Entity, &GetUpdateChain{sbInter.Hash})
		sb := m.(*GetUpdateChainReply)

		dbg.ErrFatal(err)
		if len(sb.Update) != 1 {
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		if !bytes.Equal(sb.Update[0].ParentBlockId, sbRoot.Hash) {
			t.Fatal("The intermediate SkipBlock doesn't point to the root")
		}
		if err = sb.Update[0].VerifySignatures(); err != nil {
			t.Fatal("Signature of that SkipBlock doesn't fit")
		}
	}
}

func TestService_PropagateSkipBlock(t *testing.T) {
}

func TestService_SignBlock(t *testing.T) {

}

func TestService_ForwardSignature(t *testing.T) {
}

// makes a genesis Roster-block
func makeGenesisRosterArgs(s *Service, el *sda.EntityList, parent SkipBlockID,
	vid VerifierID) *SkipBlock {
	sb := NewSkipBlock()
	sb.EntityList = el
	sb.MaximumHeight = 4
	sb.ParentBlockId = parent
	sb.VerifierId = vid
	psbrMsg, err := s.ProposeSkipBlock(nil,
		&ProposeSkipBlock{nil, sb})
	psbr := psbrMsg.(*ProposedSkipBlockReply)
	dbg.ErrFatal(err)
	return psbr.Latest
}

func makeGenesisRoster(s *Service, el *sda.EntityList) *SkipBlock {
	return makeGenesisRosterArgs(s, el, nil, VerifyNone)
}

// Makes a Host, an EntityList, and a service
func makeHELS(local *sda.LocalTest, nbr int) ([]*sda.Host, *sda.EntityList, *Service) {
	hosts := local.GenLocalHosts(nbr, false, true)
	el := local.GenEntityListFromHost(hosts...)
	return hosts, el, local.Services[hosts[0].Entity.ID][skipchainSID].(*Service)
}
