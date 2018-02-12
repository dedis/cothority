package lib

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/stretchr/testify/assert"
)

func TestFetchElection(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	_, err := FetchElection(roster, []byte{})
	assert.NotNil(t, err)

	election := &Election{Roster: roster, Stage: RUNNING}
	_ = election.GenChain(10)

	e, _ := FetchElection(roster, election.ID)
	assert.Equal(t, election.ID, e.ID)
	assert.Equal(t, RUNNING, int(e.Stage))

	election = &Election{Roster: roster, Stage: SHUFFLED}
	_ = election.GenChain(10)

	e, _ = FetchElection(roster, election.ID)
	assert.Equal(t, election.ID, e.ID)
	assert.Equal(t, SHUFFLED, int(e.Stage))

	election = &Election{Roster: roster, Stage: DECRYPTED}
	_ = election.GenChain(10)

	e, _ = FetchElection(roster, election.ID)
	assert.Equal(t, election.ID, e.ID)
	assert.Equal(t, DECRYPTED, int(e.Stage))

	election = &Election{Roster: roster, Stage: SHUFFLED}
	_ = election.GenChain(10)
	_ = election.Store(&Mix{Proof: []byte{}})

	e, _ = FetchElection(roster, election.ID)
	assert.Equal(t, CORRUPT, int(e.Stage))
}

func TestStore(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	election := &Election{Roster: roster, Stage: RUNNING}
	_ = election.GenChain(10)

	election.Store(&Ballot{User: 1000})

	chain, _ := client.GetUpdateChain(roster, election.ID)
	_, blob, _ := network.Unmarshal(chain.Update[len(chain.Update)-1].Data, Suite)
	assert.Equal(t, uint32(1000), blob.(*Ballot).User)
}

func TestBox(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	election := &Election{Roster: roster, Stage: RUNNING}
	_ = election.GenChain(10)

	box, _ := election.Box()
	assert.Equal(t, 10, len(box.Ballots))
}

func TestMixes(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	election := &Election{Roster: roster, Stage: SHUFFLED}
	_ = election.GenChain(10)

	mixes, _ := election.Mixes()
	assert.Equal(t, 3, len(mixes))
}

func TestPartials(t *testing.T) {
	local := onet.NewLocalTest(Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenBigTree(3, 3, 1, true)

	election := &Election{Roster: roster, Stage: DECRYPTED}
	_ = election.GenChain(10)

	partials, _ := election.Partials()
	assert.Equal(t, 3, len(partials))
}

func TestIsUser(t *testing.T) {
	e := &Election{Creator: 0, Users: []uint32{0}}
	assert.True(t, e.IsUser(0))
	assert.False(t, e.IsUser(1))
}

func TestIsCreator(t *testing.T) {
	e := &Election{Creator: 0, Users: []uint32{0, 1}}
	assert.True(t, e.IsCreator(0))
	assert.False(t, e.IsCreator(1))
}
