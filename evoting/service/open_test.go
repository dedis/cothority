package service

import (
	"testing"
	"time"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestOpen_NotAdmin(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	master := &lib.Master{Roster: roster, Admins: []uint32{0}}
	master.GenChain(nil)

	election := &lib.Election{End: time.Now().Unix() + 3600}
	_, err := s.Open(&evoting.Open{User: 1, ID: master.ID, Election: election})
	assert.Equal(t, errNotAdmin, err)
}

func TestOpen_EndLessThanNow(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	master := &lib.Master{Roster: roster}
	master.GenChain(nil)

	election := &lib.Election{}
	_, err := s.Open(&evoting.Open{User: 0, ID: master.ID, Election: election})
	assert.Equal(t, errInvalidEndDate, err)
}

func TestOpen_Full(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	master := &lib.Master{Roster: roster, Admins: []uint32{0}}
	master.GenChain(nil)

	election := &lib.Election{End: time.Now().Unix() + 3600}
	r, _ := s.Open(&evoting.Open{
		User:      0,
		ID:        master.ID,
		Election:  election,
		Signature: []byte{},
	})
	assert.NotNil(t, r)

	client := skipchain.NewClient()
	chain, _ := client.GetUpdateChain(roster, r.ID)
	tx := lib.NewTransaction(chain.Update[1].Data)

	assert.Equal(t, r.ID, tx.Election.ID)
	assert.Equal(t, r.Key, s.secrets[r.ID.Short()].X)
}
