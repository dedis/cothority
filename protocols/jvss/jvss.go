package jvss

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.ProtocolRegisterName("JVSS", NewJVSS)
}

// JVSS is the main protocol struct and implements the sda.ProtocolInstance
// interface.
type JVSS struct {
	*sda.Node                     // The SDA TreeNode
	keyPair      *config.KeyPair  // KeyPair of the host
	nodeList     []*sda.TreeNode  // List of TreeNodes in the JVSS group
	pubKeys      []abstract.Point // List of public keys of the above TreeNodes
	info         poly.Threshold   // JVSS thresholds
	schnorr      *poly.Schnorr    // Long-term Schnorr struct to compute distributed signatures
	ltSecret     *JVSSSecret      // Long-term shared secret
	ltSecretInit bool             // Indicator whether shared secret has been initialised or not
	dealMtx      *sync.Mutex      // Some Mutex
	Done         chan bool        // Channel to indicate when JVSS is done
}

// JVSSSecret contains all information for long- and short-term (i.e. random)
// shared secrets
type JVSSSecret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	numDeals int                // Number of good deals stored in the receiver
	dealInit bool               // Indicator whether own deal has been initialised and broadcasted or not
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	nodes := node.Tree().ListNodes()
	pk := make([]abstract.Point, len(nodes))
	for i, tn := range nodes {
		pk[i] = tn.Entity.Public
	}
	// Note: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: len(nodes), R: len(nodes), N: len(nodes)}

	// the long-term shared secret
	lts := &JVSSSecret{
		receiver: poly.NewReceiver(node.Suite(), info, kp),
		numDeals: 0,
		dealInit: false,
	}

	jv := &JVSS{
		Node:         node,
		keyPair:      kp,
		nodeList:     nodes,
		pubKeys:      pk,
		info:         info,
		schnorr:      new(poly.Schnorr),
		ltSecret:     lts,
		ltSecretInit: false,
		dealMtx:      new(sync.Mutex),
		Done:         make(chan bool, 1),
	}

	// Setup message handlers
	handlers := []interface{}{
		jv.handleSetup,
	}
	for _, h := range handlers {
		if err := jv.RegisterHandler(h); err != nil {
			return nil, errors.New("Couldn't register handler: " + err.Error())
		}
	}

	return jv, nil
}

// Start initiates the JVSS protocol by setting up a long-term shared secret
// which can be used later on by the JVSS group to sign and verify messages.
func (jv *JVSS) Start() error {
	jv.initSecret(jv.ltSecret)
	time.Sleep(1 * time.Second) // TODO: workaround

	jv.Done <- true
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

	// 1. setup a new random distributed secret for this signing request
	//		- see handleRequestSecret
	//		- setup a deal and broadcast
	//		- wait for deals from the others
	//		- setup rnd shared secret
	// 2. sign the message

	//deals := 0
	//kp := config.NewKeyPair(jv.keyPair.Suite)
	//rcv := poly.NewReceiver(jv.keyPair.Suite, *jv.info, kp)
	//deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
	//if _, err := rcv.AddDeal(jv.nodeIdx(), deal); err != nil {
	//	dbg.Errorf("Error adding deal to rnd receiver %d: %v", jv.nodeIdx(), err)
	//}
	//deals++

	// broadcast rnd deal and wait for replies

	// TODO: maybe initialise a separate struct for shared secret which can be both long-term or per request

	// add deal somewhere

	//jv.schnorr.NewRound()

	// start a signing request
	// h = suite.Hash()
	// h.Write(msg)
	// jv. newSigning(jv.secret, h)
	//	- schnorr.NewRound(random, msg)
	//  - schnorr.RevealPartialSig()
	// jv.schnorr.AddPartialSig(ps)

	return nil, nil
}

func (jv *JVSS) initSecret(secret *JVSSSecret) {
	if !secret.dealInit {
		secret.dealInit = true
		kp := config.NewKeyPair(jv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
		jv.addDeal(secret, jv.nodeIdx(), deal)
		db, _ := deal.MarshalBinary()
		jv.broadcast(&SetupMsg{Src: jv.nodeIdx(), Deal: db})
	}
}

func (jv *JVSS) addDeal(secret *JVSSSecret, idx int, deal *poly.Deal) {
	if _, err := secret.receiver.AddDeal(idx, deal); err != nil {
		dbg.Errorf("Error adding deal to receiver %d: %v", idx, err)
	}
	secret.numDeals += 1
	dbg.Lvl1(fmt.Sprintf("Node %d: deals %d/%d", jv.nodeIdx(), secret.numDeals, len(jv.nodeList)))
}

func (jv *JVSS) finaliseSecret(secret *JVSSSecret) {
	if secret.numDeals == jv.info.T {
		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			dbg.Errorf("Error node %d could not produce shared secret: %v", jv.nodeIdx(), err)
		}
		secret.secret = sec

		// Initialise long-term shared secret if not done so before
		if !jv.ltSecretInit {
			jv.ltSecretInit = true
			jv.schnorr.Init(jv.keyPair.Suite, jv.info, secret.secret)
			dbg.Lvl1(fmt.Sprintf("Node %d: long-term shared secret created", jv.nodeIdx()))
		}
	}
}

func (jv *JVSS) nodeIdx() int {
	return jv.Node.TreeNode().EntityIdx
}

func (jv *JVSS) broadcast(msg interface{}) {
	for idx, node := range jv.nodeList {
		if idx != jv.nodeIdx() {
			if err := jv.SendTo(node, msg); err != nil {
				dbg.Errorf("Error sending msg to node %d: %v", idx, err)
			}
		}
	}
}
