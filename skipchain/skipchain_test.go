package skipchain

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/satori/go.uuid.v1"
)

func init() {
	skipchainSID = onet.ServiceFactory.ServiceID(ServiceName)
}

var skipchainSID onet.ServiceID

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_StoreSkipBlock_Failure(t *testing.T) {
	storeSkipBlock(t, 4, true)
}

func TestService_StoreSkipBlock(t *testing.T) {
	storeSkipBlock(t, 4, false)
}

func storeSkipBlock(t *testing.T, nbrServers int, fail bool) {
	// First create a roster to attach the data to it
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, el, genService := local.MakeSRS(cothority.Suite, nbrServers, skipchainSID)
	services := local.GetServices(servers, skipchainSID)
	for _, s := range services {
		if fail {
			s.(*Service).bftTimeout = 10 * time.Second
		}
	}
	service := genService.(*Service)
	// This is the poor server who will play the part of the dead server
	// for us.
	deadServer := servers[len(servers)-1]

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
	psbr, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: genesis})
	if err != nil {
		t.Fatal("StoreSkipBlock:", err)
	}
	latest := psbr.Latest
	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount-1, latest.Index)
	// the genesis block has a random back-link:
	assert.Equal(t, 1, len(latest.BackLinkIDs))
	assert.NotEqual(t, 0, latest.BackLinkIDs)

	// kill one node and it should still work
	if fail {
		log.Lvl2("Pausing server", deadServer.Address())
		deadServer.Pause()
	}

	next := NewSkipBlock()
	next.Data = []byte("And the earth was without form, and void; " +
		"and darkness was upon the face of the deep. ")
	next.MaximumHeight = 2
	next.ParentBlockID = sbRoot.Hash
	next.Roster = sbRoot.Roster
	id := psbr.Latest.Hash
	if id == nil {
		t.Fatal("second block last id is nil")
	}
	psbr2, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: id, NewBlock: next})
	if err != nil {
		t.Fatal("StoreSkipBlock:", err)
	}
	assert.NotNil(t, psbr2)
	assert.NotNil(t, psbr2.Latest)
	latest2 := psbr2.Latest

	blockCount++
	assert.Equal(t, blockCount-1, latest2.Index)
	assert.Equal(t, 1, len(latest2.BackLinkIDs))
	assert.NotEqual(t, 0, latest2.BackLinkIDs)

	// And add it again, with all nodes running
	if fail {
		log.Lvl3("Unpausing server ", deadServer.Address())
		deadServer.Unpause()
	}

	next.ParentBlockID = next.Hash
	next.Data = []byte("And the Spirit of God moved upon the face of the waters.")
	psbr3, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: psbr2.Latest.Hash, NewBlock: next})
	assert.NotNil(t, psbr3)
	assert.NotNil(t, psbr3.Latest)
	latest3 := psbr3.Latest

	// As the propagation of the last forward link might take some time, wait for
	// it to be propagated by checking whether the updateChain has the new forward
	// link included.
	for {
		gucr, err := service.GetUpdateChain(&GetUpdateChain{LatestID: genesis.Hash})
		require.Nil(t, err)
		if len(gucr.Update) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// verify creation of GenesisBlock:
	blockCount++
	assert.Equal(t, blockCount-1, latest3.Index)
	assert.Equal(t, 2, len(latest3.BackLinkIDs))
	assert.NotEqual(t, 0, latest3.BackLinkIDs)

	// +1 for the root block
	assert.Equal(t, blockCount+1, service.db.Length())
}

func TestService_SetChildrenSkipBlock(t *testing.T) {
	// How many nodes in Root
	nodesRoot := 3

	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	hosts, el, genService := local.MakeSRS(cothority.Suite, nodesRoot, skipchainSID)
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
		sb, err := s.GetUpdateChain(&GetUpdateChain{LatestID: sbRoot.Hash})
		log.ErrFatal(err, "Failed in iteration="+strconv.Itoa(i)+":")
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
		if err := sbRoot.VerifyForwardSignatures(); err != nil {
			t.Fatal("Signature on child-link is not valid")
		}
	}

	// And check for the intermediate-chain to be updated
	for _, h := range hosts {
		s := local.Services[h.ServerIdentity.ID][skipchainSID].(*Service)

		sb, err := s.GetUpdateChain(&GetUpdateChain{LatestID: sbInter.Hash})

		log.ErrFatal(err)
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
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	maxlevel := 3
	if testing.Short() {
		maxlevel = 2
	}
	servers, ro, genService := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
	}
	service := genService.(*Service)

	for base := 1; base <= maxlevel; base++ {
		for maxHeight := 1; maxHeight <= base; maxHeight++ {
			if base == 1 && maxHeight > 1 {
				break
			}

			log.Lvl1("Making genesis for", base, maxHeight)
			sbRoot, err := makeGenesisRosterArgs(service, ro, nil, VerificationNone,
				base, maxHeight)
			log.ErrFatal(err)
			latest := sbRoot
			log.Lvl1("Adding blocks for", base, maxHeight)
			for sbi := 1; sbi < 10; sbi++ {
				log.Lvl3("Adding block", sbi)
				sb := NewSkipBlock()
				sb.Roster = ro
				psbr, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: latest.Hash, NewBlock: sb})
				log.ErrFatal(err)
				latest = psbr.Latest
				for n, i := range sb.BackLinkIDs {
					for ns, s := range services {
						for {
							log.Lvl3("Checking backlink", n, ns)
							bl, err := s.GetSingleBlock(&GetSingleBlock{i})
							log.ErrFatal(err)
							if len(bl.ForwardLink) == n+1 &&
								bl.ForwardLink[n].To.Equal(sb.Hash) {
								break
							}
						}
					}
				}
			}

			log.ErrFatal(checkMLForwardBackward(service, sbRoot, base, maxHeight))
			log.ErrFatal(checkMLUpdate(service, sbRoot, latest, base, maxHeight))
		}
	}
}

func TestService_Verification(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	sbLength := 4
	_, el, genService := local.MakeSRS(cothority.Suite, sbLength, skipchainSID)
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
	_, err = makeGenesisRosterArgs(service, elSub, sbRoot.Hash, sb.VerifierIDs, 1, 1)
	log.ErrFatal(err)
}

func TestService_SignBlock(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, el, genService := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	service := genService.(*Service)

	sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerificationNone, 1, 1)
	log.ErrFatal(err)
	el2 := onet.NewRoster(el.List[0:2])
	sb := NewSkipBlock()
	sb.Roster = el2
	reply, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
	log.ErrFatal(err)
	sbRoot = reply.Previous
	sbSecond := reply.Latest
	log.Lvl1("Verifying signatures")
	log.ErrFatal(sbRoot.VerifyForwardSignatures())
	log.ErrFatal(sbSecond.VerifyForwardSignatures())
}

func TestService_ProtocolVerification(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, el, s := local.MakeSRS(cothority.Suite, 3, skipchainSID)
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
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sbNext})
	log.ErrFatal(err)
	for i := 0; i < 3; i++ {
		select {
		case <-count:
		case <-time.After(time.Second):
			t.Fatal("Timeout while waiting for reply", i)
		}
	}
}

func TestService_ProtocolVerificationPanic(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, el, s := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	s1 := s.(*Service)
	verifyFunc := func(newID []byte, newSB *SkipBlock) bool {
		panic("nope")
	}
	verifyID := VerifierID(uuid.NewV1())
	for _, s := range local.Services {
		s[skipchainSID].(*Service).registerVerification(verifyID, verifyFunc)
	}

	sbRoot, err := makeGenesisRosterArgs(s1, el, nil, []VerifierID{verifyID}, 1, 1)
	require.NoError(t, err)
	sbNext := sbRoot.Copy()
	sbNext.BackLinkIDs = []SkipBlockID{sbRoot.Hash}
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sbNext})
	require.Error(t, err)

	// We expect a failure here, because the verification function is panicing.
	require.Contains(t, err.Error(), "couldn't sign forward-link")
}

func TestService_RegisterVerification(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	onet.RegisterNewService("ServiceVerify", newServiceVerify)
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	hosts, el, s1 := makeHELS(local, 3)
	VerifyTest := VerifierID(uuid.NewV5(uuid.NamespaceURL, "Test1"))
	ver := make(chan bool, 3)
	verifier := func(msg []byte, s *SkipBlock) bool {
		ver <- true
		return true
	}
	for _, h := range hosts {
		s := h.Service(ServiceName).(*Service)
		log.ErrFatal(s.registerVerification(VerifyTest, verifier))
	}
	sb, err := makeGenesisRosterArgs(s1, el, nil, []VerifierID{VerifyTest}, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ver))
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sb.Hash, NewBlock: sb})
	require.Nil(t, err)
	require.Equal(t, 3, len(ver))

	sb, err = makeGenesisRosterArgs(s1, el, nil, []VerifierID{ServiceVerifier}, 1, 1)
	log.ErrFatal(err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ServiceVerifierChan))
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sb.Hash, NewBlock: sb})
	require.Nil(t, err)
	require.Equal(t, 3, len(ServiceVerifierChan))
}

func TestService_StoreSkipBlock2(t *testing.T) {
	nbrHosts := 3
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	hosts, roster, s1 := makeHELS(local, nbrHosts)
	s2 := local.Services[hosts[1].ServerIdentity.ID][skipchainSID].(*Service)
	s3 := local.Services[hosts[2].ServerIdentity.ID][skipchainSID].(*Service)

	var cbMut sync.Mutex
	var cbCtr int
	cb := func(sbID SkipBlockID) error {
		cbMut.Lock()
		cbCtr++
		cbMut.Unlock()
		return nil
	}
	s1.RegisterStoreSkipblockCallback(cb)
	s2.RegisterStoreSkipblockCallback(cb)
	s3.RegisterStoreSkipblockCallback(cb)

	log.Lvl1("Creating root and control chain")
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	ssbr, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sbRoot})
	require.NoError(t, err)
	require.Equal(t, nbrHosts, cbCtr)
	roster2 := onet.NewRoster(roster.List[:nbrHosts-1])
	log.Lvl1("Proposing roster", roster2)
	sb1 := ssbr.Latest.Copy()
	sb1.Roster = roster2
	ssbr, err = s2.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb1})
	require.NotNil(t, err)
	log.Lvl1("Correctly proposing new roster")
	ssbr, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb1})
	require.Nil(t, err)
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
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sbErr})
	require.NotNil(t, err)
	log.Lvl1("Trying to add to non-existing skipchain")
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbErr.ParentBlockID, NewBlock: sbErr})
	// Last successful log...
	require.NotNil(t, err)

	log.Lvl1("Adding to existing skipchain")
	sbErr = ssbr.Latest.Copy()
	_, err = s3.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbr.Latest.Hash, NewBlock: sbErr})
	require.NotNil(t, err)
}

func TestService_StoreSkipBlockSpeed(t *testing.T) {
	t.Skip("This is a hidden benchmark")
	nbrHosts := 3
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
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
	ssbrep, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sbRoot})
	log.ErrFatal(err)

	last := time.Now()
	for i := 0; i < 500; i++ {
		now := time.Now()
		log.Lvl3(i, now.Sub(last))
		last = now
		ssbrep, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock: sbRoot})
		log.ErrFatal(err)
	}
}

func TestService_ParallelGUC(t *testing.T) {
	nbrRoutines := 10
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, roster, s1 := makeHELS(local, 3)
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	ssbrep, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sbRoot})
	log.ErrFatal(err)

	wg := &sync.WaitGroup{}
	wg.Add(nbrRoutines)
	for i := 0; i < nbrRoutines; i++ {
		go func(i int, latest *SkipBlock) {
			cl := NewClient()
			block := sbRoot.Copy()
			for {
				_, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: latest.Hash, NewBlock: block})
				if err == nil {
					log.Lvl1("Done with", i)
					wg.Done()
					break
				}
				for {
					time.Sleep(10 * time.Millisecond)
					update, err := cl.GetUpdateChain(latest.Roster, latest.Hash)
					if err == nil {
						latest = update.Update[len(update.Update)-1]
						break
					}
				}
			}
		}(i, ssbrep.Latest.Copy())
	}
	wg.Wait()
}

func TestService_ParallelGenesis(t *testing.T) {
	nbrRoutines := 10
	numBlocks := 50

	if testing.Short() {
		nbrRoutines = 5
		numBlocks = 10
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, roster, s1 := makeHELS(local, 5)
	sb0 := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}

	stored := make(chan bool)
	for i := 0; i < nbrRoutines; i++ {
		go func(sb *SkipBlock) {
			for j := 0; j < numBlocks; j++ {
				_, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sb})
				require.Nil(t, err)
				stored <- true
			}
		}(sb0.Copy())
	}

	for i := 0; i < nbrRoutines*numBlocks; i++ {
		<-stored
		log.Lvl2("Stored", i+1, "out of", nbrRoutines*numBlocks, "blocks")
	}

	n := s1.db.Length()
	if n != numBlocks*nbrRoutines {
		t.Error("num blocks is wrong:", n)
	}
}

// Checks that the propagation (genesis, FL, proof) is done correctly
func TestService_Propagation(t *testing.T) {
	nbrNodes := 60
	if testing.Short() {
		nbrNodes = 20
	}
	local := onet.NewLocalTest(cothority.Suite)

	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, fullRoster, genService := local.MakeSRS(cothority.Suite, nbrNodes, skipchainSID)
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
		services[i].propTimeout = 60 * time.Second
	}
	service := genService.(*Service)

	firstFiveRoster := onet.NewRoster(fullRoster.List[:5])
	sbRoot, err := makeGenesisRosterArgs(service, firstFiveRoster, nil, VerificationNone,
		3, 3)
	log.ErrFatal(err)
	require.NotNil(t, sbRoot)

	k := 9
	blocks := make([]*SkipBlock, k)
	for i := 0; i < k; i++ {
		sb := NewSkipBlock()
		if i < k-1 {
			sb.Roster = firstFiveRoster
		} else {
			sb.Roster = fullRoster
		}

		ssbr, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		log.ErrFatal(err)

		blocks[i] = ssbr.Latest
	}

	// Check that any conode can go from the genesis to the last
	// block using the highest forward link level available
	for _, s := range services {
		sb := s.db.GetByID(sbRoot.Hash)
		require.NotNil(t, sb)
		for len(sb.ForwardLink) > 0 {
			sb = s.db.GetByID(sb.ForwardLink[len(sb.ForwardLink)-1].To)
		}

		require.Equal(t, sb.Hash, blocks[k-1].Hash)

		// and backwards
		sb = s.db.GetByID(blocks[k-1].Hash)
		require.Equal(t, 3, len(sb.BackLinkIDs))
		require.Equal(t, 0, len(sb.ForwardLink))
		require.Equal(t, sbRoot.Hash, sb.BackLinkIDs[2])
	}
}

func createSkipchain(service *Service, ro *onet.Roster) (Proof, error) {
	sbRoot, err := makeGenesisRosterArgs(service, ro, nil, VerificationNone, 1, 1)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 3; i++ {
		sb := NewSkipBlock()
		sb.Roster = ro
		sb.Index = i + 1
		_, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		if err != nil {
			return nil, err
		}
	}

	blocks, err := service.db.GetProof(sbRoot.SkipChainID())
	return Proof(blocks), err
}

// Checks a forged message from a evil conode cannot add
// blocks to an existing skipchain
func TestService_ForgedPropagationMessage(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, roster, s := local.MakeSRS(cothority.Suite, 10, skipchainSID)

	service := s.(*Service)
	ro := onet.NewRoster(roster.List[:5])

	p1, err := createSkipchain(service, ro)
	require.Nil(t, err)

	p2, err := createSkipchain(service, ro)
	require.Nil(t, err)

	// Use an unknown member of the roster
	service = servers[5].GetService(ServiceName).(*Service)

	err = service.propagateProofHandler([]byte{})
	require.NotNil(t, err)

	// checks that it could propagate something, this one is correct
	err = service.propagateProofHandler(&PropagateProof{p1})
	require.Nil(t, err)

	// checks that the block cannot be tempered
	p1[2].Data = []byte{1}
	err = service.propagateProofHandler(&PropagateProof{p1})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Wrong hash")

	// checks that the targets have to match
	err = service.propagateProofHandler(&PropagateProof{append(p1[:2], p2[2], p1[3])})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "Wrong targets")

	// checks that the signature must match
	forgedBlock := NewSkipBlock()
	forgedBlock.BackLinkIDs = []SkipBlockID{p1[3].Hash}
	forgedBlock.updateHash()
	p1[3].ForwardLink = []*ForwardLink{&ForwardLink{From: p1[3].Hash, To: forgedBlock.Hash}}
	err = service.propagateProofHandler(&PropagateProof{Proof(append(p1, forgedBlock))})
	require.NotNil(t, err)
}

func TestService_AddFollow(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, ro, _ := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
		services[i].Storage.Clients = []kyber.Point{services[i].ServerIdentity().Public}
	}
	service := services[0]
	sb := NewSkipBlock()
	sb.Roster = onet.NewRoster([]*network.ServerIdentity{ro.List[0]}) // only one in roster
	sb.MaximumHeight = 2
	sb.BaseHeight = 2
	sb.Data = []byte{}
	sb.VerifierIDs = []VerifierID{VerifyBase}
	ssb := &StoreSkipBlock{TargetSkipChainID: []byte{}, NewBlock: sb, Signature: nil}

	_, err := service.StoreSkipBlock(ssb)
	require.NotNil(t, err)

	// Wrong server signature
	priv0 := local.GetPrivate(servers[0])
	priv1 := local.GetPrivate(servers[1])
	sig, err := schnorr.Sign(cothority.Suite, priv1, ssb.NewBlock.CalculateHash())
	log.ErrFatal(err)
	ssb.Signature = &sig
	_, err = service.StoreSkipBlock(ssb)
	require.NotNil(t, err)

	// Correct server signature
	log.Lvl2("correct server signature")
	sig, err = schnorr.Sign(cothority.Suite, priv0, ssb.NewBlock.CalculateHash())
	log.ErrFatal(err)
	ssb.Signature = &sig
	master0, err := service.StoreSkipBlock(ssb)
	log.ErrFatal(err)

	// Not fully authenticated roster
	log.Lvl2("2nd roster is not registered")
	services[1].Storage.FollowIDs = []SkipBlockID{[]byte{0}}
	ssb.TargetSkipChainID = master0.Latest.Hash
	sb = sb.Copy()
	ssb.NewBlock = sb
	sb.Roster = onet.NewRoster([]*network.ServerIdentity{ro.List[0], ro.List[1]}) // two in roster
	sig, err = schnorr.Sign(cothority.Suite, priv0, ssb.NewBlock.CalculateHash())
	log.ErrFatal(err)
	ssb.Signature = &sig
	require.Equal(t, 0, services[1].db.Length())
	_, err = service.StoreSkipBlock(ssb)
	require.NotNil(t, err)
	require.Equal(t, 0, services[1].db.Length())

	// make other services follow skipchain
	log.Lvl2("correct 2 node signing")
	services[1].Storage.Follow = []FollowChainType{{
		Block:    master0.Latest,
		NewChain: NewChainAnyNode,
		closing:  make(chan bool),
	}}
	sig, err = schnorr.Sign(cothority.Suite, priv0, ssb.NewBlock.CalculateHash())
	log.ErrFatal(err)
	ssb.Signature = &sig
	master1, err := service.StoreSkipBlock(ssb)
	log.ErrFatal(err)

	// update skipblock and follow the skipblock
	log.Lvl2("3 node signing with block update")
	services[2].Storage.Follow = []FollowChainType{{
		Block:    master0.Latest,
		NewChain: NewChainAnyNode,
		closing:  make(chan bool),
	}}
	sb = sb.Copy()
	sb.Roster = onet.NewRoster([]*network.ServerIdentity{ro.List[1], ro.List[0], ro.List[2]})
	sb.Hash = sb.CalculateHash()
	ssb.NewBlock = sb
	ssb.TargetSkipChainID = master1.Latest.Hash
	sig, err = schnorr.Sign(cothority.Suite, priv1, ssb.NewBlock.CalculateHash())
	log.ErrFatal(err)
	ssb.Signature = &sig
	sbs, err := service.db.getAll()
	log.ErrFatal(err)
	for _, sb := range sbs {
		services[1].db.Store(sb)
	}
	master2, err := services[1].StoreSkipBlock(ssb)
	log.ErrFatal(err)
	require.True(t, services[1].db.GetByID(master1.Latest.Hash).ForwardLink[0].To.Equal(master2.Latest.Hash))
}

func TestService_CreateLinkPrivate(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, _, _ := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	server := servers[0]
	service := local.GetServices(servers, skipchainSID)[0].(*Service)
	require.Equal(t, 0, len(service.Storage.Clients))
	links, err := service.Listlink(&Listlink{})
	log.ErrFatal(err)
	require.Equal(t, 0, len(links.Publics))
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: servers[0].ServerIdentity.Public, Signature: []byte{}})
	require.NotNil(t, err)
	msg, err := server.ServerIdentity.Public.MarshalBinary()
	require.Nil(t, err)
	sig, err := schnorr.Sign(cothority.Suite, local.GetPrivate(servers[0]), msg)
	log.ErrFatal(err)
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: servers[0].ServerIdentity.Public, Signature: sig})
	log.ErrFatal(err)

	links, err = service.Listlink(&Listlink{})
	log.ErrFatal(err)
	require.Equal(t, 1, len(links.Publics))
}

func TestService_Unlink(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, _, _ := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	server := servers[0]
	service := local.GetServices(servers, skipchainSID)[0].(*Service)

	kp := key.NewKeyPair(cothority.Suite)
	msg, _ := kp.Public.MarshalBinary()
	sig, err := schnorr.Sign(cothority.Suite, local.GetPrivate(servers[0]), msg)
	log.ErrFatal(err)
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: kp.Public, Signature: sig})
	log.ErrFatal(err)
	require.Equal(t, 1, len(service.Storage.Clients))

	// Wrong signature and wrong public key
	_, err = service.Unlink(&Unlink{
		Public:    servers[0].ServerIdentity.Public,
		Signature: sig,
	})
	require.NotNil(t, err)
	require.Equal(t, 1, len(service.Storage.Clients))

	// Inexistant public key
	msg, _ = server.ServerIdentity.Public.MarshalBinary()
	msg = append([]byte("unlink:"), msg...)
	sig, err = schnorr.Sign(cothority.Suite, local.GetPrivate(servers[0]), msg)
	_, err = service.Unlink(&Unlink{
		Public:    servers[0].ServerIdentity.Public,
		Signature: sig,
	})
	require.NotNil(t, err)
	require.Equal(t, 1, len(service.Storage.Clients))

	// Wrong signature
	msg, _ = kp.Public.MarshalBinary()
	msg = append([]byte("unlink:"), msg...)
	sig, err = schnorr.Sign(cothority.Suite, local.GetPrivate(servers[0]), msg)
	_, err = service.Unlink(&Unlink{
		Public:    kp.Public,
		Signature: sig,
	})
	require.NotNil(t, err)
	require.Equal(t, 1, len(service.Storage.Clients))

	// Correct signautre and existing public key
	msg, _ = kp.Public.MarshalBinary()
	msg = append([]byte("unlink:"), msg...)
	sig, err = schnorr.Sign(cothority.Suite, kp.Private, msg)
	require.Nil(t, err)
	_, err = service.Unlink(&Unlink{
		Public:    kp.Public,
		Signature: sig,
	})
	require.Nil(t, err)
	require.Equal(t, 0, len(service.Storage.Clients))
}

func TestService_DelFollow(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, _, _ := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	service := local.GetServices(servers, skipchainSID)[0].(*Service)

	privWrong := key.NewKeyPair(cothority.Suite).Private
	priv := setupFollow(service)
	iddel := []byte{0}
	msg := append([]byte("delfollow:"), iddel...)

	// Test wrong signature
	sig, err := schnorr.Sign(cothority.Suite, privWrong, msg)
	log.ErrFatal(err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.NotNil(t, err)
	require.Equal(t, 2, len(service.Storage.FollowIDs))

	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	log.ErrFatal(err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.Nil(t, err)
	require.Equal(t, 1, len(service.Storage.FollowIDs))

	// Test removal of Follow
	iddel = []byte{2}
	msg = append([]byte("delfollow:"), iddel...)
	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	log.ErrFatal(err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.Nil(t, err)
	require.Equal(t, 1, len(service.Storage.Follow))
}

func TestService_ListFollow(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	servers, _, _ := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	service := local.GetServices(servers, skipchainSID)[0].(*Service)

	priv := setupFollow(service)

	// Check wrong signature
	msg, err := servers[1].ServerIdentity.Public.MarshalBinary()
	log.ErrFatal(err)
	msg = append([]byte("listfollow:"), msg...)
	sig, err := schnorr.Sign(cothority.Suite, priv, msg)
	log.ErrFatal(err)
	lf, err := service.ListFollow(&ListFollow{Signature: sig})
	require.NotNil(t, err)

	msg, err = servers[0].ServerIdentity.Public.MarshalBinary()
	log.ErrFatal(err)
	msg = append([]byte("listfollow:"), msg...)
	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	log.ErrFatal(err)
	lf, err = service.ListFollow(&ListFollow{Signature: sig})
	require.Nil(t, err)
	require.Equal(t, 2, len(*lf.Follow))
	require.Equal(t, 2, len(*lf.FollowIDs))
}

func TestService_MissingForwardlink(t *testing.T) {
	// Tests how a missing forward link is handled by the system
	// by 'Pause()' the leader of the genesis-block for one forwardlink
	// and 'Unpause()' it for the next forwardlink.
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	servers, ro, _ := local.MakeSRS(cothority.Suite, 8, skipchainSID)
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
		services[i].propTimeout = 4 * time.Second
	}
	service1 := services[0]
	service2 := services[2]
	service3 := services[4]
	ro1 := onet.NewRoster(ro.List[0:4])
	ro2 := onet.NewRoster(ro.List[2:6])
	ro3 := onet.NewRoster(ro.List[4:8])

	log.Lvl1("Making genesis-block with list", ro1.List)
	sbRoot, err := makeGenesisRosterArgs(service1, ro1, nil, VerificationNone,
		2, 4)
	require.Nil(t, err)
	scid := sbRoot.SkipChainID()

	log.Lvl1("Adding block #1 with list", ro2.List)
	sb := NewSkipBlock()
	sb.Roster = ro2
	_, err = addBlockToChain(service2, scid, sb)
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	log.Lvl1("Adding block #2 while node0 is down with list", ro3.List)
	servers[0].Pause()
	sb = NewSkipBlock()
	sb.Roster = ro3
	_, err = addBlockToChain(service3, scid, sb)
	require.Nil(t, err)

	log.Lvl1("Adding block #3 while node0 is up again with list", ro3.List)
	servers[0].Unpause()
	_, err = addBlockToChain(service3, scid, sb)

	log.Lvl1("Adding block #4 while node0 is up again with list", ro3.List)
	_, err = addBlockToChain(service3, scid, sb)
	require.Nil(t, waitForwardLinks(service1, sbRoot, 3))
}

func TestService_LeaderChange(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	servers, ro, service := local.MakeSRS(cothority.Suite, 8, skipchainSID)
	leader := service.(*Service)
	sbRoot, err := makeGenesisRosterArgs(leader, ro, nil, VerificationNone, 2, 4)

	require.Nil(t, err)

	servers[0].Pause()
	leader = local.GetServices(servers, skipchainSID)[2].(*Service)

	sb := NewSkipBlock()
	sb.Roster = onet.NewRoster(append(ro.List[2:], ro.List[0], ro.List[1]))
	_, err = addBlockToChain(leader, sbRoot.Hash, sb)
	require.Nil(t, err)

	res, err := leader.getBlocks(sb.Roster, sbRoot.Hash, 2)
	require.Nil(t, err)
	require.Equal(t, 1, len(res[0].ForwardLink))
	// Forward link must be verified with the src block
	require.Nil(t, res[0].ForwardLink[0].Verify(suite, ro.ServicePublics(ServiceName)))
}

func addBlockToChain(s *Service, scid SkipBlockID, sb *SkipBlock) (latest *SkipBlock, err error) {
	reply, err := s.StoreSkipBlock(
		&StoreSkipBlock{TargetSkipChainID: scid,
			NewBlock: sb,
		})
	if err != nil {
		return nil, err
	}
	return reply.Latest, nil
}

func waitForwardLinks(s *Service, sb *SkipBlock, num int) error {
	var count int
	for len(sb.ForwardLink) < num && count < 10 {
		log.Lvlf3("%x: %d", sb.Hash, len(sb.ForwardLink))
		time.Sleep(100 * time.Millisecond)
		var err error
		sb, err = s.GetSingleBlock(&GetSingleBlock{sb.Hash})
		if err != nil {
			return err
		}
		count++
	}
	if count >= 10 {
		return errors.New("Didn't get forwardlinks in time")
	}
	return nil
}

func setupFollow(s *Service) kyber.Scalar {
	kp := key.NewKeyPair(cothority.Suite)
	s.Storage.Clients = []kyber.Point{kp.Public}
	s.Storage.FollowIDs = []SkipBlockID{{0}, {1}}
	s.Storage.Follow = []FollowChainType{
		{
			Block:   &SkipBlock{SkipBlockFix: &SkipBlockFix{Index: 0, Data: []byte{}}, Hash: []byte{2}},
			closing: make(chan bool),
		},
		{
			Block:   &SkipBlock{SkipBlockFix: &SkipBlockFix{Index: 0, Data: []byte{}}, Hash: []byte{3}},
			closing: make(chan bool),
		},
	}
	return kp.Private
}

func checkMLForwardBackward(service *Service, root *SkipBlock, base, height int) error {
	genesis := service.db.GetByID(root.Hash)
	if genesis == nil {
		return errors.New("didn't find genesis-block in service")
	}
	if len(genesis.ForwardLink) != height {
		return errors.New("genesis-block doesn't have forward-links of " +
			strconv.Itoa(height))
	}
	return nil
}

func checkMLUpdate(service *Service, root, latest *SkipBlock, base, maxHeight int) error {
	log.Lvl3("Checking ML update for:", service, root, latest, base, maxHeight)
	for updateMaxHeight := 0; updateMaxHeight <= maxHeight; updateMaxHeight++ {
		log.Lvl1("Checking update for maxHeight", maxHeight, "and updateMaxHeight", updateMaxHeight)
		chain, err := service.GetUpdateChain(&GetUpdateChain{
			LatestID:  root.Hash,
			MaxHeight: updateMaxHeight,
		})

		// Also test the fact that GetUpdateChain.MaxHeight == 0 means to use the
		// block.MaxHeight.
		blockMaxHeight := updateMaxHeight
		if blockMaxHeight == 0 {
			blockMaxHeight = maxHeight
		}

		if err != nil {
			return err
		}
		updates := chain.Update
		genesis := updates[0]
		if len(genesis.ForwardLink) != maxHeight {
			return errors.New("genesis-block doesn't have height " + strconv.Itoa(maxHeight))
		}
		if len(updates[1].BackLinkIDs) != blockMaxHeight {
			return errors.New("Second block doesn't have correct number of backlinks")
		}
		l := updates[len(updates)-1]
		if len(l.ForwardLink) != 0 {
			return errors.New("Last block still has forward-links")
		}
		if !l.Equal(latest) {
			return errors.New("Last block from update is not the same as last block")
		}
		log.Lvl2("Checking base, blockMaxHeight, len(udpates):", base, blockMaxHeight, len(updates))
		if base > 1 && blockMaxHeight > 1 && len(updates) == 10 {
			return fmt.Errorf("Shouldn't need 10 blocks with base %d and height %d",
				base, maxHeight)
		}
	}

	// Verify we get the correct number of blocks.
	for maxHe := 0; maxHe <= maxHeight; maxHe++ {
		for maxBl := 0; maxBl < 12; maxBl++ {
			updates, err := service.GetUpdateChain(&GetUpdateChain{
				LatestID:  root.Hash,
				MaxHeight: maxHe,
				MaxBlocks: maxBl,
			})
			if err != nil {
				return err
			}

			// maxHeight == 0 means to use the maximum height available
			mh := maxHe
			if mh == 0 {
				mh = maxHeight
			}

			// Calculate the number of blocks, which is a bit tricky, as the number
			// depends on the maximum forward links.
			blocks := int(math.Ceil(9./math.Pow(float64(base), float64(mh-1)))) + 1
			if maxBl > 0 && blocks > maxBl {
				blocks = maxBl
			}
			if blocks > 10 {
				blocks = 10
			}
			log.Lvlf3("base(%2d), mh(%2d), maxBl(%2d), blocks(%2d), len(updates.Update)=%2d",
				base, mh, maxBl, blocks, len(updates.Update))
			if len(updates.Update) != blocks {
				return fmt.Errorf("Should have %d blocks, but got %d", blocks, len(updates.Update))
			}
		}
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

func newServiceVerify(c *onet.Context) (onet.Service, error) {
	sv := &ServiceVerify{}
	log.ErrFatal(RegisterVerification(c, ServiceVerifier, sv.Verify))
	return sv, nil
}

// makes a genesis Roster-block
func makeGenesisRosterArgs(s *Service, el *onet.Roster, parent SkipBlockID,
	vid []VerifierID, base, maxHeight int) (*SkipBlock, error) {
	sb := NewSkipBlock()
	sb.Roster = el
	sb.MaximumHeight = maxHeight
	sb.BaseHeight = base
	sb.ParentBlockID = parent
	sb.VerifierIDs = vid
	psbr, err := s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: []byte{}, NewBlock: sb})
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

func waitPropagationFinished(t *testing.T, local *onet.LocalTest) {
	var servers []*onet.Server
	for _, s := range local.Servers {
		servers = append(servers, s)
	}
	services := make([]*Service, len(servers))
	for i, s := range local.GetServices(servers, skipchainSID) {
		services[i] = s.(*Service)
	}
	propagating := true
	for propagating {
		propagating = false
		for _, s := range services {
			if s.chains.numLocks() != 0 {
				log.Lvl1("Service", s, "is still locking chains")
				propagating = true
			}
		}
		if propagating {
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func TestService_LeaderCatchup(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()

	hosts := local.GenServers(3)
	roster := local.GenRosterFromHost(hosts...)
	leader := local.Services[hosts[0].ServerIdentity.ID][skipchainSID].(*Service)
	follower := local.Services[hosts[1].ServerIdentity.ID][skipchainSID].(*Service)

	log.Lvl1("Creating root and control chain")
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 2,
			BaseHeight:    3,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	ssbrep, err := leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: []byte{}, NewBlock: sbRoot})
	require.Nil(t, err)

	blocks := make([]*SkipBlock, 10)
	for i := 0; i < 10; i++ {
		ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock: sbRoot.Copy()})
		require.Nil(t, err)
		blocks[i] = ssbrep.Latest
	}

	// At this point, both servers have all blocks. Now remove blocks from
	// the leader's DB starting at the third one to simulate the situation where the leader
	// boots with an old backup.
	nukeBlocksFrom(t, leader.db, blocks[3].Hash)

	// Write one more onto the leader: it will need to sync it's chain in order
	// to handle this write.
	ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
		NewBlock: sbRoot})
	require.Nil(t, err)

	sb11 := leader.db.GetByID(ssbrep.Latest.Hash)
	require.Equal(t, sb11.Index, 11)

	// Simulate follower old backup.
	nukeBlocksFrom(t, follower.db, blocks[3].Hash)

	// Write onto leader; the follower will need to sync to be able to sign this.
	ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
		NewBlock: sbRoot})
	require.Nil(t, err)

	// Wait for forward-block propagation
	require.Nil(t, waitForwardLinks(leader, blocks[8], 2))
}

func nukeBlocksFrom(t *testing.T, db *SkipBlockDB, where SkipBlockID) {
	for {
		// Get to find forward links.
		sb := db.GetByID(where)
		if sb == nil {
			return
		}

		// nuke it
		log.Lvl2("nuking block", sb.Index)
		err := db.Update(func(tx *bolt.Tx) error {
			err := tx.Bucket([]byte(db.bucketName)).Delete(where)
			if err != nil {
				log.Fatal("delete error", err)
			}
			return err
		})
		if err != nil {
			log.Fatal("update error", err)
		}

		// Go to next one
		if len(sb.ForwardLink) == 0 {
			return
		}
		where = sb.ForwardLink[0].To
	}
}

func TestRosterAddCausesSync(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, _, genService := local.MakeSRS(cothority.Suite, 5, skipchainSID)
	leader := genService.(*Service)

	for base := 2; base < 4; base++ {

		// put last one to sleep, wake it up after the others have added it into the roster
		servers[4].Pause()

		log.Lvl1("Creating chain with 4 servers and base", base)
		sbRoot := &SkipBlock{
			SkipBlockFix: &SkipBlockFix{
				MaximumHeight: 2,
				BaseHeight:    base,
				Roster:        local.GenRosterFromHost(servers[0:4]...),
				Data:          []byte{},
			},
		}
		ssbrep, err := leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: []byte{}, NewBlock: sbRoot})
		require.Nil(t, err)

		log.Lvl1("Add last server into roster")
		newBlock := &SkipBlock{
			SkipBlockFix: &SkipBlockFix{
				Roster: local.GenRosterFromHost(servers...),
				Data:   []byte{},
			},
		}
		ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{
			TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock:          newBlock})
		require.Nil(t, err)
		require.Nil(t, local.WaitDone(time.Second))

		// Wake up #4. It does not know any blocks yet.
		log.Lvl1("Wake up last server")
		servers[4].Unpause()

		// Add a new block. #4 will be asked to sign a forward link on a block
		// it has never heard of, so it will need to sync.
		ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{
			TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock:          newBlock})
		require.Nil(t, err)
		log.Lvl1("Got block with servers[4]")
	}
}
