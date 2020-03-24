package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

var tSuite = cothority.Suite

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "ServiceOCS"

func init() {
	var err error
	testServiceID, err = onet.RegisterNewService(testServiceName, newService)
	log.ErrFatal(err)
}

// Tests a 3, 5 and 13-node system.
func TestOCS(t *testing.T) {
	nodes := []int{3}
	// nodes := []int{3, 5, 10}
	for _, nbrNodes := range nodes {
		log.Lvlf1("Starting setupDKG with %d nodes", nbrNodes)
		ocs(t, nbrNodes, nbrNodes-1, 32, 0, false)
	}
}

// Tests a system with failing nodes
func TestFail(t *testing.T) {
	ocs(t, 4, 2, 32, 2, false)
}

// Tests what happens if the nodes refuse to send their share
func TestRefuse(t *testing.T) {
	log.Lvl1("Starting setupDKG with 3 nodes and refusing to sign")
	ocs(t, 3, 2, 32, 0, true)
}

func TestOCSKeyLengths(t *testing.T) {
	if testing.Short() {
		t.Skip("Testing all keylengths takes some time...")
	}
	for keylen := 1; keylen < 64; keylen++ {
		log.Lvl1("Testing keylen of", keylen)
		ocs(t, 3, 2, keylen, 0, false)
	}
}

func ocs(t *testing.T, nbrNodes, threshold, keylen, fail int, refuse bool) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	// 1 - setting up - in real life uses Setup-protocol
	// Store the dkgs in the services
	dkgs, err := CreateDKGs(tSuite.(dkg.Suite), nbrNodes, threshold)
	require.NoError(t, err)
	services := local.GetServices(servers, testServiceID)
	for i := range services {
		services[i].(*testService).Shared, _, err = dkgprotocol.NewSharedSecret(dkgs[i])
		require.NoError(t, err)
	}

	// Get the collective public key
	dks, err := dkgs[0].DistKeyShare()
	require.NoError(t, err)
	X := dks.Public()

	// 2 - writer - Encrypt a symmetric key and publish U, Cs
	k := make([]byte, keylen)
	random.Bytes(k, random.New())
	U, Cs := EncodeKey(tSuite, X, k)

	// 3 - reader - Makes a request to U by giving his public key Xc
	// xc is the client's private/publick key pair
	xc := key.NewKeyPair(cothority.Suite)

	// 4 - service - starts the protocol -
	// as every node needs to have its own DKG, we
	// use a service to give the corresponding DKGs to the nodes.

	// First stop the nodes that should fail
	for _, s := range servers[1 : 1+fail] {
		log.Lvl1("Pausing", s.ServerIdentity)
		s.Pause()
	}
	pi, err := services[0].(*testService).createOCS(tree, threshold)
	require.NoError(t, err)
	protocol := pi.(*OCS)
	protocol.U = U
	protocol.Xc = xc.Public
	protocol.Poly = share.NewPubPoly(suite, suite.Point().Base(), dks.Commits)
	if !refuse {
		protocol.VerificationData = []byte("correct block")
	}
	// timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	require.Nil(t, protocol.Start())
	select {
	case <-protocol.Reencrypted:
		log.Lvl2("root-node is done")
		// Wait for other nodes
	case <-time.After(time.Second):
		t.Fatal("Didn't finish in time")
	}

	// 5 - service - Lagrange interpolate the Uis - the reader will only
	// get XhatEnc
	var XhatEnc kyber.Point
	if refuse {
		require.Nil(t, protocol.Uis, "Reencrypted request that should've been refused")
		return
	}

	require.NotNil(t, protocol.Uis)
	XhatEnc, err = share.RecoverCommit(suite, protocol.Uis, threshold, nbrNodes)
	require.Nil(t, err, "Reencryption failed")

	// 6 - reader - gets the resulting symmetric key, encrypted under Xc
	keyHat, err := DecodeKey(suite, X, Cs, XhatEnc, xc.Private)
	require.NoError(t, err)

	require.Equal(t, k, keyHat)
}

// testService allows setting the dkg-field of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialised by the test
	Shared *dkgprotocol.SharedSecret
	Poly   *share.PubPoly
}

// Creates a service-protocol and returns the ProtocolInstance.
func (s *testService) createOCS(t *onet.Tree, threshold int) (onet.ProtocolInstance, error) {
	pi, err := s.CreateProtocol(NameOCS, t)
	pi.(*OCS).Shared = s.Shared
	pi.(*OCS).Poly = s.Poly
	pi.(*OCS).Threshold = threshold
	return pi, err
}

// Store the dkg in the protocol
func (s *testService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case NameOCS:
		pi, err := NewOCS(tn)
		if err != nil {
			return nil, xerrors.Errorf("creating new OCS instance: %v", err)
		}
		ocs := pi.(*OCS)
		ocs.Shared = s.Shared
		ocs.Verify = func(rc *Reencrypt) bool {
			return rc.VerificationData != nil
		}
		return ocs, nil
	default:
		return nil, xerrors.New("unknown protocol for this service")
	}
}

// EncodeKey can be used by the writer to an onchain-secret skipchain
// to encode his symmetric key under the collective public key created
// by the DKG.
// As this method uses `Pick` to encode the key, depending on the key-length
// more than one point is needed to encode the data.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document
//
// Output:
//   - U - the schnorr commit
//   - Cs - encrypted key-slices
func EncodeKey(suite suites.Suite, X kyber.Point, key []byte) (U kyber.Point, Cs []kyber.Point) {
	r := suite.Scalar().Pick(suite.RandomStream())
	C := suite.Point().Mul(r, X)
	log.Lvl3("C:", C.String())
	U = suite.Point().Mul(r, nil)
	log.Lvl3("U is:", U.String())

	for len(key) > 0 {
		kp := suite.Point().Embed(key, suite.RandomStream())
		log.Lvl3("Keypoint:", kp.String())
		log.Lvl3("X:", X.String())
		Cs = append(Cs, suite.Point().Add(C, kp))
		log.Lvl3("Cs:", C.String())
		key = key[min(len(key), kp.EmbedLen()):]
	}
	return
}

// DecodeKey can be used by the reader of an onchain-secret to convert the
// re-encrypted secret back to a symmetric key that can be used later to
// decode the document.
//
// Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - Cs - the encrypted key-slices
//   - XhatEnc - the re-encrypted schnorr-commit
//   - xc - the private key of the reader
//
// Output:
//   - key - the re-assembled key
//   - err - an eventual error when trying to recover the data from the points
func DecodeKey(suite kyber.Group, X kyber.Point, Cs []kyber.Point, XhatEnc kyber.Point,
	xc kyber.Scalar) (key []byte, err error) {
	log.Lvl3("xc:", xc)
	xcInv := suite.Scalar().Neg(xc)
	log.Lvl3("xcInv:", xcInv)
	sum := suite.Scalar().Add(xc, xcInv)
	log.Lvl3("xc + xcInv:", sum, "::", xc)
	log.Lvl3("X:", X)
	XhatDec := suite.Point().Mul(xcInv, X)
	log.Lvl3("XhatDec:", XhatDec)
	log.Lvl3("XhatEnc:", XhatEnc)
	Xhat := suite.Point().Add(XhatEnc, XhatDec)
	log.Lvl3("Xhat:", Xhat)
	XhatInv := suite.Point().Neg(Xhat)
	log.Lvl3("XhatInv:", XhatInv)

	// Decrypt Cs to keyPointHat
	for _, C := range Cs {
		log.Lvl3("C:", C)
		keyPointHat := suite.Point().Add(C, XhatInv)
		log.Lvl3("keyPointHat:", keyPointHat)
		keyPart, err := keyPointHat.Data()
		log.Lvl3("keyPart:", keyPart)
		if err != nil {
			return nil, xerrors.Errorf("getting data from keypoint: %v", err)
		}
		key = append(key, keyPart...)
	}
	return
}

// starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
