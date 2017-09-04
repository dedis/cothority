package service

/*
Service for a Proof-of-Personhood party

Proof-of-personhood parties provide a number of "attendees" with an "anonymous
token" that enables them to "authenticate" to a service as being part of the
party.

These parties are held by a number of "organisers" who set up a party by
defining place, time and purpose of that party and by publishing a
"party configuration" that is signed by the organisers "conodes".
At the party, they "register" all attendees' public keys.
Once the party is over, they create a "party transcript" that is signed by all
organisers' conodes.

The attendees create their "pop token" by joining their private key to the
party transcript. They can now use that token to sign a "message" in a "context"
from a service and send the resulting "signature" and "tag" back to the service.

On the service's side, it can use the party transcript to verify that the
signature has been created using a private key present in the party transcript.
The tag will be unique to that attendee/context pair, but another service using
another context will not be able to link two tags to the same or different
attendee.
*/

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"gopkg.in/dedis/cothority.v1/bftcosi"
	"gopkg.in/dedis/cothority.v1/messaging"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"

	_ "encoding/base64"
)

// Name is the name to refer to the Template service from another
// package.
const Name = "PoPServer"
const cfgName = "pop.bin"
const bftSignFinal = "BFTFinal"
const bftSignMerge = "PopBFTSignMerge"

const propagFinal = "PoPPropagateFinal"

const TIMEOUT = 60 * time.Second
const SIGSIZE = 64

//const TIMEOUT = 60 * time.Second
const DELIMETER = "; "

var checkConfigID network.MessageTypeID
var checkConfigReplyID network.MessageTypeID
var mergeConfigID network.MessageTypeID
var mergeConfigReplyID network.MessageTypeID
var mergeCheckID network.MessageTypeID
var mergeCheckReplyID network.MessageTypeID

func init() {
	onet.RegisterNewService(Name, newService)
	network.RegisterMessage(&saveData{})
	checkConfigID = network.RegisterMessage(checkConfig{})
	checkConfigReplyID = network.RegisterMessage(checkConfigReply{})
	mergeConfigID = network.RegisterMessage(mergeConfig{})
	mergeConfigReplyID = network.RegisterMessage(mergeConfigReply{})
}

// Service represents data needed for one pop-party.
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	path string
	data *saveData
	// propagate final message
	PropagateFinalize messaging.PropagationFunc
	// propagate merge info
	PropagateMerging messaging.PropagationFunc
	// Sync tools
	// key of map is ID of party
	// synchronizing inside one party
	syncs map[string]*sync
}

type saveData struct {
	// Pin holds the randomly chosen pin
	Pin string
	// Public key of linked pop
	Public abstract.Point
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

func newMerge() *merge {
	mm := &merge{}
	mm.statementsMap = make(map[string]*FinalStatement)
	mm.distrib = false
	return mm
}

type sync struct {
	// channel to return the configreply
	ccChannel chan *checkConfigReply
	// channel to return the mergereply
	mcChannel chan *mergeConfigReply
}

/* ----------------Request Handlers---------------- */

// PinRequest prints out a pin if none is given, else it verifies it has the
// correct pin, and if so, it stores the public key as reference.
func (s *Service) PinRequest(req *PinRequest) (network.Message, onet.ClientError) {
	if req.Pin == "" {
		s.data.Pin = fmt.Sprintf("%06d", random.Int(big.NewInt(1000000), random.Stream))
		log.Info("PIN:", s.data.Pin)
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Read PIN in server-log")
	}
	if req.Pin != s.data.Pin {
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Wrong PIN")
	}
	s.data.Public = req.Public
	s.save()
	log.Lvl1("Successfully registered PIN/Public", s.data.Pin, req.Public)
	return nil, nil
}

// StoreConfig saves the pop-config locally
func (s *Service) StoreConfig(req *storeConfig) (network.Message, onet.ClientError) {
	log.Lvlf2("StoreConfig: %s %v %x", s.Context.ServerIdentity(), req.Desc, req.Desc.Hash())
	if req.Desc.Roster == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "no roster set")
	}
	if s.data.Public == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Not linked yet")
	}
	hash := req.Desc.Hash()
	if err := crypto.VerifySchnorr(network.Suite, s.data.Public, hash, req.Signature); err != nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Invalid signature"+err.Error())
	}
	s.data.Finals[string(hash)] = &FinalStatement{Desc: req.Desc, Signature: []byte{}}
	s.syncs[string(hash)] = &sync{
		ccChannel: make(chan *checkConfigReply, 1),
		mcChannel: make(chan *mergeConfigReply, 1),
	}
	if len(req.Desc.Parties) > 0 {
		meta := newMerge()
		s.data.merges[string(hash)] = meta
		// party is merged with itself already
		meta.statementsMap[string(hash)] = s.data.Finals[string(hash)]
	}
	s.save()
	return &storeConfigReply{hash}, nil
}

// FinalizeRequest returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
func (s *Service) FinalizeRequest(req *finalizeRequest) (network.Message, onet.ClientError) {
	log.Lvlf2("Finalize: %s %+v", s.Context.ServerIdentity(), req)
	if s.data.Public == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Not linked yet")
	}
	hash, err := req.hash()
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	if err := crypto.VerifySchnorr(network.Suite, s.data.Public, hash, req.Signature); err != nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Invalid signature:"+err.Error())
	}

	var final *FinalStatement
	var ok bool
	if final, ok = s.data.Finals[string(req.DescID)]; !ok || final == nil || final.Desc == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "No config found")
	}
	if final.Verify() == nil {
		log.Lvl2("Sending known final statement")
		return &finalizeResponse{final}, nil
	}

	// Contact all other nodes and ask them if they already have a config.
	final.Attendees = make([]abstract.Point, len(req.Attendees))
	copy(final.Attendees, req.Attendees)
	cc := &checkConfig{final.Desc.Hash(), req.Attendees}
	for _, c := range final.Desc.Roster.List {
		if !c.ID.Equal(s.ServerIdentity().ID) {
			log.Lvl2("Contacting", c, cc.Attendees)
			err := s.SendRaw(c, cc)
			if err != nil {
				return nil, onet.NewClientErrorCode(ErrorInternal, err.Error())
			}
			if syncData, ok := s.syncs[string(req.DescID)]; ok {
				rep := <-syncData.ccChannel
				if rep == nil {
					return nil, onet.NewClientErrorCode(ErrorOtherFinals,
						"Not all other conodes finalized yet")
				}
			}
		}
	}
	data, err := final.ToToml()
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	// Create signature and propagate it
	cerr := s.signAndPropagate(final, bftSignFinal, data)
	if cerr != nil {
		return nil, cerr
	}
	return &finalizeResponse{final}, nil
}

// FetchFinal returns FinalStatement by hash
// used after Finalization
func (s *Service) FetchFinal(req *fetchRequest) (network.Message,
	onet.ClientError) {
	log.Lvlf2("FetchFinal: %s %v", s.Context.ServerIdentity(), req.ID)
	var fs *FinalStatement
	var ok bool
	if fs, ok = s.data.Finals[string(req.ID)]; !ok {
		return nil, onet.NewClientErrorCode(ErrorInternal,
			"No config found")
	}
	if len(fs.Signature) <= 0 {
		return nil, onet.NewClientErrorCode(ErrorOtherFinals,
			"Not all other conodes finalized yet")
	}
	return &finalizeResponse{fs}, nil
}

// MergeRequest starts Merge process and returns FinalStatement after
// used after finalization
func (s *Service) MergeRequest(req *mergeRequest) (network.Message,
	onet.ClientError) {
	log.Lvlf2("MergeRequest: %s %v", s.Context.ServerIdentity(), req.ID)
	if s.data.Public == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Not linked yet")
	}

	if err := crypto.VerifySchnorr(network.Suite, s.data.Public, req.ID, req.Signature); err != nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Invalid signature: err")
	}

	final, ok := s.data.Finals[string(req.ID)]
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorInternal,
			"No config found")
	}
	if final.Merged {
		return &finalizeResponse{final}, nil
	}
	m, ok := s.data.merges[string(req.ID)]
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorInternal,
			"No meta found")
	}
	syncData, ok := s.syncs[string(req.ID)]
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorInternal,
			"No meta found")
	}

	if len(final.Signature) <= 0 || final.Verify() != nil {
		return nil, onet.NewClientErrorCode(ErrorOtherFinals,
			"Not all other conodes finalized yet")
	}
	if len(final.Desc.Parties) <= 1 {
		return nil, onet.NewClientErrorCode(ErrorInternal,
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
		return nil, onet.NewClientErrorCode(ErrorInternal,
			"Party is not included in merge list")
	}
	newFinal, cerr := s.Merge(final, m)
	if cerr != nil {
		if cerr.ErrorCode() == ErrorMergeInProgress {
			return final, nil
		}
		return nil, cerr
	}

	// Decode mapStatements to send it on signing
	data, err := encodeMapFinal(m.statementsMap)
	if err != nil {
		return nil, onet.NewClientError(err)
	}

	cerr = s.signAndPropagate(newFinal, bftSignMerge, data)
	if cerr != nil {
		m.distrib = false
		return nil, cerr
	}
	// refresh data
	hash := string(newFinal.Desc.Hash())
	s.data.Finals[hash] = newFinal
	s.data.merges[hash] = m
	s.syncs[hash] = syncData
	m.statementsMap = make(map[string]*FinalStatement)
	m.statementsMap[hash] = newFinal

	s.save()
	return &finalizeResponse{newFinal}, nil
}

/* ------------InterConode Messages ----------- */

// MergeConfig receives a final statement of requesting party,
// hash of local party. Checks if they are from one merge party and responses with
// own finalStatement
func (s *Service) MergeConfig(req *network.Envelope) {
	log.Lvlf2("%s gets MergeConfig from %s", s.Context.ServerIdentity().String(),
		req.ServerIdentity.String())
	mc, ok := req.Msg.(*mergeConfig)
	if !ok {
		log.Errorf("Didn't get a MergeConfig: %#v", req.Msg)
		return
	}
	if mc.Final == nil || mc.Final.Desc == nil {
		log.Error("MergeConfig is empty")
		return
	}
	mcr := &mergeConfigReply{PopStatusOK, mc.Final.Desc.Hash(), nil}

	var final *FinalStatement
	var m *merge
	if final, ok = s.data.Finals[string(mc.ID)]; !ok {
		log.Errorf("No config found")
		mcr.PopStatus = PopStatusWrongHash
		goto send
	}
	if m, ok = s.data.merges[string(mc.ID)]; !ok {
		log.Error("No merge set found")
		mcr.PopStatus = PopStatusWrongHash
		goto send
	}
	if final.Verify() != nil {
		log.Error("Local party's signature is invalid")
		mcr.PopStatus = PopStatusMergeNonFinalized
		goto send
	}
	mcr.PopStatus = final.VerifyMergeStatement(mc.Final)
	if mcr.PopStatus < PopStatusOK {
		goto send
	}
	if _, ok = m.statementsMap[string(mc.Final.Desc.Hash())]; ok {
		log.Lvl2(s.ServerIdentity(), "Party was already merged, sent from",
			req.ServerIdentity.String())
		mcr.PopStatus = PopStatusMergeError
		goto send
	} else {
		m.statementsMap[string(mc.Final.Desc.Hash())] = mc.Final
	}

	mcr.Final = final

send:
	err := s.SendRaw(req.ServerIdentity, mcr)
	if err != nil {
		log.Error("Couldn't send reply:", err)
	}
}

// MergeConfigReply processes the response after MergeConfig message
func (s Service) MergeConfigReply(req *network.Envelope) {
	log.Lvlf2("MergeConfigReply: %s from %s got %v",
		s.ServerIdentity(), req.ServerIdentity.String(), req.Msg)
	mcrVal, ok := req.Msg.(*mergeConfigReply)
	var mcr *mergeConfigReply
	mcr = func() *mergeConfigReply {
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
			log.Error("Wrong pop-status:", mcrVal.PopStatus)
			return mcrVal
		}
		if mcrVal.Final == nil {
			log.Error("Empty FinalStatement in reply")
			return nil
		}
		mcrVal.PopStatus = final.VerifyMergeStatement(mcrVal.Final)
		return mcrVal
	}()
	if syncData, ok := s.syncs[string(mcrVal.PopHash)]; ok {
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
	cc, ok := req.Msg.(*checkConfig)
	if !ok {
		log.Errorf("Didn't get a CheckConfig: %#v", req.Msg)
		return
	}

	ccr := &checkConfigReply{PopStatusOK, cc.PopHash, nil}
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
	ccrVal, ok := req.Msg.(*checkConfigReply)
	var ccr *checkConfigReply
	ccr = func() *checkConfigReply {
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
	if syncData, ok := s.syncs[string(ccrVal.PopHash)]; ok {
		if len(syncData.ccChannel) == 0 {
			syncData.ccChannel <- ccr
		}
	} else {
		log.Error("No hash for sync found")
	}
}

/* -------------Verification functions------------- */

// Verification function for signing during Finalization
func (s *Service) bftVerifyFinal(Msg []byte, Data []byte) bool {
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
		log.Error("final Statement not found")
		return false
	}

	hash, err = fs.Hash()

	if !bytes.Equal(hash, Msg) {
		log.Error("hash of lccocal Final stmt and msg are not equal")
		return false
	}
	return true
}

// Verification function for sighning during Merging
func (s *Service) bftVerifyMerge(Msg []byte, Data []byte) bool {
	stmtsMap, err := decodeMapFinal(Data)
	if err != nil {
		log.Error("VerifyMerge: can't decode Data: " + err.Error())
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
				log.Error("VerifyMerge: hashes Received and Local are not equal", s.ServerIdentity())
				return false
			}

			// check that merge config is completed in merge
			if len(stmtsMap) != len(final.Desc.Parties) {
				log.Error("VerifyMerge: length of Merge and Merge Config are not equal", s.ServerIdentity())
				return false
			}
			for _, mergeStmt := range stmtsMap {
				status := final.VerifyMergeStatement(mergeStmt)
				if status < PopStatusOK {
					log.Error("VerifyMerge: Received non valid FinalStatement", s.ServerIdentity())
					return false
				}
			}
			finals = append(finals, final)
		}
	}

	if !found {
		log.Error("VerifyMerge: no party from merge was found locally")
		return false
	}

	m := &merge{stmtsMap, true}
	var syncData *sync
	if syncData, ok = s.syncs[string(final.Desc.Hash())]; !ok {
		log.Error("VerifyMerge: No sync data with given hash")
		return false
	}

	// Merge fields
	locs := make([]string, 0)
	Roster := &onet.Roster{}
	na := make([]abstract.Point, 0)
	for _, f := range m.statementsMap {
		// although there must not be any intersection
		// in attendies list it's better to check it
		// not simply extend the list
		na = unionAttendies(na, f.Attendees)
		Roster = unionRoster(Roster, f.Desc.Roster)
		locs = append(locs, f.Desc.Location)
	}
	sort.Slice(locs, func(i, j int) bool {
		return strings.Compare(locs[i], locs[j]) < 0
	})
	final.Desc.Location = strings.Join(locs, DELIMETER)
	final.Merged = true
	sort.Slice(Roster.List, func(i, j int) bool {
		return strings.Compare(Roster.List[i].String(), Roster.List[j].String()) < 0
	})
	final.Desc.Roster = Roster
	sort.Slice(na, func(i, j int) bool {
		return strings.Compare(na[i].String(), na[j].String()) < 0
	})
	final.Attendees = na

	// check that Msg is valid
	hashLocal, err := final.Hash()
	if err != nil {
		log.Error("VerifyMerge: hash computation failed")
		return false
	}

	if !bytes.Equal(hashLocal, Msg) {
		log.Error("Msg is invalid", s.ServerIdentity())
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
	return true
}

/* --------------Propagation function-------------- */

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

/* ----------------Utilite functions--------------- */

//signs FinalStatement with BFTCosi and Propagates signature to other nodes
func (s *Service) signAndPropagate(final *FinalStatement, protoName string,
	data []byte) onet.ClientError {
	tree := final.Desc.Roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())
	if tree == nil {
		return onet.NewClientErrorCode(ErrorInternal,
			"Root does not exist")
	}
	node, err := s.CreateProtocol(protoName, tree)
	if err != nil {
		return onet.NewClientError(err)
	}

	// Register the function generating the protocol instance
	root, ok := node.(*bftcosi.ProtocolBFTCoSi)
	if !ok {
		return onet.NewClientErrorCode(ErrorInternal,
			"protocol instance is invalid")
	}

	root.Msg, err = final.Hash()
	if err != nil {
		return onet.NewClientError(err)
	}

	root.Data = data
	done := make(chan bool)
	root.RegisterOnDone(func() {
		done <- true
	})
	final.Signature = []byte{}
	go node.Start()

	select {
	case <-done:
		sig := root.Signature()
		if len(sig.Sig) >= SIGSIZE {
			final.Signature = sig.Sig[:SIGSIZE]
		} else {
			final.Signature = []byte{}
		}
	case <-time.After(TIMEOUT):
		log.Error("signing failed on timeout")
		return onet.NewClientErrorCode(ErrorTimeout,
			"signing timeout")
	}
	if len(final.Signature) <= 0 {
		log.Error("Signing failed")
		return onet.NewClientErrorCode(ErrorTimeout,
			"Signing failed")
	}

	replies, err := s.PropagateFinalize(final.Desc.Roster, final, 10000)
	if err != nil {
		return onet.NewClientError(err)
	}
	if replies != len(final.Desc.Roster.List) {
		log.Warn("Did only get", replies)
	}
	s.save()
	return nil
}

// Merge sends MergeConfig to all parties,
// Receives Replies, updates info about global merge party
// When all merge party's info is saved, merge it and starts global sighning process
// After all, sends StoreConfig request to other conodes of own party
func (s *Service) Merge(final *FinalStatement, m *merge) (*FinalStatement,
	onet.ClientError) {
	if m.distrib {
		// Used not to start merge process 2 times, when one is on run.
		log.Lvl2(s.ServerIdentity(), "Not enter merge")
		return nil, onet.NewClientErrorCode(ErrorMergeInProgress, "Merge Process in in progress")
	}
	log.Lvl2("Merge ", s.ServerIdentity())
	m.distrib = true
	// Flag indicating that there were connection with other nodes
	syncData, ok := s.syncs[string(final.Desc.Hash())]
	if !ok {
		return nil, onet.NewClientErrorCode(ErrorMerge, "Wrong Hash")
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
		mc := &mergeConfig{Final: final, ID: hash}
		for _, si := range party.Roster.List {
			log.Lvlf2("Sending from %s to %s", s.ServerIdentity(), si)
			err := s.SendRaw(si, mc)
			if err != nil {
				return nil, onet.NewClientErrorCode(ErrorInternal, err.Error())
			}
			var mcr *mergeConfigReply
			select {
			case mcr = <-syncData.mcChannel:
				break
			case <-time.After(TIMEOUT):
				return nil, onet.NewClientErrorCode(ErrorTimeout,
					"timeout on waiting response MergeConfig")
			}
			if mcr == nil {
				return nil, onet.NewClientErrorCode(ErrorMerge,
					"Error during merging")
			}
			if mcr.PopStatus == PopStatusOK {
				m.statementsMap[string(hash)] = mcr.Final
				break
			}
		}
		if _, ok = m.statementsMap[string(hash)]; !ok {
			return nil, onet.NewClientErrorCode(ErrorMerge,
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
	na := make([]abstract.Point, 0)
	for _, f := range m.statementsMap {
		// although there must not be any intersection
		// in attendies list it's better to check it
		// not simply extend the list
		na = unionAttendies(na, f.Attendees)
		Roster = unionRoster(Roster, f.Desc.Roster)
		locs = append(locs, f.Desc.Location)
	}
	sort.Slice(locs, func(i, j int) bool {
		return strings.Compare(locs[i], locs[j]) < 0
	})
	newFinal.Desc.Location = strings.Join(locs, DELIMETER)
	newFinal.Merged = true
	sort.Slice(Roster.List, func(i, j int) bool {
		return strings.Compare(Roster.List[i].String(), Roster.List[j].String()) < 0
	})
	newFinal.Desc.Roster = Roster
	sort.Slice(na, func(i, j int) bool {
		return strings.Compare(na[i].String(), na[j].String()) < 0
	})
	newFinal.Attendees = na
	newFinal.Merged = true
	return newFinal, nil
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

// Get intersection of attendees
func intersectAttendees(atts1, atts2 []abstract.Point) []abstract.Point {
	myMap := make(map[string]bool)

	for _, p := range atts1 {
		myMap[p.String()] = true
	}
	min := len(atts1)
	if min < len(atts1) {
		min = len(atts1)
	}
	na := make([]abstract.Point, 0, min)
	for _, p := range atts2 {
		if _, ok := myMap[p.String()]; ok {
			na = append(na, p)
		}
	}
	return na
}

func unionAttendies(atts1, atts2 []abstract.Point) []abstract.Point {
	myMap := make(map[string]bool)
	na := make([]abstract.Point, 0, len(atts1)+len(atts2))

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

// saves the actual identity
func (s *Service) save() {
	log.Lvl2("Saving service", s.ServerIdentity())
	err := s.Save("storage", s.data)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	if !s.DataAvailable("storage") {
		return nil
	}
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	var ok bool
	s.data, ok = msg.(*saveData)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// newService registers the request-methods.
func newService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		data:             &saveData{},
	}
	log.ErrFatal(s.RegisterHandlers(s.PinRequest, s.StoreConfig, s.FinalizeRequest,
		s.FetchFinal, s.MergeRequest), "Couldn't register messages")
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	if s.data.Finals == nil {
		s.data.Finals = make(map[string]*FinalStatement)
	}
	if s.data.merges == nil {
		s.data.merges = make(map[string]*merge)
	}
	s.syncs = make(map[string]*sync)
	var err error
	s.PropagateFinalize, err = messaging.NewPropagationFunc(c, propagFinal, s.PropagateFinal)
	log.ErrFatal(err)
	s.RegisterProcessorFunc(checkConfigID, s.CheckConfig)
	s.RegisterProcessorFunc(checkConfigReplyID, s.CheckConfigReply)
	s.RegisterProcessorFunc(mergeConfigID, s.MergeConfig)
	s.RegisterProcessorFunc(mergeConfigReplyID, s.MergeConfigReply)
	s.ProtocolRegister(bftSignFinal, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyFinal)
	})
	s.ProtocolRegister(bftSignMerge, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, s.bftVerifyMerge)
	})
	return s
}
