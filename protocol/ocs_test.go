package protocol

import (
	"testing"

	"errors"

	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

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
		ocs(t, nbrNodes, nbrNodes-1, 32)
	}
}

func TestOCSKeyLengths(t *testing.T) {
	if testing.Short() {
		t.Skip("Testing all keylengths takes some time...")
	}
	for keylen := 1; keylen < 64; keylen++ {
		log.Lvl1("Testing keylen of", keylen)
		ocs(t, 3, 2, keylen)
	}
}

func ocs(t *testing.T, nbrNodes, threshold, keylen int) {
	log.Lvl1("Running", nbrNodes, "nodes")
	start := time.Now()
	local := onet.NewLocalTest()
	defer local.CloseAll()
	servers, _, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes, true)
	log.Lvl3(tree.Dump())

	// 1 - setting up - in real life uses SetupDKG-protocol
	// Store the dkgs in the services
	dkgs, err := CreateDKGs(suite, nbrNodes, threshold)
	log.ErrFatal(err)
	services := local.GetServices(servers, testServiceID)
	for i := range services {
		services[i].(*testService).Shared, err = NewSharedSecret(dkgs[i])
		log.ErrFatal(err)
	}

	log.Lvl1("Setting up DKG without network:", time.Now().Sub(start))
	start = time.Now()

	// Get the collective public key
	dks, err := dkgs[0].DistKeyShare()
	log.ErrFatal(err)
	X := dks.Public()

	// 2 - writer - Encrypt a symmetric key and publish U, Cs
	key := random.Bytes(keylen, random.Stream)
	U, Cs := EncodeKey(suite, X, key)

	// 3 - reader - Makes a request to U by giving his public key Xc
	// xc is the client's private/publick key pair
	xc := config.NewKeyPair(network.Suite)

	log.Lvl1("Encrypting key (no interaction):", time.Now().Sub(start))
	start = time.Now()

	// 4 - service - starts the protocol -
	// as every node needs to have its own DKG, we
	// use a service to give the corresponding DKGs to the nodes.
	pi, err := services[0].(*testService).startOCS(tree)
	log.ErrFatal(err)
	protocol := pi.(*OCS)
	protocol.U = U
	protocol.Xc = xc.Public
	protocol.Poly = share.NewPubPoly(suite, suite.Point().Base(), dks.Commits)
	// timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
	log.ErrFatal(protocol.Start())
	select {
	case <-protocol.Done:
		log.Lvl2("root-node is done")
		require.NotNil(t, protocol.Uis)
		// Wait for other nodes
	case <-time.After(time.Hour):
		t.Fatal("Didn't finish in time")
	}

	log.Lvl1("Running re-encryption:", time.Now().Sub(start))
	start = time.Now()

	// 5 - service - Lagrange interpolate the Uis - the reader will only
	// get XhatEnc
	XhatEnc, err := share.RecoverCommit(suite, protocol.Uis, threshold, nbrNodes)
	log.ErrFatal(err)

	// 6 - reader - gets the resulting symmetric key, encrypted under Xc
	keyHat, err := DecodeKey(suite, X, Cs, XhatEnc, xc.Secret)
	log.ErrFatal(err)

	log.Lvl1("Decrypting the key:", time.Now().Sub(start))

	require.Equal(t, key, keyHat)
}

// testService allows setting the dkg-field of the protocol.
type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialised by the test
	Shared *SharedSecret
	Poly   *share.PubPoly
}

// Creates a service-protocol and returns the ProtocolInstance.
func (s *testService) startOCS(t *onet.Tree) (onet.ProtocolInstance, error) {
	pi, err := s.CreateProtocol(NameOCS, t)
	pi.(*OCS).Shared = s.Shared
	pi.(*OCS).Poly = s.Poly
	return pi, err
}

// Store the dkg in the protocol
func (s *testService) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case NameOCS:
		pi, err := NewOCS(tn)
		if err != nil {
			return nil, err
		}
		ocs := pi.(*OCS)
		ocs.Shared = s.Shared
		return ocs, nil
	default:
		return nil, errors.New("unknown protocol for this service")
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
func EncodeKey(suite abstract.Suite, X abstract.Point, key []byte) (U abstract.Point, Cs []abstract.Point) {
	r := suite.Scalar().Pick(random.Stream)
	// r, err := crypto.StringHexToScalar(network.Suite, "5046ADC1DBA838867B2BBBFDD0C3423E58B57970B5267A90F57960924A87F156")
	// log.ErrFatal(err)
	U = suite.Point().Mul(nil, r)
	log.Lvl3("U is:", U.String())

	rem := make([]byte, len(key))
	copy(rem, key)
	for len(rem) > 0 {
		var kp abstract.Point
		kp, rem = suite.Point().Pick(rem, random.Stream)
		log.Lvl3("Keypoint:", kp.String())
		C := suite.Point().Mul(X, r)
		log.Lvl3("X:", X.String())
		log.Lvl3("C:", C.String())
		Cs = append(Cs, C.Add(C, kp))
		log.Lvl3("Cs:", C.String())
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
func DecodeKey(suite abstract.Suite, X abstract.Point, Cs []abstract.Point, XhatEnc abstract.Point,
	xc abstract.Scalar) (key []byte, err error) {
	log.Lvl3("xc:", xc)
	xcInv := suite.Scalar().Neg(xc)
	log.Lvl3("xcInv:", xcInv)
	sum := suite.Scalar().Add(xc, xcInv)
	log.Lvl3("xc + xcInv:", sum, "::", xc)
	log.Lvl3("X:", X)
	XhatDec := suite.Point().Mul(X, xcInv)
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
			return nil, err
		}
		key = append(key, keyPart...)
	}
	return
}

// starts a new service. No function needed.
func newService(c *onet.Context) onet.Service {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s
}
