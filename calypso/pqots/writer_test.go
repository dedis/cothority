package pqots

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"testing"
	"time"
)

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
	writer1 := darc.NewSignerEd25519(nil, nil)
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
	darc1 := darc.NewDarc(darc.InitRules([]darc.Identity{writer1.Identity()},
		[]darc.Identity{writer1.Identity()}), []byte("Writer1"))
	// writer1 is the owner, while reader1 is allowed to do read
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractPQOTSWriteID),
		expression.InitOrExpr(writer1.Identity().String()))
	require.NoError(t, err)
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractPQOTSReadID),
		expression.InitOrExpr(reader1.Identity().String()))
	require.NoError(t, err)
	require.NotNil(t, darc1)
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
	wReply, err := cl.AddWrite(&wr, sigs, thr, writer1, 1, *darc1, 10)
	require.NoError(t, err)
	require.NotNil(t, wReply.InstanceID)

	prWr, err := cl.WaitProof(wReply.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.NotNil(t, prWr)

	rReply, err := cl.AddRead(prWr, reader1, 1, 10)
	require.NoError(t, err)
	prRe, err := cl.WaitProof(rReply.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prRe.InclusionProof.Match(rReply.InstanceID.Slice()))

	dkReply, err := cl.DecryptKey(&DecryptKeyRequest{
		Roster: roster,
		Read:   *prRe,
		Write:  *prWr,
	})
	require.NoError(t, err)
	require.NotNil(t, dkReply.Reencryptions)

	//recShares := make([]share.PriShare, len(roster.List))
	decShares := ElGamalDecrypt(cothority.Suite, reader1.Ed25519.Secret,
		dkReply.Reencryptions)
	for _, ds := range decShares {
		require.NotNil(t, ds)
	}

	recSecret, err := share.RecoverSecret(cothority.Suite, decShares, thr, n)
	require.NoError(t, err)
	require.True(t, recSecret.Equal(poly.Secret()))
	ptxt, err := Decrypt(recSecret, ctxt)
	require.NoError(t, err)
	require.True(t, bytes.Equal(ptxt, mesg))
}
