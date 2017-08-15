package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

var suite = network.Suite

func TestOnchain(t *testing.T) {
	// 1 - share generation
	nbrPeers := 5
	threshold := 3
	dkgs, err := CreateDKGs(suite, nbrPeers, threshold)

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
