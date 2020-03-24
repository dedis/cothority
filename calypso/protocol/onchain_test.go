package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

var suite = suites.MustFind("Ed25519")

func TestOnchain(t *testing.T) {
	// 1 - share generation
	nbrPeers := 5
	threshold := 3
	dkgs, err := CreateDKGs(suite.(dkg.Suite), nbrPeers, threshold)
	require.NoError(t, err)

	// Get aggregate public share
	dks, err := dkgs[0].DistKeyShare()
	require.NoError(t, err)
	X := dks.Public()

	// 5.1.2 - Encryption
	data := []byte("Very secret Message to be encrypted")
	var k [16]byte
	random.Bytes(k[:], random.New())

	encData, err := aeadSeal(k[:], data)
	if err != nil {
		t.Fatal(err)
	}
	U, Cs := EncodeKey(suite, X, k[:])
	// U and Cs is shared with everybody

	// Reader's keypair
	xc := key.NewKeyPair(cothority.Suite)

	// Decryption
	Ui := make([]*share.PubShare, nbrPeers)
	for i := range Ui {
		dks, err := dkgs[i].DistKeyShare()
		require.NoError(t, err)
		v := suite.Point().Mul(dks.Share.V, U)
		v.Add(v, suite.Point().Mul(dks.Share.V, xc.Public))
		Ui[i] = &share.PubShare{
			I: i,
			V: v,
		}
	}

	// XhatEnc is the re-encrypted share under the reader's public key
	XhatEnc, err := share.RecoverCommit(suite, Ui, threshold, nbrPeers)
	require.NoError(t, err)

	// Decrypt XhatEnc
	keyHat, err := DecodeKey(suite, X, Cs, XhatEnc, xc.Private)
	require.NoError(t, err)

	// Extract the message - keyHat is the recovered key
	log.Lvl2(encData)
	dataHat, err := aeadOpen(t, keyHat, encData)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, data, dataHat)
	log.Lvl1("Original data", string(data))
	log.Lvl1("Recovered data", string(dataHat))
}

// CreateDKGs is used for testing to set up a set of DKGs.
//
// Input:
//   - suite - the suite to use
//   - nbrNodes - how many nodes to set up
//   - threshold - how many nodes can recover the secret
//
// Output:
//   - dkgs - a slice of dkg-structures
//   - err - an eventual error
func CreateDKGs(suite dkg.Suite, nbrNodes, threshold int) (dkgs []*dkg.DistKeyGenerator, err error) {
	// 1 - share generation
	dkgs = make([]*dkg.DistKeyGenerator, nbrNodes)
	scalars := make([]kyber.Scalar, nbrNodes)
	points := make([]kyber.Point, nbrNodes)
	// 1a - initialisation
	for i := range scalars {
		scalars[i] = suite.Scalar().Pick(suite.RandomStream())
		points[i] = suite.Point().Mul(scalars[i], nil)
	}

	// 1b - key-sharing
	for i := range dkgs {
		dkgs[i], err = dkg.NewDistKeyGenerator(suite,
			scalars[i], points, threshold)
		if err != nil {
			err = xerrors.Errorf("creating new distirbuted key generator: %v", err)
			return
		}
	}
	// Exchange of Deals
	responses := make([][]*dkg.Response, nbrNodes)
	for i, p := range dkgs {
		responses[i] = make([]*dkg.Response, nbrNodes)
		deals, err := p.Deals()
		if err != nil {
			return nil, xerrors.Errorf("getting deals: %v", err)
		}
		for j, d := range deals {
			responses[i][j], err = dkgs[j].ProcessDeal(d)
			if err != nil {
				return nil, xerrors.Errorf("processing deals: %v", err)
			}
		}
	}
	// ProcessResponses
	for i, resp := range responses {
		for j, r := range resp {
			for k, p := range dkgs {
				if r != nil && j != k {
					log.Lvl3("Response from-to-peer:", i, j, k)
					justification, err := p.ProcessResponse(r)
					if err != nil {
						return nil,
							xerrors.Errorf("processing responses: %v", err)
					}
					if justification != nil {
						return nil,
							xerrors.New("there should be no justification")
					}
				}
			}
		}
	}

	// Verify if all is OK
	for _, p := range dkgs {
		if !p.Certified() {
			return nil, xerrors.New("one of the dkgs is not finished yet")
		}
	}
	return
}

// These functions encapsulate the kind-of messy-to-use
// Go stdlib AEAD functions. We used to use the AEAD from crypto.v0,
// but it has been removed in preference to the standard one for now.
//
// If we want to use it in more places, it should be cleaned up,
// and moved to a permanent home.

// This suggested length is from https://godoc.org/crypto/cipher#NewGCM example
const nonceLen = 12

func aeadSeal(symKey, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(symKey)
	if err != nil {
		return nil,
			xerrors.Errorf("creating aes cipher block instance: %v", err)
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, nonceLen)
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, xerrors.Errorf("reading nonce: %v", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, xerrors.Errorf("creating aesgcm instance: %v", err)
	}
	encData := aesgcm.Seal(nil, nonce, data, nil)
	encData = append(encData, nonce...)
	return encData, nil
}

func aeadOpen(t *testing.T, key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil,
			xerrors.Errorf("creating aes cipher block instance: %v", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, xerrors.Errorf("creating aesgcm instance: %v", err)
	}
	require.NoError(t, err)

	if len(ciphertext) < 12 {
		return nil, xerrors.New("ciphertext too short")
	}
	nonce := ciphertext[len(ciphertext)-nonceLen:]
	out, err := aesgcm.Open(nil, nonce, ciphertext[0:len(ciphertext)-nonceLen], nil)
	return out, cothority.ErrorOrNil(err, "decrypting ciphertext")
}
