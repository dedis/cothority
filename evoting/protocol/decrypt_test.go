package protocol

import (
	"errors"
	"testing"
	"time"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/skipchain"
)

var decryptServiceID onet.ServiceID

type decryptService struct {
	*onet.ServiceProcessor

	user      uint32
	signature []byte

	secret    *lib.SharedSecret
	election  *lib.Election
	skipchain *skipchain.Service
}

func init() {
	new := func(ctx *onet.Context) (onet.Service, error) {
		return &decryptService{
			ServiceProcessor: onet.NewServiceProcessor(ctx),
			skipchain:        ctx.Service(skipchain.ServiceName).(*skipchain.Service),
		}, nil
	}
	decryptServiceID, _ = onet.RegisterNewService(NameDecrypt, new)
}

func (s *decryptService) NewProtocol(node *onet.TreeNodeInstance, conf *onet.GenericConfig) (
	onet.ProtocolInstance, error) {

	switch node.ProtocolName() {
	case NameDecrypt:
		instance, _ := NewDecrypt(node)
		decrypt := instance.(*Decrypt)
		decrypt.User = s.user
		decrypt.Secret = s.secret
		decrypt.Election = s.election
		decrypt.Skipchain = s.skipchain
		return decrypt, nil
	default:
		return nil, errors.New("Unknown protocol")
	}
}

func TestDecryptProtocol(t *testing.T) {
	for _, nodes := range []int{3, 5} {
		runDecrypt(t, nodes)
	}
}

func runDecrypt(t *testing.T, n int) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, tree := local.GenBigTree(n, n, 1, true)
	services := local.GetServices(nodes, decryptServiceID)

	dkgs, _ := lib.DKGSimulate(n, n-1)
	shared, _ := lib.NewSharedSecret(dkgs[0])
	key := shared.X

	chain, _ := lib.NewSkipchain(services[0].(*decryptService).skipchain, roster, true)
	election := &lib.Election{
		ID:      chain.Hash,
		Roster:  roster,
		Key:     key,
		Creator: 0,
		Users:   []uint32{0, 1, 2},
	}
	for i := range services {
		services[i].(*decryptService).secret, _ = lib.NewSharedSecret(dkgs[i])
		services[i].(*decryptService).election = election
		services[i].(*decryptService).user = 0
		services[i].(*decryptService).signature = []byte{}
	}

	tx := lib.NewTransaction(election, election.Creator)
	lib.StoreUsingWebsocket(election.ID, election.Roster, tx)

	ballots := make([]*lib.Ballot, 3)
	for i := 0; i < 3; i++ {
		a, b := lib.Encrypt(key, []byte{byte(i)})
		ballots[i] = &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
		tx = lib.NewTransaction(ballots[i], election.Creator)
		err := lib.StoreUsingWebsocket(election.ID, election.Roster, tx)
		require.NoError(t, err)
	}

	mixes := make([]*lib.Mix, n)
	x, y := lib.Split(ballots)
	for i := range mixes {
		v, w, shuffleProof, err := lib.CreateShuffleProof(x, y, key)
		require.NoError(t, err)
		public := roster.Get(i).Public
		data, _ := public.MarshalBinary()
		sig, _ := schnorr.Sign(cothority.Suite, local.GetPrivate(nodes[i]), data)
		mix := &lib.Mix{
			Ballots:   lib.Combine(v, w),
			Proof:     shuffleProof,
			NodeID:    roster.Get(i).ID,
			Signature: sig,
		}
		tx = lib.NewTransaction(mix, election.Creator)
		err = lib.StoreUsingWebsocket(election.ID, election.Roster, tx)
		require.NoError(t, err)
		x, y = v, w
	}

	instance, _ := services[0].(*decryptService).CreateProtocol(NameDecrypt, tree)
	decrypt := instance.(*Decrypt)
	decrypt.Secret, _ = lib.NewSharedSecret(dkgs[0])
	decrypt.User = 0
	decrypt.Election = election
	decrypt.Skipchain = services[0].(*decryptService).skipchain
	decrypt.LeaderParticipates = true
	decrypt.Start()

	select {
	case <-decrypt.Finished:
		partials, _ := election.Partials(services[0].(*decryptService).skipchain)
		require.True(t, 2*n/3 < len(partials))
	case <-time.After(180 * time.Second):
		assert.True(t, false)
	}

	// The decrypt protocol tries to stop early as soon as 2n/3 + 1 nodes store a partial.
	// However, since the leader sends a broadcast to all the n nodes initially we
	// want the servers to be up until the goroutines terminate or the test framework complains
	// about zombie goroutines. The call to time.Sleep ensures we dont end up with
	// zombie goroutines
	time.Sleep(2 * time.Second)
}

func TestDecryptNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping", t.Name(), " in short mode")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodes, roster, tree := local.GenBigTree(7, 7, 1, true)
	services := local.GetServices(nodes, decryptServiceID)

	dkgs, _ := lib.DKGSimulate(7, 6)
	shared, _ := lib.NewSharedSecret(dkgs[0])
	key := shared.X

	chain, _ := lib.NewSkipchain(services[0].(*decryptService).skipchain, roster, true)
	election := &lib.Election{
		ID:      chain.Hash,
		Roster:  roster,
		Key:     key,
		Creator: 0,
		Users:   []uint32{0, 1, 2},
	}
	for i := range services {
		services[i].(*decryptService).secret, _ = lib.NewSharedSecret(dkgs[i])
		services[i].(*decryptService).election = election
		services[i].(*decryptService).user = 0
		services[i].(*decryptService).signature = []byte{}
	}

	tx := lib.NewTransaction(election, election.Creator)
	lib.StoreUsingWebsocket(election.ID, election.Roster, tx)

	ballots := make([]*lib.Ballot, 3)
	for i := 0; i < 3; i++ {
		a, b := lib.Encrypt(key, []byte{byte(i)})
		ballots[i] = &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
		tx = lib.NewTransaction(ballots[i], election.Creator)
		lib.StoreUsingWebsocket(election.ID, election.Roster, tx)
	}

	mixes := make([]*lib.Mix, 7)
	x, y := lib.Split(ballots)
	for i := range mixes {
		v, w, shuffleProof, err := lib.CreateShuffleProof(x, y, key)
		require.NoError(t, err)
		public := roster.Get(i).Public
		data, _ := public.MarshalBinary()
		sig, _ := schnorr.Sign(cothority.Suite, local.GetPrivate(nodes[i]), data)
		mix := &lib.Mix{
			Ballots:   lib.Combine(v, w),
			Proof:     shuffleProof,
			NodeID:    roster.Get(i).ID,
			Signature: sig,
		}
		mixes[i] = mix
		tx = lib.NewTransaction(mix, election.Creator)
		err = lib.StoreUsingWebsocket(election.ID, election.Roster, tx)
		require.NoError(t, err)
		x, y = v, w
	}

	// we're trying to simulate a decryption that failed previously
	for i := 0; i < 2; i++ {
		mix := mixes[len(mixes)-1]
		points := make([]kyber.Point, len(mix.Ballots))
		for i := range points {
			secret, _ := lib.NewSharedSecret(dkgs[i])
			points[i] = lib.Decrypt(secret.V, mix.Ballots[i].Alpha, mix.Ballots[i].Beta)
		}

		partial := &lib.Partial{
			Points: points,
			NodeID: nodes[i].ServerIdentity.ID,
		}
		data, _ := nodes[i].ServerIdentity.Public.MarshalBinary()
		index, _ := roster.Search(partial.NodeID)
		data = append(data, byte(index))
		sig, _ := schnorr.Sign(cothority.Suite, local.GetPrivate(nodes[i]), data)
		partial.Signature = sig
		transaction := lib.NewTransaction(partial, election.Creator)
		lib.StoreUsingWebsocket(election.ID, election.Roster, transaction)
	}

	rooted := onet.NewRoster(append([]*network.ServerIdentity{tree.Roster.List[0]}, tree.Roster.List[2:]...))
	protocolTree := rooted.GenerateNaryTree(1)
	instance, _ := services[0].(*decryptService).CreateProtocol(NameDecrypt, protocolTree)
	decrypt := instance.(*Decrypt)
	decrypt.Secret, _ = lib.NewSharedSecret(dkgs[0])
	decrypt.User = 0
	decrypt.Election = election
	decrypt.Skipchain = services[0].(*decryptService).skipchain
	decrypt.LeaderParticipates = false
	log.Lvl1("Starting the decrypt protocol")
	decrypt.Start()

	select {
	case <-decrypt.Finished:
		partials, _ := election.Partials(services[0].(*decryptService).skipchain)
		require.True(t, len(partials) == 5)
	case <-time.After(300 * time.Second):
		assert.True(t, false)
	}

	// The decrypt protocol tries to stop early as soon as 2n/3 + 1 nodes store a partial.
	// However, since the leader sends a broadcast to all the n nodes initially we
	// want the servers to be up until the goroutines terminate or the test framework complains
	// about zombie goroutines. The call to time.Sleep ensures we dont end up with
	// zombie goroutines
	time.Sleep(2 * time.Second)
}
