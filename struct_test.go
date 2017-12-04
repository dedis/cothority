package ocs

import (
	"testing"

	"github.com/dedis/onchain-secrets/darc"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1/network"
)

func TestWriteProof(t *testing.T) {
	encryptedData := []byte{1, 2, 3}
	encryptionKey := []byte{1, 2, 3}
	scid := []byte{4, 5, 6}
	reader := darc.NewDarc(nil, nil, nil)
	reader.Description = &[]byte{7, 8, 9}
	kp := config.NewKeyPair(network.Suite)
	wr := NewWrite(network.Suite, scid, kp.Public, reader, encryptionKey)
	wr.Data = encryptedData
	require.Nil(t, wr.CheckProof(network.Suite, scid))
	reader = darc.NewDarc(nil, nil, nil)
	reader.Description = &[]byte{10, 11, 12}
	wr.Reader = *reader
	require.NotNil(t, wr.CheckProof(network.Suite, scid))
}
