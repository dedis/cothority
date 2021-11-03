package user

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"testing"
)

func TestUserBuilder_CreateFromSpawner(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	bct.AddGenesisRules(genesisRules...)
	bct.CreateByzCoin()
	defer bct.CloseAll()

	ub, err := NewUserBuilder("test")
	require.NoError(t, err)
	user, err := ub.CreateFromDarc(bct.Client, bct.GenesisDarc.GetBaseID(), bct.Signer)
	require.NoError(t, err)

	genesisCoin := bct.CreateCoin(nil, 1e9)
	bct.TransferCoin(nil, genesisCoin, user.CoinID, 1e9)

	ub2, err := NewUserBuilder("test2")
	require.NoError(t, err)
	as := user.Spawner.Start(user.CoinID, user.Signer)
	user2, err := ub2.CreateFromSpawner(as)
	require.NoError(t, err)
	require.False(t, user.CredIID.Equal(user2.CredIID))
}
