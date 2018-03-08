package service

import (
	"testing"

	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/require"
)

func TestUpdateDarc(t *testing.T) {
	owner := darc.NewSignerEd25519(nil, nil)
	ownerID := owner.Identity()
	user1 := darc.NewSignerEd25519(nil, nil)
	user1ID := user1.Identity()
	user2 := darc.NewSignerEd25519(nil, nil)
	user2ID := user2.Identity()
	owners := []*darc.Identity{ownerID}

	darc0 := darc.NewDarc(&owners, nil, []byte("desc"))
	darc1 := darc0.Copy()
	darc1.AddUser(user1ID)
	darc1.SetEvolution(darc0, nil, owner)
	darc2 := darc1.Copy()
	darc2.AddUser(user2ID)
	darc2.SetEvolution(darc1, nil, owner)

	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(3, true)
	defer local.CloseAll()
	cl := NewClient()
	writers := &darc.Darc{}
	ocs, err := cl.CreateSkipchain(roster, writers)
	require.Nil(t, err)
	_, err = cl.EditAccount(ocs, darc0)
	require.Nil(t, err)
	_, err = cl.EditAccount(ocs, darc1)
	require.Nil(t, err)
	_, err = cl.EditAccount(ocs, darc2)
	require.Nil(t, err)

	path, err := cl.GetLatestDarc(ocs, darc2.GetID())
	require.Nil(t, err)
	require.NotNil(t, path)
	require.Equal(t, 1, len(*path))
	path, err = cl.GetLatestDarc(ocs, darc0.GetID())
	require.Nil(t, err)
	require.NotNil(t, path)
	require.Equal(t, 3, len(*path))
}
