// Package service is the evoting service designed for use at EPFL.
package service

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"

	"github.com/go-ldap/ldap/v3"
	"go.dedis.ch/cothority/v3"
	dkgprotocol "go.dedis.ch/cothority/v3/dkg/rabin"
	"go.dedis.ch/cothority/v3/evoting"
	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/evoting/protocol"
	"go.dedis.ch/cothority/v3/skipchain"
)

var errOnlyLeader = errors.New("operation only allowed on the leader node")

func init() {
	network.RegisterMessages(synchronizer{}, storage{})
	serviceID, _ = onet.RegisterNewService(evoting.ServiceName, new)
}

// timeout for protocol termination.
var timeout = 120 * time.Second

// serviceID is the onet identifier.
var serviceID onet.ServiceID

// storageKey identifies the on-disk storage.
var storageKey = []byte("storage")
var dbVersion = 1

// Service is the core structure of the application.
type Service struct {
	*onet.ServiceProcessor

	skipchain *skipchain.Service

	mutex         sync.Mutex
	finalizeMutex sync.Mutex // used for protecting shuffle and decrypt operations
	storage       *storage

	sciperMu    sync.Mutex
	sciperCache []cacheEntry

	pin string // pin is the current service number.
}

// Storage saves the shared secrets and stages for each election on disk.
type storage struct {
	Roster  *onet.Roster
	Master  skipchain.SkipBlockID
	Secrets map[string]*lib.SharedSecret
}

// synchronizer is broadcasted to all roster nodes before every protocol.
type synchronizer struct {
	ID   skipchain.SkipBlockID
	User uint32
}

// Ping message handler.
func (s *Service) Ping(req *evoting.Ping) (*evoting.Ping, error) {
	return &evoting.Ping{Nonce: req.Nonce + 1}, nil
}

// Link message handler. Generates a new master skipchain, or updates an existing one.
func (s *Service) Link(req *evoting.Link) (*evoting.LinkReply, error) {
	if req.Pin != s.pin {
		return nil, errors.New("link error: invalid pin")
	}

	var id skipchain.SkipBlockID
	var user uint32

	if req.ID != nil {
		// Update an existing master chain
		m, err := lib.GetMaster(s.skipchain, *req.ID)
		if err != nil {
			return nil, err
		}

		if req.User == nil || req.Signature == nil {
			return nil, errors.New("missing user or sig")
		}
		user = *req.User

		err = auth(*req.User, *req.Signature, m.ID, m.Key)
		if err != nil {
			return nil, err
		}
		id = m.ID
	} else {
		var err error
		genesis, err := lib.NewSkipchain(s.skipchain, req.Roster, false)
		if err != nil {
			return nil, err
		}
		id = genesis.Hash
	}

	master := &lib.Master{
		ID:     id,
		Roster: req.Roster,
		Admins: req.Admins,
		Key:    req.Key,
	}
	transaction := lib.NewTransaction(master, user)
	if _, err := lib.Store(s.skipchain, master.ID, transaction, s.ServerIdentity().GetPrivate()); err != nil {
		return nil, err
	}

	s.mutex.Lock()
	s.storage.Master = id
	s.storage.Roster = req.Roster
	s.mutex.Unlock()
	s.save()

	return &evoting.LinkReply{ID: id}, nil
}

// Open message handler. Create a new election with accompanying skipchain.
func (s *Service) Open(req *evoting.Open) (*evoting.OpenReply, error) {
	master, err := lib.GetMaster(s.skipchain, req.ID)
	if err != nil {
		return nil, err
	}
	if !s.ServerIdentity().Equal(master.Roster.List[0]) {
		return nil, errOnlyLeader
	}

	err = auth(req.User, req.Signature, master.ID, master.Key)
	if err != nil {
		return nil, err
	}

	// Check for the Election update case
	if len(req.Election.ID) > 0 {
		if !bytes.Equal(req.Election.Master, req.ID) {
			return nil, errors.New("master id mismatch")
		}

		cur, err := lib.GetElection(s.skipchain, req.Election.ID, false, 0)
		if err != nil {
			return nil, err
		}

		// Check that voting has not started.
		box, err := cur.Box(s.skipchain)
		if err != nil {
			return nil, err
		}
		if len(box.Ballots) != 0 {
			return nil, errors.New("election has started, no modifications allowed")
		}

		// Update cur with new values from req
		cur.Name = req.Election.Name
		cur.Candidates = req.Election.Candidates
		cur.MaxChoices = req.Election.MaxChoices
		cur.Subtitle = req.Election.Subtitle
		cur.MoreInfo = req.Election.MoreInfo
		cur.MoreInfoLang = req.Election.MoreInfoLang
		cur.Start = req.Election.Start
		cur.End = req.Election.End
		cur.Theme = req.Election.Theme
		cur.Footer = req.Election.Footer

		transaction := lib.NewTransaction(cur, req.User)
		if _, err := lib.Store(s.skipchain, req.Election.ID, transaction, s.ServerIdentity().GetPrivate()); err != nil {
			return nil, err
		}
		return &evoting.OpenReply{ID: cur.ID, Key: cur.Key}, nil
	}

	genesis, err := lib.NewSkipchain(s.skipchain, master.Roster, false)
	if err != nil {
		return nil, err
	}

	root := master.Roster.NewRosterWithRoot(s.ServerIdentity())
	tree := root.GenerateNaryTree(len(master.Roster.List))
	if tree == nil {
		return nil, errors.New("error while creating the tree")
	}

	instance, _ := s.CreateProtocol(dkgprotocol.Name, tree)
	proto := instance.(*dkgprotocol.Setup)
	config, _ := network.Marshal(&synchronizer{
		ID:   genesis.Hash,
		User: req.User,
	})
	proto.SetConfig(&onet.GenericConfig{Data: config})

	if err = proto.Start(); err != nil {
		return nil, err
	}
	select {
	case <-proto.Finished:
		secret, _ := lib.NewSharedSecret(proto.DKG)
		req.Election.ID = genesis.Hash
		req.Election.Master = req.ID
		req.Election.Roster = master.Roster
		req.Election.Key = secret.X
		req.Election.MasterKey = master.Key
		req.Election.Creator = req.User

		transaction := lib.NewTransaction(req.Election, req.User)
		if _, err := lib.Store(s.skipchain, req.Election.ID, transaction, s.ServerIdentity().GetPrivate()); err != nil {
			return nil, err
		}

		link := &lib.Link{ID: genesis.Hash}
		transaction = lib.NewTransaction(link, req.User)
		if _, err := lib.Store(s.skipchain, master.ID, transaction, s.ServerIdentity().GetPrivate()); err != nil {
			return nil, err
		}

		s.mutex.Lock()
		s.storage.Secrets[genesis.Short()] = secret
		s.mutex.Unlock()
		s.save()

		// Autovote mode: cast 1 empty ballot for the first user; useful to test shuffles in the UI
		// without having to login as two users. Will cause unit tests to fail.
		autovoteMode := false
		if autovoteMode {
			secret := cothority.Suite.Scalar().Pick(random.New())
			public := cothority.Suite.Point().Mul(secret, nil)
			K, C := lib.Encrypt(public, nil)
			b := &lib.Ballot{
				User:  req.Election.Users[0],
				Alpha: K,
				Beta:  C,
			}
			transaction = lib.NewTransaction(b, b.User)
			_, err := lib.Store(s.skipchain, req.Election.ID, transaction, s.ServerIdentity().GetPrivate())
			if err != nil {
				return nil, fmt.Errorf("could not cast ballot on election %x for user %v: %v", req.Election.ID, b.User, err)
			}
		}

		return &evoting.OpenReply{ID: genesis.Hash, Key: secret.X}, nil
	case <-time.After(timeout):
		return nil, errors.New("open error, protocol timeout")
	}
}

type cacheEntry struct {
	id      int
	reply   *evoting.LookupSciperReply
	expires time.Time
}

const sciperCacheLen = 100

func (s *Service) sciperGetNoLock(sciper int) *evoting.LookupSciperReply {
	for _, r := range s.sciperCache {
		if r.id == sciper && r.expires.After(time.Now()) {
			return r.reply
		}
	}
	return nil
}

// sciperGet runs through the cache looking for a match. The search is linear
// because the cache is small, and the whole thing will fit in a couple of cache lines.
func (s *Service) sciperGet(sciper int) (reply *evoting.LookupSciperReply) {
	s.sciperMu.Lock()
	reply = s.sciperGetNoLock(sciper)
	s.sciperMu.Unlock()
	return
}

// sciperPut puts an entry into the cache, if it is not present
func (s *Service) sciperPut(sciper int, reply *evoting.LookupSciperReply) {
	s.sciperMu.Lock()
	defer s.sciperMu.Unlock()

	// check that no one raced us to put their own copy in.
	if s.sciperGetNoLock(sciper) == nil {
		s.sciperCache = append(s.sciperCache, cacheEntry{
			id:      sciper,
			reply:   reply,
			expires: time.Now().Add(1 * time.Hour),
		})
		if len(s.sciperCache) > sciperCacheLen {
			from := len(s.sciperCache) - sciperCacheLen
			s.sciperCache = s.sciperCache[from:]
		}
	}

	return
}

// LookupSciper calls https://people.epfl.ch/cgi-bin/people/vCard?id=sciper
// to convert Sciper numbers to names.
func (s *Service) LookupSciper(req *evoting.LookupSciper) (*evoting.LookupSciperReply, error) {
	if len(req.Sciper) != 6 {
		return nil, errors.New("sciper should be 6 digits only")
	}
	sciper, err := strconv.Atoi(req.Sciper)
	if err != nil {
		return nil, errors.New("couldn't convert Sciper to integer")
	}

	// Try to find it in cache first
	if res := s.sciperGet(sciper); res != nil {
		log.Lvl3("Got vcard (cache hit)", res)
		return res, nil
	}

	url := "ldaps://ldap.epfl.ch"
	if req.LookupURL != "" {
		url = req.LookupURL
	}
	l, err := ldap.DialURL(url)
	if err != nil {
		return nil, err
	}
	defer l.Close()

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		"o=epfl, c=ch",
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=person)(uniqueIdentifier=%d))", sciper),
		[]string{"displayName", "mail"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// If no results, err.
	// If more than one are returned, we look only at the first one.
	if len(sr.Entries) == 0 {
		return nil, errors.New("SCIPER not found")
	}

	reply := &evoting.LookupSciperReply{}
	reply.FullName = sr.Entries[0].GetAttributeValue("displayName")
	reply.Email = sr.Entries[0].GetAttributeValue("mail")

	// Put it into the cache
	s.sciperPut(sciper, reply)

	log.Lvl3("Got vcard (cache miss): ", reply)
	return reply, nil
}

// SECURITY BUG: this authentication is completely bogus and can be replayed by an attacker.
// See https://github.com/dedis/cothority/issues/2507
func auth(u uint32, sig []byte, master skipchain.SkipBlockID, pub kyber.Point) error {
	var message []byte
	message = append(message, master...)

	for _, c := range strconv.Itoa(int(u)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}

	return schnorr.Verify(cothority.Suite, pub, message, sig)
}

// Cast message handler. Cast a ballot in a given election.
func (s *Service) Cast(req *evoting.Cast) (*evoting.CastReply, error) {
	if !s.leader() {
		return nil, errOnlyLeader
	}

	election, err := lib.GetElection(s.skipchain, req.ID, false, req.User)
	if err != nil {
		return nil, fmt.Errorf("could not cast ballot on election %x for user %v: %v", req.ID, req.User, err)
	}
	err = auth(req.User, req.Signature, election.Master, election.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("could not cast ballot on election %x for user %v: %v", req.ID, req.User, err)
	}

	transaction := lib.NewTransaction(req.Ballot, req.User)
	skipblockID, err := lib.Store(s.skipchain, req.ID, transaction, s.ServerIdentity().GetPrivate())
	if err != nil {
		return nil, fmt.Errorf("could not cast ballot on election %x for user %v: %v", req.ID, req.User, err)
	}
	return &evoting.CastReply{ID: skipblockID}, nil
}

// GetElections message handler. Return all elections in which the given user participates.
// If signature does not match the username, then only the Master structure is returned.
func (s *Service) GetElections(req *evoting.GetElections) (*evoting.GetElectionsReply, error) {
	master, err := lib.GetMaster(s.skipchain, req.Master)
	if err != nil {
		return nil, err
	}

	links, err := master.Links(s.skipchain)
	if err != nil {
		return nil, err
	}

	userValid := false
	err = auth(req.User, req.Signature, master.ID, master.Key)
	if err == nil {
		userValid = true
	}

	elections := make([]*lib.Election, 0)
	if userValid {
		for _, l := range links {
			election, err := lib.GetElection(s.skipchain, l.ID, req.CheckVoted, req.User)
			if err != nil {
				return nil, err
			}
			// Check if user is a voter or election creator.
			if election.IsUser(req.User) || election.IsCreator(req.User) {
				// Filter the election by Stage. 0 denotes no filtering.
				if req.Stage == 0 || req.Stage == election.Stage {
					elections = append(elections, election)
				}
			}
		}
	}
	out := &evoting.GetElectionsReply{Elections: elections, Master: *master}
	if userValid {
		out.IsAdmin = master.IsAdmin(req.User)
	}
	return out, nil
}

// GetBox message handler to retrieve the casted ballot in an election.
func (s *Service) GetBox(req *evoting.GetBox) (*evoting.GetBoxReply, error) {
	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	box, err := election.Box(s.skipchain)
	if err != nil {
		return nil, err
	}
	return &evoting.GetBoxReply{Box: box, Election: election}, nil
}

// GetMixes message handler. It is the caller's responsibility to check the proof
// in any Mix before relying on it.
func (s *Service) GetMixes(req *evoting.GetMixes) (*evoting.GetMixesReply, error) {
	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	mixes, err := election.Mixes(s.skipchain)
	if err != nil {
		return nil, err
	}
	return &evoting.GetMixesReply{Mixes: mixes}, nil
}

// GetPartials message handler.
func (s *Service) GetPartials(req *evoting.GetPartials) (*evoting.GetPartialsReply, error) {
	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	partials, err := election.Partials(s.skipchain)
	if err != nil {
		return nil, err
	}
	return &evoting.GetPartialsReply{Partials: partials}, nil
}

// Shuffle message handler. Initiate shuffle protocol.
func (s *Service) Shuffle(req *evoting.Shuffle) (*evoting.ShuffleReply, error) {
	s.finalizeMutex.Lock()
	defer s.finalizeMutex.Unlock()
	if !s.leader() {
		return nil, errOnlyLeader
	}

	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	err = auth(req.User, req.Signature, election.Master, election.MasterKey)
	if err != nil {
		return nil, err
	}

	// create a roster excluding nodes that have already participated
	mixes, err := election.Mixes(s.skipchain)
	if len(mixes) > 2*len(election.Roster.List)/3 {
		return &evoting.ShuffleReply{}, nil
	}
	if err != nil {
		return nil, err
	}
	participated := make(map[string]bool)
	for _, mix := range mixes {
		participated[mix.NodeID.String()] = true
	}
	filtered := []*network.ServerIdentity{}
	for _, node := range election.Roster.List[1:] {
		if _, ok := participated[node.ID.String()]; !ok {
			filtered = append(filtered, node)
		}
	}

	// shuffle the filtered list using Fischer-Yates
	// rand.Shuffle is introduced in Go 1.10
	for i := len(filtered) - 1; i > 0; i-- {
		j := rand.Intn(i)
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// add the leader to the front of the list
	filtered = append([]*network.ServerIdentity{election.Roster.List[0]}, filtered...)
	rooted := onet.NewRoster(filtered)
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}

	hasParticipated, _ := participated[election.Roster.List[0].ID.String()]
	instance, _ := s.CreateProtocol(protocol.NameShuffle, tree)
	protoShuffle := instance.(*protocol.Shuffle)
	protoShuffle.User = req.User
	protoShuffle.Election = election
	protoShuffle.Skipchain = s.skipchain
	protoShuffle.LeaderParticipates = !hasParticipated

	config, _ := network.Marshal(&synchronizer{
		ID:   req.ID,
		User: req.User,
	})
	protoShuffle.SetConfig(&onet.GenericConfig{Data: config})
	if err = protoShuffle.Start(); err != nil {
		return nil, err
	}
	select {
	case err := <-protoShuffle.Finished:
		return &evoting.ShuffleReply{}, err
	case <-time.After(timeout):
		protoShuffle.HandleTerminate(protocol.MessageTerminate{})
		return nil, errors.New("shuffle error, protocol timeout")
	}
}

// Decrypt message handler. Initiate decryption protocol.
func (s *Service) Decrypt(req *evoting.Decrypt) (*evoting.DecryptReply, error) {
	s.finalizeMutex.Lock()
	defer s.finalizeMutex.Unlock()
	if !s.leader() {
		return nil, errOnlyLeader
	}

	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	err = auth(req.User, req.Signature, election.Master, election.MasterKey)
	if err != nil {
		return nil, err
	}

	mixes, err := election.Mixes(s.skipchain)
	if err != nil {
		return nil, err
	}
	if len(mixes) < 2*len(election.Roster.List)/3+1 {
		return nil, errors.New("decrypt error: election not shuffled")
	}

	partials, err := election.Partials(s.skipchain)
	if err != nil {
		return nil, err
	}

	participated := make(map[string]bool)
	for _, partial := range partials {
		participated[partial.NodeID.String()] = true
	}
	filtered := []*network.ServerIdentity{}
	for _, node := range election.Roster.List[1:] {
		if _, ok := participated[node.ID.String()]; !ok {
			filtered = append(filtered, node)
		}
	}

	// shuffle the filtered list using Fischer-Yates
	// rand.Shuffle is introduced in Go 1.10
	for i := len(filtered) - 1; i > 0; i-- {
		j := rand.Intn(i)
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// add the leader to the front of the list
	filtered = append([]*network.ServerIdentity{election.Roster.List[0]}, filtered...)
	rooted := onet.NewRoster(filtered)
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("error while generating tree")
	}

	instance, _ := s.CreateProtocol(protocol.NameDecrypt, tree)
	protoDecrypt := instance.(*protocol.Decrypt)
	protoDecrypt.User = req.User
	protoDecrypt.Secret = s.secret(election.ID)
	protoDecrypt.Election = election
	protoDecrypt.Skipchain = s.skipchain
	protoDecrypt.LeaderParticipates = !participated[s.ServerIdentity().ID.String()]

	config, _ := network.Marshal(&synchronizer{
		ID:   req.ID,
		User: req.User,
	})
	protoDecrypt.SetConfig(&onet.GenericConfig{Data: config})
	if err = protoDecrypt.Start(); err != nil {
		return nil, err
	}
	select {
	case <-protoDecrypt.Finished:
		return &evoting.DecryptReply{}, nil
	case <-time.After(timeout):
		protoDecrypt.HandleTerminate(protocol.MessageTerminateDecrypt{})
		return nil, errors.New("decrypt error, protocol timeout")
	}
}

// Reconstruct message handler. Fully decrypt partials using Lagrange interpolation.
func (s *Service) Reconstruct(req *evoting.Reconstruct) (*evoting.ReconstructReply, error) {
	if !s.leader() {
		return nil, errOnlyLeader
	}

	election, err := lib.GetElection(s.skipchain, req.ID, false, 0)
	if err != nil {
		return nil, err
	}

	partials, err := election.Partials(s.skipchain)
	if err != nil {
		return nil, err
	} else if len(partials) <= 2*len(s.roster().List)/3 {
		return nil, errors.New("reconstruct error, election not closed yet")
	}

	points := make([]kyber.Point, 0)

	n := len(election.Roster.List)
	for i := 0; i < len(partials[0].Points); i++ {
		shares := make([]*share.PubShare, n)
		for _, partial := range partials {
			j, _ := election.Roster.Search(partial.NodeID)
			shares[j] = &share.PubShare{I: j, V: partial.Points[i]}
		}

		log.Lvl3("Recovering commits", i)
		message, err := share.RecoverCommit(cothority.Suite, shares, 2*n/3+1, n)
		if err != nil {
			return nil, err
		}
		points = append(points, message)
	}

	return &evoting.ReconstructReply{Points: points}, nil
}

// NewProtocol hooks non-root nodes into created protocols.
func (s *Service) NewProtocol(node *onet.TreeNodeInstance, conf *onet.GenericConfig) (
	onet.ProtocolInstance, error) {

	if conf == nil {
		return nil, errors.New("evoting/service.NewProtocol: missing config")
	}

	_, blob, _ := network.Unmarshal(conf.Data, cothority.Suite)
	sync := blob.(*synchronizer)

	switch node.ProtocolName() {
	case dkgprotocol.Name:
		instance, _ := dkgprotocol.NewSetup(node)
		protocol := instance.(*dkgprotocol.Setup)
		go func() {
			<-protocol.Finished
			secret, _ := lib.NewSharedSecret(protocol.DKG)
			s.mutex.Lock()
			s.storage.Secrets[sync.ID.Short()] = secret
			s.mutex.Unlock()
			s.save()
		}()
		return protocol, nil
	case protocol.NameShuffle:
		election, err := lib.GetElection(s.skipchain, sync.ID, false, 0)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewShuffle(node)
		protocol := instance.(*protocol.Shuffle)
		protocol.User = sync.User
		protocol.Election = election
		protocol.Skipchain = s.skipchain

		config, _ := network.Marshal(&synchronizer{
			ID:   sync.ID,
			User: sync.User,
		})
		protocol.SetConfig(&onet.GenericConfig{Data: config})

		return protocol, nil
	case protocol.NameDecrypt:
		election, err := lib.GetElection(s.skipchain, sync.ID, false, 0)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewDecrypt(node)
		protocol := instance.(*protocol.Decrypt)
		protocol.Secret = s.secret(sync.ID)
		protocol.User = sync.User
		protocol.Election = election
		protocol.Skipchain = s.skipchain

		config, _ := network.Marshal(&synchronizer{
			ID:   sync.ID,
			User: sync.User,
		})
		protocol.SetConfig(&onet.GenericConfig{Data: config})
		return protocol, nil
	default:
		return nil, errors.New("protocol error, unknown protocol")
	}
}

// verify is the skipchain verification handler.
func (s *Service) verify(id []byte, skipblock *skipchain.SkipBlock) bool {
	transaction := lib.UnmarshalTransaction(skipblock.Data)
	if transaction == nil {
		return false
	}

	// For txns generated by the leader, check his signature.
	// For the others (mix and partial), check the originator's signature later
	// in transaction.Verify().
	if transaction.Mix == nil && transaction.Partial == nil {
		var leaderPub kyber.Point
		latest, err := s.db().GetLatestByID(skipblock.GenesisID)
		if latest == nil {
			if skipblock.Index == 0 {
				leaderPub = skipblock.Roster.List[0].Public
			} else {
				log.Lvl3("could not find leader public key")
				return false
			}
		} else {
			leaderPub = latest.Roster.List[0].Public
		}

		txhash := transaction.Hash()
		msg := make([]byte, 8)
		binary.LittleEndian.PutUint64(msg, uint64(skipblock.Index))
		msg = append(msg, txhash...)

		err = schnorr.Verify(cothority.Suite, leaderPub, msg, transaction.Signature)
		if err != nil {
			log.Lvl2(s.ServerIdentity(), "txn sig verify failed:", err)
			return false
		}
	}

	err := transaction.Verify(skipblock.GenesisID, s.skipchain)
	if err != nil {
		log.Lvl2(s.ServerIdentity(), "verify failed:", err)
		return false
	}
	return true
}

// roster returns the roster from the storage.
func (s *Service) roster() *onet.Roster {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.storage.Roster
}

// leader returns true if this server has had it's master skipchain set,
// and is the leader of the roster.
func (s *Service) leader() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.storage.Master.IsNull() {
		return false
	}
	return s.ServerIdentity().Equal(s.storage.Roster.List[0])
}

// secret returns the shared secret for a given election.
func (s *Service) secret(id skipchain.SkipBlockID) *lib.SharedSecret {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	secret, _ := s.storage.Secrets[id.Short()]
	return secret
}

// save saves the storage onto the disk.
func (s *Service) save() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.Save(storageKey, s.storage); err != nil {
		log.Error(err)
	}
	if err := s.SaveVersion(dbVersion); err != nil {
		log.Error(err)
	}
}

// load fetches the storage from disk.
func (s *Service) load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	blob, err := s.Load(storageKey)
	if err != nil {
		return err
	} else if blob == nil {
		s.storage.Roster = onet.NewRoster([]*network.ServerIdentity{s.ServerIdentity()})
		return nil
	}

	var ok bool
	s.storage, ok = blob.(*storage)
	if !ok {
		return errors.New("service error: could not unmarshal storage")
	}

	// Don't know how this can get left to nil, but if it happens, we
	// panic in Open, so don't let it happen.
	if s.storage.Secrets == nil {
		s.storage.Secrets = make(map[string]*lib.SharedSecret)
	}
	return nil
}

func (s *Service) db() *skipchain.SkipBlockDB {
	return s.skipchain.GetDB()
}

// new initializes the service and registers all the message handlers.
func new(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
		storage: &storage{
			Secrets: make(map[string]*lib.SharedSecret),
		},
		skipchain: context.Service(skipchain.ServiceName).(*skipchain.Service),
	}

	service.RegisterHandlers(
		service.Ping,
		service.Link,
		service.Open,
		service.Cast,
		service.GetElections,
		service.GetBox,
		service.GetMixes,
		service.Shuffle,
		service.GetPartials,
		service.Decrypt,
		service.Reconstruct,
		service.LookupSciper,
	)
	skipchain.RegisterVerification(context, lib.TransactionVerifierID, service.verify)

	pin := make([]byte, 16)
	random.Bytes(pin, random.New())
	service.pin = hex.EncodeToString(pin)

	if err := service.load(); err != nil {
		return nil, err
	}

	log.Lvl1("Pin:", service.pin)
	return service, nil
}
