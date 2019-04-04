package pki

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
)

func TestStruct_PkProofs(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	serv := local.GenServers(1)[0]
	srvid := &serv.ServerIdentity.ServiceIdentities[0]

	pub, err := srvid.Public.MarshalBinary()
	require.NoError(t, err)

	pp := PkProofs{PkProof{
		Public:    pub,
		Nonce:     []byte{},
		Signature: []byte{},
	}}

	err = pp.Verify(srvid)
	require.Error(t, err)
	require.Equal(t, "nonce length does not match", err.Error())

	pp[0].Nonce = make([]byte, nonceLength)
	srvid.Suite = "unknown"
	err = pp.Verify(srvid)
	require.Error(t, err)
	require.Equal(t, "unknown suite used for the service", err.Error())

	srvid.Suite = cothority.Suite.String()
	err = pp.Verify(srvid)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed:")

	pp[0].Public = []byte{}
	err = pp.Verify(srvid)
	require.Error(t, err)
	require.Equal(t, "couldn't find a proof", err.Error())
}
