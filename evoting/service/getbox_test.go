package service

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestGetBox_NotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.register(0, false)

	_, err := s.GetBox(&evoting.GetBox{Token: ""})
	assert.NotNil(t, errNotLoggedIn, err)
}

func TestGetBox_NotPart(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(1, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
		Data:    []byte{},
	}
	_ = election.GenChain(3)

	_, err := s.GetBox(&evoting.GetBox{Token: token, ID: election.ID})
	assert.NotNil(t, errNotPart, err)
}

func TestGetBox_Full(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
		Data:    []byte{},
	}
	_ = election.GenChain(3)

	r, _ := s.GetBox(&evoting.GetBox{Token: token, ID: election.ID})
	assert.Equal(t, 3, len(r.Box.Ballots))
}
