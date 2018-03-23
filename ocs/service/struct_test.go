package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/kyber/util/key"
	"github.com/stretchr/testify/require"
)

func TestWriteProof(t *testing.T) {
	encryptedData := []byte{1, 2, 3}
	encryptionKey := []byte{1, 2, 3}
	scid := []byte{4, 5, 6}
	reader := darc.NewDarc(nil, nil, nil)
	reader.Description = &[]byte{7, 8, 9}
	kp := key.NewKeyPair(cothority.Suite)
	wr := NewWrite(cothority.Suite, scid, kp.Public, reader, encryptionKey)
	wr.Data = encryptedData
	require.Nil(t, wr.CheckProof(cothority.Suite, scid))
	reader = darc.NewDarc(nil, nil, nil)
	reader.Description = &[]byte{10, 11, 12}
	wr.Reader = *reader
	require.NotNil(t, wr.CheckProof(cothority.Suite, scid))
}
