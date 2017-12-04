package protocol

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

var suite = network.Suite

func TestOnchain(t *testing.T) {
	// 1 - share generation
	nbrPeers := 5
	threshold := 3
	dkgs, err := CreateDKGs(suite, nbrPeers, threshold)
	log.ErrFatal(err)

	// Get aggregate public share
	dks, err := dkgs[0].DistKeyShare()
	log.ErrFatal(err)
	X := dks.Public()

	// 5.1.2 - Encryption
	data := []byte("Very secret Message to be encrypted")
	key := random.Bytes(16, random.Stream)

	cipher := suite.Cipher(key)
	encData := cipher.Seal(nil, data)

	U, Cs := EncodeKey(suite, X, key)
	// U and Cs is shared with everybody

	// Reader's keypair
	xc := config.NewKeyPair(network.Suite)

	// Decryption
	Ui := make([]*share.PubShare, nbrPeers)
	for i := range Ui {
		dks, err := dkgs[i].DistKeyShare()
		log.ErrFatal(err)
		v := suite.Point().Mul(U, dks.Share.V)
		v.Add(v, suite.Point().Mul(xc.Public, dks.Share.V))
		Ui[i] = &share.PubShare{
			I: i,
			V: v,
		}
	}

	// XhatEnc is the re-encrypted share under the reader's public key
	XhatEnc, err := share.RecoverCommit(suite, Ui, threshold, nbrPeers)
	log.ErrFatal(err)

	// Decrypt XhatEnc
	keyHat, err := DecodeKey(suite, X, Cs, XhatEnc, xc.Secret)
	log.ErrFatal(err)

	// Extract the message - keyHat is the recovered key
	cipherHat := suite.Cipher(keyHat)
	log.Lvl2(encData)
	dataHat, err := cipherHat.Open(nil, encData)
	log.ErrFatal(err)
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
func CreateDKGs(suite abstract.Suite, nbrNodes, threshold int) (dkgs []*dkg.DistKeyGenerator, err error) {
	// 1 - share generation
	dkgs = make([]*dkg.DistKeyGenerator, nbrNodes)
	scalars := make([]abstract.Scalar, nbrNodes)
	points := make([]abstract.Point, nbrNodes)
	// 1a - initialisation
	for i := range scalars {
		scalars[i] = suite.Scalar().Pick(random.Stream)
		points[i] = suite.Point().Mul(nil, scalars[i])
	}

	// 1b - key-sharing
	for i := range dkgs {
		dkgs[i], err = dkg.NewDistKeyGenerator(suite,
			scalars[i], points, random.Stream, threshold)
		if err != nil {
			return
		}
	}
	// Exchange of Deals
	responses := make([][]*dkg.Response, nbrNodes)
	for i, p := range dkgs {
		responses[i] = make([]*dkg.Response, nbrNodes)
		deals, err := p.Deals()
		if err != nil {
			return nil, err
		}
		for j, d := range deals {
			responses[i][j], err = dkgs[j].ProcessDeal(d)
			if err != nil {
				return nil, err
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
						return nil, err
					}
					if justification != nil {
						return nil, errors.New("there should be no justification")
					}
				}
			}
		}
	}

	// Secret commits
	for _, p := range dkgs {
		commit, err := p.SecretCommits()
		if err != nil {
			return nil, err
		}
		for _, p2 := range dkgs {
			compl, err := p2.ProcessSecretCommits(commit)
			if err != nil {
				return nil, err
			}
			if compl != nil {
				return nil, errors.New("there should be no complaint")
			}
		}
	}

	// Verify if all is OK
	for _, p := range dkgs {
		if !p.Finished() {
			return nil, errors.New("one of the dkgs is not finished yet")
		}
	}
	return
}
