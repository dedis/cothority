package jvss

import (
	"errors"
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.ProtocolRegisterName("JVSS", NewJVSS)
}

// LTSS is the identifier of the long-term shared secret.
const LTSS = "LTSS"

// JVSS is the main protocol struct and implements the sda.ProtocolInstance
// interface.
type JVSS struct {
	*sda.Node                        // The SDA TreeNode
	keyPair   *config.KeyPair        // KeyPair of the host
	nodeList  []*sda.TreeNode        // List of TreeNodes in the JVSS group
	pubKeys   []abstract.Point       // List of public keys of the above TreeNodes
	info      poly.Threshold         // JVSS thresholds
	schnorr   *poly.Schnorr          // Long-term Schnorr struct to compute distributed signatures
	secrets   map[string]*JVSSSecret // Shared secrets (long- and short-term ones)
	ltssInit  bool                   // Indicator whether shared secret has been already initialised or not
	LTSSDone  chan bool              // Channel to indicate when long-term shared secret is ready
	STSSDone  chan bool              // Channel to indicate when a short-term shared secret is ready
	sigChan   chan *poly.SchnorrSig  // Channel for JVSS signature
}

// JVSSSecret contains all information for long- and short-term shared secrets.
type JVSSSecret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	numDeals int                // Number of collected deals in the receiver
	dealInit bool               // Indicator whether own deal has been initialised and broadcasted or not
	numPSigs int                // Number of collected partial signatures
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	nodes := node.Tree().ListNodes()
	pk := make([]abstract.Point, len(nodes))
	for i, tn := range nodes {
		pk[i] = tn.Entity.Public
	}
	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: len(nodes), R: len(nodes), N: len(nodes)}

	jv := &JVSS{
		Node:     node,
		keyPair:  kp,
		nodeList: nodes,
		pubKeys:  pk,
		info:     info,
		schnorr:  new(poly.Schnorr),
		secrets:  make(map[string]*JVSSSecret),
		ltssInit: false,
		LTSSDone: make(chan bool, 1),
		STSSDone: make(chan bool, 1),
		sigChan:  make(chan *poly.SchnorrSig),
	}

	// Setup message handlers
	handlers := []interface{}{
		jv.handleSetup,
		jv.handleSigReq,
		jv.handleSigResp,
	}
	for _, h := range handlers {
		if err := jv.RegisterHandler(h); err != nil {
			return nil, errors.New("Could not register handler: " + err.Error())
		}
	}

	return jv, nil
}

// Start initiates the JVSS protocol by setting up a long-term shared secret
// which can be used later on by the JVSS group to sign and verify messages.
func (jv *JVSS) Start() error {
	jv.initSecret(LTSS)
	return nil
}

// Verify
func (jv *JVSS) Verify(msg []byte, sig *poly.SchnorrSig) error {
	h := jv.keyPair.Suite.Hash()
	h.Write(msg)
	return jv.schnorr.VerifySchnorrSig(sig, h)
}

// Sign
func (jv *JVSS) Sign(msg []byte) (*poly.SchnorrSig, error) {

	// Initialise short-term shared secret only used for this signing request
	sid := fmt.Sprintf("STSS%d", jv.nodeIdx())
	jv.initSecret(sid)

	// Wait for setup to finish
	<-jv.STSSDone

	// Create partial signature ...
	ps, err := jv.sigPartial(sid, msg)
	if err != nil {
		return nil, err
	}

	// ... and save it
	if err := jv.schnorr.AddPartialSig(ps); err != nil {
		return nil, err
	}
	sts := jv.secrets[sid]
	sts.numPSigs++

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
		dbg.Lvl2("Initialising shared secret", sid)
		sec := &JVSSSecret{
			receiver: poly.NewReceiver(jv.keyPair.Suite, jv.info, jv.keyPair),
			numDeals: 0,
			dealInit: false,
			numPSigs: 0,
		}
		jv.secrets[sid] = sec
	}

	secret := jv.secrets[sid]

	// Initialise and broadcast our deal if necessary
	if !secret.dealInit {
		secret.dealInit = true
		kp := config.NewKeyPair(jv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
		if err := jv.addDeal(sid, deal); err != nil {
			return err
		}
		db, _ := deal.MarshalBinary()
		msg := &SetupMsg{
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

func (jv *JVSS) addDeal(sid string, deal *poly.Deal) error {
	secret, ok := jv.secrets[sid]
	if !ok {
		return fmt.Errorf("Error shared secret does not exist")
	}
	if _, err := secret.receiver.AddDeal(jv.nodeIdx(), deal); err != nil {
		return fmt.Errorf("Error adding deal to receiver %d: %v", jv.nodeIdx(), err)
	}
	secret.numDeals += 1
	dbg.Lvl2(fmt.Sprintf("Node %d: deals %d/%d", jv.nodeIdx(), secret.numDeals, len(jv.nodeList)))
	return nil
}

func (jv *JVSS) finaliseSecret(sid string) error {
	secret := jv.secrets[sid]
	if secret.numDeals == jv.info.T {
		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return fmt.Errorf("Error node %d could not create shared secret %s: %v", jv.nodeIdx(), sid, err)
		}
		secret.secret = sec
		dbg.Lvl2(fmt.Sprintf("Node %d: shared secret %s created", jv.nodeIdx(), sid))

		// Notify signing initiator that the short-term secret is ready
		if sid == fmt.Sprintf("STSS%d", jv.nodeIdx()) {
			jv.STSSDone <- true
		}

		// Initialise Schnorr struct for long-term shared secret if not done so before
		if sid == LTSS && !jv.ltssInit {
			jv.ltssInit = true
			jv.schnorr.Init(jv.keyPair.Suite, jv.info, secret.secret)
			jv.LTSSDone <- true
			dbg.Lvl2(fmt.Sprintf("Node %d: Schnorr struct for shared secret %s initialised", jv.nodeIdx(), sid))
		}
	}
	return nil
}

func (jv *JVSS) sigPartial(sid string, msg []byte) (*poly.SchnorrPartialSig, error) {
	secret := jv.secrets[sid]
	hash := jv.keyPair.Suite.Hash()
	hash.Write(msg)
	if err := jv.schnorr.NewRound(secret.secret, hash); err != nil {
		return nil, fmt.Errorf("Error node %d could not start new signing round: %v", jv.nodeIdx(), err)
	}
	ps := jv.schnorr.RevealPartialSig()
	if ps == nil {
		return nil, fmt.Errorf("Error node %d could not create partial signature", jv.nodeIdx())
	}
	return ps, nil
}

func (jv *JVSS) nodeIdx() int {
	return jv.Node.TreeNode().EntityIdx
}

func (jv *JVSS) broadcast(msg interface{}) error {
	for idx, node := range jv.nodeList {
		if idx != jv.nodeIdx() {
			if err := jv.SendTo(node, msg); err != nil {
				return fmt.Errorf("Error sending msg to node %d: %v", idx, err)
			}
		}
	}
	return nil
}
