package service

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestCast_InvalidElectionID(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, _, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0}

	_, err := s.Cast(&evoting.Cast{Token: "0", ID: []byte{}})
	assert.NotNil(t, err)
}

func TestCast_UserNotPart(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["1"] = &stamp{user: 1}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	_, err := s.Cast(&evoting.Cast{Token: "1", ID: election.ID})
	assert.Equal(t, ERR_NOT_PART, err)
}

func TestCast_ElectionAlreadyClosed(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["0"] = &stamp{user: 0}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.SHUFFLED,
	}
	_ = election.GenChain(3)

	_, err := s.Cast(&evoting.Cast{Token: "0", ID: election.ID})
	assert.Equal(t, ERR_ALREADY_CLOSED, err)

	election = &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{0},
		Stage:   lib.DECRYPTED,
	}
	_ = election.GenChain(3)

	_, err = s.Cast(&evoting.Cast{Token: "0", ID: election.ID})
	assert.Equal(t, ERR_ALREADY_CLOSED, err)
}

func TestCast_Full(t *testing.T) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	s.state.log["1000"] = &stamp{user: 1000}

	election := &lib.Election{
		Roster:  roster,
		Creator: 0,
		Users:   []uint32{1000},
		Stage:   lib.RUNNING,
	}
	_ = election.GenChain(3)

	ballot := &lib.Ballot{User: 1000}
	r, _ := s.Cast(&evoting.Cast{Token: "1000", ID: election.ID, Ballot: ballot})
	assert.NotNil(t, r)

	client := skipchain.NewClient()
	chain, _ := client.GetUpdateChain(roster, election.ID)
	_, blob, _ := network.Unmarshal(chain.Update[len(chain.Update)-1].Data, lib.Suite)
	assert.Equal(t, ballot.User, blob.(*lib.Ballot).User)
}
