package service

import (
	"testing"
	"time"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestOpen_NotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.register(0, false)

	_, err := s.Open(&evoting.Open{Token: ""})
	assert.NotNil(t, errNotLoggedIn, err)
}

func TestOpen_NotAdmin(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	_, err := s.Open(&evoting.Open{Token: token})
	assert.NotNil(t, errNotAdmin, err)
}

func TestOpen_InvalidMasterID(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, true)

	_, err := s.Open(&evoting.Open{Token: token})
	assert.NotNil(t, err)
}

func TestOpen_CloseConnection(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, true)

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	local.CloseAll()
	_, err := s.Open(&evoting.Open{Token: token, ID: master.ID})
	assert.NotNil(t, err)
}

func TestOpen_EndLessThanNow(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, true)

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	election := &lib.Election{}
	_, err := s.Open(&evoting.Open{Token: token, ID: master.ID, Election: election})
	assert.NotNil(t, err)
}

func TestOpen_Full(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, true)

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	election := &lib.Election{End: time.Now().Unix() + 3600}
	r, _ := s.Open(&evoting.Open{Token: token, ID: master.ID, Election: election})
	assert.NotNil(t, r)

	client := skipchain.NewClient()
	chain, _ := client.GetUpdateChain(roster, r.ID)
	_, blob, _ := network.Unmarshal(chain.Update[1].Data, cothority.Suite)
	assert.Equal(t, r.ID, blob.(*lib.Election).ID)

	assert.Equal(t, r.Key, s.secrets[r.ID.Short()].X)
}
