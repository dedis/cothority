package jvss

import (
	"fmt"
	"sync"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.ProtocolRegisterName("JVSS", NewJVSS)
}

// Identifiers for long- and short-term shared secrets.
const (
	LTSS = "LTSS"
	STSS = "STSS"
)

// JVSS is the main protocol struct and implements the sda.ProtocolInstance
// interface.
type JVSS struct {
	*sda.Node                         // The SDA TreeNode
	keyPair     *config.KeyPair       // KeyPair of the host
	nodeList    []*sda.TreeNode       // List of TreeNodes in the JVSS group
	pubKeys     []abstract.Point      // List of public keys of the above TreeNodes
	info        poly.Threshold        // JVSS thresholds
	schnorr     *poly.Schnorr         // Long-term Schnorr struct to compute distributed signatures
	secrets     map[string]*Secret    // Shared secrets (long- and short-term ones)
	ltssInit    bool                  // Indicator whether shared secret has been already initialised or not
	secretsDone chan bool             // Channel to indicate when shared secrets of all peers are ready
	sigChan     chan *poly.SchnorrSig // Channel for JVSS signature
}

// Secret contains all information for long- and short-term shared secrets.
type Secret struct {
	secret   *poly.SharedSecret              // Shared secret
	receiver *poly.Receiver                  // Receiver to aggregate deals
	deals    map[int]*poly.Deal              // Buffer for deals
	sigs     map[int]*poly.SchnorrPartialSig // Buffer for partial signatures
	numConfs int                             // Number of collected confirmations that shared secrets are ready
	mtx      *sync.Mutex                     // Mutex to sync access to numConfs
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	nodes := node.Tree().List()
	pk := make([]abstract.Point, len(nodes))
	for i, tn := range nodes {
		pk[i] = tn.Entity.Public
	}
	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: len(nodes), R: len(nodes), N: len(nodes)}

	jv := &JVSS{
		Node:        node,
		keyPair:     kp,
		nodeList:    nodes,
		pubKeys:     pk,
		info:        info,
		schnorr:     new(poly.Schnorr),
		secrets:     make(map[string]*Secret),
		ltssInit:    false,
		secretsDone: make(chan bool, 1),
		sigChan:     make(chan *poly.SchnorrSig),
	}

	// Setup message handlers
	handlers := []interface{}{
		jv.handleSecInit,
		jv.handleSecConf,
		jv.handleSigReq,
		jv.handleSigResp,
	}
	for _, h := range handlers {
		if err := jv.RegisterHandler(h); err != nil {
			return nil, fmt.Errorf("Error, could not register handler: " + err.Error())
		}
	}

	return jv, nil
}

// Start initiates the JVSS protocol by setting up a long-term shared secret
// which can be used later on by the JVSS group to sign and verify messages.
func (jv *JVSS) Start() error {
	jv.initSecret(LTSS)
	<-jv.secretsDone
	return nil
}

// Verify verifies the given message against the given Schnorr signature.
// Returns nil if the signature is valid and an error otherwise.
func (jv *JVSS) Verify(msg []byte, sig *poly.SchnorrSig) error {
	h := jv.keyPair.Suite.Hash()
	h.Write(msg)
	return jv.schnorr.VerifySchnorrSig(sig, h)
}

// Sign starts a new signing request amongst the JVSS group and returns a
// Schnorr signature on success.
func (jv *JVSS) Sign(msg []byte) (*poly.SchnorrSig, error) {

	if !jv.ltssInit {
		return nil, fmt.Errorf("Error, long-term shared secret has not been initialised")
	}

	// Initialise short-term shared secret only used for this signing request
	sid := fmt.Sprintf("%s%d", STSS, jv.nodeIdx())
	jv.initSecret(sid)

	// Wait for setup of shared secrets to finish
	<-jv.secretsDone

	// Create partial signature ...
	ps, err := jv.sigPartial(sid, msg)
	if err != nil {
		return nil, err
	}

	// ... and buffer it
	secret := jv.secrets[sid]
	secret.sigs[jv.nodeIdx()] = ps

	// Broadcast signing request
	req := &SigReqMsg{
		Src: jv.nodeIdx(),
		SID: sid,
		Msg: msg,
	}
	if err := jv.broadcast(req); err != nil {
		return nil, err
	}

	// Wait for complete signature
	sig := <-jv.sigChan

	return sig, nil
}

func (jv *JVSS) initSecret(sid string) error {

	// Initialise shared secret of given type if necessary
	if _, ok := jv.secrets[sid]; !ok {
		//dbg.Lvl1(fmt.Sprintf("Node %d: Initialising %s shared secret", jv.nodeIdx(), sid))
		sec := &Secret{
			receiver: poly.NewReceiver(jv.keyPair.Suite, jv.info, jv.keyPair),
			deals:    make(map[int]*poly.Deal),
			sigs:     make(map[int]*poly.SchnorrPartialSig),
			numConfs: 0,
			mtx:      new(sync.Mutex),
		}
		jv.secrets[sid] = sec
	}

	secret := jv.secrets[sid]

	// Initialise and broadcast our deal if necessary
	if len(secret.deals) == 0 {
		kp := config.NewKeyPair(jv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
		//dbg.Lvl1(fmt.Sprintf("Node %d: Initialising %s deal", jv.nodeIdx(), sid))
		secret.deals[jv.nodeIdx()] = deal
		db, _ := deal.MarshalBinary()
		msg := &SecInitMsg{
			Src:  jv.nodeIdx(),
			SID:  sid,
			Deal: db,
		}
		if err := jv.broadcast(msg); err != nil {
			return err
		}
	}
	return nil
}

func (jv *JVSS) finaliseSecret(sid string) error {
	secret, ok := jv.secrets[sid]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	//dbg.Lvl1(fmt.Sprintf("Node %d: %s deals %d/%d", jv.nodeIdx(), sid, len(secret.deals), len(jv.nodeList)))

	if len(secret.deals) == jv.info.T {

		for i := 0; i < len(secret.deals); i++ {
			if _, err := secret.receiver.AddDeal(jv.nodeIdx(), secret.deals[i]); err != nil {
				return err
			}
		}

		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return err
		}
		secret.secret = sec
		secret.mtx.Lock()
		secret.numConfs++
		secret.mtx.Unlock()
		//dbg.Lvl1(fmt.Sprintf("Node %d: shared secret %s created", jv.nodeIdx(), sid))

		// Initialise Schnorr struct for long-term shared secret if not done so before
		if sid == LTSS && !jv.ltssInit {
			jv.ltssInit = true
			jv.schnorr.Init(jv.keyPair.Suite, jv.info, secret.secret)
			//dbg.Lvl1(fmt.Sprintf("Node %d: %s Schnorr struct initialised", jv.nodeIdx(), sid))
		}

		// Broadcast that we have finished setting up our shared secret
		msg := &SecConfMsg{
			Src: jv.nodeIdx(),
			SID: sid,
		}
		if err := jv.broadcast(msg); err != nil {
			return err
		}
	}
	return nil
}

func (jv *JVSS) sigPartial(sid string, msg []byte) (*poly.SchnorrPartialSig, error) {
	secret, ok := jv.secrets[sid]
	if !ok {
		return nil, fmt.Errorf("Error, shared secret does not exist")
	}

	hash := jv.keyPair.Suite.Hash()
	hash.Write(msg)
	if err := jv.schnorr.NewRound(secret.secret, hash); err != nil {
		return nil, err
	}
	ps := jv.schnorr.RevealPartialSig()
	if ps == nil {
		return nil, fmt.Errorf("Error, node %d could not create partial signature", jv.nodeIdx())
	}
	return ps, nil
}

func (jv *JVSS) nodeIdx() int {
	return jv.Node.TreeNode().EntityIdx
}

func (jv *JVSS) broadcast(msg interface{}) error {
	for _, node := range jv.nodeList {
		if node != jv.TreeNode() {
			if err := jv.SendTo(node, msg); err != nil {
				return err
			}
		}
	}
	return nil
}
