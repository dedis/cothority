package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/onet"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestGetBox_NotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: false}

	_, err := s.GetBox(&evoting.GetBox{Token: ""})
	assert.NotNil(t, ERR_NOT_LOGGED_IN, err)
}

func TestGetBox_NotPart(t *testing.T) {
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

	_, err := s.GetBox(&evoting.GetBox{Token: "1", ID: election.ID})
	assert.NotNil(t, ERR_NOT_PART, err)
}

func TestGetBox_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0, admin: false}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	r, _ := s.GetBox(&evoting.GetBox{Token: "0", ID: election.ID})
	assert.Equal(t, 3, len(r.Box.Ballots))
}
