package calypso

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

// Tests the client function CreateLTS
func TestClient_CreateLTS(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	l.GetServices(servers, calypsoID)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = 100 * time.Millisecond
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.Nil(t, err)
	//Create a Calypso Client (Byzcoin + Onet)
	calypsoClient := NewClient(c)
	//Invoke CreateLTS
	ltsReply, err := calypsoClient.CreateLTS()
	require.Nil(t, err)
	require.NotNil(t, ltsReply.LTSID)
	require.NotNil(t, ltsReply.X)
}

// Tests the client api's AddRead, AddWrite, DecryptKey
func TestClient_Calypso(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	l.GetServices(servers, calypsoID)
	defer l.CloseAll()

	admin := darc.NewSignerEd25519(nil, nil)
	provider1 := darc.NewSignerEd25519(nil, nil)
	reader1 := darc.NewSignerEd25519(nil, nil)
	provider2 := darc.NewSignerEd25519(nil, nil)
	reader2 := darc.NewSignerEd25519(nil, nil)
	// Initialise the genesis message and send it to the service.
	// The admin has the privilege to spawn darcs
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + byzcoin.ContractDarcID},
		admin.Identity())

	msg.BlockInterval = 100 * time.Millisecond
	require.Nil(t, err)
	// The darc inside it should be valid.
	gDarc := msg.GenesisDarc
	require.Nil(t, gDarc.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.Nil(t, err)
	//Create a Calypso Client (Byzcoin + Onet)
	calypsoClient := NewClient(c)

	//Create the LTS
	ltsReply, err := calypsoClient.CreateLTS()
	require.Nil(t, err)
	//If no error, assign it
	calypsoClient.ltsReply = ltsReply

	//Create a signer, darc for data point #1
	darc1 := darc.NewDarc(darc.InitRules([]darc.Identity{provider1.Identity()},
		[]darc.Identity{provider1.Identity()}), []byte("Provider1"))
	// provider1 is the owner, while reader1 is allowed to do read
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(provider1.Identity().String()))
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader1.Identity().String()))
	require.NotNil(t, darc1)
	require.Nil(t, err)
	_, err = calypsoClient.SpawnDarc(admin, gDarc, *darc1, 10)
	require.Nil(t, err)
	//Create a similar darc for provider2, reader2
	darc2 := darc.NewDarc(darc.InitRules([]darc.Identity{provider2.Identity()},
		[]darc.Identity{provider2.Identity()}), []byte("Provider2"))
	// provider1 is the owner, while reader1 is allowed to do read
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(provider2.Identity().String()))
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader2.Identity().String()))
	//Spawn it
	_, err = calypsoClient.SpawnDarc(admin, gDarc, *darc2, 10)
	require.Nil(t, err)
	//Create a secret key
	key1 := []byte("secret key 1")
	//Create a Write instance
	write1 := NewWrite(cothority.Suite, calypsoClient.ltsReply.LTSID,
		darc1.GetBaseID(), calypsoClient.ltsReply.X, key1)
	//Write it to calypso
	wr1, err := calypsoClient.AddWrite(write1, provider1, *darc1, 10)
	require.Nil(t, err)
	require.NotNil(t, wr1.InstanceID)
	//Get the write proof
	prWr1, err := calypsoClient.WaitProof(wr1.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.NotNil(t, prWr1)

	re1, err := calypsoClient.AddRead(prWr1, reader1, *darc1, 10)
	require.Nil(t, err)
	prRe1, err := calypsoClient.WaitProof(re1.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.True(t, prRe1.InclusionProof.Match())

	key2 := []byte("secret key 2")
	//Create a Write instance
	write2 := NewWrite(cothority.Suite, calypsoClient.ltsReply.LTSID,
		darc2.GetBaseID(), calypsoClient.ltsReply.X, key2)
	wr2, err := calypsoClient.AddWrite(write2, provider2, *darc2, 10)
	require.Nil(t, err)
	prWr2, err := calypsoClient.WaitProof(wr2.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.True(t, prWr2.InclusionProof.Match())
	re2, err := calypsoClient.AddRead(prWr2, reader2, *darc2, 10)
	require.Nil(t, err)
	prRe2, err := calypsoClient.WaitProof(re2.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.True(t, prRe2.InclusionProof.Match())

	// Make sure you can't decrypt with non-matching proofs
	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	require.NotNil(t, err)
	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	require.NotNil(t, err)

	// Make sure you can actually decrypt
	dk1, err := calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.Nil(t, err)
	require.True(t, dk1.X.Equal(calypsoClient.ltsReply.X))
	keyCopy1, err := DecodeKey(cothority.Suite, calypsoClient.ltsReply.X,
		dk1.Cs, dk1.XhatEnc, reader1.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key1, keyCopy1)
}

func TestClient_ReadBatch(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	l.GetServices(servers, calypsoID)
	defer l.CloseAll()

	admin := darc.NewSignerEd25519(nil, nil)
	provider1 := darc.NewSignerEd25519(nil, nil)
	reader := darc.NewSignerEd25519(nil, nil)
	provider2 := darc.NewSignerEd25519(nil, nil)
	//reader2 := darc.NewSignerEd25519(nil, nil)
	// Initialise the genesis message and send it to the service.
	// The admin has the privilege to spawn darcs
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + byzcoin.ContractDarcID},
		admin.Identity())

	msg.BlockInterval = 100 * time.Millisecond
	require.Nil(t, err)
	// The darc inside it should be valid.
	gDarc := msg.GenesisDarc
	require.Nil(t, gDarc.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.Nil(t, err)
	//Create a Calypso Client (Byzcoin + Onet)
	calypsoClient := NewClient(c)

	//Create the LTS
	ltsReply, err := calypsoClient.CreateLTS()
	require.Nil(t, err)
	//If no error, assign it
	calypsoClient.ltsReply = ltsReply

	batchData := make([]*BatchData, 2)

	//Create a signer, darc for data point #1
	darc1 := darc.NewDarc(darc.InitRules([]darc.Identity{provider1.Identity()},
		[]darc.Identity{provider1.Identity()}), []byte("Provider1"))
	// provider1 is the owner, while reader is allowed to do read
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(provider1.Identity().String()))
	darc1.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader.Identity().String()))
	require.NotNil(t, darc1)
	require.Nil(t, err)
	_, err = calypsoClient.SpawnDarc(admin, gDarc, *darc1, 10)
	require.Nil(t, err)
	//Create a similar darc for provider2, reader
	darc2 := darc.NewDarc(darc.InitRules([]darc.Identity{provider2.Identity()},
		[]darc.Identity{provider2.Identity()}), []byte("Provider2"))
	// provider1 is the owner, while reader is allowed to do read
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(provider2.Identity().String()))
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader.Identity().String()))
	//Spawn it
	_, err = calypsoClient.SpawnDarc(admin, gDarc, *darc2, 10)
	require.Nil(t, err)
	//Create a secret key
	key1 := []byte("secret key 1")
	//Create a Write instance
	write1 := NewWrite(cothority.Suite, calypsoClient.ltsReply.LTSID,
		darc1.GetBaseID(), calypsoClient.ltsReply.X, key1)
	//Write it to calypso
	wr1, err := calypsoClient.AddWrite(write1, provider1, *darc1, 10)
	require.Nil(t, err)
	require.NotNil(t, wr1.InstanceID)
	//Get the write proof
	prWr1, err := calypsoClient.WaitProof(wr1.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.NotNil(t, prWr1)
	require.True(t, prWr1.InclusionProof.Match())
	batchData[0] = &BatchData{Proof: prWr1, Signer: reader, Darc: *darc1}

	//re1, err := calypsoClient.AddRead(prWr1, reader, *darc1, 10)
	//require.Nil(t, err)
	//prRe1, err := calypsoClient.WaitProof(re1.InstanceID, time.Second, nil)
	//require.Nil(t, err)
	//require.True(t, prRe1.InclusionProof.Match())

	key2 := []byte("secret key 2")
	//Create a Write instance
	write2 := NewWrite(cothority.Suite, calypsoClient.ltsReply.LTSID,
		darc2.GetBaseID(), calypsoClient.ltsReply.X, key2)
	wr2, err := calypsoClient.AddWrite(write2, provider2, *darc2, 10)
	require.Nil(t, err)
	require.NotNil(t, wr2.InstanceID)
	prWr2, err := calypsoClient.WaitProof(wr2.InstanceID, time.Second, nil)
	require.Nil(t, err)
	require.NotNil(t, prWr1)
	require.True(t, prWr2.InclusionProof.Match())
	batchData[1] = &BatchData{Proof: prWr2, Signer: reader, Darc: *darc2}

	//re2, err := calypsoClient.AddRead(prWr2, reader, *darc2, 10)
	//require.Nil(t, err)
	//prRe2, err := calypsoClient.WaitProof(re2.InstanceID, time.Second, nil)
	//require.Nil(t, err)
	//require.True(t, prRe2.InclusionProof.Match())

	batchReply, err := calypsoClient.AddReadBatch(batchData, 4)
	require.Nil(t, err)
	require.NotNil(t, batchReply)

	// Make sure you can't decrypt with non-matching proofs
	//_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	//require.NotNil(t, err)
	//_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	//require.NotNil(t, err)

	// Make sure you can actually decrypt
	require.True(t, batchReply.IIDValid[0])
	require.True(t, batchReply.IIDValid[1])
	rp1, err := calypsoClient.WaitProof(batchReply.IIDBatch[0].ID, 0, nil)
	require.Nil(t, err)
	rp2, err := calypsoClient.WaitProof(batchReply.IIDBatch[1].ID, 0, nil)
	require.Nil(t, err)
	dk1, err := calypsoClient.DecryptKey(&DecryptKey{Read: *rp1, Write: *prWr1})
	require.Nil(t, err)
	require.True(t, dk1.X.Equal(calypsoClient.ltsReply.X))
	keyCopy1, err := DecodeKey(cothority.Suite, calypsoClient.ltsReply.X,
		dk1.Cs, dk1.XhatEnc, reader.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key1, keyCopy1)

	dk2, err := calypsoClient.DecryptKey(&DecryptKey{Read: *rp2, Write: *prWr2})
	require.Nil(t, err)
	require.True(t, dk2.X.Equal(calypsoClient.ltsReply.X))
	keyCopy2, err := DecodeKey(cothority.Suite, calypsoClient.ltsReply.X,
		dk2.Cs, dk2.XhatEnc, reader.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key2, keyCopy2)
}
