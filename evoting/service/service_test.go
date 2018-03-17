package service

import (
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func mockMasterSkipchain(master *lib.Master, links ...skipchain.SkipBlockID) {
	chain, _ := lib.NewSkipchain(master.Roster, skipchain.VerificationStandard, nil)

	master.ID = chain.Hash
	lib.Store(master.ID, master.Roster, master)

	for _, link := range links {
		lib.Store(master.ID, master.Roster, &lib.Link{ID: link})
	}
}

// func mockElectionSkipchain(election *lib.Election, size int) []*dkg.DistKeyGenerator {
// 	chain, _ := lib.NewSkipchain(election.Roster, evoting.VerificationFunction, nil)

// 	n := len(election.Roster.List)
// 	dkgs, _ := lib.DKGSimulate(n, n-1)
// 	shared, _ := lib.NewSharedSecret(dkgs[0])

// 	key := shared.X

// 	election.ID = chain.Hash
// 	election.Key = key
// 	fmt.Println("ZZZZZZZZZZZZZZZ")
// 	tx := lib.NewTransaction(election, election.Creator)
// 	lib.Store(election.ID, election.Roster, lib.NewTransaction(election, election.Creator))

// 	// Create ballots
// 	ballots := make([]*lib.Ballot, size)
// 	for i := range ballots {
// 		a, b := lib.Encrypt(key, []byte{byte(i)})
// 		ballots[i] = &lib.Ballot{User: uint32(i), Alpha: a, Beta: b}
// 		lib.Store(election.ID, election.Roster, ballots[i])
// 	}

// 	// Create mixes
// 	mixes := make([]*lib.Mix, n)
// 	x, y := lib.Split(ballots)
// 	for i := range mixes {
// 		v, w, prover := shuffle.Shuffle(cothority.Suite, nil, key, x, y, random.New())
// 		proof, _ := proof.HashProve(cothority.Suite, "", prover)
// 		mixes[i] = &lib.Mix{Ballots: lib.Combine(v, w), Proof: proof, Node: string(i)}
// 		x, y = v, w
// 	}

// 	// Create partials
// 	partials, latest := make([]*lib.Partial, n), mixes[len(mixes)-1]
// 	for i, gen := range dkgs {
// 		secret, _ := lib.NewSharedSecret(gen)
// 		points := make([]kyber.Point, len(latest.Ballots))
// 		for j, ballot := range latest.Ballots {
// 			points[j] = lib.Decrypt(secret.V, ballot.Alpha, ballot.Beta)
// 		}
// 		partials[i] = &lib.Partial{Points: points, Node: string(i)}
// 	}

// 	if election.Stage == lib.Shuffled {
// 		for _, m := range mixes {
// 			lib.Store(election.ID, election.Roster, m)
// 		}
// 	} else if election.Stage == lib.Decrypted {
// 		for _, m := range mixes {
// 			lib.Store(election.ID, election.Roster, m)
// 		}
// 		for _, p := range partials {
// 			lib.Store(election.ID, election.Roster, p)
// 		}
// 	}
// 	return dkgs
// }

func TestService(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	idAdmin := uint32(111111)
	idUser1 := uint32(111112)
	idUser2 := uint32(111113)
	idUser3 := uint32(111114)
	idCand1 := uint32(123456)
	bufCand1 := []byte{byte(idCand1 & 0xff), byte((idCand1 >> 8) & 0xff), byte((idCand1 >> 16) & 0xff)}
	idCand2 := uint32(123457)
	bufCand2 := []byte{byte(idCand2 & 0xff), byte((idCand2 >> 8) & 0xff), byte((idCand2 >> 16) & 0xff)}

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)
	pin := "1234"
	s.pin = pin

	// Creating master skipchain
	replyLink, err := s.Link(&evoting.Link{
		Pin:    pin,
		Roster: roster,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)

	// Create a new election
	replyOpen, err := s.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Name:     "bla",
			Creator:  idAdmin,
			Users:    []uint32{idUser1, idUser2, idUser3, idAdmin},
			Subtitle: "test",
			End:      time.Now().Unix() + 86400,
		},
		User:      idAdmin,
		Signature: []byte{},
	})
	require.Nil(t, err)

	vote := func(user uint32, bufCand []byte) *evoting.CastReply {
		k, c := lib.Encrypt(replyOpen.Key, bufCand)
		ballot := &lib.Ballot{
			User:  user,
			Alpha: k,
			Beta:  c,
		}
		cast, err := s.Cast(&evoting.Cast{
			ID:        replyOpen.ID,
			Ballot:    ballot,
			User:      user,
			Signature: []byte{},
		})
		require.Nil(t, err)
		return cast
	}

	// User votes
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Shuffle all votes
	_, err = s.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: []byte{},
	})
	require.Nil(t, err)

	// Decrypt all votes
	_, err = s.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: []byte{},
	})
	require.Nil(t, err)

	// Reconstruct votes
	reconstructReply, err := s.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Nil(t, err)

	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
	}
}
