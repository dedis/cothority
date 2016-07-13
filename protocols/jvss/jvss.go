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
	"fmt"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.ProtocolRegisterName("JVSS", NewJVSS)
}

// SID is the type of shared secret identifiers
type SID string

// Identifiers for long- and short-term shared secrets.
const (
	LTSS SID = "LTSS"
	STSS SID = "STSS"
)

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

	longTermSecDone  chan bool // Channel to indicate when long-term shared secrets of all peers are ready
	shortTermSecDone chan bool // Channel to indicate when short-term shared secrets of all peers are ready

	sigChan chan *poly.SchnorrSig // Channel for JVSS signature
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	n := len(node.List())
	pk := make([]abstract.Point, n)
	for i, tn := range node.List() {
		pk[i] = tn.ServerIdentity.Public
	}
	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: n, R: n, N: n}

	jv := &JVSS{
		TreeNodeInstance: node,
		keyPair:          kp,
		pubKeys:          pk,
		info:             info,
		schnorr:          new(poly.Schnorr),
		secrets:          newSecrets(),
		ltssInit:         false,
		longTermSecDone:  make(chan bool, n),
		shortTermSecDone: make(chan bool, n),
		sigChan:          make(chan *poly.SchnorrSig),
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
		log.Error(err)
	}
	return jv, err
}

// Start initiates the JVSS protocol by setting up a long-term shared secret
// which can be used later on by the JVSS group to sign and verify messages.
func (jv *JVSS) Start() error {
	err := jv.initSecret(LTSS)
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

	// Initialise short-term shared secret only used for this signing request
	sid := SID(fmt.Sprintf("%s%d", STSS, jv.Index()))
	if err := jv.initSecret(sid); err != nil {
		return nil, err
	}

	// Wait for setup of shared secrets to finish
	log.Lvl2("Waiting on short-term secrets:", jv.Name())
	<-jv.shortTermSecDone
	log.Lvl2("Done waiting on short-term secrets", jv.Name())
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
	// Initialise shared secret of given type if necessary
	if sec, err := jv.secrets.secret(sid); sec == nil && err != nil {
		log.Lvl2(fmt.Sprintf("Node %d: Initialising %s shared secret", jv.Index(), sid))
		sec := &secret{
			receiver: poly.NewReceiver(jv.keyPair.Suite, jv.info, jv.keyPair),
			deals:    make(map[int]*poly.Deal),
			sigs:     make(map[int]*poly.SchnorrPartialSig),
			numConfs: 0,
		}
		jv.secrets.addSecret(sid, sec)
	}

	secret, err := jv.secrets.secret(sid)
	if err != nil { // this should never happen here
		return err
	}

	// Initialise and broadcast our deal if necessary
	if len(secret.deals) == 0 {
		kp := config.NewKeyPair(jv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
		log.Lvl2(fmt.Sprintf("Node %d: Initialising %v deal", jv.Index(), sid))
		secret.deals[jv.Index()] = deal
		db, _ := deal.MarshalBinary()
		msg := &SecInitMsg{
			Src:  jv.Index(),
			SID:  sid,
			Deal: db,
		}
		if err := jv.Broadcast(msg); err != nil {
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

	log.Lvl2(fmt.Sprintf("Node %d: %s deals %d/%d", jv.Index(), sid, len(secret.deals), len(jv.List())))

	if len(secret.deals) == jv.info.T {

		for _, deal := range secret.deals {
			if _, err := secret.receiver.AddDeal(jv.Index(), deal); err != nil {
				return err
			}
		}

		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return err
		}
		secret.secret = sec
		secret.incrementConfirms()
		log.Lvl2(fmt.Sprintf("Node %d: %v created", jv.Index(), sid))

		// Initialise Schnorr struct for long-term shared secret if not done so before
		if sid == LTSS && !jv.ltssInit {
			jv.ltssInit = true
			jv.schnorr.Init(jv.keyPair.Suite, jv.info, secret.secret)
			log.Lvl2(fmt.Sprintf("Node %d: %v Schnorr struct initialised", jv.Index(), sid))
		}

		// Broadcast that we have finished setting up our shared secret
		msg := &SecConfMsg{
			Src: jv.Index(),
			SID: sid,
		}
		if err := jv.Broadcast(msg); err != nil {
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
	sigs         map[int]*poly.SchnorrPartialSig // Buffer for partial signatures
	nConfirmsMtx sync.Mutex                      // Mutex to sync access to numConfs
	numConfs     int                             // Number of collected confirmations that shared secrets are ready
}

func (s *secret) incrementConfirms() {
	s.nConfirmsMtx.Lock()
	defer s.nConfirmsMtx.Unlock()
	s.numConfs++
}

func (s *secret) numConfirms() int {
	s.nConfirmsMtx.Lock()
	defer s.nConfirmsMtx.Unlock()
	return s.numConfs
}

func (s *secret) resetConfirms() {
	s.nConfirmsMtx.Lock()
	defer s.nConfirmsMtx.Unlock()
	s.numConfs = 0
}
