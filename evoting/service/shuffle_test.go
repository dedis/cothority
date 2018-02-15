package service

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestShuffle_UserNotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log.Store("0", &stamp{user: 0, admin: false})

	_, err := s.Shuffle(&evoting.Shuffle{Token: ""})
	assert.Equal(t, errNotLoggedIn, err)
}

func TestShuffle_UserNotAdmin(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log.Store("1", &stamp{user: 1, admin: false})

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "1", ID: election.ID})
	assert.Equal(t, errNotAdmin, err)
}

func TestShuffle_UserNotCreator(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log.Store("1", &stamp{user: 1, admin: true})

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0, 1},
		Stage:   lib.Running,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "1", ID: election.ID})
	assert.Equal(t, errNotCreator, err)
}

func TestShuffle_ElectionClosed(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log.Store("0", &stamp{user: 0, admin: true})

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Shuffled,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.Equal(t, errAlreadyShuffled, err)

	election = &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Decrypted,
	}
	_ = election.GenChain(3)

	_, err = s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.Equal(t, errAlreadyShuffled, err)
}

func TestShuffle_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log.Store("0", &stamp{user: 0, admin: true})

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
	}
	_ = election.GenChain(3)

	r, _ := s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.NotNil(t, r)
}
