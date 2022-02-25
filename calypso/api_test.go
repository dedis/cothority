package calypso

import (
	"encoding/binary"
	"testing"
	"time"

	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/key"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/onet/v3"
)

// Tests the client function CreateLTS
func TestClient_CreateLTS(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:dummy", "spawn:" + ContractLongTermSecretID},
		signer.Identity())
	msg.BlockInterval = 500 * time.Millisecond
	require.NoError(t, err)
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	// Create the clients
	c, _, err := byzcoin.NewLedger(msg, false)
	require.NoError(t, err)
	calypsoClient := NewClient(c)
	for _, who := range roster.List {
		err := calypsoClient.Authorize(who, c.ID)
		require.NoError(t, err)
	}

	// Invoke CreateLTS
	ltsReply, err := calypsoClient.CreateLTS(roster, d.GetBaseID(), []darc.Signer{signer}, []uint64{1})
	require.NoError(t, err)
	require.NotNil(t, ltsReply.ByzCoinID)
	require.NotNil(t, ltsReply.InstanceID)
	require.NotNil(t, ltsReply.X)
}

func TestClient_Authorize(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:dummy", "spawn:" + ContractLongTermSecretID},
		signer.Identity())
	msg.BlockInterval = 500 * time.Millisecond
	require.NoError(t, err)
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	// Create the clients
	c, _, err := byzcoin.NewLedger(msg, false)
	require.NoError(t, err)

	reply := &AuthorizeReply{}
	who := roster.List[0]
	cl := NewClient(nil)
	ts := time.Now().Unix() - 100
	sigMsg := append(c.ID, make([]byte, 8)...)
	binary.LittleEndian.PutUint64(sigMsg[32:], uint64(ts))
	sig, err := schnorr.Sign(cothority.Suite, who.GetPrivate(), sigMsg)
	require.NoError(t, err)
	auth := &Authorize{
		ByzCoinID: c.ID,
		Timestamp: ts,
		Signature: sig,
	}
	err = cl.c.SendProtobuf(who, auth, reply)
	require.Error(t, err)

	auth.Timestamp += 200
	binary.LittleEndian.PutUint64(sigMsg[32:], uint64(auth.Timestamp))
	auth.Signature, err = schnorr.Sign(cothority.Suite, who.GetPrivate(), sigMsg)
	require.NoError(t, err)
	err = cl.c.SendProtobuf(who, auth, reply)
	require.Error(t, err)

	auth.Timestamp -= 100
	binary.LittleEndian.PutUint64(sigMsg[32:], uint64(auth.Timestamp))
	auth.Signature, err = schnorr.Sign(cothority.Suite, who.GetPrivate(), sigMsg)
	require.NoError(t, err)
	err = cl.c.SendProtobuf(who, auth, reply)
	require.NoError(t, err)
}

// TODO(jallen): Write TestClient_Reshare (and add api.go part too, I guess)

// Tests the client api's AddRead, AddWrite, DecryptKey
func TestClient_Calypso(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	admin := darc.NewSignerEd25519(nil, nil)
	adminCt := uint64(1)
	provider1 := darc.NewSignerEd25519(nil, nil)
	reader1 := darc.NewSignerEd25519(nil, nil)
	provider2 := darc.NewSignerEd25519(nil, nil)
	reader2 := darc.NewSignerEd25519(nil, nil)
	// Initialise the genesis message and send it to the service.
	// The admin has the privilege to spawn darcs
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + ContractLongTermSecretID},
		admin.Identity())

	msg.BlockInterval = 500 * time.Millisecond
	require.NoError(t, err)
	// The darc inside it should be valid.
	gDarc := msg.GenesisDarc
	require.Nil(t, gDarc.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.NoError(t, err)
	//Create a Calypso Client (Byzcoin + Onet)
	calypsoClient := NewClient(c)

	//Create the LTS
	for _, who := range roster.List {
		err := calypsoClient.Authorize(who, c.ID)
		require.NoError(t, err)
	}
	ltsReply, err := calypsoClient.CreateLTS(roster, gDarc.GetBaseID(), []darc.Signer{admin}, []uint64{adminCt})
	adminCt++
	require.NoError(t, err)
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
	require.NoError(t, err)
	_, err = calypsoClient.SpawnDarc(admin, adminCt, gDarc, *darc1, 10)
	adminCt++
	require.NoError(t, err)

	//Create a similar darc for provider2, reader2
	darc2 := darc.NewDarc(darc.InitRules([]darc.Identity{provider2.Identity()},
		[]darc.Identity{provider2.Identity()}), []byte("Provider2"))
	// provider1 is the owner, while reader1 is allowed to do read
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(provider2.Identity().String()))
	darc2.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(reader2.Identity().String()))
	//Spawn it
	_, err = calypsoClient.SpawnDarc(admin, adminCt, gDarc, *darc2, 10)
	adminCt++
	require.NoError(t, err)
	//Create a secret key
	key1 := []byte("secret key 1")
	//Create a Write instance
	write1 := NewWrite(cothority.Suite, calypsoClient.ltsReply.InstanceID,
		darc1.GetBaseID(), calypsoClient.ltsReply.X, key1)
	//Write it to calypso
	wr1, err := calypsoClient.AddWrite(write1, provider1, 1, *darc1, 10)
	require.NoError(t, err)
	require.NotNil(t, wr1.InstanceID)
	//Get the write proof
	prWr1, err := calypsoClient.WaitProof(wr1.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.NotNil(t, prWr1)

	re1, err := calypsoClient.AddRead(prWr1, reader1, 1, 10)
	require.NoError(t, err)
	prRe1, err := calypsoClient.WaitProof(re1.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prRe1.InclusionProof.Match(re1.InstanceID.Slice()))

	key2 := []byte("secret key 2")
	//Create a Write instance
	write2 := NewWrite(cothority.Suite, calypsoClient.ltsReply.InstanceID,
		darc2.GetBaseID(), calypsoClient.ltsReply.X, key2)
	wr2, err := calypsoClient.AddWrite(write2, provider2, 1, *darc2, 10)
	require.NoError(t, err)
	prWr2, err := calypsoClient.WaitProof(wr2.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prWr2.InclusionProof.Match(wr2.InstanceID.Slice()))
	re2, err := calypsoClient.AddRead(prWr2, reader2, 1, 10)
	require.NoError(t, err)
	prRe2, err := calypsoClient.WaitProof(re2.InstanceID, time.Second, nil)
	require.NoError(t, err)
	require.True(t, prRe2.InclusionProof.Match(re2.InstanceID.Slice()))

	// Make sure you can't decrypt with non-matching proofs
	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	require.Error(t, err)
	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	require.Error(t, err)

	// Make sure you can actually decrypt
	dk1, err := calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.NoError(t, err)
	require.True(t, dk1.X.Equal(calypsoClient.ltsReply.X))
	keyCopy1, err := dk1.RecoverKey(reader1.Ed25519.Secret)
	require.NoError(t, err)
	require.Equal(t, key1, keyCopy1)

	// use keyCopy to unlock the stuff in writeInstance.Data
}

// Tests the Calypso system with a simple write/read scenario.
// But it does the signing outside of the `Read` and `Write` methods for
// integration of the MPC signing by OneKey.
func TestClient_Calypso_Simple(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	admin := darc.NewSignerEd25519(nil, nil)
	adminCt := uint64(1)
	user := darc.NewSignerTSM(nil)
	//user := darc.NewSignerEd25519(nil, nil)
	// Initialise the genesis message and send it to the service.
	// The admin has the privilege to spawn darcs
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + ContractLongTermSecretID},
		admin.Identity())

	msg.BlockInterval = 500 * time.Millisecond
	require.NoError(t, err)
	// The darc inside it should be valid.
	gDarc := msg.GenesisDarc
	require.Nil(t, gDarc.Verify(true))
	//Create Ledger
	c, _, err := byzcoin.NewLedger(msg, false)
	require.NoError(t, err)
	//Create a Calypso Client (Byzcoin + Onet)
	calypsoClient := NewClient(c)

	//Create the LTS
	for _, who := range roster.List {
		err := calypsoClient.Authorize(who, c.ID)
		require.NoError(t, err)
	}
	ltsReply, err := calypsoClient.CreateLTS(roster, gDarc.GetBaseID(), []darc.Signer{admin}, []uint64{adminCt})
	adminCt++
	require.NoError(t, err)
	//If no error, assign it
	//calypsoClient.ltsReply = ltsReply

	//Create a signer darc
	userDarc := darc.NewDarc(darc.InitRules([]darc.Identity{user.Identity()},
		[]darc.Identity{user.Identity()}), []byte("Provider1"))
	// user can read and write.
	// This can be changed to two different public keys.
	err = userDarc.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
		expression.InitOrExpr(user.Identity().String()))
	require.NoError(t, err)
	err = userDarc.Rules.AddRule(darc.Action("spawn:"+ContractReadID),
		expression.InitOrExpr(user.Identity().String()))
	require.NoError(t, err)
	require.NotNil(t, userDarc)
	_, err = calypsoClient.SpawnDarc(admin, adminCt, gDarc, *userDarc, 10)
	adminCt++
	require.NoError(t, err)

	data := []byte("Some secret data - or the user's private key")
	// Create a Write structure
	write1, err := NewWriteData(cothority.Suite,
		ltsReply.InstanceID,
		userDarc.GetBaseID(), ltsReply.X, data)
	require.NoError(t, err)

	// Create a write-instance and send it to Byzcoin - here
	// the instruction and the transaction is created manually,
	// so that an external signer can sign the hash of the instruction.
	wrInst, err := ContractWriteSpawnInstruction(write1, userDarc)
	require.NoError(t, err)
	wrInst.SignerCounter = []uint64{1}
	wrInst.SignerIdentities = []darc.Identity{user.Identity()}
	wrTx, err := calypsoClient.bcClient.CreateTransaction(*wrInst)
	require.NoError(t, err)
	digest := wrTx.Instructions.Hash()

	// This signature can be replaced by an external signature.
	//TODO add sepior signature of digest here
	signature, err := user.Sign(digest)
	require.NoError(t, err)
	wrTx.Instructions[0].Signatures = [][]byte{signature}

	// Send the transaction to ByzCoin
	_, err = calypsoClient.bcClient.AddTransactionAndWait(wrTx, 10)
	require.NoError(t, err)
	wrID := wrTx.Instructions[0].DeriveID("")
	proofWr, err := calypsoClient.WaitProof(wrID, time.Second, nil)
	require.NoError(t, err)

	// Create a read-instance and send it to ByzCoin.
	ephemeral := key.NewKeyPair(cothority.Suite)
	readInst, err := ContractReadSpawnInstruction(wrID, ephemeral.Public)
	require.NoError(t, err)
	readInst.SignerCounter = []uint64{2}
	readInst.SignerIdentities = []darc.Identity{user.Identity()}
	readTx, err := calypsoClient.bcClient.CreateTransaction(*readInst)
	require.NoError(t, err)
	digest = readTx.Instructions.Hash()

	// This signature can be replaced by an external signature
	// TODO add signature on digest from MPC nodes
	signature, err = user.Sign(digest)
	require.NoError(t, err)
	readTx.Instructions[0].Signatures = [][]byte{signature}
	readID := readTx.Instructions[0].DeriveID("")

	// Send the transaction to ByzCoin
	_, err = calypsoClient.bcClient.AddTransactionAndWait(readTx, 10)
	require.NoError(t, err)
	proofRd, err := calypsoClient.WaitProof(readID, time.Second,
		nil)
	require.NoError(t, err)

	// Make sure you can actually decrypt
	dk, err := calypsoClient.DecryptKey(&DecryptKey{Read: *proofRd,
		Write: *proofWr})
	require.NoError(t, err)
	require.True(t, dk.X.Equal(ltsReply.X))
	keyCopy, err := dk.RecoverKey(ephemeral.Private)
	require.NoError(t, err)
	var wrCopy Write
	require.NoError(t, proofWr.VerifyAndDecode(cothority.Suite, ContractWriteID,
		&wrCopy))
	dataDecrypt, err := wrCopy.Decrypt(keyCopy)
	require.NoError(t, err)
	require.Equal(t, data, dataDecrypt)
}
