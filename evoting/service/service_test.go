package service

import (
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting"
	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/proof"
	"go.dedis.ch/kyber/v3/shuffle"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

var defaultTimeout = 20 * time.Second
var yesterday = time.Now().AddDate(0, 0, -1)
var tomorrow = time.Now().AddDate(0, 0, 1)

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
	require.NoError(t, err)

	idAdminSig := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)

	elec := &lib.Election{
		Name: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		Subtitle: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		MoreInfoLang: map[string]string{
			"en": "https://epfl.ch/elections",
			"fr": "httsp://epfl.ch/votations",
		},
		Creator: idAdmin,
		Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
		Roster:  roster,
		Start:   yesterday.Unix(),
		End:     tomorrow.Unix(),
	}

	// Try to create a new election on server[1], should fail.
	replyOpen, err := s1.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Equal(t, err, errOnlyLeader)

	// Create a new election
	replyOpen, err = s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)
	elec.ID = replyOpen.ID

	// Try to modify an election on another master chain.
	save := elec.Master
	elec.Master = append([]byte{}, replyLink.ID...)
	elec.Master[0]++
	_, err = s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Error(t, err)
	elec.Master = save

	// Update the election name.
	elec.Name["en"] = "The new name"
	_, err = s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)

	// Make sure the change stuck.
	box, err := s0.GetBox(&evoting.GetBox{ID: elec.ID})
	require.NoError(t, err)
	require.Equal(t, box.Election.Name["en"], elec.Name["en"])

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

	// Cast a vote for no candidates at all: should work.
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
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Cast a vote with empty points - will fail
	log.Lvl1("Casting empty ballot (no points)")
	ballot = &lib.Ballot{
		User: idUser1,
	}
	_, err = s0.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser1,
		Signature: idUser1Sig,
	})
	require.Error(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Try to modify the election after a vote is cast.
	_, err = s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Error(t, err)

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
		require.NoError(t, err)
		return cast
	}
	// User votes
	log.Lvl1("Casting votes for correct users")
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Try to decrypt before shuffling; will fail
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.Error(t, err)

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
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Reconstruct on non-leader
	_, err = s1.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Equal(t, err, errOnlyLeader)

	// Reconstruct votes
	reconstructReply, err := s0.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.NoError(t, err)

	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
	}
}

// This is an end-to end test, just like TestService, so it has a lot of copy-paste
// stuff. It is useful to have this as it's own test because I wanted to investigate
// the behaviour of decryption of the badly encrypted points.
func TestBadEncryption(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
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
	require.NoError(t, err)

	idAdminSig := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)

	elec := &lib.Election{
		Name: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		Subtitle: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		MoreInfoLang: map[string]string{
			"en": "https://epfl.ch/elections",
			"fr": "httsp://epfl.ch/votations",
		},
		Creator: idAdmin,
		Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
		Roster:  roster,
		Start:   yesterday.Unix(),
		End:     tomorrow.Unix(),
	}

	// Create a new election
	replyOpen, err := s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)
	elec.ID = replyOpen.ID

	log.Lvl1("Casting a ballot with incorrect encryption")
	k0, c0 := lib.Encrypt(replyOpen.Key, bufCand1)
	// k0, c0 are correctly encrypted right now, let's corrupt them.
	noise := cothority.Suite.Point().Pick(random.New())
	k1 := k0.Clone().Add(noise, k0)

	ballot := &lib.Ballot{
		User:  idUser1,
		Alpha: k1,
		Beta:  c0,
	}
	_, err = s0.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser1,
		Signature: generateSignature(nodeKP.Private, replyLink.ID, idUser1),
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Need two ballots to be able to shuffle; this ballot is correctly encrypted.
	ballot = &lib.Ballot{
		User:  idUser2,
		Alpha: k0,
		Beta:  c0,
	}
	_, err = s0.Cast(&evoting.Cast{
		ID:        replyOpen.ID,
		Ballot:    ballot,
		User:      idUser2,
		Signature: generateSignature(nodeKP.Private, replyLink.ID, idUser2),
	})
	require.Nil(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Shuffle all votes
	log.Lvl1("Shuffling s0")
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)

	// Decrypt all votes
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Reconstruct votes
	reconstructReply, err := s0.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.NoError(t, err)

	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
		a, err := p.Data()
		if err != nil {
			t.Log("decode point:", err)
		} else {
			log.Lvl2("Data is:", a)
		}
	}
}

func TestAfterEnd(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
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
	require.NoError(t, err)

	idAdminSig := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)
	idUser1Sig := generateSignature(nodeKP.Private, replyLink.ID, idUser1)

	elec := &lib.Election{
		Name: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		Subtitle: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		MoreInfoLang: map[string]string{
			"en": "https://epfl.ch/elections",
			"fr": "httsp://epfl.ch/votations",
		},
		Creator: idAdmin,
		Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
		Roster:  roster,
		Start:   yesterday.Unix(),
		End:     time.Now().Unix() + 1, /* second */
	}

	// Create a new election
	replyOpen, err := s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)
	elec.ID = replyOpen.ID

	// Cast a vote, will fail because the election is already ended.
	log.Lvl1("Casting a ballot after election.")

	time.Sleep(3 * time.Second)
	k0, c0 := lib.Encrypt(replyOpen.Key, []byte{})
	ballot := &lib.Ballot{
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
	require.Error(t, err)
}

func TestBeforeStart(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)
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
	require.NoError(t, err)

	idAdminSig := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)
	idUser1Sig := generateSignature(nodeKP.Private, replyLink.ID, idUser1)

	elec := &lib.Election{
		Name: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		Subtitle: map[string]string{
			"en": "name in english",
			"fr": "name in french",
		},
		MoreInfoLang: map[string]string{
			"en": "https://epfl.ch/elections",
			"fr": "httsp://epfl.ch/votations",
		},
		Creator: idAdmin,
		Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
		Roster:  roster,
		Start:   tomorrow.Unix(),
		End:     tomorrow.Unix() + 1,
	}

	// Create a new election
	replyOpen, err := s0.Open(&evoting.Open{
		ID:        replyLink.ID,
		Election:  elec,
		User:      idAdmin,
		Signature: idAdminSig,
	})
	require.NoError(t, err)
	elec.ID = replyOpen.ID

	// Cast a vote, will fail because the election starts tomorrow.
	log.Lvl1("Casting empty ballot")
	k0, c0 := lib.Encrypt(replyOpen.Key, []byte{})
	ballot := &lib.Ballot{
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
	require.Error(t, err)
}

func runAnElection(t *testing.T, local *onet.LocalTest, s *Service, replyLink *evoting.LinkReply, nodeKP *key.Pair, admin uint32) {
	adminSig := generateSignature(nodeKP.Private, replyLink.ID, admin)

	log.Lvl1("Opening")
	replyOpen, err := s.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Creator: admin,
			Users:   []uint32{idUser1, idUser2, idUser3, admin},
			Start:   yesterday.Unix(),
			End:     tomorrow.Unix(),
		},
		User:      admin,
		Signature: adminSig,
	})
	require.NoError(t, err)
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
		require.NoError(t, err)
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
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(time.Second))

	// Decrypt all votes
	log.Lvl1("Decrypt")
	_, err = s.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      admin,
		Signature: adminSig,
	})
	require.NoError(t, err)
	require.Nil(t, local.WaitDone(defaultTimeout))

	// Reconstruct votes
	log.Lvl1("Reconstruct")
	_, err = s.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
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
			Start:   yesterday.Unix(),
			End:     tomorrow.Unix(),
		},
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NoError(t, err)

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
		require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Append two Mixes manually to simulate a shuffle gone bad
	election, err := lib.GetElection(s0.skipchain, electionID, false, 0)
	require.NoError(t, err)

	genMix := func(ballots []*lib.Ballot, election *lib.Election, serverIdentity *network.ServerIdentity, private kyber.Scalar) *lib.Mix {
		a, b := lib.Split(ballots)
		g, d, prov := shuffle.Shuffle(cothority.Suite, nil, election.Key, a, b, random.New())
		proof, err := proof.HashProve(cothority.Suite, "", prov)
		require.NoError(t, err)
		mix := &lib.Mix{
			Ballots: lib.Combine(g, d),
			Proof:   proof,
			NodeID:  serverIdentity.ID,
		}
		data, err := serverIdentity.Public.MarshalBinary()
		require.NoError(t, err)
		sig, err := schnorr.Sign(cothority.Suite, private, data)
		require.NoError(t, err)
		mix.Signature = sig
		return mix
	}

	box, err := election.Box(s0.skipchain)
	require.NoError(t, err)
	mix := genMix(box.Ballots, election, roster.Get(0), local.GetPrivate(nodes[0]))
	tx := lib.NewTransaction(mix, idAdmin)
	_, err = lib.Store(s0.skipchain, election.ID, tx, nil)
	require.NoError(t, err)
	mix2 := genMix(mix.Ballots, election, roster.Get(1), local.GetPrivate(nodes[1]))
	tx = lib.NewTransaction(mix2, idAdmin)
	_, err = lib.Store(s0.skipchain, election.ID, tx, nil)
	require.NoError(t, err)

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
	require.NoError(t, err)
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
	require.NoError(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NoError(t, err)

	// Close 2 Nodes
	nodes[2].Close()
	nodes[5].Close()

	// Try to decrypt
	_, err = s0.Decrypt(&evoting.Decrypt{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NoError(t, err)
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
	require.NoError(t, err)
	log.Lvl2("Wrote the roster")

	electionID := setupElection(t, s0, rl, nodeKP)
	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	// Shuffle all votes
	_, err = s0.Shuffle(&evoting.Shuffle{
		ID:        electionID,
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NoError(t, err)

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
	require.NoError(t, err)
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
	require.NoError(t, err)
	log.Lvl2("Wrote the roster")

	adminSig := generateSignature(nodeKP.Private, rl.ID, idAdmin)

	replyOpen, err := s0.Open(&evoting.Open{
		ID: rl.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			Start:   yesterday.Unix(),
			End:     tomorrow.Unix(),
		},
		User:      idAdmin,
		Signature: adminSig,
	})
	require.NoError(t, err)

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
		require.NoError(t, err)
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
	require.NoError(t, err)
}

func TestLookupSciper(t *testing.T) {
	// Comment this out when you want to run this unit test for dev work.
	t.Skip("unit tests should not call external servers")

	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	nodes, _, _ := local.GenBigTree(1, 1, 1, true)
	s0 := local.GetServices(nodes, serviceID)[0].(*Service)

	_, err := s0.LookupSciper(&evoting.LookupSciper{Sciper: ""})
	require.NotNil(t, err)
	_, err = s0.LookupSciper(&evoting.LookupSciper{Sciper: "12345"})
	require.NotNil(t, err)
	_, err = s0.LookupSciper(&evoting.LookupSciper{Sciper: "1234567"})
	require.NotNil(t, err)
	_, err = s0.LookupSciper(&evoting.LookupSciper{Sciper: "000000"})
	require.NotNil(t, err)

	reply, err := s0.LookupSciper(&evoting.LookupSciper{Sciper: "257875"})
	require.NoError(t, err)
	require.Equal(t, reply.FullName, "Bryan Alexander Ford")
	require.Equal(t, reply.Email, "bryan.ford@epfl.ch")
}
