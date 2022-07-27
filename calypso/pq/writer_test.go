package pq

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"testing"
)

func TestWriter_CreatePoly(t *testing.T) {
	GenerateSSPoly(5)
}

func TestSign(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	fmt.Println(roster.List[0].Public.String())
	fmt.Println(roster.List[0].GetPrivate().String())

	mesg := []byte("Hello world!")
	sig, err := schnorr.Sign(cothority.Suite, roster.List[0].GetPrivate(), mesg)
	require.NoError(t, err)
	err = schnorr.Verify(cothority.Suite, roster.List[0].Public, mesg, sig)
	require.NoError(t, err)
}

func TestAll(t *testing.T) {
	f := 3
	n := 2*f + 1
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(n, true)
	defer l.CloseAll()

	poly := GenerateSSPoly(f + 1)
	shares, rands, commitments, err := GenerateCommitments(poly, n)
	require.NoError(t, err)
	require.NotNil(t, shares)
	require.NotNil(t, rands)
	require.NotNil(t, commitments)

	mesg := []byte("Hello world!")
	ctxt, ctxtHash, err := Encrypt(poly.Secret(), mesg)
	require.NoError(t, err)
	require.NotNil(t, ctxt)
	require.NotNil(t, ctxtHash)

	wr := Write{Commitments: commitments, Publics: roster.Publics(),
		CtxtHash: ctxtHash}

	cl := NewClient()
	replies := cl.VerifyWriteAll(roster, &wr, shares, rands)
	for _, r := range replies {
		require.NotNil(t, r)
	}
}
