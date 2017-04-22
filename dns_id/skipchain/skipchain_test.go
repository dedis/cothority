package skipchain

import (
	"testing"

	"bytes"

	"strconv"

	"errors"
	"fmt"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m, 2)
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
	local := onet.NewLocalTest()
	hosts, el, _ := local.GenTree(2, false)
	defer local.CloseAll()
	sbd1 := NewSkipBlock()
	sbd1.Roster = el
	sbd1.Height = 1
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := NewSkipBlock()
	sbd2.Roster = local.GenRosterFromHost(hosts[0])
	sbd2.Height = 1
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestService_ProposeSkipBlock(t *testing.T) {
	// First create a roster to attach the data to it
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, genService := local.MakeHELS(5, skipchainSID)
	service := genService.(*Service)
	service.SkipBlocks = make(map[string]*SkipBlock)

	// Setting up root roster
	sbRoot, err := makeGenesisRoster(service, el)
	log.ErrFatal(err)

	// send a ProposeBlock
	genesis := NewSkipBlock()
	genesis.Data = []byte("In the beginning God created the heaven and the earth.")
	genesis.MaximumHeight = 2
	genesis.BaseHeight = 2
	genesis.ParentBlockID = sbRoot.Hash
	genesis.Roster = sbRoot.Roster
	blockCount := 0
	psbrMsg, err := service.ProposeSkipBlock(nil, &ProposeSkipBlock{nil, genesis})
	assert.Nil(t, err)
	psbr := psbrMsg.(*ProposedSkipBlockReply)
	latest := psbr.Latest
	// verify creation of GenesisBlock:
	assert.Equal(t, blockCount, latest.Index)
	// the genesis block has a random back-link:
	assert.Equal(t, 1, len(latest.BackLinkIds))
	assert.NotEqual(t, 0, latest.BackLinkIds)

	next := NewSkipBlock()
	next.Data = []byte("And the earth was without form, and void; " +
		"and darkness was upon the face of the deep. " +
		"And the Spirit of God moved upon the face of the waters.")
	next.MaximumHeight = 2
	next.ParentBlockID = sbRoot.Hash
	next.Roster = sbRoot.Roster
	id := psbr.Latest.Hash
	psbrMsg, err = service.ProposeSkipBlock(nil, &ProposeSkipBlock{id, next})
	assert.Nil(t, err)
	psbr2 := psbrMsg.(*ProposedSkipBlockReply)
	log.Lvl2(psbr2)
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
	assert.Equal(t, 3, service.lenSkipBlocks())
}

func TestService_GetUpdateChain(t *testing.T) {
	// Create a small chain and test whether we can get from one element
	// of the chain to the last element with a valid slice of SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	sbLength := 3
	_, el, gs := local.MakeHELS(sbLength, skipchainSID)
	s := gs.(*Service)
	sbs := make([]*SkipBlock, sbLength)
	var err error
	sbs[0], err = makeGenesisRoster(s, el)
	log.ErrFatal(err)
	log.Lvl1("Initialize skipchain.")
	// init skipchain
	for i := 1; i < sbLength; i++ {
		log.Lvl2("Doing skipblock", i)
		newSB := NewSkipBlock()
		newSB.Roster = el
		psbrMsg, err := s.ProposeSkipBlock(nil,
			&ProposeSkipBlock{sbs[i-1].Hash, newSB})
		assert.Nil(t, err)
		reply := psbrMsg.(*ProposedSkipBlockReply)
		sbs[i] = reply.Latest
	}

	for i := 0; i < sbLength; i++ {
		m, err := s.GetUpdateChain(nil, &GetUpdateChain{sbs[i].Hash})
		sbc := m.(*GetUpdateChainReply)
		log.ErrFatal(err)
		if !sbc.Update[0].Equal(sbs[i]) {
			t.Fatal("First hash is not from our SkipBlock")
		}
		if !sbc.Update[len(sbc.Update)-1].Equal(sbs[sbLength-1]) {
			log.Lvl2(sbc.Update[len(sbc.Update)-1].Hash)
			log.Lvl2(sbs[sbLength-1].Hash)
			t.Fatal("Last Hash is not equal to last SkipBlock for", i)
		}
		for up, sb1 := range sbc.Update {
			log.ErrFatal(sb1.VerifySignatures())
			if up < len(sbc.Update)-1 {
				sb2 := sbc.Update[up+1]
				h1 := sb1.Height
				h2 := sb2.Height
				log.Lvl2("sbc1.Height=", sb1.Height)
				log.Lvl2("sbc2.Height=", sb2.Height)
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

	local := onet.NewLocalTest()
	defer local.CloseAll()
	hosts, el, genService := local.MakeHELS(nodesRoot, skipchainSID)
	service := genService.(*Service)

	// Setting up two chains and linking one to the other
	sbRoot, err := makeGenesisRoster(service, el)
	log.ErrFatal(err)
	sbInter, err := makeGenesisRosterArgs(service, el, sbRoot.Hash, VerifyNone, 1, 1)
	log.ErrFatal(err)
	scsb := &SetChildrenSkipBlock{sbRoot.Hash, sbInter.Hash}
	service.SetChildrenSkipBlock(nil, scsb)
	// Verifying other nodes also got the updated chains
	// Check for the root-chain
	for i, h := range hosts {
		log.Lvlf2("%x", skipchainSID)
		s := local.Services[h.ServerIdentity.ID][skipchainSID].(*Service)
		m, err := s.GetUpdateChain(h.ServerIdentity, &GetUpdateChain{sbRoot.Hash})
		log.ErrFatal(err, "Failed in iteration="+strconv.Itoa(i)+":")
		sb := m.(*GetUpdateChainReply)
		log.Lvl2(s.Context)
		if len(sb.Update) != 1 {
			// we expect only the first block
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		link := sb.Update[0].ChildSL
		if !bytes.Equal(link.Hash, sbInter.Hash) {
			t.Fatal("The child-link doesn't point to our intermediate SkipBlock", i)
		}
		// We need to verify the signature on the child-link, too. This
		// has to be signed by the collective signature of sbRoot.
		if err = sbRoot.VerifySignatures(); err != nil { //???????????????????????????? How this command verifies the
			//signature on the chil-link?
			t.Fatal("Signature on child-link is not valid")
		}
	}

	// And check for the intermediate-chain to be updated
	for _, h := range hosts {
		s := local.Services[h.ServerIdentity.ID][skipchainSID].(*Service)

		m, err := s.GetUpdateChain(h.ServerIdentity, &GetUpdateChain{sbInter.Hash})
		sb := m.(*GetUpdateChainReply)

		log.ErrFatal(err)
		if len(sb.Update) != 1 {
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		if !bytes.Equal(sb.Update[0].ParentBlockID, sbRoot.Hash) {
			t.Fatal("The intermediate SkipBlock doesn't point to the root")
		}
		if err = sb.Update[0].VerifySignatures(); err != nil {
			t.Fatal("Signature of that SkipBlock doesn't fit")
		}
	}
}

func TestService_MultiLevel(t *testing.T) {
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, genService := local.MakeHELS(3, skipchainSID)
	service := genService.(*Service)

	for base := 1; base <= 3; base++ {
		for height := 1; height <= 3; height++ {
			if base == 1 && height > 1 {
				break
			}
			sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerifyNone,
				base, height)
			log.ErrFatal(err)
			latest := sbRoot
			log.Lvl1("Adding blocks for", base, height)
			for sbi := 1; sbi < 10; sbi++ {
				sb := NewSkipBlock()
				sb.Roster = el
				psbr, err := service.ProposeSkipBlock(nil,
					&ProposeSkipBlock{latest.Hash, sb})
				log.ErrFatal(err)
				latest = psbr.(*ProposedSkipBlockReply).Latest
			}

			log.ErrFatal(checkMLForwardBackward(service, sbRoot, base, height))
			log.ErrFatal(checkMLUpdate(service, sbRoot, latest, base, height))
		}
	}
	// Setting up two chains and linking one to the other
}

func checkMLForwardBackward(service *Service, root *SkipBlock, base, height int) error {
	genesis, ok := service.getSkipBlockByID(root.Hash)
	if !ok {
		return errors.New("Didn't find genesis-block in service")
	}
	if len(genesis.ForwardLink) != height {
		return errors.New("Genesis-block doesn't have forward-links of " +
			strconv.Itoa(height))
	}
	return nil
}

func checkMLUpdate(service *Service, root, latest *SkipBlock, base, height int) error {
	chain, err := service.GetUpdateChain(nil, &GetUpdateChain{root.Hash})
	if err != nil {
		return err
	}
	updates := chain.(*GetUpdateChainReply).Update
	genesis := updates[0]
	if len(genesis.ForwardLink) != height {
		return errors.New("Genesis-block doesn't have height " + strconv.Itoa(height))
	}
	if len(updates[1].BackLinkIds) != height {
		return errors.New("Second block doesn't have correct number of backlinks")
	}
	l := updates[len(updates)-1]
	if len(l.ForwardLink) != 0 {
		return errors.New("Last block still has forward-links")
	}
	if !l.Equal(latest) {
		return errors.New("Last block from update is not the same as last block")
	}
	log.Lvl2(base, height, len(updates))
	if base > 1 && height > 1 && len(updates) == 10 {
		return fmt.Errorf("Shouldn't need 10 blocks with base %d and height %d",
			base, height)
	}
	return nil
}

func TestService_Verification(t *testing.T) {
	local := onet.NewLocalTest()
	defer local.CloseAll()
	sbLength := 4
	_, el, genService := local.MakeHELS(sbLength, skipchainSID)
	service := genService.(*Service)

	elRoot := onet.NewRoster(el.List[0:3])
	sbRoot, err := makeGenesisRoster(service, elRoot)
	log.ErrFatal(err)

	log.Lvl1("Creating non-conforming skipBlock")
	sb := NewSkipBlock()
	sb.Roster = el
	sb.MaximumHeight = 1
	sb.BaseHeight = 1
	sb.ParentBlockID = sbRoot.Hash
	sb.VerifierID = VerifyShard
	_, err = service.ProposeSkipBlock(nil,
		&ProposeSkipBlock{nil, sb})
	require.NotNil(t, err, "Shouldn't accept a non-confoirming skipblock")

	log.Lvl1("Creating skipblock with same Roster as root")
	sbInter, err := makeGenesisRosterArgs(service, elRoot, sbRoot.Hash, VerifyShard, 1, 1)
	log.ErrFatal(err)
	log.Lvl1("Creating skipblock with sub-Roster from root")
	elSub := onet.NewRoster(el.List[0:2])
	sbInter, err = makeGenesisRosterArgs(service, elSub, sbRoot.Hash, VerifyShard, 1, 1)
	log.ErrFatal(err)
	scsb := &SetChildrenSkipBlock{sbRoot.Hash, sbInter.Hash}
	service.SetChildrenSkipBlock(nil, scsb)
}

func TestCopy(t *testing.T) {
	// Test if copy is deep or only shallow
	b1 := NewBlockLink()
	b1.Signature = []byte{1}
	b2 := b1.Copy()
	b2.Signature = []byte{2}
	if bytes.Equal(b1.Signature, b2.Signature) {
		t.Fatal("They should not be equal")
	}

	sb1 := NewSkipBlock()
	sb1.ChildSL = NewBlockLink()
	sb2 := sb1.Copy()
	sb1.ChildSL.Signature = []byte{1}
	sb2.ChildSL.Signature = []byte{2}
	if bytes.Equal(sb1.ChildSL.Signature, sb2.ChildSL.Signature) {
		t.Fatal("They should not be equal")
	}
	sb1.Height = 10
	sb2.Height = 20
	if sb1.Height == sb2.Height {
		t.Fatal("Should not be equal")
	}
}

func TestService_SignBlock(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, genService := local.MakeHELS(3, skipchainSID)
	service := genService.(*Service)

	sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerifyNone, 1, 1)
	log.ErrFatal(err)
	el2 := onet.NewRoster(el.List[0:2])
	sb := NewSkipBlock()
	sb.Roster = el2
	psbr, err := service.ProposeSkipBlock(nil,
		&ProposeSkipBlock{sbRoot.Hash, sb})
	log.ErrFatal(err)
	reply := psbr.(*ProposedSkipBlockReply)
	sbRoot = reply.Previous
	sbSecond := reply.Latest
	log.Lvl1("Verifying signatures")
	log.ErrFatal(sbRoot.VerifySignatures())
	log.ErrFatal(sbSecond.VerifySignatures())
}

func TestService_ProtocolVerification(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	hosts, el, s := local.MakeHELS(3, skipchainSID)
	s1 := s.(*Service)
	s2 := local.Services[hosts[1].ServerIdentity.ID][skipchainSID].(*Service)
	s3 := local.Services[hosts[2].ServerIdentity.ID][skipchainSID].(*Service)
	services := []*Service{s1, s2, s3}

	sb, err := makeGenesisRosterArgs(s1, el, nil, VerifyNone, 1, 1)
	log.ErrFatal(err)
	for i := 0; i < 3; i++ {
		sb = launchVerification(t, services, i, sb)
	}
}

func launchVerification(t *testing.T, services []*Service, n int, prev *SkipBlock) *SkipBlock {
	next := NewSkipBlock()
	next.Roster = prev.Roster
	for _, s := range services {
		s.testVerify = false
	}
	sb, err := services[n].ProposeSkipBlock(nil,
		&ProposeSkipBlock{prev.Hash, next})
	log.ErrFatal(err)
	for _, s := range services {
		if !s.testVerify {
			t.Fatal("Service", n, "didn't verify")
		}
	}
	return sb.(*ProposedSkipBlockReply).Latest
}

func TestService_ForwardSignature(t *testing.T) {
}

func TestService_RegisterVerification(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	onet.RegisterNewService("ServiceVerify", newServiceVerify)
	local := onet.NewLocalTest()
	defer local.CloseAll()
	hosts, el, s1 := makeHELS(local, 3)
	VerifyTest := VerifierID(uuid.NewV5(uuid.NamespaceURL, "Test1"))
	ver := make(chan bool, 3)
	verifier := func(msg []byte, s *SkipBlock) bool {
		ver <- true
		return true
	}
	for _, h := range hosts {
		s := h.GetService(ServiceName).(*Service)
		log.ErrFatal(s.RegisterVerification(VerifyTest, verifier))
	}
	sb, err := makeGenesisRosterArgs(s1, el, nil, VerifyTest, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 3, len(ver))

	sb, err = makeGenesisRosterArgs(s1, el, nil, ServiceVerifier, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 3, len(ServiceVerifierChan))
}

var ServiceVerifier = VerifierID(uuid.NewV5(uuid.NamespaceURL, "ServiceVerifier"))
var ServiceVerifierChan = make(chan bool, 3)

type ServiceVerify struct {
	*onet.ServiceProcessor
}

func (sv *ServiceVerify) Verify(msg []byte, sb *SkipBlock) bool {
	ServiceVerifierChan <- true
	return true
}

func (sv *ServiceVerify) NewProtocol(tn *onet.TreeNodeInstance, c *onet.GenericConfig) (onet.ProtocolInstance, error) {
	return nil, nil
}

func newServiceVerify(c *onet.Context, path string) onet.Service {
	sv := &ServiceVerify{}
	log.ErrFatal(RegisterVerification(c, ServiceVerifier, sv.Verify))
	return sv
}

// makes a genesis Roster-block
func makeGenesisRosterArgs(s *Service, el *onet.Roster, parent SkipBlockID,
	vid VerifierID, base, height int) (*SkipBlock, error) {
	sb := NewSkipBlock()
	sb.Roster = el
	sb.MaximumHeight = height
	sb.BaseHeight = base
	sb.ParentBlockID = parent
	sb.VerifierID = vid
	psbrMsg, err := s.ProposeSkipBlock(nil,
		&ProposeSkipBlock{nil, sb})
	if err != nil {
		return nil, err
	}
	psbr := psbrMsg.(*ProposedSkipBlockReply)
	return psbr.Latest, nil
}

func makeGenesisRoster(s *Service, el *onet.Roster) (*SkipBlock, error) {
	return makeGenesisRosterArgs(s, el, nil, VerifyNone, 1, 1)
}

// Makes a Host, an Roster, and a service
func makeHELS(local *onet.LocalTest, nbr int) ([]*onet.Conode, *onet.Roster, *Service) {
	hosts := local.GenConodes(nbr)
	el := local.GenRosterFromHost(hosts...)
	return hosts, el, local.Services[hosts[0].ServerIdentity.ID][skipchainSID].(*Service)
}
