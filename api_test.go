package ocs_test

import (
	"testing"

	"github.com/dedis/kyber/suites"
	"github.com/dedis/onchain-secrets"
	"github.com/dedis/onchain-secrets/darc"
	_ "github.com/dedis/onchain-secrets/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestUpdateDarc(t *testing.T) {
	owner := &darc.Signer{Ed25519: darc.NewEd25519Signer(nil, nil)}
	ownerID, err := darc.NewIdentity(nil, darc.NewEd25519Identity(owner.Ed25519.Point))
	require.Nil(t, err)
	user1 := &darc.Signer{Ed25519: darc.NewEd25519Signer(nil, nil)}
	user1ID, err := darc.NewIdentity(nil, darc.NewEd25519Identity(user1.Ed25519.Point))
	user2 := &darc.Signer{Ed25519: darc.NewEd25519Signer(nil, nil)}
	user2ID, err := darc.NewIdentity(nil, darc.NewEd25519Identity(user2.Ed25519.Point))
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
	cl := ocs.NewClient()
	writers := &darc.Darc{}
	ocs, cerr := cl.CreateSkipchain(roster, writers)
	require.Nil(t, cerr)
	log.Printf("%#v", *ocs)
	_, cerr = cl.EditAccount(ocs, darc0)
	require.Nil(t, cerr)
	_, cerr = cl.EditAccount(ocs, darc1)
	require.Nil(t, cerr)
	_, cerr = cl.EditAccount(ocs, darc2)
	require.Nil(t, cerr)

	path, cerr := cl.GetLatestDarc(ocs, darc2.GetID())
	require.Nil(t, cerr)
	require.NotNil(t, path)
	require.Equal(t, 1, len(*path))
	path, cerr = cl.GetLatestDarc(ocs, darc0.GetID())
	require.Nil(t, cerr)
	require.NotNil(t, path)
	require.Equal(t, 3, len(*path))
}
