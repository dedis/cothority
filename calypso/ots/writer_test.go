package ots

import (
	"bytes"
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
	writer1 := darc.NewSignerEd25519(nil, nil)
	writer2 := darc.NewSignerEd25519(nil, nil)
	reader1 := darc.NewSignerEd25519(nil, nil)
	reader2 := darc.NewSignerEd25519(nil, nil)

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
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractOTSWriteID),
		expression.InitOrExpr(writer1.Identity().String()))
	require.NoError(t, err)
	err = darc1.Rules.AddRule(darc.Action("spawn:"+ContractOTSReadID),
		expression.InitOrExpr(reader1.Identity().String()))
	require.NoError(t, err)
	require.NotNil(t, darc1)
	_, err = cl.SpawnDarc(admin, adminCt, gDarc, *darc1, 10)
	adminCt++
	require.NoError(t, err)

	//Create a similar darc for writer2, reader2
	darc2 := darc.NewDarc(darc.InitRules([]darc.Identity{writer2.Identity()},
		[]darc.Identity{writer2.Identity()}), []byte("Writer2"))
	// provider1 is the owner, while reader1 is allowed to do read
	err = darc2.Rules.AddRule(darc.Action("spawn:"+ContractOTSWriteID),
		expression.InitOrExpr(writer2.Identity().String()))
	require.NoError(t, err)
	err = darc2.Rules.AddRule(darc.Action("spawn:"+ContractOTSReadID),
		expression.InitOrExpr(reader2.Identity().String()))
	require.NoError(t, err)
	require.NotNil(t, darc2)
	_, err = cl.SpawnDarc(admin, adminCt, gDarc, *darc2, 10)
	adminCt++
	require.NoError(t, err)

	shares1, pubPoly1, proofs1, secret1, err := RunPVSS(cothority.Suite, n, thr,
		roster.Publics(), darc1.GetID())
	require.NoError(t, err)
	require.NotNil(t, shares1)
	require.NotNil(t, pubPoly1)
	require.NotNil(t, proofs1)

	mesg1 := []byte("Hello regular OTS #1!")
	ctxt1, ctxtHash1, err := Encrypt(cothority.Suite, secret1, mesg1)
	require.NoError(t, err)
	require.NotNil(t, ctxt1)

	wr1 := Write{
		PolicyID: darc1.GetID(),
		Shares:   shares1,
		Proofs:   proofs1,
		Publics:  roster.Publics(),
		CtxtHash: ctxtHash1,
	}

	wReply1, err := cl.AddWrite(&wr1, writer1, 1, *darc1, 10)
	require.NoError(t, err)
	require.NotNil(t, wReply1.InstanceID)

	prWr1, err := cl.WaitProof(wReply1.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.NotNil(t, prWr1)

	re1, err := cl.AddRead(prWr1, reader1, 1, 10)
	require.NoError(t, err)
	prRe1, err := cl.WaitProof(re1.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prRe1.InclusionProof.Match(re1.InstanceID.Slice()))

	// Create the second write + read

	shares2, pubPoly2, proofs2, secret2, err := RunPVSS(cothority.Suite, n,
		thr, roster.Publics(), darc2.GetID())
	require.NoError(t, err)
	require.NotNil(t, shares2)
	require.NotNil(t, pubPoly2)
	require.NotNil(t, proofs2)

	mesg2 := []byte("Hello regular OTS #2!")
	ctxt2, ctxtHash2, err := Encrypt(cothority.Suite, secret2, mesg2)
	require.NoError(t, err)
	require.NotNil(t, ctxt2)

	wr2 := Write{
		PolicyID: darc2.GetID(),
		Shares:   shares2,
		Proofs:   proofs2,
		Publics:  roster.Publics(),
		CtxtHash: ctxtHash2,
	}

	wReply2, err := cl.AddWrite(&wr2, writer2, 1, *darc2, 10)
	require.NoError(t, err)
	require.NotNil(t, wReply2.InstanceID)

	prWr2, err := cl.WaitProof(wReply2.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.NotNil(t, prWr2)

	re2, err := cl.AddRead(prWr2, reader2, 1, 10)
	require.NoError(t, err)
	prRe2, err := cl.WaitProof(re2.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prRe2.InclusionProof.Match(re2.InstanceID.Slice()))

	// These requests should fail
	_, err = cl.DecryptKey(&DecryptKeyRequest{Roster: roster, Read: *prRe2,
		Write: *prWr1})
	require.NotNil(t, err)
	_, err = cl.DecryptKey(&DecryptKeyRequest{Roster: roster, Read: *prRe1,
		Write: *prWr2})
	require.NotNil(t, err)

	// Valid decrypt request for reader1
	dkr1, err := cl.DecryptKey(&DecryptKeyRequest{
		Roster: roster,
		Read:   *prRe1,
		Write:  *prWr1,
	})
	require.NoError(t, err)
	require.NotNil(t, dkr1.Reencryptions)

	var keys1 []kyber.Point
	var encShares1 []*pvss.PubVerShare

	decShares1 := ElGamalDecrypt(cothority.Suite, reader1.Ed25519.Secret,
		dkr1.Reencryptions)
	for _, ds := range decShares1 {
		require.NotNil(t, ds)
		keys1 = append(keys1, roster.Publics()[ds.S.I])
		encShares1 = append(encShares1, shares1[ds.S.I])
	}

	g := cothority.Suite.Point().Base()
	recSecret1, err := pvss.RecoverSecret(cothority.Suite, g, keys1, encShares1,
		decShares1, thr, n)
	require.NoError(t, err)
	require.NotNil(t, recSecret1)

	ptxt1, err := Decrypt(recSecret1, ctxt1)
	require.NoError(t, err)
	require.True(t, bytes.Equal(ptxt1, mesg1))

	// Valid decrypt request for reader2
	dkr2, err := cl.DecryptKey(&DecryptKeyRequest{
		Roster: roster,
		Read:   *prRe2,
		Write:  *prWr2,
	})
	require.NoError(t, err)
	require.NotNil(t, dkr2.Reencryptions)

	var keys2 []kyber.Point
	var encShares2 []*pvss.PubVerShare

	decShares2 := ElGamalDecrypt(cothority.Suite, reader2.Ed25519.Secret,
		dkr2.Reencryptions)
	for _, ds := range decShares2 {
		require.NotNil(t, ds)
		keys2 = append(keys2, roster.Publics()[ds.S.I])
		encShares2 = append(encShares2, shares2[ds.S.I])
	}

	//g := cothority.Suite.Point().Base()
	recSecret2, err := pvss.RecoverSecret(cothority.Suite, g, keys2, encShares2,
		decShares2, thr, n)
	require.NoError(t, err)
	require.NotNil(t, recSecret2)

	ptxt2, err := Decrypt(recSecret2, ctxt2)
	require.NoError(t, err)
	require.True(t, bytes.Equal(ptxt2, mesg2))
}
