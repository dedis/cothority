package service

import (
	"testing"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestLogin_InvalidMasterID(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	_, err := s.Login(&evoting.Login{ID: nil})
	assert.NotNil(t, err)
}

func TestLogin_InvalidLink(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	master := &lib.Master{Roster: roster}
	master.GenChain([]byte{})

	_, err := s.Login(&evoting.Login{ID: master.ID})
	assert.NotNil(t, err)
}

func TestLogin_InvalidSignature(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	x, X := lib.RandomKeyPair()
	master := &lib.Master{Roster: roster, Key: X}
	master.GenChain()

	l := &evoting.Login{User: 0, ID: master.ID}
	l.Sign(x)
	l.Signature = append(l.Signature, byte(0))

	_, err := s.Login(l)
	assert.NotNil(t, err)
}

func TestLogin_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.Running,
	}
	_ = election.GenChain(3)

	x, X := lib.RandomKeyPair()
	master := &lib.Master{Roster: roster, Key: X}
	master.GenChain(election.ID)

	l := &evoting.Login{User: 0, ID: master.ID}
	l.Sign(x)

	r, _ := s.Login(l)
	assert.Equal(t, election.ID, r.Elections[0].ID)
}
