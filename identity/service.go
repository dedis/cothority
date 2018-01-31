/*
Identity is a service that allows storing of key/value pairs that belong to
a given identity that is shared between multiple devices. In order to
add/remove devices or add/remove key/value-pairs, a 'threshold' of devices
need to vote on those changes.

The key/value-pairs are stored in a personal blockchain and signed by the
cothority using forward-links, so that an external observer can check the
collective signatures and be assured that the blockchain is valid.
*/

package identity

import (
	"reflect"
	"sync"

	"errors"

	"fmt"
	"math/big"

	"github.com/dedis/cothority/messaging"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/satori/go.uuid.v1"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Identity"

// Size of nonce used in autentication
const nonceSize = 64

// Default number of skipchains, each user can create
const defaultNumberSkipchains = 5

var identityService onet.ServiceID

// VerificationIdentity gives a combined VerifyBase + verifyIdentity.
var VerificationIdentity = []skipchain.VerifierID{skipchain.VerifyBase, VerifyIdentity}

// VerifyIdentity makes sure that each new block is signed by a threshold of devices.
var VerifyIdentity = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Identity"))

func init() {
	identityService, _ = onet.RegisterNewService(ServiceName, newIdentityService)
	network.RegisterMessage(&StorageMap{})
	network.RegisterMessage(&Storage{})
}

// Service handles identities
type Service struct {
	*onet.ServiceProcessor
	*StorageMap
	anonSuite          anon.Suite
	propagateIdentity  messaging.PropagationFunc
	propagateSkipBlock messaging.PropagationFunc
	propagateData      messaging.PropagationFunc
	identitiesMutex    sync.Mutex
	skipchain          *skipchain.Client
	// limits on number of skipchain creation. Map keys are link tags
	tagsLimits map[string]int8
	// limits on number of skipchain creation. Map keys are public keys
	pointsLimits map[string]int8
	auth         authData
}

// StorageMap holds the map to the storages so it can be marshaled.
type StorageMap struct {
	Identities map[string]*Storage
}

// Storage stores one identity together with the skipblocks.
type Storage struct {
	sync.Mutex
	Latest   *Data
	Proposed *Data
	SCRoot   *skipchain.SkipBlock
	SCData   *skipchain.SkipBlock
}

type authData struct {
	// set of pins and keys
	pins map[string]struct{}
	// sets of public keys to verify linkable ring signatures
	sets []anon.Set
	// list of public keys to verify simple authentication with Schnorr sig
	keys []kyber.Point
	// list of adminKeys
	adminKeys []kyber.Point
	// set of nonces
	nonces map[string]struct{}
}

/*
 * API messages
 */

// PinRequest will check PIN of admin or print it in case PIN is not provided
// then save the admin's public key
func (s *Service) PinRequest(req *PinRequest) (network.Message, error) {
	log.Lvl3("PinRequest", s.ServerIdentity())
	if req.PIN == "" {
		pin := fmt.Sprintf("%06d", random.Int(big.NewInt(1000000), s.Suite().RandomStream()))
		s.auth.pins[pin] = struct{}{}
		log.Info("PIN:", pin)
		return nil, errors.New("Read PIN in server-log")
	}
	if _, ok := s.auth.pins[req.PIN]; !ok {
		return nil, errors.New("Wrong PIN")
	}
	s.auth.adminKeys = append(s.auth.adminKeys, req.Public)
	s.save()
	log.Lvl1("Successfully registered PIN/Public", req.PIN, req.Public)
	return nil, nil
}

// StoreKeys accepts finalStatement, verifies it and saves public credentials from it
func (s *Service) StoreKeys(req *StoreKeys) (network.Message, error) {
	log.Lvl3("Store key", s.ServerIdentity())
	var msg []byte
	var err error
	switch req.Type {
	// check FinalStatement
	case PoPAuth:
		if req.Final == nil {
			log.Error("No final statement in request")
			return nil, errors.New(
				"Invalid request")

		}
		if req.Final.Verify() != nil {
			log.Error(s.ServerIdentity(), "Invalid FinalStatement")
			return nil, errors.New(
				"Signature of final statement is invalid")

		}
		msg, err = req.Final.Hash()
		if err != nil {
			return nil, err
		}
	case PublicAuth:
		if req.Publics == nil || len(req.Publics) == 0 {
			log.Error("No public keys in request")
			return nil, errors.New(
				"Invalid request")

		}

		h := s.Suite().(kyber.HashFactory).Hash()

		for _, k := range req.Publics {
			b, err := k.MarshalBinary()
			if err != nil {
				log.Error("failed to marshal public key")
				return nil, err
			}
			_, err = h.Write(b)
			if err != nil {
				log.Error("failed to hash public key")
				return nil, err
			}

		}
		msg = h.Sum(nil)
	default:
		return nil, errors.New(
			"No such type of authentication")

	}

	// check Signature
	valid := false
	for _, key := range s.auth.adminKeys {
		if schnorr.Verify(s.Suite(), key, msg, req.Sig) == nil {
			valid = true
			break
		}
	}
	if !valid {
		log.Error(s.ServerIdentity(), "No keys for sent signature are stored")
		return nil, errors.New(
			"Invalid signature on StoreKeys")

	}
	switch req.Type {
	case PoPAuth:
		s.auth.sets = append(s.auth.sets, anon.Set(req.Final.Attendees))
	case PublicAuth:
		s.auth.keys = append(s.auth.keys, req.Publics...)
	}
	return nil, nil
}

// Authenticate will create nonce and ctx and send it to user
// It saves nonces in set
// Replay attack is impossible, because after successful authentification nonce will
// be deleted.
func (s *Service) Authenticate(ap *Authenticate) (network.Message, error) {
	ap.Ctx = []byte(ServiceName + s.ServerIdentity().String())
	ap.Nonce = make([]byte, nonceSize)
	random.Bytes(ap.Nonce, s.Suite().RandomStream())
	s.auth.nonces[string(ap.Nonce)] = struct{}{}
	return ap, nil
}

// CreateIdentity will register a new SkipChain and add it to our list of
// managed identities.
func (s *Service) CreateIdentity(ai *CreateIdentity) (network.Message, error) {
	ctx := []byte(ServiceName + s.ServerIdentity().String())
	if _, ok := s.auth.nonces[string(ai.Nonce)]; !ok {
		log.Error("Given nonce is not stored on ", s.ServerIdentity())
		return nil, fmt.Errorf("Given nonce is not stored on %s", s.ServerIdentity())
	}
	valid := false
	var tag string
	switch ai.Type {
	case PoPAuth:
		if ai.Public != nil {
			log.Error("Wrong authentication message")
			ai.Public = nil
		}
		for _, set := range s.auth.sets {
			t, err := anon.Verify(s.anonSuite, ai.Nonce, set, ctx, ai.Sig)
			if err == nil {
				tag = string(t)
				valid = true
				// The counter will be decremented in propagation handler
				if n, ok := s.tagsLimits[tag]; !ok {
					s.tagsLimits[tag] = defaultNumberSkipchains
				} else {
					if n <= 0 {
						return nil, errors.New(
							"No more skipchains is allowed to create")

					}
				}
				// authentication succeeded. we need to delete the nonce
				delete(s.auth.nonces, string(ai.Nonce))
				break
			}
		}
	case PublicAuth:
		if ai.Public == nil {
			log.Error("nil public key or signature")
			return nil, errors.New(
				"wrong public key authentication data")

		}
		found := false
		for _, k := range s.auth.keys {
			if k.Equal(ai.Public) {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New(
				"No such key is stored")

		}
		if schnorr.Verify(s.Suite(), ai.Public, ai.Nonce, *ai.SchnSig) != nil {
			valid = false
		} else {
			valid = true
		}
		str := ai.Public.String()
		if n, ok := s.pointsLimits[str]; !ok {
			s.pointsLimits[str] = defaultNumberSkipchains
		} else {
			if n <= 0 {
				return nil, errors.New(
					"No more skipchains is allowed to create")

			}
		}
	default:
		return nil, errors.New("Wrong authentication type")
	}
	if !valid {
		log.Error(s.ServerIdentity(), "Authentication is failed")
		return nil, errors.New(
			"Invalid Signature on CreateIdentity")

	}

	log.Lvlf3("%s Creating new identity with data %+v", s.ServerIdentity(), ai.Data)
	ids := &Storage{
		Latest: ai.Data,
	}
	log.Lvl3("Creating Root-skipchain")
	var err error
	ids.SCRoot, err = s.skipchain.CreateGenesis(ai.Roster, 10, 10,
		[]skipchain.VerifierID{}, nil, nil)
	if err != nil {
		return nil, err
	}
	log.Lvl3("Creating Data-skipchain", ai.Data)
	ids.SCData, err = s.skipchain.CreateGenesis(ids.SCRoot.Roster, 10, 10,
		VerificationIdentity, ai.Data, ids.SCRoot.Hash)
	if err != nil {
		return nil, err
	}

	roster := ids.SCRoot.Roster
	replies, err := s.propagateIdentity(roster, &PropagateIdentity{ids, tag, ai.Public}, propagateTimeout)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	log.Lvlf2("New chain is\n%x", []byte(ids.SCData.Hash))

	return &CreateIdentityReply{
		Root: ids.SCRoot,
		Data: ids.SCData,
	}, nil
}

// DataUpdate returns a new data-update
func (s *Service) DataUpdate(cu *DataUpdate) (network.Message, error) {
	// Check if there is something new on the skipchain - in case we've been
	// offline
	sid := s.getIdentityStorage(cu.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	reply, err := s.skipchain.GetUpdateChain(sid.SCRoot.Roster, sid.SCData.Hash)
	if err != nil {
		return nil, err
	}
	if len(reply.Update) > 1 {
		log.Lvl3("Got new data")
		// TODO: check that update-chain has correct forward-links and fits into existing blocks
		sid.SCData = reply.Update[len(reply.Update)-1]
		_, dataInt, err := network.Unmarshal(sid.SCData.Data, s.Suite())
		if err != nil {
			return nil, err
		}
		var ok bool
		sid.Latest, ok = dataInt.(*Data)
		if !ok {
			return nil, errors.New("did get invalid block from skipchain")
		}
	}
	log.Lvl3(s, "Sending data-update")
	return &DataUpdateReply{
		Data: sid.Latest,
	}, nil
}

// ProposeSend only stores the proposed data internally. Signatures
// come later.
func (s *Service) ProposeSend(p *ProposeSend) (network.Message, error) {
	log.Lvl2(s, "Storing new proposal")
	sid := s.getIdentityStorage(p.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	roster := sid.SCRoot.Roster
	replies, err := s.propagateData(roster, p, propagateTimeout)
	if err != nil {
		return nil, err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil, nil
}

// ProposeUpdate returns an eventual data-proposition
func (s *Service) ProposeUpdate(cnc *ProposeUpdate) (network.Message, error) {
	log.Lvl3(s, "Sending proposal-update to client")
	sid := s.getIdentityStorage(cnc.ID)
	if sid == nil {
		return nil, errors.New("Didn't find Identity")
	}
	sid.Lock()
	defer sid.Unlock()
	return &ProposeUpdateReply{
		Propose: sid.Proposed,
	}, nil
}

// ProposeVote takes int account a vote for the proposed data. It also verifies
// that the voter is in the latest data.
// An empty signature signifies that the vote has been rejected.
func (s *Service) ProposeVote(v *ProposeVote) (network.Message, error) {
	log.Lvl2(s, "Voting on proposal")
	// First verify if the signature is legitimate
	sid := s.getIdentityStorage(v.ID)
	if sid == nil {
		return nil, errors.New("Didn't find identity")
	}

	// Putting this in a function so that we can use defer Unlock
	// to be sure to release the lock no matter which error happens.
	err := func() error {
		sid.Lock()
		defer sid.Unlock()
		owner, ok := sid.Latest.Device[v.Signer]
		if !ok {
			return errors.New("Didn't find signer")
		}
		if sid.Proposed == nil {
			return errors.New("No proposed block")
		}
		log.Lvl3("Voting on", sid.Proposed.Device)
		hash, err := sid.Proposed.Hash(s.Suite().(kyber.HashFactory))
		if err != nil {
			return errors.New("Couldn't get hash")
		}
		if oldvote := sid.Proposed.Votes[v.Signer]; oldvote != nil {
			// It can either be an update-vote (accepted), or a second
			// vote (refused).
			if schnorr.Verify(s.Suite(), owner.Point, hash, oldvote) == nil {
				return errors.New("Already voted for that block")
			}
		}
		log.Lvl3(v.Signer, "voted", v.Signature)
		if v.Signature != nil {
			err = schnorr.Verify(s.Suite(), owner.Point, hash, v.Signature)
			if err != nil {
				return errors.New("Wrong signature: " + err.Error())
			}
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Propagate the vote
	_, err = s.propagateData(sid.SCRoot.Roster, v, propagateTimeout)
	if err != nil {
		return nil, err
	}
	votesCnt := len(sid.Proposed.Votes)
	if votesCnt >= sid.Latest.Threshold ||
		votesCnt == len(sid.Latest.Device) {
		// If we have enough signatures, make a new data-skipblock and
		// propagate it
		log.Lvl3("Having majority or all votes")

		// Making a new data-skipblock
		log.Lvl3("Sending data-block with", sid.Proposed.Device)
		reply, err := s.skipchain.StoreSkipBlock(sid.SCData, nil, sid.Proposed)
		if err != nil {
			return nil, err
		}
		_, msg, _ := network.Unmarshal(reply.Latest.Data, s.Suite())
		log.Lvl3("SB signed is", msg.(*Data).Device)
		usb := &UpdateSkipBlock{
			ID:     v.ID,
			Latest: reply.Latest,
		}
		_, err = s.propagateSkipBlock(sid.SCRoot.Roster, usb, propagateTimeout)
		if err != nil {
			return nil, err
		}
		return &ProposeVoteReply{sid.SCData}, nil
	}
	return nil, nil
}

// VerifyBlock makes sure that the new block is legit. This function will be
// called by the skipchain on all nodes before they sign.
func (s *Service) VerifyBlock(sbID []byte, sb *skipchain.SkipBlock) bool {
	// Putting it all in a function for easier error-printing
	err := func() error {
		if sb.Index == 0 {
			log.Lvl4("Always accepting genesis-block")
			return nil
		}
		_, dataInt, err := network.Unmarshal(sb.Data, s.Suite())
		if err != nil {
			return errors.New("got unknown packet")
		}
		data, ok := dataInt.(*Data)
		if !ok {
			return fmt.Errorf("got packet-type %s", reflect.TypeOf(dataInt))
		}
		hash, err := data.Hash(s.Suite().(kyber.HashFactory))
		if err != nil {
			return err
		}
		// Verify that all signatures work out
		if len(sb.BackLinkIDs) == 0 {
			return errors.New("No backlinks stored")
		}
		s.identitiesMutex.Lock()
		defer s.identitiesMutex.Unlock()
		var latest *skipchain.SkipBlock
		for _, id := range s.Identities {
			if id.SCData.Hash.Equal(sb.BackLinkIDs[0]) {
				latest = id.SCData
			}
		}
		if latest == nil {
			return errors.New("Backlink was not our latest block")
		}
		_, dataInt, err = network.Unmarshal(latest.Data, s.Suite())
		if err != nil {
			return err
		}
		dataLatest := dataInt.(*Data)
		sigCnt := 0
		for dev, sig := range data.Votes {
			if pub := dataLatest.Device[dev]; pub != nil {
				if err := schnorr.Verify(s.Suite(), pub.Point, hash, sig); err != nil {
					return err
				}
				sigCnt++
			} else {
				log.Lvl2("Not representative signature detected:", dev)
			}
		}
		if sigCnt >= dataLatest.Threshold || sigCnt == len(dataLatest.Device) {
			return nil
		}
		return errors.New("not enough signatures")
	}()
	if err != nil {
		log.Lvl2("Error while validating block:", err)
		return false
	}
	return true
}

/*
 * Internal messages
 */

// propagateData handles propagation of all configuration-proposals in the identity-service.
func (s *Service) propagateDataHandler(msg network.Message) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	id := ID(nil)
	switch msg.(type) {
	case *ProposeSend:
		id = msg.(*ProposeSend).ID
	case *ProposeVote:
		id = msg.(*ProposeVote).ID
	default:
		log.Errorf("Got an unidentified propagation-request: %v", msg)
		return
	}

	if id != nil {
		sid := s.getIdentityStorage(id)
		if sid == nil {
			log.Error("Didn't find entity in", s)
			return
		}
		sid.Lock()
		defer sid.Unlock()
		switch msg.(type) {
		case *ProposeSend:
			p := msg.(*ProposeSend)
			sid.Proposed = p.Propose
		case *ProposeVote:
			v := msg.(*ProposeVote)
			d := sid.Latest.Device[v.Signer]
			if d == nil {
				log.Error("Got signature from unknown device", v.Signer)
				return
			}
			hash, err := sid.Proposed.Hash(s.Suite().(kyber.HashFactory))
			if err != nil {
				log.Error("Couldn't hash proposed block:", err)
				return
			}
			err = schnorr.Verify(s.Suite(), d.Point, hash, v.Signature)
			if err != nil {
				log.Error("Got invalid signature:", err)
				return
			}
			if len(sid.Proposed.Votes) == 0 {
				// Make sure the map is initialised
				sid.Proposed.Votes = make(map[string][]byte)
			}
			sid.Proposed.Votes[v.Signer] = v.Signature
		}
		s.save()
	}
}

// propagateSkipBlock saves a new skipblock to the identity
func (s *Service) propagateSkipBlockHandler(msg network.Message) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	usb, ok := msg.(*UpdateSkipBlock)
	if !ok {
		log.Error("Wrong message-type")
		return
	}
	sid := s.getIdentityStorage(usb.ID)
	if sid == nil {
		log.Error("Didn't find entity in", s)
		return
	}
	sid.Lock()
	defer sid.Unlock()
	skipblock := msg.(*UpdateSkipBlock).Latest
	_, msgLatest, err := network.Unmarshal(skipblock.Data, s.Suite())
	if err != nil {
		log.Error(err)
		return
	}
	al, ok := msgLatest.(*Data)
	if !ok {
		log.Error(err)
		return
	}
	sid.SCData = skipblock
	sid.Latest = al
	sid.Proposed = nil
	s.save()
}

// propagateIdentity stores a new identity in all nodes.
func (s *Service) propagateIdentityHandler(msg network.Message) {
	log.Lvlf4("Got msg %+v %v", msg, reflect.TypeOf(msg).String())
	pi, ok := msg.(*PropagateIdentity)
	if !ok {
		log.Error("Got a wrong message for propagation")
		return
	}
	if pi.Tag != "" {
		if n, ok := s.tagsLimits[string(pi.Tag)]; ok {
			if n <= 0 {
				// unreachable in normal work mode of nodes
				log.Error("No more skipchains is allowed to create")
				return
			}
		} else {
			s.tagsLimits[string(pi.Tag)] = defaultNumberSkipchains
		}
		s.tagsLimits[string(pi.Tag)]--
	} else if pi.Public != nil {
		str := pi.Public.String()
		if n, ok := s.pointsLimits[str]; ok {
			if n <= 0 {
				// unreachable in normal work mode of nodes
				log.Error("No more skipchains is allowed to create")
				return
			}
		} else {
			s.pointsLimits[str] = defaultNumberSkipchains
		}
		s.pointsLimits[str]--
	}
	id := ID(pi.SCData.Hash)
	if s.getIdentityStorage(id) != nil {
		log.Error("Couldn't store new identity")
		return
	}
	log.Lvl3("Storing identity in", s)
	s.setIdentityStorage(id, pi.Storage)
	return
}

// getIdentityStorage returns the corresponding IdentityStorage or nil
// if none was found
func (s *Service) getIdentityStorage(id ID) *Storage {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	is, ok := s.Identities[string(id)]
	if !ok {
		return nil
	}
	return is
}

// setIdentityStorage saves an IdentityStorage
func (s *Service) setIdentityStorage(id ID, is *Storage) {
	s.identitiesMutex.Lock()
	defer s.identitiesMutex.Unlock()
	log.Lvlf3("%s %x %v", s.Context.ServerIdentity(), id[0:8], is.Latest.Device)
	s.Identities[string(id)] = is
	s.save()
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	err := s.Save("storage", s.StorageMap)
	if err != nil {
		log.Error("Couldn't save file:", err)
	}
}

func (s *Service) clearIdentities() {
	s.Identities = make(map[string]*Storage)
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.StorageMap, ok = msg.(*StorageMap)
	if !ok {
		return errors.New("Data of wrong type")
	}
	if s.Identities == nil {
		s.Identities = make(map[string]*Storage)
	}
	log.Lvl3("Successfully loaded")
	return nil
}

func newIdentityService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		StorageMap:       &StorageMap{make(map[string]*Storage)},
		skipchain:        skipchain.NewClient(),
	}
	if as, ok := c.Suite().(anon.Suite); ok {
		s.anonSuite = as
	} else {
		return nil, errors.New("suite does not implement anon.Suite")
	}

	var err error
	s.propagateIdentity, err =
		messaging.NewPropagationFunc(c, "IdentityPropagateID", s.propagateIdentityHandler, 0)
	if err != nil {
		return nil, err
	}
	s.propagateSkipBlock, err =
		messaging.NewPropagationFunc(c, "IdentityPropagateSB", s.propagateSkipBlockHandler, 0)
	if err != nil {
		return nil, err
	}
	s.propagateData, err =
		messaging.NewPropagationFunc(c, "IdentityPropagateConf", s.propagateDataHandler, 0)
	if err != nil {
		return nil, err
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := s.RegisterHandlers(s.ProposeSend, s.ProposeVote,
		s.CreateIdentity, s.ProposeUpdate, s.DataUpdate, s.PinRequest,
		s.StoreKeys, s.Authenticate); err != nil {
		log.Error("Registration error:", err)
		return nil, err
	}
	skipchain.RegisterVerification(c, VerifyIdentity, s.VerifyBlock)
	s.auth.pins = make(map[string]struct{})
	s.auth.nonces = make(map[string]struct{})
	s.auth.sets = make([]anon.Set, 0)
	s.auth.adminKeys = make([]kyber.Point, 0)
	s.tagsLimits = make(map[string]int8)
	s.pointsLimits = make(map[string]int8)
	return s, nil
}
