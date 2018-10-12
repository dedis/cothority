package calypso

import (
	"testing"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/byzcoin"
	"time"
	"github.com/stretchr/testify/require"
	"github.com/dedis/onet"
)

// waitInstID this is a utility testing function to wait for the proof
// of the write transaction
// TODO(islam): This function is the same procedure as in calypso/service_test
// We want to create an interface for both ts and client and have them both have
// waitInstID once
func (c *Client) waitInstID(t *testing.T, instID byzcoin.InstanceID,
	wait time.Duration) *byzcoin.Proof {
	var err error
	var pr *byzcoin.Proof
	for i := 0; i < 10; i++ {
		pr, err = c.byzcoin.WaitProof(instID, wait, nil)
		if err == nil {
			require.Nil(t, pr.Verify(c.byzcoin.ID))
			break
		}
	}
	if err != nil {
		require.Fail(t, "didn't find proof")
	}
	return pr
}


func TestClient_CreateLTS(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	l.GetServices(servers, CalypsoID)
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
func TestClient_DecryptKey(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	l.GetServices(servers, CalypsoID)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + ContractWriteID, "spawn:" + ContractReadID}, signer.Identity())
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

	ltsReply, err := calypsoClient.CreateLTS()
	require.Nil(t, err)

	calypsoClient.ltsReply = ltsReply

	key1 := []byte("secret key 1")
	wr1, err := calypsoClient.AddWrite(key1, signer)
	require.Nil(t, err)
	require.NotNil(t, wr1.InstanceID)
	prWr1 := calypsoClient.waitInstID(t, wr1.InstanceID, 2*time.Second)
	re1, err := calypsoClient.AddRead(prWr1, signer)
	require.Nil(t, err)
	prRe1 := calypsoClient.waitInstID(t, re1.InstanceID, 2*time.Second)
	key2 := []byte("secret key 2")
	wr2, err := calypsoClient.AddWrite(key2, signer)
	require.Nil(t, err)
	prWr2 := calypsoClient.waitInstID(t, wr2.InstanceID, 2*time.Second)
	require.Nil(t, err)
	re2, err := calypsoClient.AddRead(prWr2, signer)
	require.Nil(t, err)
	prRe2 := calypsoClient.waitInstID(t, re2.InstanceID, 2*time.Second)

	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	require.NotNil(t, err)
	_, err = calypsoClient.DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	require.NotNil(t, err)

	dk1, err := calypsoClient.DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.Nil(t, err)
	require.True(t, dk1.X.Equal(ltsReply.X))
	keyCopy1, err := DecodeKey(cothority.Suite, ltsReply.X, dk1.Cs, dk1.XhatEnc, signer.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key1, keyCopy1)

	dk2, err := calypsoClient.DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr2})
	require.Nil(t, err)
	require.True(t, dk2.X.Equal(ltsReply.X))
	keyCopy2, err := DecodeKey(cothority.Suite, ltsReply.X, dk2.Cs, dk2.XhatEnc, signer.Ed25519.Secret)
	require.Nil(t, err)
	require.Equal(t, key2, keyCopy2)
}