package service

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestGetMixes_UserNotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.register(0, false)

	_, err := s.GetMixes(&evoting.GetMixes{Token: ""})
	assert.NotNil(t, errNotLoggedIn, err)
}

func TestGetMixes_UserNotPart(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(1, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Shuffled,
	}
	_ = election.GenChain(3)

	_, err := s.GetMixes(&evoting.GetMixes{Token: token, ID: election.ID})
	assert.NotNil(t, errNotPart, err)
}

func TestGetMixes_ElectionNotShuffled(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
	}
	_ = election.GenChain(3)

	_, err := s.GetMixes(&evoting.GetMixes{Token: token, ID: election.ID})
	assert.NotNil(t, errNotShuffled, err)
}

func TestGetMixes_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Shuffled,
	}
	_ = election.GenChain(10)

	r, _ := s.GetMixes(&evoting.GetMixes{Token: token, ID: election.ID})
	assert.Equal(t, 3, len(r.Mixes))
}
