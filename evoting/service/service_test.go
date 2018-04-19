package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"

	"github.com/stretchr/testify/require"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func generateSignature(private kyber.Scalar, ID []byte, sciper uint32) []byte {
	message := ID
	for _, c := range strconv.Itoa(int(sciper)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	sig, err := schnorr.Sign(cothority.Suite, private, message)
	if err != nil {
		panic("cannot sign:" + err.Error())
	}
	return sig
}

var (
	idAdmin  = uint32(111111)
	idAdmin2 = uint32(111112)
	idUser1  = uint32(111113)
	idUser2  = uint32(111114)
	idUser3  = uint32(111115)
	idCand1  = uint32(123456)
	bufCand1 = []byte{byte(idCand1 & 0xff), byte((idCand1 >> 8) & 0xff), byte((idCand1 >> 16) & 0xff)}
	idCand2  = uint32(123457)
	bufCand2 = []byte{byte(idCand2 & 0xff), byte((idCand2 >> 8) & 0xff), byte((idCand2 >> 16) & 0xff)}
)

func TestService(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	s1 := local.GetServices(nodes, serviceID)[1].(*Service)

	// Creating master skipchain
	replyLink, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: roster,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)

	idAdminSig := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)

	// Try to create a new election on server[1], should fail.
	replyOpen, err := s1.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			Roster:  roster,
			End:     time.Now().Unix() + 86400,
		},
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Create a new election
	replyOpen, err = s0.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			Roster:  roster,
			End:     time.Now().Unix() + 86400,
		},
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Nil(t, err)

	// Try to cast a vote on a non-leader, should fail.
	idUser1Sig := generateSignature(nodeKP.Private, replyLink.ID, idUser1)

	k, c := lib.Encrypt(replyOpen.Key, bufCand1)
	ballot := &lib.Ballot{
		User:  idUser1,
		Alpha: k,
		Beta:  c,
	}
	_, err = s1.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser1,
		Signature: idUser1Sig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Try to cast a vote for another person. (i.e. t.User != t.Ballot.User)
	ballot = &lib.Ballot{
		User:  idUser2,
		Alpha: k,
		Beta:  c,
	}
	_, err = s0.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser1,
		Signature: idUser1Sig,
	})
	// expect a failure
	require.NotNil(t, err)

	// Prepare a helper for testing voting.
	vote := func(user uint32, bufCand []byte) *evoting.CastReply {
		k, c := lib.Encrypt(replyOpen.Key, bufCand)
		ballot := &lib.Ballot{
			User:  user,
			Alpha: k,
			Beta:  c,
		}
		cast, err := s0.Cast(&evoting.Cast{
			ID:        replyOpen.ID,
			Ballot:    ballot,
			User:      user,
			Signature: generateSignature(nodeKP.Private, replyLink.ID, user),
		})
		require.Nil(t, err)
		return cast
	}

	// User votes
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Shuffle on non-leader
	_, err = s1.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Nil(t, err)

	// Decrypt on non-leader
	_, err = s1.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Decrypt all votes
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Nil(t, err)

	// Reconstruct on non-leader
	reconstructReply, err := s1.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Equal(t, err, errOnlyLeader)

	// Reconstruct votes
	reconstructReply, err = s0.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Nil(t, err)

	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
	}
}

func runAnElection(t *testing.T, s *Service, replyLink *evoting.LinkReply, nodeKP *key.Pair, admin uint32) {
	adminSig := generateSignature(nodeKP.Private, replyLink.ID, admin)

	replyOpen, err := s.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Creator: admin,
			Users:   []uint32{idUser1, idUser2, idUser3, admin},
			End:     time.Now().Unix() + 86400,
		},
		User:      admin,
		Signature: adminSig,
	})
	require.Nil(t, err)

	// Prepare a helper for testing voting.
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
			Signature: generateSignature(nodeKP.Private, replyLink.ID, user),
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
		User:      admin,
		Signature: adminSig,
	})
	require.Nil(t, err)

	// Decrypt all votes
	_, err = s.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      admin,
		Signature: adminSig,
	})
	require.Nil(t, err)

	// Reconstruct votes
	_, err = s.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Nil(t, err)
}

func TestEvolveRoster(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)

	// Creating master skipchain with the first 5 nodes
	ro1 := onet.NewRoster(roster.List[0:5])
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro1,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote 1st roster")

	runAnElection(t, s0, rl, nodeKP, idAdmin)

	// Try to change master as idAdmin2: it should not be allowed.
	idAdmin2Sig := generateSignature(nodeKP.Private, rl.ID, idAdmin2)
	_, err = s0.Link(&evoting.Link{
		ID:        &rl.ID,
		User:      &idAdmin2,
		Signature: &idAdmin2Sig,
		Pin:       s0.pin,
		Roster:    ro1,
		Key:       nodeKP.Public,
		Admins:    []uint32{idAdmin2},
	})
	require.NotNil(t, err)

	// Change roster to all 7 nodes. Set new nodeKP. Change admin user.
	idAdminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)
	nodeKP = key.NewKeyPair(cothority.Suite)
	rl, err = s0.Link(&evoting.Link{
		ID:        &rl.ID,
		User:      &idAdmin,
		Signature: &idAdminSig,
		Pin:       s0.pin,
		Roster:    roster,
		Key:       nodeKP.Public,
		Admins:    []uint32{idAdmin, idAdmin2},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote 2nd roster")

	// Run an election on the new set of conodes, the new nodeKP, and the new
	// election admin.
	runAnElection(t, s0, rl, nodeKP, idAdmin2)

	// There was a test here before to try to replace the leader.
	// It didn't work. For the time being, that is not supported.
}
