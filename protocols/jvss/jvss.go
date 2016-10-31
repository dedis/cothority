// Package jvss provides a threshold signing scheme based on Shamir's joint verifiable
// secret sharing algorithm and Schnorr signatures. The protocl runs in two
// phases. During the protocol setup a long-term shared secret is establised
// between all participants. Afterwards, any of the members can request a
// signature, which triggers the creation of another, short-term shared secret.
// Each member then sends its partial signature to the requester which finally
// puts everything together to get the final Schnorr signature. To verify a
// given Schnorr signature a member still has to be able to access the
// long-term shared secret from which that particular signature was created.
package jvss

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.GlobalProtocolRegister("JVSS", NewJVSS)
}

// SID is the type of shared secret identifiers
type SID string

// Identifiers for long- and short-term shared secrets.
const (
	LTSS SID = "LTSS"
	STSS SID = "STSS"
)

// randomLength is the length of random bytes that will be appended to SID to
// make them unique per signing requests
const randomLength = 32

// JVSS is the main protocol struct and implements the sda.ProtocolInstance
// interface.
type JVSS struct {
	*sda.TreeNodeInstance                  // The SDA TreeNode
	keyPair               *config.KeyPair  // KeyPair of the host
	nodeList              []*sda.TreeNode  // List of TreeNodes in the JVSS group
	pubKeys               []abstract.Point // List of public keys of the above TreeNodes
	info                  poly.Threshold   // JVSS thresholds
	schnorr               *poly.Schnorr    // Long-term Schnorr struct to compute distributed signatures
	secrets               *sharedSecrets   // Shared secrets (long- and short-term ones)
	ltssInit              bool             // Indicator whether shared secret has been already initialised or not
	treeIndex             int              // the index of this node in the flattened tree

	longTermSecDone  chan bool // Channel to indicate when long-term shared secrets of all peers are ready
	shortTermSecDone chan bool // Channel to indicate when short-term shared secrets of all peers are ready

	sigChan chan *poly.SchnorrSig // Channel for JVSS signature

	// keeps the set of SID this node has started/initiated
	sidStore *sidStore
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	n := len(node.List())
	pk := make([]abstract.Point, n)
	var idx int
	for i, tn := range node.List() {
		if tn.ServerIdentity.Public.Equal(node.Public()) {
			idx = i
		}
		pk[i] = tn.ServerIdentity.Public
	}
	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: n, R: n, N: n}

	jv := &JVSS{
		TreeNodeInstance: node,
		keyPair:          kp,
		pubKeys:          pk,
		treeIndex:        idx,
		info:             info,
		schnorr:          new(poly.Schnorr),
		secrets:          newSecrets(),
		ltssInit:         false,
		longTermSecDone:  make(chan bool, 1),
		shortTermSecDone: make(chan bool, 1),
		sigChan:          make(chan *poly.SchnorrSig),
		sidStore:         newSidStore(),
	}

	// Setup message handlers
	h := []interface{}{
		jv.handleSecInit,
		jv.handleSecConf,
		jv.handleSigReq,
		jv.handleSigResp,
	}
	err := jv.RegisterHandlers(h...)
	if err != nil {
		return nil, err
	}
	return jv, err
}

// Start initiates the JVSS protocol by setting up a long-term shared secret
// which can be used later on by the JVSS group to sign and verify messages.
func (jv *JVSS) Start() error {
	log.Lvl2(jv.Name(), "index", jv.Index(), " Starts()")
	sid := newSID(LTSS)
	jv.sidStore.insert(sid)
	err := jv.initSecret(sid)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Lvl2("Waiting on long-term secrets:", jv.Name())
	<-jv.longTermSecDone
	log.Lvl2("Done waiting on long-term secrets:", jv.Name())
	return err
}

// Verify verifies the given message against the given Schnorr signature.
// Returns nil if the signature is valid and an error otherwise.
func (jv *JVSS) Verify(msg []byte, sig *poly.SchnorrSig) error {

	if !jv.ltssInit {
		return fmt.Errorf("Error, long-term shared secret has not been initialised")
	}

	h := jv.keyPair.Suite.Hash()
	_, _ = h.Write(msg) // ignore error; verification wil fail anyways
	return jv.schnorr.VerifySchnorrSig(sig, h)
}

// Sign starts a new signing request amongst the JVSS group and returns a
// Schnorr signature on success.
func (jv *JVSS) Sign(msg []byte) (*poly.SchnorrSig, error) {

	if !jv.ltssInit {
		return nil, fmt.Errorf("Error, long-term shared secret has not been initialised")
	}

	log.Lvl3(jv.Name(), "index", jv.Index(), " => Sign starting")

	// Initialise short-term shared secret only used for this signing request
	sid := newSID(STSS)
	jv.sidStore.insert(sid)
	if err := jv.initSecret(sid); err != nil {
		return nil, err
	}

	// Wait for setup of shared secrets to finish
	log.Lvl2("Waiting on short-term secrets:", jv.Name())
	<-jv.shortTermSecDone
	// Create partial signature ...
	ps, err := jv.sigPartial(sid, msg)
	if err != nil {
		return nil, err
	}

	// ... and buffer it
	secret, err := jv.secrets.secret(sid)
	if err != nil {
		log.Error("Didn't find secret. Still continuing:", err)
	}

	secret.sigs[jv.Index()] = ps

	// Broadcast signing request
	req := &SigReqMsg{
		Src: jv.Index(),
		SID: sid,
		Msg: msg,
	}
	if err := jv.Broadcast(req); err != nil {
		return nil, err
	}

	// Wait for complete signature
	sig := <-jv.sigChan

	return sig, nil
}

func (jv *JVSS) initSecret(sid SID) error {
	if sid.IsLTSS() && jv.ltssInit {
		return errors.New("Only one longterm secret allowed per JVSS instance")
	}

	// Initialise shared secret of given type if necessary
	if sec, err := jv.secrets.secret(sid); sec == nil && err != nil {
		log.Lvlf4("Node %d: Initialising %s shared secret", jv.Index(),
			sid)
		sec := &secret{
			receiver:         poly.NewReceiver(jv.keyPair.Suite, jv.info, jv.keyPair),
			deals:            make(map[int]*poly.Deal),
			sigs:             make(map[int]*poly.SchnorrPartialSig),
			numLongtermConfs: 0,
		}
		jv.secrets.addSecret(sid, sec)
	}

	secret, err := jv.secrets.secret(sid)
	if err != nil { // this should never happen here
		log.Error(err)
		return err
	}

	// Initialise and broadcast our deal if necessary
	if len(secret.deals) == 0 {
		kp := config.NewKeyPair(jv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
		log.Lvlf4("Node %d: Initialising %v deal", jv.Index(), sid)
		secret.deals[jv.Index()] = deal
		db, _ := deal.MarshalBinary()
		msg := &SecInitMsg{
			Src:  jv.Index(),
			SID:  sid,
			Deal: db,
		}
		if err := jv.Broadcast(msg); err != nil {
			log.Print(jv.Name(), "Error broadcast secInit:", err)
			return err
		}
	}
	return nil
}

func (jv *JVSS) finaliseSecret(sid SID) error {
	secret, err := jv.secrets.secret(sid)
	if err != nil {
		return err
	}

	log.Lvlf4("Node %d: %s deals %d/%d", jv.Index(), sid, len(secret.deals),
		len(jv.List()))

	if len(secret.deals) == jv.info.T {
		for _, deal := range secret.deals {
			if _, err := secret.receiver.AddDeal(jv.Index(), deal); err != nil {
				log.Error(jv.Index(), err)
				return err
			}
		}

		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return err
		}
		secret.secret = sec
		isShortTermSecret := strings.HasPrefix(string(sid), string(STSS))
		if isShortTermSecret {
			secret.nShortConfirmsMtx.Lock()
			defer secret.nShortConfirmsMtx.Unlock()
			secret.numShortConfs++
		} else {
			secret.nLongConfirmsMtx.Lock()
			defer secret.nLongConfirmsMtx.Unlock()
			secret.numLongtermConfs++
		}

		log.Lvlf4("Node %d: %v created", jv.Index(), sid)

		// Initialise Schnorr struct for long-term shared secret if not done so before
		if sid.IsLTSS() && !jv.ltssInit {
			jv.ltssInit = true
			jv.schnorr.Init(jv.keyPair.Suite, jv.info, secret.secret)
			log.Lvlf4("Node %d: %v Schnorr struct initialised",
				jv.Index(), sid)
		}

		// Broadcast that we have finished setting up our shared secret
		msg := &SecConfMsg{
			Src: jv.Index(),
			SID: sid,
		}
		if err := jv.Broadcast(msg); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

func (jv *JVSS) sigPartial(sid SID, msg []byte) (*poly.SchnorrPartialSig, error) {
	secret, err := jv.secrets.secret(sid)
	if err != nil {
		return nil, err
	}

	hash := jv.keyPair.Suite.Hash()
	if _, err := hash.Write(msg); err != nil {
		return nil, err
	}
	if err := jv.schnorr.NewRound(secret.secret, hash); err != nil {
		return nil, err
	}
	ps := jv.schnorr.RevealPartialSig()
	if ps == nil {
		return nil, fmt.Errorf("Error, node %d could not create partial signature", jv.Index())
	}
	return ps, nil
}

// Index returns the index of this node in the flattened tree.
func (jv *JVSS) Index() int {
	return jv.treeIndex
}

// thread safe helpers for accessing shared (long and short-term) secrets:
type sharedSecrets struct {
	sync.Mutex
	secrets map[SID]*secret
}

func (s *sharedSecrets) secret(sid SID) (*secret, error) {
	s.Lock()
	defer s.Unlock()
	sec, ok := s.secrets[sid]
	if !ok {
		return nil, fmt.Errorf("Error, shared secret does not exist")
	}
	return sec, nil
}

func (s *sharedSecrets) addSecret(sid SID, sec *secret) {
	s.Lock()
	defer s.Unlock()
	s.secrets[sid] = sec
}

func (s *sharedSecrets) remove(sid SID) {
	s.Lock()
	defer s.Unlock()
	delete(s.secrets, sid)
}

func newSecrets() *sharedSecrets {
	return &sharedSecrets{secrets: make(map[SID]*secret)}
}

// secret contains all information for long- and short-term shared secrets.
type secret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	// XXX potentially get rid of deals buffer later:
	deals map[int]*poly.Deal // Buffer for deals
	// XXX potentially get rid of sig buffer later:
	sigs map[int]*poly.SchnorrPartialSig // Buffer for partial signatures

	// Number of collected confirmations that shared secrets are ready
	numLongtermConfs int
	nLongConfirmsMtx sync.Mutex

	// Number of collected (short-term) confirmations that shared secrets are ready
	numShortConfs     int
	nShortConfirmsMtx sync.Mutex
}

// newSID takes a TYPE of Secret,i.e. STSS or LTSS and append some random bytes
// to it
func newSID(t SID) SID {
	random := randomString(randomLength)
	return SID(string(t) + " - " + random)
}

// IsLTSS returns true if this SID is of type LTSS - longterm secret
func (s SID) IsLTSS() bool {
	return strings.HasPrefix(string(s), string(LTSS))
}

// IsSTSS returns true if this SID is of type STSS - shorterm secret
func (s SID) IsSTSS() bool {
	return strings.HasPrefix(string(s), string(STSS))
}

// randomString will read n bytes from crypto/rand, will encode these bytes in
// base64 and returns the resulting string
func randomString(n int) string {
	var buff = make([]byte, n)
	_, err := rand.Read(buff)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString([]byte(buff))
}

// sidStore stores all sid in a thred safe manner
type sidStore struct {
	mutex sync.Mutex
	store map[SID]bool
}

func newSidStore() *sidStore {
	return &sidStore{
		store: make(map[SID]bool),
	}
}

// exists return true if the given sid is stored
// false otherwise.
func (s *sidStore) exists(sid SID) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, exists := s.store[sid]
	return exists
}

// insert will store the sid and returns true if it already existed before or
// false if the sid is a new entry.
func (s *sidStore) insert(sid SID) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, exists := s.store[sid]
	s.store[sid] = true
	return exists
}

// remove will delete the sid from the store and returns true if it was present
// or false otherwise
func (s *sidStore) remove(sid SID) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, exists := s.store[sid]
	delete(s.store, sid)
	return exists
}
