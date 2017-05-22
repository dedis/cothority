package skipchain

import (
	"testing"

	"bytes"

	"strconv"

	"errors"
	"fmt"

	"time"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_StoreSkipBlock(t *testing.T) {
	// First create a roster to attach the data to it
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, genService := local.MakeHELS(5, skipchainSID)
	service := genService.(*Service)
	service.Sbm.SkipBlocks = make(map[string]*SkipBlock)

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
	genesis.VerifierIDs = VerificationStandard
	blockCount := 0
	psbr, err := service.StoreSkipBlock(&StoreSkipBlock{nil, genesis})
	assert.Nil(t, err)
	latest := psbr.Latest
	// verify creation of GenesisBlock:
	assert.Equal(t, blockCount, latest.Index)
	// the genesis block has a random back-link:
	assert.Equal(t, 1, len(latest.BackLinkIDs))
	assert.NotEqual(t, 0, latest.BackLinkIDs)

	next := NewSkipBlock()
	next.Data = []byte("And the earth was without form, and void; " +
		"and darkness was upon the face of the deep. " +
		"And the Spirit of God moved upon the face of the waters.")
	next.MaximumHeight = 2
	next.ParentBlockID = sbRoot.Hash
	next.Roster = sbRoot.Roster
	id := psbr.Latest.Hash
	psbr2, err := service.StoreSkipBlock(&StoreSkipBlock{id, next})
	assert.Nil(t, err)
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
	assert.Equal(t, 1, len(latest2.BackLinkIDs))
	assert.NotEqual(t, 0, latest2.BackLinkIDs)

	// We've added 2 blocks, + root block = 3
	assert.Equal(t, 3, service.Sbm.Length())
}

func TestService_GetUpdateChain(t *testing.T) {
	// Create a small chain and test whether we can get from one element
	// of the chain to the last element with a valid slice of SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	conodes := 10
	sbCount := conodes - 1
	servers, el, gs := local.MakeHELS(conodes, skipchainSID)
	s := gs.(*Service)
	sbs := make([]*SkipBlock, sbCount)
	var err error
	sbs[0], err = makeGenesisRoster(s, onet.NewRoster(el.List[0:2]))
	log.ErrFatal(err)
	log.Lvl1("Initialize skipchain.")
	// init skipchain
	for i := 1; i < sbCount; i++ {
		newSB := NewSkipBlock()
		newSB.Roster = onet.NewRoster(el.List[i : i+2])
		service := local.Services[servers[i].ServerIdentity.ID][skipchainSID].(*Service)
		log.Lvl2("Doing skipblock", i, servers[i].ServerIdentity, newSB.Roster.List)
		reply, err := service.StoreSkipBlock(&StoreSkipBlock{sbs[i-1].Hash, newSB})
		assert.Nil(t, err)
		require.NotNil(t, reply.Latest)
		sbs[i] = reply.Latest
	}

	for i := 0; i < sbCount; i++ {
		m, err := s.GetUpdateChain(&GetUpdateChain{sbs[i].Hash})
		log.ErrFatal(err)
		sbc := m.(*GetUpdateChainReply)
		if !sbc.Update[0].Equal(sbs[i]) {
			t.Fatal("First hash is not from our SkipBlock")
		}
		require.True(t, len(sbc.Update) > 0, "Empty update-chain")
		if !sbc.Update[len(sbc.Update)-1].Equal(sbs[sbCount-1]) {
			log.Lvl2(sbc.Update[len(sbc.Update)-1].Hash)
			log.Lvl2(sbs[sbCount-1].Hash)
			t.Fatal("Last Hash is not equal to last SkipBlock for", i)
		}
		for up, sb1 := range sbc.Update {
			log.ErrFatal(sb1.VerifyForwardSignatures())
			if up < len(sbc.Update)-1 {
				sb2 := sbc.Update[up+1]
				h1 := sb1.Height
				h2 := sb2.Height
				log.Lvl3("sbc1.Height=", sb1.Height)
				log.Lvl3("sbc2.Height=", sb2.Height)
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
	sbInter, err := makeGenesisRosterArgs(service, el, sbRoot.Hash, VerificationNone, 1, 1)
	log.ErrFatal(err)
	// Verifying other nodes also got the updated chains
	// Check for the root-chain
	for i, h := range hosts {
		log.Lvlf2("%x", skipchainSID)
		s := local.Services[h.ServerIdentity.ID][skipchainSID].(*Service)
		m, err := s.GetUpdateChain(&GetUpdateChain{sbRoot.Hash})
		log.ErrFatal(err, "Failed in iteration="+strconv.Itoa(i)+":")
		sb := m.(*GetUpdateChainReply)
		log.Lvl2(s.Context)
		if len(sb.Update) != 1 {
			// we expect only the first block
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		require.Equal(t, 1, len(sb.Update[0].ChildSL), "No child-entry found")
		link := sb.Update[0].ChildSL[0]
		if !link.Equal(sbInter.Hash) {
			t.Fatal("The child-link doesn't point to our intermediate SkipBlock", i)
		}
		// We need to verify the signature on the child-link, too. This
		// has to be signed by the collective signature of sbRoot.
		if cerr := sbRoot.VerifyForwardSignatures(); cerr != nil {
			t.Fatal("Signature on child-link is not valid")
		}
	}

	// And check for the intermediate-chain to be updated
	for _, h := range hosts {
		s := local.Services[h.ServerIdentity.ID][skipchainSID].(*Service)

		m, cerr := s.GetUpdateChain(&GetUpdateChain{sbInter.Hash})
		sb := m.(*GetUpdateChainReply)

		log.ErrFatal(cerr)
		if len(sb.Update) != 1 {
			t.Fatal("There should be only 1 SkipBlock in the update")
		}
		if !bytes.Equal(sb.Update[0].ParentBlockID, sbRoot.Hash) {
			t.Fatal("The intermediate SkipBlock doesn't point to the root")
		}
		if err := sb.Update[0].VerifyForwardSignatures(); err != nil {
			t.Fatal("Signature of that SkipBlock doesn't fit")
		}
	}
}

func TestService_MultiLevel(t *testing.T) {
	local := onet.NewLocalTest()
	defer local.CloseAll()
	servers, el, genService := local.MakeHELS(3, skipchainSID)
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
	}
	service := genService.(*Service)

	for base := 1; base <= 3; base++ {
		for height := 1; height <= base; height++ {
			log.Lvl1("Making genesis for", base, height)
			if base == 1 && height > 1 {
				break
			}
			sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerificationNone,
				base, height)
			log.ErrFatal(err)
			latest := sbRoot
			log.Lvl1("Adding blocks for", base, height)
			for sbi := 1; sbi < 10; sbi++ {
				log.Lvl3("Adding block", sbi)
				sb := NewSkipBlock()
				sb.Roster = el
				psbr, err := service.StoreSkipBlock(&StoreSkipBlock{latest.Hash, sb})
				log.ErrFatal(err)
				latest = psbr.Latest
				checkBacklinks(services, latest)
			}

			log.ErrFatal(checkMLForwardBackward(service, sbRoot, base, height))
			log.ErrFatal(checkMLUpdate(service, sbRoot, latest, base, height))
		}
	}
	// Setting up two chains and linking one to the other
	//time.Sleep(time.Second)
}

func checkBacklinks(services []*Service, sb *SkipBlock) {
	for n, i := range sb.BackLinkIDs {
		for ns, s := range services {
			for {
				log.Lvl3("Checking backlink", n, ns)
				bl, err := s.GetSingleBlock(&GetSingleBlock{i})
				log.ErrFatal(err)
				bl.fwMutex.Lock()
				if len(bl.ForwardLink) == n+1 &&
					bl.ForwardLink[n].Hash.Equal(sb.Hash) {
					bl.fwMutex.Unlock()
					break
				}
				bl.fwMutex.Unlock()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
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
	sb.VerifierIDs = VerificationStandard
	//_, err = service.ProposeSkipBlock(&ProposeSkipBlock{nil, sb})
	//require.NotNil(t, err, "Shouldn't accept a non-conforming skipblock")

	log.Lvl1("Creating skipblock with same Roster as root")
	sbInter, err := makeGenesisRosterArgs(service, elRoot, sbRoot.Hash, sb.VerifierIDs, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sbInter)
	log.Lvl1("Creating skipblock with sub-Roster from root")
	elSub := onet.NewRoster(el.List[0:2])
	sbInter, err = makeGenesisRosterArgs(service, elSub, sbRoot.Hash, sb.VerifierIDs, 1, 1)
	log.ErrFatal(err)
}

func TestService_SignBlock(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, genService := local.MakeHELS(3, skipchainSID)
	service := genService.(*Service)

	sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerificationNone, 1, 1)
	log.ErrFatal(err)
	el2 := onet.NewRoster(el.List[0:2])
	sb := NewSkipBlock()
	sb.Roster = el2
	reply, err := service.StoreSkipBlock(&StoreSkipBlock{sbRoot.Hash, sb})
	log.ErrFatal(err)
	sbRoot = reply.Previous
	sbSecond := reply.Latest
	log.Lvl1("Verifying signatures")
	log.ErrFatal(sbRoot.VerifyForwardSignatures())
	log.ErrFatal(sbSecond.VerifyForwardSignatures())
}

func TestService_ProtocolVerification(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(3, skipchainSID)
	s1 := s.(*Service)
	count := make(chan bool, 3)
	verifyFunc := func(newID []byte, newSB *SkipBlock) bool {
		count <- true
		return true
	}
	verifyID := VerifierID(uuid.NewV1())
	for _, s := range local.Services {
		s[skipchainSID].(*Service).registerVerification(verifyID, verifyFunc)
	}

	sbRoot, err := makeGenesisRosterArgs(s1, el, nil, []VerifierID{verifyID}, 1, 1)
	log.ErrFatal(err)
	sbNext := sbRoot.Copy()
	sbNext.BackLinkIDs = []SkipBlockID{sbRoot.Hash}
	_, cerr := s1.StoreSkipBlock(&StoreSkipBlock{sbRoot.Hash, sbNext})
	log.ErrFatal(cerr)
	for i := 0; i < 3; i++ {
		select {
		case <-count:
		case <-time.After(time.Second):
			t.Fatal("Timeout while waiting for reply", i)
		}
	}
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
		log.ErrFatal(s.registerVerification(VerifyTest, verifier))
	}
	sb, err := makeGenesisRosterArgs(s1, el, nil, []VerifierID{VerifyTest}, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ver))

	sb, err = makeGenesisRosterArgs(s1, el, nil, []VerifierID{ServiceVerifier}, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ServiceVerifierChan))
}

func TestService_StoreSkipBlock2(t *testing.T) {
	nbrHosts := 3
	local := onet.NewLocalTest()
	defer local.CloseAll()
	hosts, roster, s1 := makeHELS(local, nbrHosts)
	s2 := local.Services[hosts[1].ServerIdentity.ID][skipchainSID].(*Service)
	s3 := local.Services[hosts[2].ServerIdentity.ID][skipchainSID].(*Service)

	log.Lvl1("Creating root and control chain")
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	ssbr, cerr := s1.StoreSkipBlock(&StoreSkipBlock{nil, sbRoot})
	log.ErrFatal(cerr)
	roster2 := onet.NewRoster(roster.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", roster2)
	sb1 := ssbr.Latest.Copy()
	sb1.Roster = roster2
	ssbr, cerr = s2.StoreSkipBlock(&StoreSkipBlock{sbRoot.Hash, sb1})
	log.ErrFatal(cerr)
	require.NotNil(t, ssbr.Latest)

	// Error testing
	sbErr := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	sbErr.ParentBlockID = SkipBlockID([]byte{1, 2, 3})
	_, cerr = s1.StoreSkipBlock(&StoreSkipBlock{nil, sbErr})
	require.NotNil(t, cerr)
	_, cerr = s1.StoreSkipBlock(&StoreSkipBlock{sbErr.ParentBlockID, sbErr})
	// Last successful log...
	require.NotNil(t, cerr)
	sbErr = ssbr.Latest.Copy()
	_, cerr = s3.StoreSkipBlock(&StoreSkipBlock{ssbr.Latest.Hash, sbErr})
	require.NotNil(t, cerr)
}

func TestService_StoreSkipBlockSpeed(t *testing.T) {
	t.Skip("This is a hidden benchmark")
	nbrHosts := 3
	local := onet.NewLocalTest()
	defer local.CloseAll()
	_, roster, s1 := makeHELS(local, nbrHosts)

	log.Lvl1("Creating root and control chain")
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	ssbrep, cerr := s1.StoreSkipBlock(&StoreSkipBlock{nil, sbRoot})
	log.ErrFatal(cerr)

	last := time.Now()
	for i := 0; i < 500; i++ {
		now := time.Now()
		log.Lvl3(i, now.Sub(last))
		last = now
		ssbrep, cerr = s1.StoreSkipBlock(&StoreSkipBlock{ssbrep.Latest.Hash,
			sbRoot})
		log.ErrFatal(cerr)
	}
}

func checkMLForwardBackward(service *Service, root *SkipBlock, base, height int) error {
	genesis := service.Sbm.GetByID(root.Hash)
	if genesis == nil {
		return errors.New("Didn't find genesis-block in service")
	}
	if len(genesis.ForwardLink) != height {
		return errors.New("Genesis-block doesn't have forward-links of " +
			strconv.Itoa(height))
	}
	return nil
}

func checkMLUpdate(service *Service, root, latest *SkipBlock, base, height int) error {
	log.Lvl3(service, root, latest, base, height)
	chain, err := service.GetUpdateChain(&GetUpdateChain{root.Hash})
	if err != nil {
		return err
	}
	updates := chain.(*GetUpdateChainReply).Update
	genesis := updates[0]
	if len(genesis.ForwardLink) != height {
		return errors.New("Genesis-block doesn't have height " + strconv.Itoa(height))
	}
	if len(updates[1].BackLinkIDs) != height {
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

func newServiceVerify(c *onet.Context) onet.Service {
	sv := &ServiceVerify{}
	log.ErrFatal(RegisterVerification(c, ServiceVerifier, sv.Verify))
	return sv
}

// makes a genesis Roster-block
func makeGenesisRosterArgs(s *Service, el *onet.Roster, parent SkipBlockID,
	vid []VerifierID, base, height int) (*SkipBlock, error) {
	sb := NewSkipBlock()
	sb.Roster = el
	sb.MaximumHeight = height
	sb.BaseHeight = base
	sb.ParentBlockID = parent
	sb.VerifierIDs = vid
	psbr, err := s.StoreSkipBlock(&StoreSkipBlock{nil, sb})
	if err != nil {
		return nil, err
	}
	return psbr.Latest, nil
}

func makeGenesisRoster(s *Service, el *onet.Roster) (*SkipBlock, error) {
	return makeGenesisRosterArgs(s, el, nil, VerificationNone, 1, 1)
}

// Makes a Host, an Roster, and a service
func makeHELS(local *onet.LocalTest, nbr int) ([]*onet.Server, *onet.Roster, *Service) {
	hosts := local.GenServers(nbr)
	el := local.GenRosterFromHost(hosts...)
	return hosts, el, local.Services[hosts[0].ServerIdentity.ID][skipchainSID].(*Service)
}
