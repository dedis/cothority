package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

var suite = network.Suite

func TestOnchain(t *testing.T) {
	// 1 - share generation
	nbrPeers := 3
	threshold := 2
	peers := make([]*Peer, nbrPeers)
	publics := make([]abstract.Point, nbrPeers)
	// 1a - initialisation
	for i := range peers {
		peers[i] = NewPeer(suite)
		publics[i] = peers[i].Public()
	}

	// 1b - key-sharing
	for _, p := range peers {
		log.ErrFatal(p.GenerateKey(publics, threshold))
	}
	// Exchange of Deals
	responses := make([][]*dkg.Response, nbrPeers)
	for i, p := range peers {
		responses[i] = make([]*dkg.Response, nbrPeers)
		deals, err := p.DKG.Deals()
		log.ErrFatal(err)
		for j, d := range deals {
			responses[i][j], err = peers[j].DKG.ProcessDeal(d)
			log.ErrFatal(err)
		}
	}
	// ProcessResponses
	for i, resp := range responses {
		for j, r := range resp {
			for k, p := range peers {
				if r != nil && j != k {
					log.Print("Response from-to-peer:", i, j, k)
					justification, err := p.DKG.ProcessResponse(r)
					log.ErrFatal(err)
					require.Nil(t, justification)
					log.Print("Certified:", i, j, k, p.DKG.Certified())
				}
			}
		}
	}

	// Secret commits
	for _, p := range peers {
		commit, err := p.DKG.SecretCommits()
		log.ErrFatal(err)
		for _, p2 := range peers {
			compl, err := p2.DKG.ProcessSecretCommits(commit)
			log.ErrFatal(err)
			require.Nil(t, compl)
		}
	}

	// Verify if all is OK
	for _, p := range peers {
		require.True(t, p.DKG.Finished())
	}

	// Get aggregate public share
	dks, err := peers[0].DKG.DistKeyShare()
	log.ErrFatal(err)
	X := dks.Public()

	// 5.1.2 - Encryption
	data := []byte("Very secret Message to be encrypted")
	key := random.Bytes(16, random.Stream)

	cipher := suite.Cipher(key)
	encData := cipher.Seal(nil, data)

	keyPoint, rem := suite.Point().Pick(key, random.Stream)
	require.Equal(t, 0, len(rem))

	r := suite.Scalar().Pick(random.Stream)
	C := X.Clone().Mul(X, r)
	C.Add(C, keyPoint)
	U := suite.Point().Mul(nil, r)
	// U and C is shared with everybody

	// Decryption
	Ui := make([]*share.PubShare, nbrPeers)
	for i := range Ui {
		Ui[i] = &share.PubShare{
			I: i,
			V: peers[i].MulWithSecret(U),
		}
	}

	// Xhat is the re-encrypted share under the reader's public key
	Xhat, err := share.RecoverCommit(suite, Ui, threshold, nbrPeers)
	log.ErrFatal(err)
	XhatInv := suite.Point().Neg(Xhat)

	keyPointHat := suite.Point().Add(C, XhatInv)

	require.True(t, keyPointHat.Equal(keyPoint))

	// Extract the message - keyHat is the recovered key
	keyHat, err := keyPointHat.Data()
	log.ErrFatal(err)
	cipherHat := suite.Cipher(keyHat)
	log.Print(encData)
	dataHat, err := cipherHat.Open(nil, encData)
	log.ErrFatal(err)
	require.Equal(t, data, dataHat)
	log.Lvl1("Original data", string(data))
	log.Lvl1("Recovered data", string(dataHat))
}
