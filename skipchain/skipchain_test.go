package skipchain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	bbolt "go.etcd.io/bbolt"
	uuid "gopkg.in/satori/go.uuid.v1"
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
	require.NoError(t, err)
	// make sure the correct default signature scheme is set to BDN
	assert.Equal(t, BdnSignatureSchemeIndex, sbRoot.SignatureScheme)

	// send a ProposeBlock
	genesis := NewSkipBlock()
	genesis.Data = []byte("In the beginning God created the heaven and the earth.")
	genesis.MaximumHeight = 2
	genesis.BaseHeight = 2
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
	assert.Equal(t, BdnSignatureSchemeIndex, latest.SignatureScheme)
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

	// Get the forward link of the genesis to test that forward links are correctly
	// ignored
	genesis, err = service.GetSingleBlock(&GetSingleBlock{ID: genesis.Hash})
	require.NoError(t, err)
	require.Equal(t, 1, len(genesis.ForwardLink))

	next.Data = []byte("And the Spirit of God moved upon the face of the waters.")
	next.ForwardLink = genesis.ForwardLink
	psbr3, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: psbr2.Latest.Hash, NewBlock: next})
	require.NoError(t, err)
	assert.NotNil(t, psbr3)
	assert.NotNil(t, psbr3.Latest)
	latest3 := psbr3.Latest

	// As the propagation of the last forward link might take some time, wait for
	// it to be propagated by checking whether the updateChain has the new forward
	// link included.
	for {
		gucr, err := service.GetUpdateChain(&GetUpdateChain{LatestID: genesis.Hash})
		require.NoError(t, err)
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

func TestService_StoreCorruptedSkipBlock(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, el, genService := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	service := genService.(*Service)

	sbRoot, err := makeGenesisRoster(service, el)
	require.NoError(t, err)

	genesis := NewSkipBlock()
	genesis.Data = []byte("In the beginning God created the heaven and the earth.")
	genesis.MaximumHeight = 2
	genesis.BaseHeight = 2
	genesis.Roster = sbRoot.Roster
	genesis.VerifierIDs = VerificationStandard
	psbr, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: genesis})
	require.NoError(t, err)

	// bypass StoreSkipBlock to simulate a malicious node
	csb := NewSkipBlock()
	csb.Roster = el
	csb.VerifierIDs = VerificationStandard
	csb.BackLinkIDs = []SkipBlockID{psbr.Latest.Hash}
	csb.Index = 1
	csb.MaximumHeight = 2
	csb.BaseHeight = 2
	csb.Height = 1
	csb.SignatureScheme = 0

	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.Error(t, err)

	csb.GenesisID = psbr.Latest.Hash
	csb.Index = 2
	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.Error(t, err)

	csb.Index = 1
	csb.MaximumHeight = 42
	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.Error(t, err)

	csb.MaximumHeight = 2
	csb.BaseHeight = 42
	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.Error(t, err)

	csb.BaseHeight = 2
	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.Error(t, err)

	csb.SignatureScheme = 1
	err = service.forwardLinkLevel0(psbr.Latest, csb)
	require.NoError(t, err)
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
			require.NoError(t, err)
			latest := sbRoot
			log.Lvl1("Adding blocks for", base, maxHeight)
			for sbi := 1; sbi < 10; sbi++ {
				log.Lvl3("Adding block", sbi)
				sb := NewSkipBlock()
				sb.Roster = ro
				psbr, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: latest.Hash, NewBlock: sb})
				require.NoError(t, err)
				latest = psbr.Latest
				for n, i := range sb.BackLinkIDs {
					for ns, s := range services {
						for {
							log.Lvl3("Checking backlink", n, ns)
							bl, err := s.GetSingleBlock(&GetSingleBlock{i})
							require.NoError(t, err)
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
	require.NoError(t, err)

	log.Lvl1("Creating non-conforming skipBlock")
	sb := NewSkipBlock()
	sb.Roster = el
	sb.MaximumHeight = 2
	sb.BaseHeight = 1
	sb.VerifierIDs = VerificationStandard
	_, err = service.StoreSkipBlockInternal(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sb})
	require.Error(t, err)
	require.Equal(t, "Set maximumHeight to 1 when baseHeight is 1", err.Error())

	sb.BaseHeight = 0
	_, err = service.StoreSkipBlockInternal(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sb})
	require.Error(t, err)
	require.Equal(t, "Set a baseHeight > 0", err.Error())

	sb.MaximumHeight = 0
	_, err = service.StoreSkipBlockInternal(&StoreSkipBlock{TargetSkipChainID: nil, NewBlock: sb})
	require.Error(t, err)
	require.Equal(t, "Set a maximumHeight > 0", err.Error())

	log.Lvl1("Creating skipblock with same Roster as root")
	sbInter, err := makeGenesisRosterArgs(service, elRoot, sbRoot.Hash, sb.VerifierIDs, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, sbInter)
	log.Lvl1("Creating skipblock with sub-Roster from root")
	elSub := onet.NewRoster(el.List[0:2])
	_, err = makeGenesisRosterArgs(service, elSub, sbRoot.Hash, sb.VerifierIDs, 1, 1)
	require.NoError(t, err)
}

func TestService_SignBlock(t *testing.T) {
	// Testing whether we sign correctly the SkipBlocks
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	_, el, genService := local.MakeSRS(cothority.Suite, 3, skipchainSID)
	service := genService.(*Service)

	sbRoot, err := makeGenesisRosterArgs(service, el, nil, VerificationNone, 1, 1)
	require.NoError(t, err)
	el2 := onet.NewRoster(el.List[0:2])
	sb := NewSkipBlock()
	sb.Roster = el2
	reply, err := service.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
	require.NoError(t, err)
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
	require.NoError(t, err)
	sbNext := sbRoot.Copy()
	sbNext.BackLinkIDs = []SkipBlockID{sbRoot.Hash}
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sbNext})
	require.NoError(t, err)
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
	require.NoError(t, err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ver))
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sb.Hash, NewBlock: sb})
	require.NoError(t, err)
	require.Equal(t, 3, len(ver))

	sb, err = makeGenesisRosterArgs(s1, el, nil, []VerifierID{ServiceVerifier}, 1, 1)
	require.NoError(t, err)
	require.NotNil(t, sb.Data)
	require.Equal(t, 0, len(ServiceVerifierChan))
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sb.Hash, NewBlock: sb})
	require.NoError(t, err)
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
	_, err = s2.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb1})
	require.Error(t, err)
	log.Lvl1("Correctly proposing new roster")
	ssbr, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb1})
	require.NoError(t, err)
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
	log.Lvl1("Trying to add to non-existing skipchain")
	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: SkipBlockID([]byte{1, 2, 3}), NewBlock: sbErr})
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
	require.NoError(t, err)

	last := time.Now()
	for i := 0; i < 500; i++ {
		now := time.Now()
		log.Lvl3(i, now.Sub(last))
		last = now
		ssbrep, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock: sbRoot})
		require.NoError(t, err)
	}
}

func TestService_GetUpdateChain(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer waitPropagationFinished(t, local)
	defer local.CloseAll()
	hosts, roster, s1 := makeHELS(local, 3)
	roster12 := onet.NewRoster(roster.List[:2])
	services := make([]*Service, 3)
	for i, h := range hosts {
		services[i] = h.GetService(ServiceName).(*Service)
	}
	sbRoot := &SkipBlock{
		SkipBlockFix: &SkipBlockFix{
			MaximumHeight: 1,
			BaseHeight:    1,
			Roster:        roster,
			Data:          []byte{},
		},
	}
	root, err := s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: nil,
		NewBlock: sbRoot})
	require.NoError(t, err)
	scID := root.Latest.SkipChainID()

	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: scID,
		NewBlock: &SkipBlock{SkipBlockFix: &SkipBlockFix{Roster: roster12}}})
	require.NoError(t, err)
	getLengths(t, scID, services, []int{2, 2, 1})

	_, err = s1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: scID,
		NewBlock: &SkipBlock{SkipBlockFix: &SkipBlockFix{Roster: roster}}})
	require.NoError(t, err)
	getLengths(t, scID, services, []int{3, 3, 3})
}

func getLengths(t *testing.T, scID SkipBlockID, services []*Service,
	results []int) {
	require.Equal(t, len(services), len(results))
	for i, s := range services {
		log.Lvl2("Testing service", s.ServerIdentity(), "for", results[i],
			"blocks")
		uc, err := s.GetUpdateChain(&GetUpdateChain{LatestID: scID})
		require.NoError(t, err)
		require.Equal(t, results[i], len(uc.Update))
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
	require.NoError(t, err)

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
				require.NoError(t, err)
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
	require.NoError(t, err)
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
		require.NoError(t, err)

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
	require.NoError(t, err)

	p2, err := createSkipchain(service, ro)
	require.NoError(t, err)

	// Use an unknown member of the roster
	service = servers[5].GetService(ServiceName).(*Service)

	err = service.propagateProofHandler([]byte{})
	require.NotNil(t, err)

	// checks that it could propagate something, this one is correct
	err = service.propagateProofHandler(&PropagateProof{p1})
	require.NoError(t, err)

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
	require.NoError(t, err)
	ssb.Signature = &sig
	_, err = service.StoreSkipBlock(ssb)
	require.NotNil(t, err)

	// Correct server signature
	log.Lvl2("correct server signature")
	sig, err = schnorr.Sign(cothority.Suite, priv0, ssb.NewBlock.CalculateHash())
	require.NoError(t, err)
	ssb.Signature = &sig
	master0, err := service.StoreSkipBlock(ssb)
	require.NoError(t, err)

	// Not fully authenticated roster
	log.Lvl2("2nd roster is not registered")
	services[1].Storage.FollowIDs = []SkipBlockID{[]byte{0}}
	ssb.TargetSkipChainID = master0.Latest.Hash
	sb = sb.Copy()
	ssb.NewBlock = sb
	sb.Roster = onet.NewRoster([]*network.ServerIdentity{ro.List[0], ro.List[1]}) // two in roster
	sig, err = schnorr.Sign(cothority.Suite, priv0, ssb.NewBlock.CalculateHash())
	require.NoError(t, err)
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
	require.NoError(t, err)
	ssb.Signature = &sig
	master1, err := service.StoreSkipBlock(ssb)
	require.NoError(t, err)

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
	require.NoError(t, err)
	ssb.Signature = &sig
	sbs, err := service.db.getAll()
	require.NoError(t, err)
	for _, sb := range sbs {
		services[1].db.Store(sb)
	}
	master2, err := services[1].StoreSkipBlock(ssb)
	require.NoError(t, err)
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
	require.NoError(t, err)
	require.Equal(t, 0, len(links.Publics))
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: servers[0].ServerIdentity.Public, Signature: []byte{}})
	require.NotNil(t, err)
	msg, err := server.ServerIdentity.Public.MarshalBinary()
	require.NoError(t, err)
	sig, err := schnorr.Sign(cothority.Suite, local.GetPrivate(servers[0]), msg)
	require.NoError(t, err)
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: servers[0].ServerIdentity.Public, Signature: sig})
	require.NoError(t, err)

	links, err = service.Listlink(&Listlink{})
	require.NoError(t, err)
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
	require.NoError(t, err)
	_, err = service.CreateLinkPrivate(&CreateLinkPrivate{Public: kp.Public, Signature: sig})
	require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	_, err = service.Unlink(&Unlink{
		Public:    kp.Public,
		Signature: sig,
	})
	require.NoError(t, err)
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
	require.NoError(t, err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.NotNil(t, err)
	require.Equal(t, 2, len(service.Storage.FollowIDs))

	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	require.NoError(t, err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.NoError(t, err)
	require.Equal(t, 1, len(service.Storage.FollowIDs))

	// Test removal of Follow
	iddel = []byte{2}
	msg = append([]byte("delfollow:"), iddel...)
	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	require.NoError(t, err)
	_, err = service.DelFollow(&DelFollow{SkipchainID: iddel, Signature: sig})
	require.NoError(t, err)
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
	require.NoError(t, err)
	msg = append([]byte("listfollow:"), msg...)
	sig, err := schnorr.Sign(cothority.Suite, priv, msg)
	require.NoError(t, err)
	_, err = service.ListFollow(&ListFollow{Signature: sig})
	require.NotNil(t, err)

	msg, err = servers[0].ServerIdentity.Public.MarshalBinary()
	require.NoError(t, err)
	msg = append([]byte("listfollow:"), msg...)
	sig, err = schnorr.Sign(cothority.Suite, priv, msg)
	require.NoError(t, err)
	lf, err := service.ListFollow(&ListFollow{Signature: sig})
	require.NoError(t, err)
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
	require.NoError(t, err)
	scid := sbRoot.SkipChainID()

	log.Lvl1("Adding block #1 with list", ro2.List)
	sb := NewSkipBlock()
	sb.Roster = ro2
	_, err = addBlockToChain(service2, scid, sb)
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	log.Lvl1("Adding block #2 while node0 is down with list", ro3.List)
	servers[0].Pause()
	sb = NewSkipBlock()
	sb.Roster = ro3
	_, err = addBlockToChain(service3, scid, sb)
	require.NoError(t, err)

	log.Lvl1("Adding block #3 while node0 is up again with list", ro3.List)
	servers[0].Unpause()
	_, err = addBlockToChain(service3, scid, sb)
	require.NoError(t, err)

	log.Lvl1("Adding block #4 while node0 is up again with list", ro3.List)
	_, err = addBlockToChain(service3, scid, sb)
	require.NoError(t, err)
	require.Nil(t, waitForwardLinks(service1, sbRoot, 3))
}

func TestService_LeaderChange(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	servers, ro, service := local.MakeSRS(cothority.Suite, 8, skipchainSID)
	leader := service.(*Service)
	sbRoot, err := makeGenesisRosterArgs(leader, ro, nil, VerificationNone, 2, 4)

	require.NoError(t, err)

	servers[0].Pause()
	leader = local.GetServices(servers, skipchainSID)[2].(*Service)

	sb := NewSkipBlock()
	sb.Roster = onet.NewRoster(append(ro.List[2:], ro.List[0], ro.List[1]))
	_, err = addBlockToChain(leader, sbRoot.Hash, sb)
	require.NoError(t, err)

	res, err := leader.getBlocks(sb.Roster, sbRoot.Hash, 2)
	require.NoError(t, err)
	require.Equal(t, 1, len(res[0].ForwardLink))
	// Forward link must be verified with the src block
	require.Nil(t, res[0].ForwardLink[0].VerifyWithScheme(suite, ro.ServicePublics(ServiceName), BdnSignatureSchemeIndex))
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
	require.NoError(t, err)

	blocks := make([]*SkipBlock, 10)
	for i := 0; i < 10; i++ {
		ssbrep, err = leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock: sbRoot.Copy()})
		require.NoError(t, err)
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
	require.NoError(t, err)

	sb11 := leader.db.GetByID(ssbrep.Latest.Hash)
	require.Equal(t, sb11.Index, 11)

	// Simulate follower old backup.
	nukeBlocksFrom(t, follower.db, blocks[3].Hash)

	// Write onto leader; the follower will need to sync to be able to sign this.
	_, err = leader.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: ssbrep.Latest.Hash,
		NewBlock: sbRoot})
	require.NoError(t, err)

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
		err := db.Update(func(tx *bbolt.Tx) error {
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
		require.NoError(t, err)

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
		require.NoError(t, err)
		require.Nil(t, local.WaitDone(time.Second))

		// Wake up #4. It does not know any blocks yet.
		log.Lvl1("Wake up last server")
		servers[4].Unpause()

		// Add a new block. #4 will be asked to sign a forward link on a block
		// it has never heard of, so it will need to sync.
		_, err = leader.StoreSkipBlock(&StoreSkipBlock{
			TargetSkipChainID: ssbrep.Latest.Hash,
			NewBlock:          newBlock})
		require.NoError(t, err)
		log.Lvl1("Got block with servers[4]")
	}
}

func TestService_WaitBlock(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, ro, service := local.MakeSRS(cothority.Suite, 1, skipchainSID)

	s := service.(*Service)

	sb, doCatchUp := s.WaitBlock(SkipBlockID{}, nil)
	require.Nil(t, sb)
	require.True(t, doCatchUp)

	sbRoot, err := makeGenesisRoster(s, ro)
	require.NoError(t, err)

	sb, doCatchUp = s.WaitBlock(sbRoot.Hash, sbRoot.Hash)
	require.NotNil(t, sb)
	require.False(t, doCatchUp)

	sb, doCatchUp = s.WaitBlock(sbRoot.Hash, SkipBlockID{})
	require.Nil(t, sb)
	require.True(t, doCatchUp)

	newSb := NewSkipBlock()
	newSb.Index = 1
	newSb.GenesisID = sbRoot.Hash
	newSb.updateHash()
	s.blockBuffer.add(newSb)
	sb, doCatchUp = s.WaitBlock(sbRoot.Hash, newSb.Hash)
	require.Nil(t, sb)
	require.False(t, doCatchUp)
}

// The test scenario is the following:
// - 4 nodes
// - level 1 link from index 0 to index 2
// - level 1 link from index 2 to index 4
//
// The roster starts with 3 conodes and evolves to 4 to go back to 3 again.
// This is to test to whom the forward link is propagated when there is a
// difference between the source and the distant roster.
func TestService_MissingForwardLinks(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, ro, service := local.MakeSRS(cothority.Suite, 4, skipchainSID)

	s := service.(*Service)

	roWithout4 := onet.NewRoster(ro.List[:3])

	sbRoot, err := makeGenesisRosterArgs(s, roWithout4, nil, VerificationStandard, 2, 2)
	require.NoError(t, err)

	sb := NewSkipBlock()
	sb.Roster = ro
	_, err = s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
	require.NoError(t, err)

	cc := make([]*network.ServerIdentity, 3)
	cc[0] = ro.List[0]
	cc[1] = ro.List[1]
	cc[2] = ro.List[3]

	roWithout3 := onet.NewRoster(cc)

	sb.Roster = roWithout3
	_, err = s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
	require.NoError(t, err)

	sb.Roster = onet.NewRoster(ro.List[:3])
	for i := 0; i < 5; i++ {
		_, err = s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		require.NoError(t, err)
	}

	req := &GetSingleBlockByIndex{Genesis: sbRoot.Hash, Index: 2}
	sbAtIndex2FromC1, err := s.GetSingleBlockByIndex(req)
	require.NoError(t, err)
	// C1 is always in the roster so always has the forward links.
	require.Equal(t, 2, len(sbAtIndex2FromC1.SkipBlock.ForwardLink))

	sbAtIndex2FromC3, err := servers[2].Service(ServiceName).(*Service).GetSingleBlockByIndex(req)
	require.NoError(t, err)
	// C3 is reinserted in the roster so it should get the level 1 forward link.
	require.Equal(t, 2, len(sbAtIndex2FromC3.SkipBlock.ForwardLink))

	sbAtIndex2FromC4, err := servers[3].Service(ServiceName).(*Service).GetSingleBlockByIndex(req)
	require.NoError(t, err)
	// C4 is removed from the roster so it should not get the level 1 forward link
	// as it's not participating to the cothority anymore.
	require.Equal(t, 1, len(sbAtIndex2FromC4.SkipBlock.ForwardLink))

	// When propagating a new block, a proof is provided and it should then update
	// previous forward links.
	sb.Roster = ro
	_, err = s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
	require.NoError(t, err)

	sbAtIndex2FromC4, err = servers[3].Service(ServiceName).(*Service).GetSingleBlockByIndex(req)
	require.NoError(t, err)
	// C4 is re-inserted and should then update the missing forward link with the new block.
	require.Equal(t, 2, len(sbAtIndex2FromC4.SkipBlock.ForwardLink))
}

func TestService_ForwardLinkVerification(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, ro, service := local.MakeSRS(cothority.Suite, 4, skipchainSID)

	s := service.(*Service)

	sbRoot, err := makeGenesisRosterArgs(s, ro, nil, VerificationStandard, 2, 2)
	require.NoError(t, err)

	sb := NewSkipBlock()
	sb.Roster = ro
	sbs := []*SkipBlock{}
	for i := 0; i < 2; i++ {
		r, err := s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		require.NoError(t, err)

		sbs = append(sbs, r.Latest)
	}

	fs := &ForwardSignature{}

	marshal := func(msg *ForwardSignature) []byte {
		buf, err := network.Marshal(msg)
		require.NoError(t, err)
		return buf
	}

	log.OutputToBuf()
	defer log.OutputToOs()

	require.False(t, s.bftForwardLink([]byte{}, []byte{}))
	require.Contains(t, log.GetStdErr(), "EOF")
	require.False(t, s.bftForwardLink([]byte{}, marshal(fs)))
	require.Contains(t, log.GetStdErr(), "newest block does not match its hash")

	fs.Newest = NewSkipBlock()
	fs.Newest.updateHash()
	fs.TargetHeight = 123456789
	require.False(t, s.bftForwardLink([]byte{}, marshal(fs)))
	require.Contains(t, log.GetStdErr(), "unexpected target height")

	fs.Newest = NewSkipBlock()
	fs.Newest.BackLinkIDs = []SkipBlockID{[]byte{1, 2, 3}}
	fs.Newest.updateHash()
	fs.TargetHeight = 0
	require.False(t, s.bftForwardLink([]byte{}, marshal(fs)))
	require.Contains(t, log.GetStdErr(), "don't have src-block")

	fs.Newest = sbs[1]
	fs.Newest.BackLinkIDs[0] = sbs[0].Hash
	fs.Newest.updateHash()
	fs.TargetHeight = 0
	require.False(t, s.bftForwardLink([]byte{}, marshal(fs)))
	require.Contains(t, log.GetStdErr(), "target height does not match")
}

// Test if the optimization works as expected for a normal situation.
func TestService_OptimizeProofSimple(t *testing.T) {
	testOptimizeProof(t, 23, 2, 32, 5)
}

// Test if the optimization works as expected when the maximum height
// has to be used.
func TestService_OptimizeProofMaxHeight(t *testing.T) {
	testOptimizeProof(t, 23, 2, 2, 13)
}

// Test that a base of 1 doesn't panic (division by zero because of the log)
func TestService_OptimizeProofBase1(t *testing.T) {
	testOptimizeProof(t, 15, 1, 1, 16)
}

func testOptimizeProof(t *testing.T, numBlock, base, max, expected int) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	srvs, ro, service := local.MakeSRS(cothority.Suite, 6, skipchainSID)

	roster := onet.NewRoster(ro.List[:5])
	sk1 := service.(*Service)
	sk5 := srvs[4].Service(ServiceName).(*Service)
	sk6 := srvs[5].Service(ServiceName).(*Service)

	sbRoot, err := makeGenesisRosterArgs(sk1, roster, nil, VerificationStandard, base, max)
	require.NoError(t, err)

	sb := NewSkipBlock()
	sb.Roster = roster

	sk1.disableForwardLink = true

	var reply *StoreSkipBlockReply
	for i := 0; i < numBlock; i++ {
		reply, err = sk1.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		require.NoError(t, err)
	}

	sk1.disableForwardLink = false

	log.Lvl1("Request to optimize the proof")

	// Ask host 5 to optimize (not the leader)
	opr, err := sk5.OptimizeProof(&OptimizeProofRequest{Roster: ro, ID: reply.Latest.Hash})
	require.NoError(t, err)
	require.Equal(t, expected, len(opr.Proof))

	// And verify the proof is propagated to the roster we asked for
	sbs, err := sk6.db.GetProofForID(reply.Latest.Hash)
	require.NoError(t, err)
	require.Equal(t, expected, len(sbs))
}
