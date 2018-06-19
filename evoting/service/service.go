// Package service is the evoting service designed for use at EPFL.
package service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/evoting/protocol"
	"github.com/dedis/cothority/skipchain"
)

var errOnlyLeader = errors.New("operation only allowed on the leader node")

func init() {
	network.RegisterMessages(synchronizer{}, storage{})
	serviceID, _ = onet.RegisterNewService(evoting.ServiceName, new)

	raiseFdLimit()
}

// timeout for protocol termination.
var timeout = 120 * time.Second

// serviceID is the onet identifier.
var serviceID onet.ServiceID

// storageKey identifies the on-disk storage.
var storageKey = []byte("storage")

// Service is the core structure of the application.
type Service struct {
	*onet.ServiceProcessor

	skipchain *skipchain.Service
	rl        recentLog

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
	ID        skipchain.SkipBlockID
	User      uint32
	Signature []byte
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

	var genesis *skipchain.SkipBlock
	var user uint32
	sig := []byte{}

	if req.ID != nil {
		// Update an existing master chain
		id := *req.ID
		genesis = s.db().GetByID(id)
		if genesis == nil {
			return nil, errors.New("cannot find master chain to update")
		}
		if req.User == nil || req.Signature == nil {
			return nil, errors.New("missing user or sig")
		}
		user = *req.User
		sig = *req.Signature
	} else {
		var err error
		genesis, err = lib.NewSkipchain(s.skipchain, req.Roster, lib.TransactionVerifiers)
		if err != nil {
			return nil, err
		}
	}

	master := &lib.Master{
		ID:     genesis.Hash,
		Roster: req.Roster,
		Admins: req.Admins,
		Key:    req.Key,
	}
	transaction := lib.NewTransaction(master, user, sig)

	if _, err := lib.Store(s.skipchain, master.ID, transaction); err != nil {
		return nil, err
	}

	s.mutex.Lock()
	s.storage.Master = genesis.Hash
	s.storage.Roster = req.Roster
	s.mutex.Unlock()
	s.save()

	return &evoting.LinkReply{ID: genesis.Hash}, nil
}

// Open message hander. Create a new election with accompanying skipchain.
func (s *Service) Open(req *evoting.Open) (*evoting.OpenReply, error) {
	master, err := lib.GetMaster(s.skipchain, req.ID)
	if err != nil {
		return nil, err
	}

	if !s.ServerIdentity().Equal(master.Roster.List[0]) {
		return nil, errOnlyLeader
	}

	genesis, err := lib.NewSkipchain(s.skipchain, master.Roster, lib.TransactionVerifiers)
	if err != nil {
		return nil, err
	}

	root := master.Roster.NewRosterWithRoot(s.ServerIdentity())
	tree := root.GenerateNaryTree(len(master.Roster.List))
	if tree == nil {
		return nil, errors.New("error while creating the tree")
	}

	instance, _ := s.CreateProtocol(protocol.NameDKG, tree)
	protocol := instance.(*protocol.SetupDKG)
	config, _ := network.Marshal(&synchronizer{
		ID:        genesis.Hash,
		User:      req.User,
		Signature: req.Signature,
	})
	protocol.SetConfig(&onet.GenericConfig{Data: config})

	if err = protocol.Start(); err != nil {
		return nil, err
	}
	select {
	case <-protocol.Done:
		secret, _ := lib.NewSharedSecret(protocol.DKG)
		req.Election.ID = genesis.Hash
		req.Election.Master = req.ID
		req.Election.Roster = master.Roster
		req.Election.Key = secret.X
		req.Election.MasterKey = master.Key
		// req.User is untrusted in this moment, but lib.Store below will refuse to write
		// req.Election into the skipchain if req.User+req.Signature is not valid,
		// so IF it is written, then it is trusted.
		req.Election.Creator = req.User

		transaction := lib.NewTransaction(req.Election, req.User, req.Signature)
		if _, err := lib.Store(s.skipchain, req.Election.ID, transaction); err != nil {
			return nil, err
		}

		link := &lib.Link{ID: genesis.Hash}
		transaction = lib.NewTransaction(link, req.User, req.Signature)
		if _, err := lib.Store(s.skipchain, master.ID, transaction); err != nil {
			return nil, err
		}

		s.mutex.Lock()
		s.storage.Secrets[genesis.Short()] = secret
		s.mutex.Unlock()
		s.save()

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
		log.Lvl3("Got vcard (cache hit): ", res)
		return res, nil
	}

	url := "https://people.epfl.ch/cgi-bin/people/vCard"
	if req.LookupURL != "" {
		url = req.LookupURL
	}

	// Make sure the only variable expansion in there is what we want it to be.
	if strings.Contains(url, "%") {
		return nil, errors.New("percent not allowed in LookupURL")
	}
	url = fmt.Sprintf(url+"?id=%06d", sciper)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-type") != "text/x-vcard; charset=utf-8" {
		return nil, errors.New("invalid or unknown sciper")
	}

	bodyLimit := io.LimitReader(resp.Body, 1<<17)
	body, err := ioutil.ReadAll(bodyLimit)
	if err != nil {
		return nil, err
	}

	reply := &evoting.LookupSciperReply{}
	search := regexp.MustCompile("[:;]")
	for _, line := range strings.Split(string(body), "\n") {
		fr := search.Split(line, 2)
		if len(fr) != 2 {
			continue
		}
		value := strings.Replace(fr[1], "CHARSET=UTF-8:", "", 1)
		switch fr[0] {
		case "FN":
			reply.FullName = value
		case "EMAIL":
			reply.Email = value
		case "TITLE":
			reply.Title = value
		case "URL":
			reply.URL = value
		}
	}

	// Put it into the cache
	s.sciperPut(sciper, reply)

	log.Lvl3("Got vcard (cache miss): ", reply)
	return reply, nil
}

// Cast message handler. Cast a ballot in a given election.
func (s *Service) Cast(req *evoting.Cast) (*evoting.CastReply, error) {
	if !s.leader() {
		return nil, errOnlyLeader
	}
	transaction := lib.NewTransaction(req.Ballot, req.User, req.Signature)
	skipblockID, err := lib.Store(s.skipchain, req.ID, transaction)
	if err != nil {
		return nil, err
	}
	return &evoting.CastReply{ID: skipblockID}, nil
}

// GetElections message handler. Return all elections in which the given user participates.
// If signature does not match the username, then only the Master structure is returned.
func (s *Service) GetElections(req *evoting.GetElections) (*evoting.GetElectionsReply, error) {
	master, err := lib.GetMaster(s.skipchain, req.Master)
	if err != nil {
		log.LLvlf4("get master bad forward link: starting from %x", req.Master)
		buf := &bytes.Buffer{}
		lib.DebugDumpChain(s.skipchain, buf, req.Master)
		log.LLvl4(string(buf.Bytes()))
		return nil, err
	}

	links, err := master.Links(s.skipchain)
	if err != nil {
		return nil, err
	}

	// At this point, req.User is untrusted input from the bad
	// guys. We need to validate req.User before using
	// it. Usually, we count on lib.Store
	// (->skipchain.StoreSkipblock->verifier) to check the userID
	// signature for us, but since GetElections is a read-only method,
	// there is no call to lib.Store to check req.User for us.
	digest := master.ID
	for _, c := range strconv.Itoa(int(req.User)) {
		d, _ := strconv.Atoi(string(c))
		digest = append(digest, byte(d))
	}
	userValid := true
	err = schnorr.Verify(cothority.Suite, master.Key, digest, req.Signature)
	if err != nil {
		userValid = false
	}

	elections := make([]*lib.Election, 0)
	if userValid {
		for _, l := range links {
			election, err := lib.GetElection(s.skipchain, l.ID, req.CheckVoted, req.User)
			if err != nil {
				log.LLvlf4("getelection bad forward link: starting from %x", l.ID)
				buf := &bytes.Buffer{}
				lib.DebugDumpChain(s.skipchain, buf, l.ID)
				log.LLvl4(string(buf.Bytes()))
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
	return &evoting.GetBoxReply{Box: box}, nil
}

// GetMixes message handler. Vet all created mixes.
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

// GetPartials message handler. Vet all created partial decryptions.
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
	protocol := instance.(*protocol.Shuffle)
	protocol.User = req.User
	protocol.Signature = req.Signature
	protocol.Election = election
	protocol.Skipchain = s.skipchain
	protocol.LeaderParticipates = !hasParticipated

	config, _ := network.Marshal(&synchronizer{
		ID:        req.ID,
		User:      req.User,
		Signature: req.Signature,
	})
	protocol.SetConfig(&onet.GenericConfig{Data: config})
	if err = protocol.Start(); err != nil {
		return nil, err
	}
	select {
	case err := <-protocol.Finished:
		return &evoting.ShuffleReply{}, err
	case <-time.After(timeout):
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
	protocol := instance.(*protocol.Decrypt)
	protocol.User = req.User
	protocol.Signature = req.Signature
	protocol.Secret = s.secret(election.ID)
	protocol.Election = election
	protocol.Skipchain = s.skipchain
	protocol.LeaderParticipates = !participated[s.ServerIdentity().ID.String()]

	config, _ := network.Marshal(&synchronizer{
		ID:        req.ID,
		User:      req.User,
		Signature: req.Signature,
	})
	protocol.SetConfig(&onet.GenericConfig{Data: config})
	if err = protocol.Start(); err != nil {
		return nil, err
	}
	select {
	case <-protocol.Finished:
		return &evoting.DecryptReply{}, nil
	case <-time.After(timeout):
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
	case protocol.NameDKG:
		instance, _ := protocol.NewSetupDKG(node)
		protocol := instance.(*protocol.SetupDKG)
		go func() {
			<-protocol.Done
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
		protocol.Signature = sync.Signature
		protocol.Election = election
		protocol.Skipchain = s.skipchain

		config, _ := network.Marshal(&synchronizer{
			ID:        sync.ID,
			User:      sync.User,
			Signature: sync.Signature,
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
		protocol.Signature = sync.Signature
		protocol.Election = election
		protocol.Skipchain = s.skipchain

		config, _ := network.Marshal(&synchronizer{
			ID:        sync.ID,
			User:      sync.User,
			Signature: sync.Signature,
		})
		protocol.SetConfig(&onet.GenericConfig{Data: config})
		return protocol, nil
	default:
		return nil, errors.New("protocol error, unknown protocol")
	}
}

// verify is the skpchain verification handler.
func (s *Service) verify(id []byte, skipblock *skipchain.SkipBlock) bool {
	transaction := lib.UnmarshalTransaction(skipblock.Data)
	if transaction == nil {
		return false
	}

	err := transaction.Verify(skipblock.GenesisID, s.skipchain)
	if err != nil {
		log.Lvl2("verify failed:", err)
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
		rl:        recentLog{N: 100},
	}

	service.skipchain.SetBFTTimeout(5 * time.Minute)
	service.skipchain.SetPropTimeout(5 * time.Minute)

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

	// Register the log listener AFTER the PIN is already printed,
	// so it does not get published.
	log.RegisterListener(&service.rl)
	service.RegisterStatusReporter("evoting", service)

	// Fix dangling forward pointers: for each skipchain,
	// for each block, check the forward links looking for
	// a dangling one. Remove them.
	db := service.db()
	chains, err := db.GetAllSkipchains()
	if err != nil {
		log.Fatal("fix failed: getAllSkipchains %v", err)
	}
	for chid, b := range chains {
		found := false
		b = db.GetByID(b.GenesisID)
		for b != nil {
			fls := b.ForwardLink
			write := false
			for i, f := range fls {
				if db.GetByID(f.To) == nil {
					found = true
					write = true
					log.LLvlf1("  block %x: forward link to %x at level %v invalid, fixing it.", b.Hash, f.To, i)
					if i == len(b.ForwardLink)-1 {
						// This is the last one, so trim it.
						if len(b.ForwardLink) == 1 {
							// Don't leave a []SkipblockID{}, instead leave nil.
							b.ForwardLink = nil
						} else {
							b.ForwardLink = b.ForwardLink[0:i]
						}
					} else {
						b.ForwardLink[i] = nil
					}
				}
			}
			if write {
				log.LLvlf1("  writing block %x", b.Hash)
				db.StoreStompFL(b)
			}

			if len(b.ForwardLink) == 0 {
				b = nil
			} else {
				b = db.GetByID(b.ForwardLink[0].To)
			}
		}
		if found {
			log.LLvlf1("chain %x had invalid forward links", chid)
		}
	}

	return service, nil
}

// GetStatus is a function that returns the status report of the server.
func (s *Service) GetStatus() *onet.Status {
	st := &onet.Status{Field: make(map[string]string)}
	for i, msg := range s.rl.logs {
		k := fmt.Sprintf("log-%02v", i)
		st.Field[k] = msg
	}
	return st
}

type recentLog struct {
	N    int
	logs []string
}

// Log implements onet/log.Listener
func (r *recentLog) Log(lvl int, msg string) {
	if lvl <= 3 {
		r.push(msg)
	}
}

// push adds a message into the log, rolling old messages off as necessary
func (r *recentLog) push(msg string) {
	r.logs = append(r.logs, msg)
	if len(r.logs) > r.N {
		from := len(r.logs) - r.N
		r.logs = r.logs[from:]
	}
}
