package service

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/evoting/protocol"
	"github.com/dedis/cothority/skipchain"
)

// timeout for protocol termination.
const timeout = 60 * time.Second

var (
	errInvalidPin       = errors.New("Invalid pin")
	errInvalidSignature = errors.New("Invalid signature")
	errNotLoggedIn      = errors.New("User is not logged in")
	errNotAdmin         = errors.New("Admin privileges required")
	errNotCreator       = errors.New("User is not election creator")
	errNotPart          = errors.New("User is not part of election")

	errNotShuffled      = errors.New("Election has not been shuffled yet")
	errNotDecrypted     = errors.New("Election has not been decrypted yet")
	errAlreadyShuffled  = errors.New("Election has already been shuffled")
	errAlreadyDecrypted = errors.New("Election has already been decrypted")
	errAlreadyClosed    = errors.New("Election has already been closed")
	errCorrupt          = errors.New("Election skipchain is corrupt")

	errProtocolUnknown = errors.New("Protocol unknown")
	errProtocolTimeout = errors.New("Protocol timeout")
)

// serviceID is the onet identifier.
var serviceID onet.ServiceID

// Service is the core structure of the application.
type Service struct {
	*onet.ServiceProcessor

	secrets map[string]*lib.SharedSecret // secrets is map a of DKG products.

	state *state       // state is the log of currently logged in users.
	node  *onet.Roster // nodes is a unitary roster.
	pin   string       // pin is the current service number.
}

// synchronizer is broadcasted to all roster nodes before every protocol.
type synchronizer struct {
	ID skipchain.SkipBlockID
}

func init() {
	network.RegisterMessage(synchronizer{})
	serviceID, _ = onet.RegisterNewService(evoting.ServiceName, new)
}

// Ping message handler.
func (s *Service) Ping(req *evoting.Ping) (*evoting.Ping, error) {
	return &evoting.Ping{Nonce: req.Nonce + 1}, nil
}

// Link message handler. Generates a new master skipchain.
func (s *Service) Link(req *evoting.Link) (*evoting.LinkReply, error) {
	if req.Pin != s.pin {
		return nil, errInvalidPin
	}

	genesis, err := lib.New(req.Roster, nil)
	if err != nil {
		return nil, err
	}

	master := &lib.Master{
		ID:     genesis.Hash,
		Roster: req.Roster,
		Admins: req.Admins,
		Key:    req.Key,
	}
	if err := master.Store(master); err != nil {
		return nil, err
	}
	return &evoting.LinkReply{ID: genesis.Hash}, nil
}

// Open message handler. Generates a new election.
func (s *Service) Open(req *evoting.Open) (*evoting.OpenReply, error) {
	if _, err := s.vet(req.Token, nil, true); err != nil {
		return nil, err
	}

	master, err := lib.FetchMaster(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	genesis, err := lib.New(master.Roster, nil)
	if err != nil {
		return nil, err
	}

	size := len(master.Roster.List)
	rooted := master.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return nil, errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(size)
	if tree == nil {
		return nil, errors.New("error while creating the tree")
	}
	instance, err := s.CreateProtocol(protocol.NameDKG, tree)
	protocol := instance.(*protocol.SetupDKG)

	config, _ := network.Marshal(&synchronizer{genesis.Hash})
	protocol.SetConfig(&onet.GenericConfig{Data: config})

	if err = protocol.Start(); err != nil {
		return nil, err
	}

	select {
	case <-protocol.Done:
		secret, _ := lib.NewSharedSecret(protocol.DKG)
		req.Election.ID = genesis.Hash
		req.Election.Roster = master.Roster
		req.Election.Key = secret.X
		s.secrets[genesis.Short()] = secret

		if err := req.Election.Store(req.Election); err != nil {
			return nil, err
		}

		if err = master.Store(&lib.Link{ID: genesis.Hash}); err != nil {
			return nil, err
		}

		return &evoting.OpenReply{ID: genesis.Hash, Key: secret.X}, nil
	case <-time.After(timeout):
		return nil, errProtocolTimeout
	}
}

// Login message handler. Log potential user in state.
func (s *Service) Login(req *evoting.Login) (*evoting.LoginReply, error) {
	master, err := lib.FetchMaster(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	if req.Verify(master.Key) != nil {
		return nil, errInvalidSignature
	}

	links, err := master.Links()
	if err != nil {
		return nil, err
	}

	elections := make([]*lib.Election, 0)
	for _, link := range links {
		election, err := lib.FetchElection(s.node, link.ID)
		if err != nil {
			return nil, err
		}

		if election.IsUser(req.User) || election.IsCreator(req.User) {
			elections = append(elections, election)
		}
	}

	admin := master.IsAdmin(req.User)
	token := s.state.register(req.User, admin)
	return &evoting.LoginReply{Token: token, Admin: admin, Elections: elections}, nil
}

// LookupSciper calls https://people.epfl.ch/cgi-bin/people/vCard?id=sciper
// to convert scipers to names
func (s *Service) LookupSciper(req *evoting.LookupSciper) (*evoting.LookupSciperReply, error) {
	if len(req.Sciper) != 6 {
		return nil, errors.New("sciper should be 6 digits only")
	}
	sciper, err := strconv.Atoi(req.Sciper)
	if err != nil {
		return nil, errors.New("couldn't convert sciper to integer")
	}

	url := "https://people.epfl.ch/cgi-bin/people/vCard?id=%06d"
	if req.LookupURL != "" {
		url = req.LookupURL
	}
	url = fmt.Sprintf(url, sciper)

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
	resp.Body.Close()

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

	log.Lvl3("Got vcard: %v", reply)
	return reply, nil
}

// Cast message handler. Cast a ballot in a given election.
func (s *Service) Cast(req *evoting.Cast) (*evoting.CastReply, error) {
	election, err := s.vet(req.Token, req.ID, false)
	if err != nil {
		return nil, err
	}

	if election.Stage >= lib.Shuffled {
		return nil, errAlreadyClosed
	}

	if err = election.Store(req.Ballot); err != nil {
		return nil, err
	}

	return &evoting.CastReply{}, nil
}

// GetBox message handler. Vet accumulated encrypted ballots.
func (s *Service) GetBox(req *evoting.GetBox) (*evoting.GetBoxReply, error) {
	election, err := s.vet(req.Token, req.ID, false)
	if err != nil {
		return nil, err
	}

	box, err := election.Box()
	if err != nil {
		return nil, err
	}

	return &evoting.GetBoxReply{Box: box}, nil
}

// GetMixes message handler. Vet all created mixes.
func (s *Service) GetMixes(req *evoting.GetMixes) (*evoting.GetMixesReply, error) {
	election, err := s.vet(req.Token, req.ID, false)
	if err != nil {
		return nil, err
	}

	if election.Stage < lib.Shuffled {
		return nil, errNotShuffled
	}

	mixes, err := election.Mixes()
	if err != nil {
		return nil, err
	}

	return &evoting.GetMixesReply{Mixes: mixes}, nil
}

// GetPartials message handler. Vet all created partial decryptions.
func (s *Service) GetPartials(req *evoting.GetPartials) (*evoting.GetPartialsReply, error) {
	election, err := s.vet(req.Token, req.ID, false)
	if err != nil {
		return nil, err
	}

	if election.Stage < lib.Decrypted {
		return nil, errNotDecrypted
	}

	partials, err := election.Partials()
	if err != nil {
		return nil, err
	}

	return &evoting.GetPartialsReply{Partials: partials}, nil
}

// Shuffle message handler. Initiate shuffle protocol.
func (s *Service) Shuffle(req *evoting.Shuffle) (*evoting.ShuffleReply, error) {
	election, err := s.vet(req.Token, req.ID, true)
	if err != nil {
		return nil, err
	}

	if election.Stage >= lib.Shuffled {
		return nil, errAlreadyShuffled
	}

	rooted := election.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return nil, errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}
	instance, _ := s.CreateProtocol(protocol.NameShuffle, tree)
	protocol := instance.(*protocol.Shuffle)
	protocol.Election = election

	config, _ := network.Marshal(&synchronizer{election.ID})
	protocol.SetConfig(&onet.GenericConfig{Data: config})

	if err = protocol.Start(); err != nil {
		return nil, err
	}

	select {
	case <-protocol.Finished:
		return &evoting.ShuffleReply{}, nil
	case <-time.After(timeout):
		return nil, errProtocolTimeout
	}
}

// Decrypt message handler. Initiate decryption protocol.
func (s *Service) Decrypt(req *evoting.Decrypt) (*evoting.DecryptReply, error) {
	election, err := s.vet(req.Token, req.ID, true)
	if err != nil {
		return nil, err
	}

	if election.Stage >= lib.Decrypted {
		return nil, errAlreadyDecrypted
	} else if election.Stage < lib.Shuffled {
		return nil, errNotShuffled
	}

	rooted := election.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return nil, errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("error while generating tree")
	}
	instance, _ := s.CreateProtocol(protocol.NameDecrypt, tree)
	protocol := instance.(*protocol.Decrypt)
	protocol.Secret = s.secrets[skipchain.SkipBlockID(election.ID).Short()]
	protocol.Election = election

	config, _ := network.Marshal(&synchronizer{election.ID})
	protocol.SetConfig(&onet.GenericConfig{Data: config})

	if err = protocol.Start(); err != nil {
		return nil, err
	}

	select {
	case <-protocol.Finished:
		return &evoting.DecryptReply{}, nil
	case <-time.After(timeout):
		return nil, errProtocolTimeout
	}
}

// Reconstruct message handler. Fully decrypt partials using Lagrange interpolation.
func (s *Service) Reconstruct(req *evoting.Reconstruct) (*evoting.ReconstructReply, error) {
	election, err := s.vet(req.Token, req.ID, false)
	if err != nil {
		return nil, err
	}

	if election.Stage < lib.Decrypted {
		return nil, errNotDecrypted
	}

	partials, err := election.Partials()
	if err != nil {
		return nil, err
	}

	points := make([]kyber.Point, 0)

	n := len(election.Roster.List)
	for i := 0; i < len(partials[0].Points); i++ {
		shares := make([]*share.PubShare, n)
		for j, partial := range partials {
			shares[j] = &share.PubShare{I: j, V: partial.Points[i]}
		}

		message, _ := share.RecoverCommit(cothority.Suite, shares, n, n)
		points = append(points, message)
	}

	return &evoting.ReconstructReply{Points: points}, nil
}

// NewProtocol hooks non-root nodes into created protocols.
func (s *Service) NewProtocol(node *onet.TreeNodeInstance, conf *onet.GenericConfig) (
	onet.ProtocolInstance, error) {

	_, blob, _ := network.Unmarshal(conf.Data, cothority.Suite)
	id := blob.(*synchronizer).ID

	switch node.ProtocolName() {
	case protocol.NameDKG:
		instance, _ := protocol.NewSetupDKG(node)
		protocol := instance.(*protocol.SetupDKG)
		go func() {
			<-protocol.Done
			secret, _ := lib.NewSharedSecret(protocol.DKG)
			s.secrets[id.Short()] = secret
		}()
		return protocol, nil
	case protocol.NameShuffle:
		election, err := lib.FetchElection(s.node, id)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewShuffle(node)
		protocol := instance.(*protocol.Shuffle)
		protocol.Election = election

		config, _ := network.Marshal(&synchronizer{election.ID})
		protocol.SetConfig(&onet.GenericConfig{Data: config})

		return protocol, nil
	case protocol.NameDecrypt:
		election, err := lib.FetchElection(s.node, id)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewDecrypt(node)
		protocol := instance.(*protocol.Decrypt)
		protocol.Secret = s.secrets[id.Short()]
		protocol.Election = election

		config, _ := network.Marshal(&synchronizer{election.ID})
		protocol.SetConfig(&onet.GenericConfig{Data: config})

		return protocol, nil
	default:
		return nil, errProtocolUnknown
	}
}

// vet checks the user stamp and fetches the election corresponding to the
// given id while making sure the user is either a voter or the creator.
func (s *Service) vet(token string, id skipchain.SkipBlockID, admin bool) (
	*lib.Election, error) {

	stamp := s.state.get(token)
	if stamp == nil {
		return nil, errNotLoggedIn
	} else if admin && !stamp.admin {
		return nil, errNotAdmin
	}

	if id != nil {
		election, err := lib.FetchElection(s.node, id)
		if err != nil {
			return nil, err
		} else if election.Stage == lib.Corrupt {
			return nil, errCorrupt
		}

		if admin && !election.IsCreator(stamp.user) {
			return nil, errNotCreator
		} else if !admin && !election.IsUser(stamp.user) {
			return nil, errNotPart
		}
		return election, nil
	}
	return nil, nil
}

// new initializes the service and registers all the message handlers.
func new(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
		secrets:          make(map[string]*lib.SharedSecret),
		state:            &state{log: make(map[string]*stamp)},
		pin:              nonce(48),
	}

	err := service.RegisterHandlers(service.Ping, service.Link, service.Open, service.Login,
		service.Cast, service.GetBox, service.GetMixes, service.Shuffle,
		service.GetPartials, service.Decrypt, service.Reconstruct, service.LookupSciper,
	)
	if err != nil {
		return nil, err
	}

	service.node = onet.NewRoster([]*network.ServerIdentity{service.ServerIdentity()})

	log.Lvl1("Pin:", service.pin)

	return service, nil
}
