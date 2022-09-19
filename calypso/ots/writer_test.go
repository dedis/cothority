package ots

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/onet/v3"
	"testing"
	"time"
)

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
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractOTSWriteID),
		expression.InitOrExpr(provider1.Identity().String()))
	require.NoError(t, err)
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractOTSReadID),
		expression.InitOrExpr(reader1.Identity().String()))
	require.NoError(t, err)
	require.NotNil(t, darc1)
	_, err = cl.SpawnDarc(admin, adminCt, gDarc, *darc1, 10)
	adminCt++
	require.NoError(t, err)

	shares, pubPoly, proofs, secret, err := RunPVSS(cothority.Suite, n, thr,
		roster.Publics(), darc1.GetID())
	require.NoError(t, err)
	require.NotNil(t, shares)
	require.NotNil(t, pubPoly)
	require.NotNil(t, proofs)

	mesg := []byte("Hello regular OTS!")
	ctxt, ctxtHash, err := Encrypt(cothority.Suite, secret, mesg)
	require.NoError(t, err)
	require.NotNil(t, ctxt)

	wr := Write{
		PolicyID: darc1.GetID(),
		Shares:   shares,
		Proofs:   proofs,
		Publics:  roster.Publics(),
		CtxtHash: ctxtHash,
	}

	wReply, err := cl.AddWrite(&wr, provider1, 1, *darc1, 10)
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

	var keys []kyber.Point
	var encShares []*pvss.PubVerShare

	decShares := ElGamalDecrypt(cothority.Suite, reader1.Ed25519.Secret,
		dkReply.Reencryptions)
	for _, ds := range decShares {
		require.NotNil(t, ds)
		keys = append(keys, roster.Publics()[ds.S.I])
		encShares = append(encShares, shares[ds.S.I])
	}

	g := cothority.Suite.Point().Base()
	recSecret, err := pvss.RecoverSecret(cothority.Suite, g, keys, encShares,
		decShares, thr, n)
	require.NoError(t, err)
	require.NotNil(t, recSecret)

	ptxt, err := Decrypt(recSecret, ctxt)
	require.NoError(t, err)
	require.True(t, bytes.Equal(ptxt, mesg))
	fmt.Println(string(ptxt))
}
