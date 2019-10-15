package service

import (
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/evoting"
	"go.dedis.ch/cothority/v4/evoting/lib"
	"go.dedis.ch/cothority/v4/skipchain"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/proof"
	"go.dedis.ch/kyber/v4/shuffle"
	"go.dedis.ch/kyber/v4/sign/schnorr"
	"go.dedis.ch/kyber/v4/util/key"
	"go.dedis.ch/kyber/v4/util/random"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

var defaultTimeout = 20 * time.Second

func TestMain(m *testing.M) {
	flag.Parse()
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
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

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
			Name: map[string]string{
				"en": "name in english",
				"fr": "name in french",
			},
			Subtitle: map[string]string{
				"en": "name in english",
				"fr": "name in french",
			},
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
	log.Lvl1("Casting vote on non-leader")
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
	require.Nil(t, local.WaitDone(time.Second))

	// Try to cast a vote for another person. (i.e. t.User != t.Ballot.User)
	log.Lvl1("Casting vote for another user - this will fail")
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
	require.Nil(t, local.WaitDone(time.Second))

	// Cast a vote for no users at all: should work.
	log.Lvl1("Casting empty ballot")
	k0, c0 := lib.Encrypt(replyOpen.Key, []byte{})
	ballot = &lib.Ballot{
		User:  idUser1,
		Alpha: k0,
		Beta:  c0,
	}
	_, err = s0.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser1,
		Signature: idUser1Sig,
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

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
	log.Lvl1("Casting votes for correct users")
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Shuffle on non-leader
	log.Lvl1("Shuffling s1")
	_, err = s1.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Shuffle all votes
	log.Lvl1("Shuffling s0")
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Nil(t, err)

	// Decrypt on non-leader
	log.Lvl1("Decrypting")
	_, err = s1.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, errOnlyLeader, err)

	// Decrypt all votes
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

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

func runAnElection(t *testing.T, local *onet.LocalTest, s *Service, replyLink *evoting.LinkReply, nodeKP *key.Pair, admin uint32) {
	adminSig := generateSignature(nodeKP.Private, replyLink.ID, admin)

	log.Lvl1("Opening")
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
	require.Nil(t, local.WaitDone(time.Second))

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
		require.Nil(t, local.WaitDone(time.Second))
		return cast
	}

	// User votes
	log.Lvl1("Vote for users")
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Shuffle all votes
	log.Lvl1("Shuffle")
	_, err = s.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      admin,
		Signature: adminSig,
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Decrypt all votes
	log.Lvl1("Decrypt")
	_, err = s.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      admin,
		Signature: adminSig,
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(defaultTimeout))

	// Reconstruct votes
	log.Lvl1("Reconstruct")
	_, err = s.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))
}

func TestEvolveRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("not using evolveRoster in travis")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

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

	runAnElection(t, local, s0, rl, nodeKP, idAdmin)

	// Try to change master as idAdmin2: it should not be allowed.
	log.Lvl1("Check rejection of invalid admin")
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
	require.Nil(t, local.WaitDone(time.Second))

	// Change roster to all 7 nodes. Set new nodeKP. Change admin user.
	log.Lvl1("Check changing of roster")
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
	require.Nil(t, local.WaitDone(time.Second))

	// Run an election on the new set of conodes, the new nodeKP, and the new
	// election admin.
	log.Lvl1("Running an election")
	runAnElection(t, local, s0, rl, nodeKP, idAdmin2)
	require.Nil(t, local.WaitDone(time.Second))

	// There was a test here before to try to replace the leader.
	// It didn't work. For the time being, that is not supported.
}

func setupElection(t *testing.T, s0 *Service, rl *evoting.LinkReply, nodeKP *key.Pair) skipchain.SkipBlockID {
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	replyOpen, err := s0.Open(&evoting.Open{
		ID: rl.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			End:     time.Now().Unix() + 86400,
		},
		User:      idAdmin,
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
		cast, err := s0.Cast(&evoting.Cast{
			ID:        replyOpen.ID,
			Ballot:    ballot,
			User:      user,
			Signature: generateSignature(nodeKP.Private, rl.ID, user),
		})
		require.Nil(t, err)
		return cast
	}

	// User votes
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	return replyOpen.ID
}

func TestShuffleBenignNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping", t.Name(), " in short mode")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

	// Create the master skipchain
	ro := onet.NewRoster(roster.List)
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Pause 2 nodes
	nodes[5].Close()
	nodes[6].Close()

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)
}

func TestShuffleCatastrophicNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("not using ShuffleCatastrophic in travis")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	oldTimeout := timeout
	defer func() {
		timeout = oldTimeout
	}()
	timeout = defaultTimeout

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

	// Create the master skipchain
	ro := onet.NewRoster(roster.List)
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Append two Mixes manually to simulate a shuffle gone bad
	election, err := lib.GetElection(s0.skipchain, electionID, false, 0)
	require.Nil(t, err)

	genMix := func(ballots []*lib.Ballot, election *lib.Election, serverIdentity *network.ServerIdentity, private kyber.Scalar) *lib.Mix {
		a, b := lib.Split(ballots)
		g, d, prov := shuffle.Shuffle(cothority.Suite, nil, election.Key, a, b, random.New())
		proof, err := proof.HashProve(cothority.Suite, "", prov)
		require.Nil(t, err)
		mix := &lib.Mix{
			Ballots: lib.Combine(g, d),
			Proof:   proof,
			NodeID:  serverIdentity.ID,
		}
		data, err := serverIdentity.Public.MarshalBinary()
		require.Nil(t, err)
		sig, err := schnorr.Sign(cothority.Suite, private, data)
		require.Nil(t, err)
		mix.Signature = sig
		return mix
	}

	box, err := election.Box(s0.skipchain)
	mix := genMix(box.Ballots, election, roster.Get(0), local.GetPrivate(nodes[0]))
	tx := lib.NewTransaction(mix, idAdmin)
	_, err = lib.Store(s0.skipchain, election.ID, tx, nil)
	require.Nil(t, err)
	mix2 := genMix(mix.Ballots, election, roster.Get(1), local.GetPrivate(nodes[1]))
	tx = lib.NewTransaction(mix2, idAdmin)
	_, err = lib.Store(s0.skipchain, election.ID, tx, nil)
	require.Nil(t, err)

	// Fail 3 nodes. New blocks cannot be added now because consensus cannot be reached.
	nodes[2].Pause()
	nodes[5].Pause()
	nodes[6].Pause()

	// Shuffle all votes
	log.Lvl1("Shuffling with too many nodes down - should fail!")
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NotNil(t, err)
	log.Lvl2("Verifying lingering protocols")
	require.Nil(t, local.WaitDone(timeout))

	// Unpause the nodes and retry shuffling
	nodes[2].Unpause()
	nodes[5].Unpause()
	nodes[6].Unpause()

	log.Lvl1("Shuffling with nodes back up - should pass")
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)
}

func TestDecryptBenignNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping", t.Name(), " in short mode")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

	// Create the master skipchain
	ro := onet.NewRoster(roster.List)
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)

	// Close 2 Nodes
	nodes[2].Close()
	nodes[5].Close()

	// Try to decrypt
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)
}

func TestDecryptCatastrophicNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("not using DecryptCatastrophic in travis")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	oldTimeout := timeout
	defer func() {
		timeout = oldTimeout
	}()
	timeout = defaultTimeout

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

	// Create the master skipchain
	ro := onet.NewRoster(roster.List)
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)

	// Fail 3 nodes
	nodes[2].Pause()
	nodes[4].Pause()
	nodes[5].Pause()

	// Try a decrypt now. It should timeout because no blocks can be stored
	log.Lvl1("Decrypting with too many nodes down - this will fail")
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NotNil(t, err)
	log.Lvl2("Waiting for protocols to finish")
	require.Nil(t, local.WaitDone(time.Second))

	// Unpause the nodes
	nodes[2].Unpause()
	nodes[4].Unpause()
	nodes[5].Unpause()

	// Try a decrypt now. It should succeed
	log.Lvl1("Decrypting with all nodes up again")
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)
}

func TestCastNodeFailureShuffleAllOk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping", t.Name(), " in short mode")
	}
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)
	nodes, roster, _ := local.GenBigTree(7, 7, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
	sc0 := local.GetServices(nodes, onet.ServiceFactory.ServiceID(skipchain.ServiceName))[0].(*skipchain.Service)
	// Set a lower timeout for the tests
	sc0.SetPropTimeout(defaultTimeout)

	// Create the master skipchain
	ro := onet.NewRoster(roster.List)
	rl, err := s0.Link(&evoting.Link{
		Pin:    s0.pin,
		Roster: ro,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)
	log.Lvl2("Wrote the roster")

	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	replyOpen, err := s0.Open(&evoting.Open{
		ID: rl.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			End:     time.Now().Unix() + 86400,
		},
		User:      idAdmin,
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
		cast, err := s0.Cast(&evoting.Cast{
			ID:        replyOpen.ID,
			Ballot:    ballot,
			User:      user,
			Signature: generateSignature(nodeKP.Private, rl.ID, user),
		})
		require.Nil(t, err)
		return cast
	}

	log.Lvl1("Voting with all nodes")
	vote(idUser1, bufCand1)
	nodes[5].Pause()
	log.Lvl1("Voting with one node paused")
	vote(idUser2, bufCand1)
	log.Lvl1("Unpausing node again")
	nodes[5].Unpause()

	electionID := replyOpen.ID

	// Shuffle all votes
	adminSig = generateSignature(nodeKP.Private, rl.ID, idAdmin)
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.Nil(t, err)
}
