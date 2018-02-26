package service

import (
	"sort"
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestReconstruct_UserNotLoggedIn(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.register(0, false)

	_, err := s.Reconstruct(&evoting.Reconstruct{Token: ""})
	assert.Equal(t, errNotLoggedIn, err)
}

func TestReconstruct_ElectionNotDecrypted(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Shuffled,
		Data:    []byte{},
	}
	_ = election.GenChain(3)

	_, err := s.Reconstruct(&evoting.Reconstruct{Token: token, ID: election.ID})
	assert.Equal(t, errNotDecrypted, err)
}

func TestReconstruct_Full(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	token := s.state.register(0, false)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Decrypted,
		Data:    []byte{},
	}
	_ = election.GenChain(7)

	r, _ := s.Reconstruct(&evoting.Reconstruct{Token: token, ID: election.ID})
	assert.Equal(t, 7, len(r.Points))

	messages := make([]int, 7)
	for i, point := range r.Points {
		data, _ := point.Data()
		messages[i] = int(data[0])
	}
	sort.Ints(messages)
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6}, messages)
}
