package protocol

import (
	"errors"
	"testing"
	"time"

	"go.dedis.ch/onet/v3"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/skipchain"
)

var shuffleServiceID onet.ServiceID

type shuffleService struct {
	*onet.ServiceProcessor
	user      uint32
	signature []byte
	election  *lib.Election
	skipchain *skipchain.Service
}

func init() {
	new := func(ctx *onet.Context) (onet.Service, error) {
		return &shuffleService{
			ServiceProcessor: onet.NewServiceProcessor(ctx),
			skipchain:        ctx.Service(skipchain.ServiceName).(*skipchain.Service),
		}, nil
	}
	shuffleServiceID, _ = onet.RegisterNewService(NameShuffle, new)
}

func (s *shuffleService) NewProtocol(n *onet.TreeNodeInstance, c *onet.GenericConfig) (
	onet.ProtocolInstance, error) {

	switch n.ProtocolName() {
	case NameShuffle:
		instance, _ := NewShuffle(n)
		shuffle := instance.(*Shuffle)
		shuffle.User = s.user
		shuffle.Election = s.election
		shuffle.Skipchain = s.skipchain
		return shuffle, nil
	default:
		return nil, errors.New("Unknown protocol")
	}
}

func TestShuffleProtocol(t *testing.T) {
	for _, nodes := range []int{3, 5} {
		runShuffle(t, nodes)
	}
}

func TestShuffleNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping", t.Name(), " in short mode")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, tree := local.GenBigTree(5, 5, 1, true)

	services := local.GetServices(nodes, shuffleServiceID)

	dkgs, _ := lib.DKGSimulate(5, 4)
	shared, _ := lib.NewSharedSecret(dkgs[0])
	key := shared.X

	chain, _ := lib.NewSkipchain(services[0].(*shuffleService).skipchain, roster, true)
	election := &lib.Election{
		ID:      chain.Hash,
		Roster:  roster,
		Key:     key,
		Creator: 0,
		Users:   []uint32{0, 1, 2},
	}
	for i := range services {
		services[i].(*shuffleService).election = election
		services[i].(*shuffleService).user = 0
		services[i].(*shuffleService).signature = []byte{}
	}

	tx := lib.NewTransaction(election, election.Creator)
	lib.Store(services[0].(*shuffleService).skipchain, election.ID, tx, nil)

	for i := 0; i < 3; i++ {
		a, b := lib.Encrypt(key, []byte{byte(i)})
		ballot := &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
		tx = lib.NewTransaction(ballot, election.Creator)
		lib.Store(services[0].(*shuffleService).skipchain, election.ID, tx, nil)
	}
	nodes[3].Stop()

	instance, _ := services[0].(*shuffleService).CreateProtocol(NameShuffle, tree)
	shuffle := instance.(*Shuffle)
	shuffle.User = 0
	shuffle.Election = election
	shuffle.Skipchain = services[0].(*shuffleService).skipchain
	shuffle.LeaderParticipates = true
	shuffle.Start()

	select {
	case <-shuffle.Finished:
		box, _ := election.Box(services[0].(*shuffleService).skipchain)
		mixes, _ := election.Mixes(services[0].(*shuffleService).skipchain)

		require.Equal(t, 4, len(mixes))
		in1, in2 := lib.Split(box.Ballots)
		for i := range mixes {
			out1, out2 := lib.Split(mixes[i].Ballots)
			require.Nil(t, lib.Verify(mixes[i].Proof, election.Key, in1, in2, out1, out2))
			in1, in2 = out1, out2
		}
	case <-time.After(180 * time.Second):
		t.Fatal("Protocol timeout")
	}

}
func runShuffle(t *testing.T, n int) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, tree := local.GenBigTree(n, n, 1, true)
	services := local.GetServices(nodes, shuffleServiceID)

	dkgs, _ := lib.DKGSimulate(n, n-1)
	shared, _ := lib.NewSharedSecret(dkgs[0])
	key := shared.X

	chain, _ := lib.NewSkipchain(services[0].(*shuffleService).skipchain, roster, true)
	election := &lib.Election{
		ID:      chain.Hash,
		Roster:  roster,
		Key:     key,
		Creator: 0,
		Users:   []uint32{0, 1, 2},
	}
	for i := range services {
		services[i].(*shuffleService).election = election
		services[i].(*shuffleService).user = 0
		services[i].(*shuffleService).signature = []byte{}
	}

	tx := lib.NewTransaction(election, election.Creator)
	lib.StoreUsingWebsocket(election.ID, election.Roster, tx)

	for i := 0; i < 3; i++ {
		a, b := lib.Encrypt(key, []byte{byte(i)})
		ballot := &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
		tx = lib.NewTransaction(ballot, election.Creator)
		lib.StoreUsingWebsocket(election.ID, election.Roster, tx)
	}

	instance, _ := services[0].(*shuffleService).CreateProtocol(NameShuffle, tree)
	shuffle := instance.(*Shuffle)
	shuffle.User = 0
	shuffle.Election = election
	shuffle.Skipchain = services[0].(*shuffleService).skipchain
	shuffle.LeaderParticipates = true
	shuffle.Start()

	select {
	case err := <-shuffle.Finished:
		require.Nil(t, err)
		box, _ := election.Box(services[0].(*shuffleService).skipchain)
		mixes, _ := election.Mixes(services[0].(*shuffleService).skipchain)

		require.Equal(t, 2*n/3+1, len(mixes))
		in1, in2 := lib.Split(box.Ballots)
		for i := range mixes {
			out1, out2 := lib.Split(mixes[i].Ballots)
			require.Nil(t, lib.Verify(mixes[i].Proof, election.Key, in1, in2, out1, out2))
			in1, in2 = out1, out2
		}
	case <-time.After(60 * time.Second):
		t.Fatal("Protocol timeout")
	}
}
