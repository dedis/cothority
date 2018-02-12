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
	s.state.log["0"] = &stamp{user: 0, admin: false}

	_, err := s.Shuffle(&evoting.Shuffle{Token: ""})
	assert.Equal(t, ERR_NOT_LOGGED_IN, err)
}

func TestShuffle_UserNotAdmin(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["1"] = &stamp{user: 1, admin: false}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "1", ID: election.ID})
	assert.Equal(t, ERR_NOT_ADMIN, err)
}

func TestShuffle_UserNotCreator(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["1"] = &stamp{user: 1, admin: true}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0, 1},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "1", ID: election.ID})
	assert.Equal(t, ERR_NOT_CREATOR, err)
}

func TestShuffle_ElectionClosed(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: true}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.SHUFFLED,
	}
	_ = election.GenChain(3)

	_, err := s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.Equal(t, ERR_ALREADY_SHUFFLED, err)

	election = &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.DECRYPTED,
	}
	_ = election.GenChain(3)

	_, err = s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.Equal(t, ERR_ALREADY_SHUFFLED, err)
}

func TestShuffle_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: true}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	r, _ := s.Shuffle(&evoting.Shuffle{Token: "0", ID: election.ID})
	assert.NotNil(t, r)
}
