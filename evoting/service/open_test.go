package service

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestOpen_NotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: false}

	_, err := s.Open(&evoting.Open{Token: ""})
	assert.NotNil(t, errNotLoggedIn, err)
}

func TestOpen_NotAdmin(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: false}

	_, err := s.Open(&evoting.Open{Token: "0"})
	assert.NotNil(t, errNotAdmin, err)
}

func TestOpen_InvalidMasterID(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: true}

	_, err := s.Open(&evoting.Open{Token: "0"})
	assert.NotNil(t, err)
}

func TestOpen_CloseConnection(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: true}

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	local.CloseAll()
	_, err := s.Open(&evoting.Open{Token: "0", ID: master.ID})
	assert.NotNil(t, err)
}

func TestOpen_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: true}

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	election := &lib.Election{}
	r, _ := s.Open(&evoting.Open{Token: "0", ID: master.ID, Election: election})
	assert.NotNil(t, r)

	client := skipchain.NewClient()
	chain, _ := client.GetUpdateChain(roster, r.ID)
	_, blob, _ := network.Unmarshal(chain.Update[1].Data, lib.Suite)
	assert.Equal(t, r.ID, blob.(*lib.Election).ID)

	assert.Equal(t, r.Key, s.secrets[r.ID.Short()].X)
}
