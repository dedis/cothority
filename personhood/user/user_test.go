package user

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/onet/v3/log"
	"testing"
)

func TestMain(m *testing.M) {
	log.SetShowTime(true)
	log.MainTest(m)
}

func TestNewFromByzcoin(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules("spawn:" + contracts.ContractCredentialID)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)
	pubArgs := user.GetPublic()
	require.Equal(t, userName, string(pubArgs[0].Value))
}

func TestNew(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules("spawn:" + contracts.ContractCredentialID)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)

	userCopy, err := New(bct.Client, user.CredIID)
	require.NoError(t, err)
	pubArgs := userCopy.GetPublic()
	require.Equal(t, userName, string(pubArgs[0].Value))
}
