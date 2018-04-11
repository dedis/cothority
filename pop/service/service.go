// Package service implements a Proof-of-Personhood (pop) party.
//
// Proof-of-personhood parties provide a number of "attendees" with an
// "anonymous token" that enables them to "authenticate" to a service as being
// part of the party.
//
// These parties are held by a number of "organisers" who set up a party by
// defining place, time and purpose of that party and by publishing a "party
// configuration" that is signed by the organisers "conodes".  At the party,
// they "register" all attendees' public keys.  Once the party is over, they
// create a "party transcript" that is signed by all organisers' conodes.
//
// The attendees create their "pop token" by joining their private key to the
// party transcript. They can now use that token to sign a "message" in a
// "context" from a service and send the resulting "signature" and "tag" back
// to the service.
//
// On the service's side, it can use the party transcript to verify that the
// signature has been created using a private key present in the party
// transcript.  The tag will be unique to that attendee/context pair, but
// another service using another context will not be able to link two tags to
// the same or different attendee.
package service

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoinx"
	"github.com/dedis/cothority/ftcosi/protocol"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func init() {
	// This service depends on EDDSA signatures, so it must not
	// be instantiated with other suites.
	if cothority.Suite == suites.MustFind("Ed25519") {
		onet.RegisterNewService(Name, newService)
		network.RegisterMessage(&saveData{})
		checkConfigID = network.RegisterMessage(CheckConfig{})
		checkConfigReplyID = network.RegisterMessage(CheckConfigReply{})
		mergeConfigID = network.RegisterMessage(MergeConfig{})
		mergeConfigReplyID = network.RegisterMessage(MergeConfigReply{})
	}
}

// Name is the name to refer to the Template service from another
// package.
const Name = "PoPServer"
const cfgName = "pop.bin"
const bftSignFinal = "BFTFinal"
const bftSignMerge = "PopBFTSignMerge"
const propagFinal = "PoPPropagateFinal"
const propagDescription = "PoPPropagateDescription"
const timeout = 60 * time.Second

// SIGSIZE size of signature
const SIGSIZE = 64

// DELIMETER in locations field in PopDesc
// const timeout = 60 * time.Second
const DELIMETER = "; "

var checkConfigID network.MessageTypeID
var checkConfigReplyID network.MessageTypeID
var mergeConfigID network.MessageTypeID
var mergeConfigReplyID network.MessageTypeID
var mergeCheckID network.MessageTypeID
var mergeCheckReplyID network.MessageTypeID

var storageKey = []byte("storage")

// Service represents data needed for one pop-party.
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	path string
	data *saveData
	// propagate final message
	propagateFinalize messaging.PropagationFunc
	// propagate possible new descriptions
	propagateDescription messaging.PropagationFunc
	// Sync tools
	// key of map is ID of party
	// synchronizing inside one party
	syncs map[string]*syncChans
	// verifyFinalBuffer is a temporary buffer for bftVerifyFinal results
	// it is set by the bftVerifyFinal if the verification is successful,
	// and unset by bftVerifyFinalAck. Every key/value pair stored in it
	// should be cleared at the end of the bft protocol.
	verifyFinalBuffer sync.Map
	// verifyMergeBuffer is a temporary buffer for bftVerifyMerge results.
	// The logic is the same for verifyFinalBuffer above.
	verifyMergeBuffer sync.Map
	// all proposed configurations - they don't need to be saved. Every time they
	// are retrived, they will be deleted.
	proposedDescription []PopDesc
}

type saveData struct {
	// Pin holds the randomly chosen pin
	Pin string
	// Public key of linked pop
	Public kyber.Point
	// The final statements
	// key of map is ID of party
	Finals map[string]*FinalStatement
	// The info used in merge process
	// key is ID of party
	merges map[string]*merge
}

type merge struct {
	// Map of final statements of parties that are going to be merged together
	statementsMap map[string]*FinalStatement
	// Flag tells that message distribution has already started
	distrib bool
}

type syncChans struct {
	// channel to return the configreply
	ccChannel chan *CheckConfigReply
	// channel to return the mergereply
	mcChannel chan *MergeConfigReply
}

// ErrorReadPIN means that there is a PIN to read in the server-logs
var ErrorReadPIN = errors.New("Read PIN in server-log")

// PinRequest prints out a pin if none is given, else it verifies it has the
// correct pin, and if so, it stores the public key as reference.
func (s *Service) PinRequest(req *PinRequest) (network.Message, error) {
	if req.Pin == "" {
		s.data.Pin = fmt.Sprintf("%06d", random.Int(big.NewInt(1000000), s.Suite().RandomStream()))
		log.Info("PIN:", s.data.Pin)
		return nil, ErrorReadPIN
	}
	if req.Pin != s.data.Pin {
		return nil, errors.New("Wrong PIN")
	}
	s.data.Public = req.Public
	s.save()
	log.Lvl1("Successfully registered PIN/Public", s.data.Pin, req.Public)
	return nil, nil
}

// StoreConfig saves the pop-config locally
func (s *Service) StoreConfig(req *StoreConfig) (network.Message, error) {
	log.Lvlf2("StoreConfig: %s %v %x", s.Context.ServerIdentity(), req.Desc, req.Desc.Hash())
	if req.Desc.Roster == nil {
		return nil, errors.New("no roster set")
	}
	if s.data.Public == nil {
		return nil, errors.New("Not linked yet")
	}
	hash := req.Desc.Hash()
	if err := schnorr.Verify(s.Suite(), s.data.Public, hash, req.Signature); err != nil {
		return nil, errors.New("Invalid signature" + err.Error())
	}
	s.data.Finals[string(hash)] = &FinalStatement{Desc: req.Desc, Signature: []byte{}}
	s.syncs[string(hash)] = &syncChans{
		ccChannel: make(chan *CheckConfigReply, 1),
		mcChannel: make(chan *MergeConfigReply, 1),
	}
	if len(req.Desc.Parties) > 0 {
		meta := newMerge()
		s.data.merges[string(hash)] = meta
		// party is merged with itself already
		meta.statementsMap[string(hash)] = s.data.Finals[string(hash)]
	}
	s.save()

	// And send the proposed config to all other nodes, so that an eventual client
	// can fetch it from there.
	replies, err := s.propagateDescription(req.Desc.Roster, req.Desc, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if replies != len(req.Desc.Roster.List) {
		log.Warn("Did only get", replies)
	}
	return &StoreConfigReply{hash}, nil
}

// GetProposals returns all collected proposals so far and deletes the proposals.
func (s *Service) GetProposals(req *GetProposals) (*GetProposalsReply, error) {
	tmp := s.proposedDescription
	s.proposedDescription = make([]PopDesc, 0)
	log.Lvlf2("Sending proposals: %+v", tmp)
	return &GetProposalsReply{
		Proposals: tmp,
	}, nil
}

// FinalizeRequest returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
func (s *Service) FinalizeRequest(req *FinalizeRequest) (network.Message, error) {
	log.Lvlf2("Finalize: %s %+v", s.Context.ServerIdentity(), req)
	if s.data.Public == nil {
		return nil, errors.New("Not linked yet")
	}
	hash, err := req.hash()
	if err != nil {
		return nil, err
	}
	if err := schnorr.Verify(s.Suite(), s.data.Public, hash, req.Signature); err != nil {
		return nil, errors.New("Invalid signature:" + err.Error())
	}

	var final *FinalStatement
	var ok bool
	if final, ok = s.data.Finals[string(req.DescID)]; !ok || final == nil || final.Desc == nil {
		return nil, errors.New("No config found")
	}
	if final.Verify() == nil {
		log.Lvl2("Sending known final statement")
		return &FinalizeResponse{final}, nil
	}

	// Contact all other nodes and ask them if they already have a config.
	final.Attendees = make([]kyber.Point, len(req.Attendees))
	copy(final.Attendees, req.Attendees)
	cc := &CheckConfig{final.Desc.Hash(), req.Attendees}
	for _, c := range final.Desc.Roster.List {
		if !c.ID.Equal(s.ServerIdentity().ID) {
			log.Lvl2("Contacting", c, cc.Attendees)
			err := s.SendRaw(c, cc)
			if err != nil {
				return nil, err
			}
			if syncData, ok := s.syncs[string(req.DescID)]; ok {
				rep := <-syncData.ccChannel
				if rep == nil {
					return nil, errors.New(
						"Not all other conodes finalized yet")

				}
			}
		}
	}
	data, err := final.ToToml()
	if err != nil {
		return nil, err
	}
	// Create signature and propagate it
	err = s.signAndPropagate(final, bftSignFinal, data)
	if err != nil {
		return nil, err
	}
	return &FinalizeResponse{final}, nil
}

// FetchFinal returns FinalStatement by hash
// used after Finalization
func (s *Service) FetchFinal(req *FetchRequest) (network.Message,
	error) {
	log.Lvlf2("FetchFinal: %s %v", s.Context.ServerIdentity(), req.ID)
	var fs *FinalStatement
	var ok bool
	if fs, ok = s.data.Finals[string(req.ID)]; !ok {
		return nil, errors.New(
			"No config found")

	}
	if len(fs.Signature) <= 0 {
		return nil, errors.New(
			"Not all other conodes finalized yet")

	}
	return &FinalizeResponse{fs}, nil
}

// MergeRequest starts Merge process and returns FinalStatement after
// used after finalization
func (s *Service) MergeRequest(req *MergeRequest) (network.Message,
	error) {
	log.Lvlf2("MergeRequest: %s %v", s.Context.ServerIdentity(), req.ID)
	if s.data.Public == nil {
		return nil, errors.New("Not linked yet")
	}

	if err := schnorr.Verify(s.Suite(), s.data.Public, req.ID, req.Signature); err != nil {
		return nil, errors.New("Invalid signature: err")
	}

	final, ok := s.data.Finals[string(req.ID)]
	if !ok {
		return nil, errors.New(
			"No config found")

	}
	if final.Merged {
		return &FinalizeResponse{final}, nil
	}
	m, ok := s.data.merges[string(req.ID)]
	if !ok {
		return nil, errors.New(
			"No meta found")

	}
	syncData, ok := s.syncs[string(req.ID)]
	if !ok {
		return nil, errors.New(
			"No meta found")

	}

	if len(final.Signature) <= 0 || final.Verify() != nil {
		return nil, errors.New(
			"Not all other conodes finalized yet")

	}
	if len(final.Desc.Parties) <= 1 {
		return nil, errors.New(
			"Party is unmergeable")

	}

	// Check if the party is the merge list
	found := false
	for _, party := range final.Desc.Parties {
		if Equal(party.Roster, final.Desc.Roster) {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New(
			"Party is not included in merge list")

	}
	newFinal, err := s.merge(final, m)
	if err != nil {
		return nil, err
	}

	// Decode mapStatements to send it on signing
	data, err := encodeMapFinal(m.statementsMap)
	if err != nil {
		return nil, err
	}

	err = s.signAndPropagate(newFinal, bftSignMerge, data)
	if err != nil {
		m.distrib = false
		return nil, err
	}
	// refresh data
	hash := string(newFinal.Desc.Hash())
	s.data.Finals[hash] = newFinal
	s.data.merges[hash] = m
	s.syncs[hash] = syncData
	m.statementsMap = make(map[string]*FinalStatement)
	m.statementsMap[hash] = newFinal

	s.save()
	return &FinalizeResponse{newFinal}, nil
}

// MergeConfig receives a final statement of requesting party,
// hash of local party. Checks if they are from one merge party and responses with
// own finalStatement
func (s *Service) MergeConfig(req *network.Envelope) {
	log.Lvlf2("%s gets MergeConfig from %s", s.Context.ServerIdentity().String(),
		req.ServerIdentity.String())
	mc, ok := req.Msg.(*MergeConfig)
	if !ok {
		log.Errorf("Didn't get a MergeConfig: %#v", req.Msg)
		return
	}
	if mc.Final == nil || mc.Final.Desc == nil {
		log.Error("MergeConfig is empty")
		return
	}
	mcr := &MergeConfigReply{PopStatusOK, mc.Final.Desc.Hash(), nil}
	var final *FinalStatement
	func() {
		var m *merge
		if final, ok = s.data.Finals[string(mc.ID)]; !ok {
			log.Errorf("No config found")
			mcr.PopStatus = PopStatusWrongHash
			return
		}
		if m, ok = s.data.merges[string(mc.ID)]; !ok {
			log.Error("No merge set found")
			mcr.PopStatus = PopStatusWrongHash
			return
		}
		if final.Verify() != nil {
			log.Error("Local party's signature is invalid")
			mcr.PopStatus = PopStatusMergeNonFinalized
			return
		}
		mcr.PopStatus = final.VerifyMergeStatement(mc.Final)
		if mcr.PopStatus < PopStatusOK {
			return
		}
		if _, ok = m.statementsMap[string(mc.Final.Desc.Hash())]; ok {
			log.Lvl2(s.ServerIdentity(), "Party was already merged, sent from",
				req.ServerIdentity.String())
			mcr.PopStatus = PopStatusMergeError
		} else {
			m.statementsMap[string(mc.Final.Desc.Hash())] = mc.Final
		}
	}()

	if mcr.PopStatus == PopStatusOK {
		mcr.Final = final
	}

	err := s.SendRaw(req.ServerIdentity, mcr)
	if err != nil {
		log.Error("Couldn't send reply:", err)
	}
}

// MergeConfigReply processes the response after MergeConfig message
func (s *Service) MergeConfigReply(req *network.Envelope) {
	log.Lvlf2("MergeConfigReply: %s from %s got %v",
		s.ServerIdentity(), req.ServerIdentity.String(), req.Msg)
	mcrVal, ok := req.Msg.(*MergeConfigReply)

	if syncData, found := s.syncs[string(mcrVal.PopHash)]; found {
		mcr := func() *MergeConfigReply {
			if !ok {
				log.Errorf("Didn't get a CheckConfigReply: %v", req.Msg)
				return nil
			}
			var final *FinalStatement
			if final, ok = s.data.Finals[string(mcrVal.PopHash)]; !ok {
				log.Error("No party with given hash")
				return nil
			}
			if mcrVal.PopStatus < PopStatusOK {
				log.Lvl2("Wrong pop-status:", mcrVal.PopStatus)
				return mcrVal
			}
			if mcrVal.Final == nil {
				log.Error("Empty FinalStatement in reply")
				return nil
			}
			mcrVal.PopStatus = final.VerifyMergeStatement(mcrVal.Final)
			return mcrVal
		}()
		if len(syncData.mcChannel) == 0 {
			syncData.mcChannel <- mcr
		}
	} else {
		log.Error("No hash for sync found")
	}
}

// CheckConfig receives a hash for a config and a list of attendees. It returns
// a CheckConfigReply filled according to this structure's description. If
// the config has been found, it strips its own attendees from the one missing
// in the other configuration.
func (s *Service) CheckConfig(req *network.Envelope) {
	cc, ok := req.Msg.(*CheckConfig)
	if !ok {
		log.Errorf("Didn't get a CheckConfig: %#v", req.Msg)
		return
	}

	ccr := &CheckConfigReply{PopStatusOK, cc.PopHash, nil}
	if len(s.data.Finals) > 0 {
		var final *FinalStatement
		if final, ok = s.data.Finals[string(cc.PopHash)]; !ok {
			ccr.PopStatus = PopStatusWrongHash
		} else {
			final.Attendees = intersectAttendees(final.Attendees, cc.Attendees)
			if len(final.Attendees) == 0 {
				ccr.PopStatus = PopStatusNoAttendees
			} else {
				ccr.PopStatus = PopStatusOK
				ccr.Attendees = final.Attendees
			}
		}
	}
	log.Lvl2(s.Context.ServerIdentity(), ccr.PopStatus, ccr.Attendees)
	err := s.SendRaw(req.ServerIdentity, ccr)
	if err != nil {
		log.Error("Couldn't send reply:", err)
	}
}

// CheckConfigReply strips the attendees missing in the reply, if the
// PopStatus == PopStatusOK.
func (s *Service) CheckConfigReply(req *network.Envelope) {
	ccrVal, ok := req.Msg.(*CheckConfigReply)

	if syncData, found := s.syncs[string(ccrVal.PopHash)]; found {
		ccr := func() *CheckConfigReply {
			if !ok {
				log.Errorf("Didn't get a CheckConfigReply: %v", req.Msg)
				return nil
			}
			var final *FinalStatement
			if final, ok = s.data.Finals[string(ccrVal.PopHash)]; !ok {
				log.Error("No party with given hash")
				return nil
			}
			if ccrVal.PopStatus < PopStatusOK {
				log.Error("Wrong pop-status:", ccrVal.PopStatus)
				return nil
			}
			final.Attendees = intersectAttendees(final.Attendees, ccrVal.Attendees)
			return ccrVal
		}()
		if len(syncData.ccChannel) == 0 {
			syncData.ccChannel <- ccr
		}
	} else {
		log.Error("No hash for sync found")
	}
}

// VerifyMergeStatement checks that received mergeFinal is valid and can be merged with final
func (final *FinalStatement) VerifyMergeStatement(mergeFinal *FinalStatement) int {
	if len(mergeFinal.Signature) <= 0 {
		log.Error("Received party is not finished")
		return PopStatusMergeNonFinalized
	}
	if mergeFinal.Verify() != nil {
		log.Error("Received config party signature is invalid")
		return PopStatusMergeError
	}

	if final.Desc.DateTime != mergeFinal.Desc.DateTime {
		log.Error("Parties were held in different times")
		return PopStatusMergeError
	}

	// Check if the party is the merge list
	found := false
	party := PopDesc{}
	party.Name = final.Desc.Name
	party.DateTime = final.Desc.DateTime
	party.Parties = final.Desc.Parties
	var hash []byte
	hash2 := final.Desc.Hash()
	for _, sf := range final.Desc.Parties {
		party.Location = sf.Location
		party.Roster = sf.Roster
		hash = party.Hash()
		if bytes.Equal(hash, hash2) {
			found = true
			break
		}
	}
	if !found {
		log.Error("Party is not included in merge list")
		return PopStatusMergeError
	}

	return PopStatusOK
}

// Verification function for signing during Finalization
func (s *Service) bftVerifyFinal(Msg, Data []byte) bool {
	final, err := NewFinalStatementFromToml(Data)
	if err != nil {
		log.Error(err.Error())
		return false
	}
	hash, err := final.Hash()
	if err != nil {
		log.Error(err.Error())
		return false
	}
	if !bytes.Equal(hash, Msg) {
		log.Error("hash of received Final stmt and msg are not equal")
		return false
	}
	var fs *FinalStatement
	var ok bool

	if fs, ok = s.data.Finals[string(final.Desc.Hash())]; !ok {
		log.Error(s.ServerIdentity(), "final Statement not found")
		return false
	}

	hash, err = fs.Hash()

	if !bytes.Equal(hash, Msg) {
		log.Error("hash of lccocal Final stmt and msg are not equal")
		return false
	}
	s.verifyFinalBuffer.Store(sliceToArr(Msg), true)
	return true
}

func (s *Service) bftVerifyFinalAck(msg, data []byte) bool {
	arr := sliceToArr(msg)
	_, ok := s.verifyFinalBuffer.Load(arr)
	if ok {
		s.verifyFinalBuffer.Delete(arr)
	} else {
		log.Error(s.ServerIdentity().Address, "ack failed for msg", msg)
	}
	return ok
}

// Verification function for sighning during Merging
func (s *Service) bftVerifyMerge(Msg []byte, Data []byte) bool {
	stmtsMap, err := decodeMapFinal(Data)
	if err != nil {
		log.Lvl2("VerifyMerge: can't decode Data: " + err.Error())
		return false
	}

	// We need to find all local parties are supposed to merge
	// verify that everything is correct
	var final, finalReceived *FinalStatement
	// local parties which will me merged during current process
	finals := make([]*FinalStatement, 0)
	var ok, found bool
	for _, finalReceived = range stmtsMap {
		if f, ok := s.data.Finals[string(finalReceived.Desc.Hash())]; ok {

			found = true
			final = f

			// Check that info from local party is included in mergeMeta
			hashLocal, err := final.Hash()
			if err != nil {
				log.Error("VerifyMerge: hash computation failed")
				return false
			}
			hashReceived, err := finalReceived.Hash()
			if err != nil {
				log.Error("VerifyMerge: hash computation failed")
				return false
			}
			if !bytes.Equal(hashLocal, hashReceived) {
				log.Lvl2("VerifyMerge: hashes Received and Local are not equal", s.ServerIdentity())
				return false
			}

			// check that merge config is completed in merge
			if len(stmtsMap) != len(final.Desc.Parties) {
				log.Lvl2("VerifyMerge: length of Merge and Merge Config are not equal", s.ServerIdentity())
				return false
			}
			for _, mergeStmt := range stmtsMap {
				status := final.VerifyMergeStatement(mergeStmt)
				if status < PopStatusOK {
					log.Lvl2("VerifyMerge: Received non valid FinalStatement", s.ServerIdentity())
					return false
				}
			}
			finals = append(finals, final)
		}
	}

	if !found {
		log.Lvl2("VerifyMerge: no party from merge was found locally")
		return false
	}

	m := &merge{stmtsMap, true}
	var syncData *syncChans
	if syncData, ok = s.syncs[string(final.Desc.Hash())]; !ok {
		log.Lvl2("VerifyMerge: No sync data with given hash")
		return false
	}

	// Merge fields
	locs := make([]string, 0)
	Roster := &onet.Roster{}
	na := make([]kyber.Point, 0)
	for _, f := range m.statementsMap {
		// although there must not be any intersection
		// in attendies list it's better to check it
		// not simply extend the list
		na = unionAttendies(na, f.Attendees)
		Roster = unionRoster(Roster, f.Desc.Roster)
		locs = append(locs, f.Desc.Location)
	}
	sortAll(locs, Roster.List, na)
	final.Desc.Location = strings.Join(locs, DELIMETER)
	final.Merged = true
	final.Desc.Roster = Roster
	final.Attendees = na

	// check that Msg is valid
	hashLocal, err := final.Hash()
	if err != nil {
		log.Error("VerifyMerge: hash computation failed")
		return false
	}

	if !bytes.Equal(hashLocal, Msg) {
		log.Lvl2("Msg is invalid", s.ServerIdentity())
		return false
	}

	// update local data
	newHash := string(final.Desc.Hash())
	s.data.Finals[newHash] = final
	s.data.merges[newHash] = m
	s.syncs[newHash] = syncData
	m.statementsMap = make(map[string]*FinalStatement)
	m.statementsMap[newHash] = final

	// change final statement for all parties which were going to merge
	// to backward compatibility with orgs, who can't get hash of new final statement
	for _, f := range finals {
		// but signature on this conodes will be invalid
		// because it's impossible to save signature on old hashes
		s.data.Finals[string(f.Desc.Hash())] = final
		// there is no need to support consistency of syncData and merge
		// for old parties because their finalStatements are rewritten
	}

	s.save()
	s.verifyMergeBuffer.Store(sliceToArr(Msg), true)
	return true
}

func (s *Service) bftVerifyMergeAck(msg, data []byte) bool {
	arr := sliceToArr(msg)
	_, ok := s.verifyMergeBuffer.Load(arr)
	if ok {
		s.verifyMergeBuffer.Delete(arr)
	} else {
		log.Error(s.ServerIdentity().Address, "ack failed for msg", msg)
	}
	return ok
}

// PropagateFinal saves the new final statement
func (s *Service) PropagateFinal(msg network.Message) {
	fs, ok := msg.(*FinalStatement)
	if !ok {
		log.Error("Couldn't convert to a FinalStatement")
		return
	}
	if err := fs.Verify(); err != nil {
		log.Error(err)
		return
	}
	*s.data.Finals[string(fs.Desc.Hash())] = *fs
	s.save()
	log.Lvlf2("%s Stored final statement %v", s.ServerIdentity(), fs)
}

// PropagateDescription is called to store new descriptions on the nodes that
// are supposed to participate.
func (s *Service) PropagateDescription(msg network.Message) {
	pd, ok := msg.(*PopDesc)
	if !ok {
		log.Error("Couldn't convert to a PopDesc")
		return
	}
	if pd.Roster.List[0].Equal(s.ServerIdentity()) {
		log.Lvl2("Not storing proposition on leader")
		return
	}
	s.proposedDescription = append(s.proposedDescription, *pd)
	log.Lvl2("Stored proposed description on", s.ServerIdentity())
}

// signs FinalStatement with BFTCosi and Propagates signature to other nodes
func (s *Service) signAndPropagate(final *FinalStatement, protoName string,
	data []byte) error {
	rooted := final.Desc.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(len(final.Desc.Roster.List))
	if tree == nil {
		return errors.New(
			"Root does not exist")
	}

	node, err := s.CreateProtocol(protoName, tree)
	if err != nil {
		return err
	}

	// Register the function generating the protocol instance
	root, ok := node.(*byzcoinx.ByzCoinX)
	if !ok {
		return errors.New(
			"protocol instance is invalid")

	}
	root.Msg, err = final.Hash()
	if err != nil {
		return err
	}

	root.Data = data
	root.Timeout = 5 * time.Second
	root.CreateProtocol = s.CreateProtocol

	final.Signature = []byte{}

	err = node.Start()
	if err != nil {
		return err
	}

	select {
	case sig := <-root.FinalSignatureChan:
		if len(sig.Sig) >= SIGSIZE {
			final.Signature = sig.Sig[:SIGSIZE]
		} else {
			final.Signature = []byte{}
		}
	case <-time.After(timeout):
		log.Error("signing failed on timeout")
		return errors.New(
			"signing timeout")

	}
	if len(final.Signature) <= 0 {
		log.Error("Signing failed")
		return errors.New(
			"Signing failed")

	}

	replies, err := s.propagateFinalize(final.Desc.Roster, final, 10*time.Second)
	if err != nil {
		return err
	}
	if replies != len(final.Desc.Roster.List) {
		log.Warn("Did only get", replies)
	}
	s.save()
	return nil
}

// merge sends MergeConfig to all parties,
// Receives Replies, updates info about global merge party
// When all merge party's info is saved, merge it and starts global sighning process
// After all, sends StoreConfig request to other conodes of own party
func (s *Service) merge(final *FinalStatement, m *merge) (*FinalStatement,
	error) {
	if m.distrib {
		// Used not to start merge process 2 times, when one is on run.
		log.Lvl2(s.ServerIdentity(), "Not enter merge")
		return nil, errors.New("Merge Process in in progress")
	}
	log.Lvl2("Merge ", s.ServerIdentity())
	m.distrib = true
	// Flag indicating that there were connection with other nodes
	syncData, ok := s.syncs[string(final.Desc.Hash())]
	if !ok {
		return nil, errors.New("Wrong Hash")
	}
	for _, party := range final.Desc.Parties {
		popDesc := PopDesc{
			Name:     final.Desc.Name,
			DateTime: final.Desc.DateTime,
			Location: party.Location,
			Roster:   party.Roster,
			Parties:  final.Desc.Parties,
		}
		hash := popDesc.Hash()
		if _, ok := m.statementsMap[string(hash)]; ok {
			// that's unlikely due to running in cycle
			continue
		}
		mc := &MergeConfig{Final: final, ID: hash}
		for _, si := range party.Roster.List {
			log.Lvlf2("Sending from %s to %s", s.ServerIdentity(), si)
			err := s.SendRaw(si, mc)
			if err != nil {
				return nil, err
			}
			var mcr *MergeConfigReply
			select {
			case mcr = <-syncData.mcChannel:
				break
			case <-time.After(timeout):
				return nil, errors.New(
					"timeout on waiting response MergeConfig")

			}
			if mcr == nil {
				return nil, errors.New(
					"Error during merging")

			}
			if mcr.PopStatus == PopStatusOK {
				m.statementsMap[string(hash)] = mcr.Final
				break
			}
		}
		if _, ok = m.statementsMap[string(hash)]; !ok {
			return nil, errors.New(
				"merge with party failed")

		}
	}

	newFinal := &FinalStatement{}
	*newFinal = *final
	newFinal.Desc = &PopDesc{}
	*newFinal.Desc = *final.Desc

	// Unite the lists
	locs := make([]string, 0)
	Roster := &onet.Roster{}
	na := make([]kyber.Point, 0)
	for _, f := range m.statementsMap {
		// although there must not be any intersection
		// in attendies list it's better to check it
		// not simply extend the list
		na = unionAttendies(na, f.Attendees)
		Roster = unionRoster(Roster, f.Desc.Roster)
		locs = append(locs, f.Desc.Location)
	}
	sortAll(locs, Roster.List, na)
	newFinal.Desc.Location = strings.Join(locs, DELIMETER)
	newFinal.Desc.Roster = Roster
	newFinal.Attendees = na
	newFinal.Merged = true
	return newFinal, nil
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl2("Saving service", s.ServerIdentity())
	err := s.Save(storageKey, s.data)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	msg, err := s.Load(storageKey)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.data, ok = msg.(*saveData)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// Get intersection of attendees
func intersectAttendees(atts1, atts2 []kyber.Point) []kyber.Point {
	myMap := make(map[string]bool)

	for _, p := range atts1 {
		myMap[p.String()] = true
	}
	min := len(atts1)
	if min < len(atts1) {
		min = len(atts1)
	}
	na := make([]kyber.Point, 0, min)
	for _, p := range atts2 {
		if _, ok := myMap[p.String()]; ok {
			na = append(na, p)
		}
	}
	return na
}

func unionAttendies(atts1, atts2 []kyber.Point) []kyber.Point {
	myMap := make(map[string]bool)
	na := make([]kyber.Point, 0, len(atts1)+len(atts2))

	na = append(na, atts1...)
	for _, p := range atts1 {
		myMap[p.String()] = true
	}

	for _, p := range atts2 {
		if _, ok := myMap[p.String()]; !ok {
			na = append(na, p)
		}
	}
	return na
}

func unionRoster(r1, r2 *onet.Roster) *onet.Roster {
	myMap := make(map[string]bool)
	na := make([]*network.ServerIdentity, 0, len(r1.List)+len(r2.List))

	na = append(na, r1.List...)
	for _, s := range r1.List {
		myMap[s.String()] = true
	}
	for _, s := range r2.List {
		if _, ok := myMap[s.String()]; !ok {
			na = append(na, s)
		}
	}
	return onet.NewRoster(na)
}

// newService registers the request-methods.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		data:             &saveData{},
	}
	err := s.RegisterHandlers(s.PinRequest, s.StoreConfig, s.FinalizeRequest,
		s.FetchFinal, s.MergeRequest, s.GetProposals)
	if err != nil {
		return nil, err
	}
	if err := s.tryLoad(); err != nil {
		return nil, err
	}
	if s.data.Finals == nil {
		s.data.Finals = make(map[string]*FinalStatement)
	}
	if s.data.merges == nil {
		s.data.merges = make(map[string]*merge)
	}
	s.syncs = make(map[string]*syncChans)
	s.propagateFinalize, err = messaging.NewPropagationFunc(c, propagFinal, s.PropagateFinal, 0)
	if err != nil {
		return nil, err
	}
	s.propagateDescription, err = messaging.NewPropagationFunc(c, propagDescription, s.PropagateDescription, 0)
	if err != nil {
		return nil, err
	}

	s.RegisterProcessorFunc(checkConfigID, s.CheckConfig)
	s.RegisterProcessorFunc(checkConfigReplyID, s.CheckConfigReply)
	s.RegisterProcessorFunc(mergeConfigID, s.MergeConfig)
	s.RegisterProcessorFunc(mergeConfigReplyID, s.MergeConfigReply)
	if err := byzcoinx.InitBFTCoSiProtocol(protocol.EdDSACompatibleCosiSuite, s.Context,
		s.bftVerifyFinal, s.bftVerifyFinalAck, bftSignFinal); err != nil {
		return nil, err
	}
	if err := byzcoinx.InitBFTCoSiProtocol(protocol.EdDSACompatibleCosiSuite, s.Context,
		s.bftVerifyMerge, s.bftVerifyMergeAck, bftSignMerge); err != nil {
		return nil, err
	}
	return s, nil
}
