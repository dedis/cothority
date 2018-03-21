// Package service is the evoting service designed for use at EPFL.
package service

import (
	"encoding/hex"
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

func init() {
	network.RegisterMessage(synchronizer{})
	serviceID, _ = onet.RegisterNewService(evoting.ServiceName, new)
}

// timeout for protocol termination.
const timeout = 60 * time.Second

// serviceID is the onet identifier.
var serviceID onet.ServiceID

// Service is the core structure of the application.
type Service struct {
	*onet.ServiceProcessor

	secrets map[string]*lib.SharedSecret // secrets is map a of DKG products.

	node *onet.Roster // nodes is a unitary roster.
	pin  string       // pin is the current service number.
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

// Link message handler. Generates a new master skipchain.
func (s *Service) Link(req *evoting.Link) (*evoting.LinkReply, error) {
	if req.Pin != s.pin {
		return nil, errors.New("link error: invalid pin")
	}

	genesis, err := lib.NewSkipchain(req.Roster, lib.TransactionVerifiers, nil)
	if err != nil {
		return nil, err
	}

	master := &lib.Master{
		ID:     genesis.Hash,
		Roster: req.Roster,
		Admins: req.Admins,
		Key:    req.Key,
	}
	transaction := lib.NewTransaction(master, 0, []byte{})
	if err := lib.Store(master.ID, master.Roster, transaction); err != nil {
		return nil, err
	}
	return &evoting.LinkReply{ID: genesis.Hash}, nil
}

// Open message hander. Create a new election with accompanying skipchain.
func (s *Service) Open(req *evoting.Open) (*evoting.OpenReply, error) {
	master, err := lib.GetMaster(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	genesis, err := lib.NewSkipchain(master.Roster, lib.TransactionVerifiers, nil)
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
		s.secrets[genesis.Short()] = secret

		transaction := lib.NewTransaction(req.Election, req.User, req.Signature)
		if err = transaction.Verify(genesis.Hash, s.node); err != nil {
			return nil, err
		}
		if err = lib.Store(req.Election.ID, s.node, transaction); err != nil {
			return nil, err
		}

		link := &lib.Link{ID: genesis.Hash}
		transaction = lib.NewTransaction(link, req.User, req.Signature)
		if err = lib.Store(master.ID, s.node, transaction); err != nil {
			return nil, err
		}
		return &evoting.OpenReply{ID: genesis.Hash, Key: secret.X}, nil
	case <-time.After(timeout):
		return nil, errors.New("open error, protocol timeout")
	}
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

	log.Lvl3("Got vcard: %v", reply)
	return reply, nil
}

// Cast message handler. Cast a ballot in a given election.
func (s *Service) Cast(req *evoting.Cast) (*evoting.CastReply, error) {
	transaction := lib.NewTransaction(req.Ballot, req.User, req.Signature)
	if err := transaction.Verify(req.ID, s.node); err != nil {
		return nil, err
	}
	if err := lib.Store(req.ID, s.node, transaction); err != nil {
		return nil, err
	}
	return &evoting.CastReply{}, nil
}

// GetElections message handler. Return all elections in which the given user participates.
func (s *Service) GetElections(req *evoting.GetElections) (*evoting.GetElectionsReply, error) {
	master, err := lib.GetMaster(s.node, req.Master)
	if err != nil {
		return nil, err
	}

	links, err := master.Links()
	if err != nil {
		return nil, err
	}

	elections := make([]*lib.Election, 0)
	for _, l := range links {
		election, err := lib.GetElection(s.node, l.ID)
		if err != nil {
			return nil, err
		}
		if election.IsUser(req.User) || election.IsCreator(req.User) {
			elections = append(elections, election)
		}
	}
	return &evoting.GetElectionsReply{Elections: elections}, nil
}

// GetBox message handler to retrieve the casted ballot in an election.
func (s *Service) GetBox(req *evoting.GetBox) (*evoting.GetBoxReply, error) {
	election, err := lib.GetElection(s.node, req.ID)
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
	election, err := lib.GetElection(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	mixes, err := election.Mixes()
	if err != nil {
		return nil, err
	}
	return &evoting.GetMixesReply{Mixes: mixes}, nil
}

// GetPartials message handler. Vet all created partial decryptions.
func (s *Service) GetPartials(req *evoting.GetPartials) (*evoting.GetPartialsReply, error) {
	election, err := lib.GetElection(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	partials, err := election.Partials()
	if err != nil {
		return nil, err
	}
	return &evoting.GetPartialsReply{Partials: partials}, nil
}

// Shuffle message handler. Initiate shuffle protocol.
func (s *Service) Shuffle(req *evoting.Shuffle) (*evoting.ShuffleReply, error) {
	decoy := lib.NewTransaction(&lib.Mix{}, req.User, req.Signature)
	if err := decoy.Verify(req.ID, s.node); err != nil {
		return nil, err
	}

	election, err := lib.GetElection(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	rooted := election.Roster.NewRosterWithRoot(s.ServerIdentity())
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}

	instance, _ := s.CreateProtocol(protocol.NameShuffle, tree)
	protocol := instance.(*protocol.Shuffle)
	protocol.User = req.User
	protocol.Signature = req.Signature
	protocol.Election = election

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
		return &evoting.ShuffleReply{}, nil
	case <-time.After(timeout):
		return nil, errors.New("shuffle error, protocol timeout")
	}
}

// Decrypt message handler. Initiate decryption protocol.
func (s *Service) Decrypt(req *evoting.Decrypt) (*evoting.DecryptReply, error) {
	decoy := lib.NewTransaction(&lib.Partial{}, req.User, req.Signature)
	if err := decoy.Verify(req.ID, s.node); err != nil {
		return nil, err
	}

	election, err := lib.GetElection(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	rooted := election.Roster.NewRosterWithRoot(s.ServerIdentity())
	tree := rooted.GenerateNaryTree(1)
	if tree == nil {
		return nil, errors.New("error while generating tree")
	}
	instance, _ := s.CreateProtocol(protocol.NameDecrypt, tree)
	protocol := instance.(*protocol.Decrypt)
	protocol.User = req.User
	protocol.Signature = req.Signature
	protocol.Secret = s.secrets[skipchain.SkipBlockID(election.ID).Short()]
	protocol.Election = election

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
	election, err := lib.GetElection(s.node, req.ID)
	if err != nil {
		return nil, err
	}

	if election.Stage < lib.Decrypted {
		return nil, errors.New("reconstruct error, election not closed yet")
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
	sync := blob.(*synchronizer)

	switch node.ProtocolName() {
	case protocol.NameDKG:
		instance, _ := protocol.NewSetupDKG(node)
		protocol := instance.(*protocol.SetupDKG)
		go func() {
			<-protocol.Done
			secret, _ := lib.NewSharedSecret(protocol.DKG)
			s.secrets[sync.ID.Short()] = secret
		}()
		return protocol, nil
	case protocol.NameShuffle:
		election, err := lib.GetElection(s.node, sync.ID)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewShuffle(node)
		protocol := instance.(*protocol.Shuffle)
		protocol.User = sync.User
		protocol.Signature = sync.Signature
		protocol.Election = election

		config, _ := network.Marshal(&synchronizer{
			ID:        sync.ID,
			User:      sync.User,
			Signature: sync.Signature,
		})
		protocol.SetConfig(&onet.GenericConfig{Data: config})
		return protocol, nil
	case protocol.NameDecrypt:
		election, err := lib.GetElection(s.node, sync.ID)
		if err != nil {
			return nil, err
		}

		instance, _ := protocol.NewDecrypt(node)
		protocol := instance.(*protocol.Decrypt)
		protocol.Secret = s.secrets[sync.ID.Short()]
		protocol.User = sync.User
		protocol.Signature = sync.Signature
		protocol.Election = election

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

func (s *Service) verify(id []byte, skipblock *skipchain.SkipBlock) bool {
	transaction := lib.UnmarshalTransaction(skipblock.Data)
	if transaction == nil {
		return false
	}

	if transaction.Verify(skipblock.GenesisID, skipblock.Roster) != nil {
		return false
	}
	return true
}

// new initializes the service and registers all the message handlers.
func new(context *onet.Context) (onet.Service, error) {
	service := &Service{
		ServiceProcessor: onet.NewServiceProcessor(context),
		secrets:          make(map[string]*lib.SharedSecret),
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

	service.node = onet.NewRoster([]*network.ServerIdentity{service.ServerIdentity()})

	log.Lvl1("Pin:", service.pin)

	return service, nil
}
