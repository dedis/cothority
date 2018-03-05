package service

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestLink_WrongPin(t *testing.T) {
	sp := lib.NewSpeed()
	defer sp.Done()
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	_, err := s.Link(&evoting.Link{Pin: ""})
	assert.NotNil(t, errInvalidPin, err)
}

func TestLink_InvalidRoster(t *testing.T) {
	sp := lib.NewSpeed()
	defer sp.Done()
	local := onet.NewLocalTest(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	local.CloseAll()

	_, err := s.Link(&evoting.Link{Pin: s.pin, Roster: roster})
	assert.NotNil(t, err)
}

func TestLink_Full(t *testing.T) {
	sp := lib.NewSpeed()
	defer sp.Done()
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	r, _ := s.Link(&evoting.Link{Pin: s.pin, Roster: roster})
	assert.NotNil(t, r)

	client := skipchain.NewClient()
	chain, _ := client.GetUpdateChain(roster, r.ID)
	_, blob, _ := network.Unmarshal(chain.Update[1].Data, cothority.Suite)
	assert.Equal(t, r.ID, blob.(*lib.Master).ID)
}
