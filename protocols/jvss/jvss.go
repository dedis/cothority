package jvss

import (
	"errors"
	"sync"

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
	*sda.Node                    // The SDA TreeNode
	keyPair   *config.KeyPair    // KeyPair of the host
	nodeList  []*sda.TreeNode    // List of TreeNodes in the JVSS group
	pubKeys   []abstract.Point   // List of public keys of the above TreeNodes
	info      poly.Threshold     // Information on the thresholds of JVSS
	secret    *poly.SharedSecret //
	schnorr   *poly.Schnorr      //
	receiver  *poly.Receiver     //
	dealMtx   *sync.Mutex        //
	numDeals  int                // number of good deals already received
	setupDone chan bool          // Channel to indicate whether the shared secret has been initialised or not
	Done      chan bool          // Channel to indicate when JVSS is done
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	nodes := node.Tree().ListNodes()
	pk := make([]abstract.Point, len(nodes))
	for i, tn := range nodes {
		pk[i] = tn.Entity.Public
	}
	info := poly.Threshold{T: len(nodes), R: len(nodes), N: len(nodes)}

	jv := &JVSS{
		Node:     node,
		keyPair:  kp,
		nodeList: nodes,
		pubKeys:  pk,
		info:     info,
		schnorr:  new(poly.Schnorr),
		receiver: poly.NewReceiver(node.Suite(), info, kp),
		dealMtx:  new(sync.Mutex),
		numDeals: 0,
		Done:     make(chan bool, 1),
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

	//go jv.waitForSetup()

	return jv, nil
}

// Start initiates the JVSS protocol.
func (jv *JVSS) Start() error {

	// Initiate the long-term shared key pair which can be used later on by the
	// JVSS group to sign messages.
	deal := jv.newDeal()
	jv.addDeal(jv.nodeIdx(), deal)
	db, _ := deal.MarshalBinary()

	// broadcast our deal and wait for the deals from the others
	msg := &MsgSetup{
		Src:  jv.nodeIdx(),
		Deal: db,
	}
	jv.broadcast(msg)

	jv.Done <- true

	return nil
}

func (jv *JVSS) Verify(msg []byte, sig *poly.SchnorrSig) error {
	h := jv.keyPair.Suite.Hash()
	h.Write(msg)
	return jv.schnorr.VerifySchnorrSig(sig, h)
}

func (jv *JVSS) Sign(msg []byte) (*poly.SchnorrSig, error) {
	return nil, nil
}

func (jv *JVSS) waitForSetup() {

}

func (jv *JVSS) newDeal() *poly.Deal {
	kp := config.NewKeyPair(jv.keyPair.Suite)
	return new(poly.Deal).ConstructDeal(kp, jv.keyPair, jv.info.T, jv.info.R, jv.pubKeys)
}

func (jv *JVSS) addDeal(idx int, deal *poly.Deal) {
	if _, err := jv.receiver.AddDeal(idx, deal); err != nil {
		dbg.Error("Error adding deal to receiver %d: %v", idx, err)
	}
	jv.numDeals += 1
}

func (jv *JVSS) nodeIdx() int {
	return jv.Node.TreeNode().EntityIdx
}

func (jv *JVSS) broadcast(msg interface{}) {
	for idx, node := range jv.nodeList {
		if idx != jv.nodeIdx() {
			if err := jv.SendTo(node, msg); err != nil {
				dbg.Errorf("Couldn't send msg to node %d: %v", idx, err)
			}
		}
	}
}
