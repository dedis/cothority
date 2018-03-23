package protocol

import (
	"errors"
	"testing"
	"time"

	"github.com/dedis/kyber/proof"
	"github.com/dedis/kyber/shuffle"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

var decryptServiceID onet.ServiceID

type decryptService struct {
	*onet.ServiceProcessor

	user      uint32
	signature []byte

	secret   *lib.SharedSecret
	election *lib.Election
}

func init() {
	new := func(ctx *onet.Context) (onet.Service, error) {
		return &decryptService{ServiceProcessor: onet.NewServiceProcessor(ctx)}, nil
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
		decrypt.Signature = s.signature
		decrypt.Secret = s.secret
		decrypt.Election = s.election
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

	chain, _ := lib.NewSkipchain(roster, skipchain.VerificationStandard, nil)
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

	tx := lib.NewTransaction(election, election.Creator, []byte{})
	lib.Store(election.ID, election.Roster, tx)

	ballots := make([]*lib.Ballot, 3)
	for i := 0; i < 3; i++ {
		a, b := lib.Encrypt(key, []byte{byte(i)})
		ballots[i] = &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
		tx = lib.NewTransaction(ballots[i], election.Creator, []byte{})
		lib.Store(election.ID, election.Roster, tx)
	}

	mixes := make([]*lib.Mix, n)
	x, y := lib.Split(ballots)
	for i := range mixes {
		v, w, prover := shuffle.Shuffle(cothority.Suite, nil, key, x, y, random.New())
		proof, _ := proof.HashProve(cothority.Suite, "", prover)
		mix := &lib.Mix{Ballots: lib.Combine(v, w), Proof: proof, Node: string(i)}
		tx = lib.NewTransaction(mix, election.Creator, []byte{})
		lib.Store(election.ID, election.Roster, tx)
		x, y = v, w
	}

	instance, _ := services[0].(*decryptService).CreateProtocol(NameDecrypt, tree)
	decrypt := instance.(*Decrypt)
	decrypt.Secret, _ = lib.NewSharedSecret(dkgs[0])
	decrypt.User = 0
	decrypt.Signature = []byte{}
	decrypt.Election = election
	decrypt.Start()

	select {
	case <-decrypt.Finished:
		partials, _ := election.Partials()
		for _, partial := range partials {
			require.True(t, partial.Flag)
		}
	case <-time.After(60 * time.Second):
		assert.True(t, false)
	}
}
