package pq

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"testing"
	"time"
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
	f := 2
	thr := 2*f + 1
	n := 3*f + 1
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(n, true)
	defer l.CloseAll()

	admin := darc.NewSignerEd25519(nil, nil)
	adminCt := uint64(1)
	provider1 := darc.NewSignerEd25519(nil, nil)
	reader1 := darc.NewSignerEd25519(nil, nil)

	// Initialise the genesis message and send it to the service.
	// The admin has the privilege to spawn darcs
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		nil, admin.Identity())

	msg.BlockInterval = 500 * time.Millisecond
	require.NoError(t, err)
	// The darc inside it should be valid.
	gDarc := msg.GenesisDarc
	require.Nil(t, gDarc.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.NoError(t, err)
	cl := NewClient(c)

	//Create a signer, darc for data point #1
	darc1 := darc.NewDarc(darc.InitRules([]darc.Identity{provider1.Identity()},
		[]darc.Identity{provider1.Identity()}), []byte("Provider1"))
	// provider1 is the owner, while reader1 is allowed to do read
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractPQWriteID),
		expression.InitOrExpr(provider1.Identity().String()))
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader1.Identity().String()))
	require.NotNil(t, darc1)
	require.NoError(t, err)
	_, err = cl.SpawnDarc(admin, adminCt, gDarc, *darc1, 10)
	adminCt++
	require.NoError(t, err)

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

	sigs := make(map[int][]byte)
	replies := cl.VerifyWriteAll(roster, &wr, shares, rands)
	for i, r := range replies {
		require.NotNil(t, r)
		sigs[i] = r.Sig
	}
	wReply, err := cl.AddWrite(&wr, sigs, thr, provider1, 1, *darc1, 10)
	require.NoError(t, err)
	require.NotNil(t, wReply.InstanceID)
}
