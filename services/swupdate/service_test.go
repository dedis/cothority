package swupdate

import (
	"errors"
	"testing"

	"os"

	"strconv"

	"flag"
	"runtime/pprof"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	initGlobals(3)
}

func TestMain(m *testing.M) {
	os.RemoveAll("config")
	rc := map[string]string{}
	mon := monitor.NewMonitor(monitor.NewStats(rc))
	go func() { log.ErrFatal(mon.Listen()) }()
	local := "localhost:" + strconv.Itoa(monitor.DefaultSinkPort)
	log.ErrFatal(monitor.ConnectSink(local))

	// Copy of log.MainTest because we need to close the monitor before
	// the tests end, else the go-routine will show up in 'log.AfterTest'.
	flag.Parse()
	log.TestOutput(testing.Verbose(), 2)
	done := make(chan int)
	go func() {
		code := m.Run()
		done <- code
	}()
	select {
	case code := <-done:
		monitor.EndAndCleanup()
		log.AfterTest(nil)
		os.Exit(code)
	case <-time.After(log.MainTestWait):
		log.Error("Didn't finish in time")
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		os.Exit(1)
	}
}

func TestServiceSaveLoad(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, _, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)

	service.save()

	s2 := &Service{path: service.path}
	log.ErrFatal(s2.tryLoad())
}

func TestService_CreatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	release1 := chain1.blocks[0].release
	policy1 := chain1.blocks[0].policy
	sigs2 := chain2.blocks[1].sigs
	// This should fail as the signatures are wrong
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{
			Roster:  roster,
			Release: &Release{policy1, sigs2, false},
			Base:    2,
			Height:  10,
		})
	assert.NotNil(t, err, "Accepted wrong signatures")
	cpr, err = service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)

	sc := cpr.(*CreatePackageRet).SwupChain
	assert.NotNil(t, sc.Data)
	policy := sc.Release.Policy
	assert.Equal(t, *policy1, *policy)
	assert.Equal(t, *policy1, *service.Storage.SwupChains[policy.Name].Release.Policy)
}

func TestService_UpdatePackage(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	release1 := chain1.blocks[0].release
	sigs1 := chain1.blocks[0].sigs
	release2 := chain1.blocks[1].release
	policy2 := chain1.blocks[1].policy
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain: sc,
			Release:   &Release{policy2, sigs1, false},
		})
	assert.NotNil(t, err, "Updating packages with wrong signature should fail")
	upr, err = service.UpdatePackage(nil,
		&UpdatePackage{
			SwupChain: sc,
			Release:   release2,
		})
	log.ErrFatal(err)
	sc = upr.(*UpdatePackageRet).SwupChain
	assert.NotNil(t, sc)
	assert.Equal(t, *policy2, *sc.Release.Policy)
}

func TestPairMarshalling(t *testing.T) {
	root := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	t1 := time.Now().Unix()

	buff := MarshalPair(root, t1)

	root2, t2 := UnmarshalPair(buff)
	assert.Equal(t, root, []byte(root2))
	assert.Equal(t, t1, t2)
}

// insertChain will insert every release into the service skipchains and returns
// the last swupchain returned.
func insertChain(service *Service, r *sda.Roster, c *packageChain) *SwupChain {
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{r, c.blocks[0].release, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain

	for _, block := range c.blocks[1:] {
		upr, err := service.UpdatePackage(nil,
			&UpdatePackage{
				SwupChain: sc,
				Release:   &Release{block.policy, block.sigs, false},
			})
		log.ErrFatal(err)
		sc = upr.(*UpdatePackageRet).SwupChain
	}
	return sc
}

func checkChain(s *Service, r *sda.Roster, name string, last *SwupChain) error {
	body2, err := s.LatestBlock(nil, &LatestBlock{last.Data.Hash})
	if err != nil {
		return err
	}
	lbr := body2.(*LatestBlockRet)

	tr, err := s.TimestampProof(nil, &TimestampRequest{name})
	if err != nil {
		return err
	}
	proof := tr.(*TimestampRet).Proof
	leaf := lbr.Update[len(lbr.Update)-1].Hash

	// verify proof
	return verifyProof(proof, lbr.Timestamp.Root, crypto.HashID(leaf), lbr.Timestamp.SignatureResponse.Timestamp, r.Publics(), lbr.Timestamp.SignatureResponse.Signature)
}

func verifyProof(proof crypto.Proof, root, leaf crypto.HashID, ts int64, publics []abstract.Point, sig []byte) error {
	c := proof.Check(HashFunc(), root, leaf)
	if !c {
		return errors.New("Proof verification incorrect")
	}
	// verify timestamp signature
	msg := MarshalPair(root, ts)
	return swupdate.VerifySignature(network.Suite, publics, msg, sig)
}

// Same as TestService_TimestampProof but checking all chains instead of just
// one package / chain
func TestService_TimestampProofBatch(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	sw1 := insertChain(service, roster, chain1)
	sw2 := insertChain(service, roster, chain2)
	// get all latests blocks
	names := []string{chain1.packag, chain2.packag}
	ids := []skipchain.SkipBlockID{sw1.Data.Hash, sw2.Data.Hash}
	b, err := service.LatestBlocks(nil, &LatestBlocks{ids})
	log.ErrFatal(err)
	lbr := b.(*LatestBlocksRet)
	// get all proofs
	b, err = service.TimestampProofs(nil, &TimestampRequests{names})
	log.ErrFatal(err)
	tr := b.(*TimestampRets)

	for i := range ids {
		updates := lbr.Updates[i]
		last := updates[len(updates)-1]
		name := names[i]
		proof, ok := tr.Proofs[name]
		if !ok {
			t.Fatal("Did not find the name in the proof responses")
		}
		e := verifyProof(proof, lbr.Timestamp.Root, crypto.HashID(last.Hash), lbr.Timestamp.SignatureResponse.Timestamp, roster.Publics(), lbr.Timestamp.SignatureResponse.Signature)
		log.ErrFatal(e)
	}
}
func TestService_TimestampProof(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	// XXX Why these methods return Body and not message ?
	service := s.(*Service)
	sw1 := insertChain(service, roster, chain1)
	sw2 := insertChain(service, roster, chain2)

	assert.Nil(t, checkChain(service, roster, chain1.packag, sw1))
	assert.Nil(t, checkChain(service, roster, chain2.packag, sw2))
}

func TestService_PackageSC(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
	release1 := chain1.blocks[0].release
	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain

	pscret, err := service.PackageSC(nil, &PackageSC{"unknown"})
	require.NotNil(t, err)
	pscret, err = service.PackageSC(nil, &PackageSC{release1.Policy.Name})
	log.ErrFatal(err)
	sc2 := pscret.(*PackageSCRet).Last

	require.Equal(t, sc.Data.Hash, sc2.Hash)
	sc3 := service.Storage.SwupChains[release1.Policy.Name]
	require.Equal(t, sc.Data.Hash, sc3.Data.Hash)
}

func TestService_LatestBlock(t *testing.T) {
	TestInitializePackages(t)
}

func TestService_PropagateBlock(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)

	cpr, err := service.CreatePackage(nil,
		&CreatePackage{roster, chain1.blocks[0].release, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	//verifyExistence(t, hosts, sc, policy1.Name, true)

	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{sc, chain1.blocks[1].release})
	log.ErrFatal(err)
	sc = upr.(*UpdatePackageRet).SwupChain
	//verifyExistence(t, hosts, sc, policy2.Name, false)
}

// func verifyExistence(t *testing.T, hosts []*sda.Host, sc *SwupChain,
//name string, genesis bool) {
//for _, h := range hosts {
//	log.Lvl2("Verifying host", h)
//	s := h.GetService(ServiceName).(*Service)
//	if genesis {
//		swup, ok := s.Storage.SwupChainsGenesis[name]
//		require.True(t, ok)
//		require.Equal(t, swup.Data.Hash, sc.Data.Hash)
//	}
//	swup, ok := s.Storage.SwupChains[name]
//	require.True(t, ok)
//	require.Equal(t, swup.Data.Hash, sc.Data.Hash)
//}
//}

// packageChain tracks all test releases for one fake package
type packageChain struct {
	packag string
	blocks []*packageBlock
}

// packageBlock tracks all information on one release of a package
type packageBlock struct {
	keys       []*PGP
	keysPublic []string
	policy     *Policy
	release    *Release
	sigs       []string
}

var chain1 *packageChain
var chain2 *packageChain

var keys []*PGP
var keysPublic []string

func initGlobals(nbrKeys int) {
	for i := 0; i < nbrKeys; i++ {
		keys = append(keys, NewPGP())
		keysPublic = append(keysPublic, keys[i].ArmorPublic())
	}

	createBlock := func(name, version string) *packageBlock {
		policy1 := &Policy{
			Name:       name,
			Version:    version,
			Source:     "https://github.com/dedis/cothority",
			Threshold:  3,
			Keys:       keysPublic,
			BinaryHash: "0",
		}
		p1, err := network.MarshalRegisteredType(policy1)
		log.ErrFatal(err)
		var sigs1 []string
		for _, k := range keys {
			s1, err := k.Sign(p1)
			log.ErrFatal(err)
			sigs1 = append(sigs1, s1)
		}
		return &packageBlock{
			keys:       keys,
			keysPublic: keysPublic,
			policy:     policy1,
			release:    &Release{policy1, sigs1, false},
			sigs:       sigs1,
		}
	}

	createChain := func(pack string) *packageChain {
		b1 := createBlock(pack, "1.2")
		b2 := createBlock(pack, "1.3")
		return &packageChain{
			packag: pack,
			blocks: []*packageBlock{b1, b2},
		}
	}

	chain1 = createChain("test")
	chain2 = createChain("test2")
}
