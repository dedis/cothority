package user

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	contracts2 "go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/onet/v3/log"
	"testing"
)

func TestMain(m *testing.M) {
	log.SetShowTime(true)
	log.MainTest(m)
}

var genesisRules = []string{
	"spawn:" + contracts.ContractCredentialID,
	"spawn:" + contracts.ContractSpawnerID,
	"spawn:" + contracts2.ContractCoinID,
	"invoke:" + contracts2.ContractCoinID + ".mint",
	"invoke:" + contracts2.ContractCoinID + ".transfer",
}

func TestUser_NewFromByzcoin(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)
	pubArgs := user.GetPublic()
	require.Equal(t, userName, string(pubArgs[0].Value))
}

func TestUser_SwitchKey(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)

	log.Lvl1("All set up - switching key for the 1st time")

	signer1 := user.Signer.Identity()
	require.NoError(t, user.SwitchKey())
	signer2 := user.Signer.Identity()
	require.False(t, signer1.Equal(&signer2))

	log.Lvl1("Switching key for the 2nd time")

	require.NoError(t, user.SwitchKey())
	require.False(t, signer1.Equal(&signer2))
	signer3 := user.Signer.Identity()
	require.False(t, signer2.Equal(&signer3))
}

// TestUser_AddDevice
func TestUser_AddDevice(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)

	genesisCoin := bct.CreateCoin(nil, 1e9)
	bct.TransferCoin(nil, genesisCoin, user.CoinID, 1e9)

	log.Lvl1("All set up - adding device")
	deviceStr, err := user.AddDevice("https://something.com", "supplementary")
	require.NoError(t, err)

	user2, err := NewFromURL(bct.Client, deviceStr)
	require.NoError(t, err)

	require.Equal(t, user.CredIID, user2.CredIID)
	user2ID := user2.Signer.Identity()
	require.False(t, user.Signer.Identity().Equal(&user2ID))
	devices, err := user2.GetDevices()
	require.NoError(t, err)
	require.Equal(t, 2, len(devices))
	require.NoError(t, user2.SwitchKey())

	_, err = user2.AddDevice("https://something.com", "s2")
	require.NoError(t, err)

	devices, err = user2.GetDevices()
	require.NoError(t, err)
	require.Equal(t, 3, len(devices))
}

func TestUser_CreateNewUser(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)

	genesisCoin := bct.CreateCoin(nil, 1e9)
	bct.TransferCoin(nil, genesisCoin, user.CoinID, 1e9)

	_, err = user.CreateNewUser("testUser2", "test@user2.com")
	require.NoError(t, err)
	require.NoError(t, user.UpdateCredential())
	require.Equal(t, 32, len(user.credStruct.GetPublic(contracts.APContacts)))

	_, err = user.CreateNewUser("testUser3", "test@user3.com")
	require.NoError(t, err)
	require.NoError(t, user.UpdateCredential())
	require.Equal(t, 64, len(user.credStruct.GetPublic(contracts.APContacts)))
}

func TestUser_Recover(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	userName := "testUser"
	user, err := NewFromByzcoin(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer,
		userName)
	require.NoError(t, err)

	genesisCoin := bct.CreateCoin(nil, 1e9)
	bct.TransferCoin(nil, genesisCoin, user.CoinID, 1e9)

	user2, err := user.CreateNewUser("testUser2", "test@user2.com")
	require.NoError(t, err)
	require.NoError(t, user.UpdateCredential())
	require.Equal(t, 32, len(user.credStruct.GetPublic(contracts.APContacts)))

	recoverStr, err := user.Recover(user2.CredIID, "https://something.com")
	require.NoError(t, err)

	user2recover, err := NewFromURL(bct.Client, recoverStr)
	require.NoError(t, err)
	require.Equal(t, user2.CredIID, user2recover.CredIID)
}
