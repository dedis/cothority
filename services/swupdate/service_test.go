package swupdate

import (
	"testing"

	"os"

	"strconv"

	"flag"
	"runtime/pprof"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
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

func TestService_Timestamp(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)

	service := s.(*Service)
	body, err := service.CreatePackage(nil,
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)
	// XXX Why these methods return Body and not message ?
	cpr := body.(*CreatePackageRet)

	now := time.Now()
	service.timestamp(now)

	body2, err := service.LatestBlock(nil, &LatestBlock{cpr.SwupChain.Data.Hash})
	log.ErrFatal(err)
	lbr := body2.(*LatestBlockRet)

	tr, err := service.TimestampProof(nil, &TimestampRequest{release1.Policy.Name})
	log.ErrFatal(err)
	proof := tr.(*TimestampRet).Proof
	leaf := lbr.Update[len(lbr.Update)-1].Hash

	// verify proof
	c := proof.Check(HashFunc(), lbr.Timestamp.Root, leaf)
	assert.True(t, c)
	// verify timestamp signature
	log.Printf("Verifying cosi signature with public %x msg %x", roster.Aggregate, []byte(lbr.Timestamp.Root.String()))
	msg := MarshalPair(lbr.Timestamp.Root, lbr.Timestamp.SignatureResponse.Timestamp)
	assert.Nil(t, swupdate.VerifySignature(network.Suite, roster.Publics(), msg, lbr.Timestamp.SignatureResponse.Signature))
}

func TestService_PackageSC(t *testing.T) {
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, roster, s := local.MakeHELS(5, swupdateService)
	service := s.(*Service)
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
		&CreatePackage{roster, release1, 2, 10})
	log.ErrFatal(err)
	sc := cpr.(*CreatePackageRet).SwupChain
	//verifyExistence(t, hosts, sc, policy1.Name, true)

	upr, err := service.UpdatePackage(nil,
		&UpdatePackage{sc, release2})
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

var keys []*PGP
var keysPublic []string

var policy1 *Policy
var policy2 *Policy

var sigs1 []string
var sigs2 []string

var release1 *Release
var release2 *Release

func initGlobals(nbrKeys int) {
	for i := 0; i < nbrKeys; i++ {
		keys = append(keys, NewPGP())
		keysPublic = append(keysPublic, keys[i].ArmorPublic())
	}

	policy1 = &Policy{
		Name:       "test",
		Version:    "1.2",
		Source:     "https://github.com/dedis/cothority",
		Threshold:  3,
		Keys:       keysPublic,
		BinaryHash: "0",
	}
	policy2 = &Policy{
		Name:       "test",
		Version:    "1.3",
		Source:     "https://github.com/dedis/cothority",
		Threshold:  3,
		Keys:       keysPublic,
		BinaryHash: "0",
	}

	p1, err := network.MarshalRegisteredType(policy1)
	log.ErrFatal(err)
	p2, err := network.MarshalRegisteredType(policy2)
	log.ErrFatal(err)
	for _, k := range keys {
		s1, err := k.Sign(p1)
		log.ErrFatal(err)
		s2, err := k.Sign(p2)
		log.ErrFatal(err)

		sigs1 = append(sigs1, s1)
		sigs2 = append(sigs2, s2)
	}

	release1 = &Release{policy1, sigs1, false}
	release2 = &Release{policy2, sigs2, false}
}
