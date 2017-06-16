package onchain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestOnchain(t *testing.T) {
	// 1 - share generation
	nbrPeers := 5
	peers := make([]*Peer, nbrPeers)
	// 1a - initialisation
	for i := range peers {
		peers[i] = NewPeer()
	}

	// 1b - key-sharing
	for _, p1 := range peers {
		p1.GenerateKey()
		//for _, p2 := range peers {
		// Everybody has to send a share to everybody else
		//}
	}
	X := peers[0].MulWithSecret(network.Suite.Point().Base())

	// 5.1.2 - Encryption

	data := []byte("Very secret Message to be encrypted")
	key := random.Bytes(16, random.Stream)

	cipher := network.Suite.Cipher(key)
	encData := cipher.Seal(nil, data)

	keyPoint, rem := network.Suite.Point().Pick(key, random.Stream)
	require.Equal(t, 0, len(rem))

	r := network.Suite.Scalar().Pick(random.Stream)
	C := X.Clone().Mul(X, r)
	C.Add(C, keyPoint)
	U := network.Suite.Point().Mul(nil, r)
	// u and c is shared with everybody

	// Decryption
	Ui := make([]abstract.Point, nbrPeers)
	for i := range Ui {
		Ui[i] = peers[i].MulWithSecret(U)
	}

	Xhat := LagrangeInterpolate(Ui)
	XhatInv := network.Suite.Point().Neg(Xhat)

	keyPointHat := network.Suite.Point().Add(C, XhatInv)

	require.True(t, keyPointHat.Equal(keyPoint))

	// Extract the message
	keyHat, err := keyPointHat.Data()
	log.ErrFatal(err)
	cipherHat := network.Suite.Cipher(keyHat)
	log.Print(encData)
	dataHat, err := cipherHat.Open(nil, encData)
	log.ErrFatal(err)
	require.Equal(t, data, dataHat)
}
