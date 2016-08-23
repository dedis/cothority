// Package medco contains the key switching protocol which permits to switch a ciphertext
// encrypted under a specific key by using an El-Gamal encryption (probabilistic) to a ciphertext encrypted
// under another key.
// The El-Gamal ciphertext should be encrypted by the collective public key of the cothority. In that case,
// each cothority server (node) can remove his El-Gamal secret contribution and add a new
// secret contribution containing the new key. By doing that the ciphertext is never decrypted.
// This is done by creating a circuit between the servers. The ciphertext is sent through this circuit and
// each server applies its transformation on the ciphertext and forwards it to the next node in the circuit
// until it comes back to the server who started the protocol.
package medco

import (
	"errors"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/abstract"
)

// KeySwitchingProtocolName is the registered name for the key switching protocol.
const KeySwitchingProtocolName = "KeySwitching"

// KeySwitchedCipherMessage contains cipherVector under switching.
type KeySwitchedCipherMessage struct {
	Data                  map[libmedco.TempID]libmedco.CipherVector
	NewKey                abstract.Point
	OriginalEphemeralKeys map[libmedco.TempID][]abstract.Point
}

type keySwitchedCipherStruct struct {
	*sda.TreeNode
	KeySwitchedCipherMessage
}

func init() {
	network.RegisterPacketType(KeySwitchedCipherMessage{})
	sda.ProtocolRegisterName(KeySwitchingProtocolName, NewKeySwitchingProtocol)
}

// KeySwitchingProtocol is a struct holding the state of a protocol instance.
type KeySwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan map[libmedco.TempID]libmedco.CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan keySwitchedCipherStruct

	// Protocol state data
	nextNodeInCircuit *sda.TreeNode
	TargetOfSwitch    *map[libmedco.TempID]libmedco.CipherVector
	TargetPublicKey   *abstract.Point
	originalEphemKeys map[libmedco.TempID][]abstract.Point
}

// NewKeySwitchingProtocol is constructor of Key Switching protocol instances.
func NewKeySwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	ksp := &KeySwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan map[libmedco.TempID]libmedco.CipherVector),
	}

	if err := ksp.RegisterChannel(&ksp.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			ksp.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return ksp, nil
}

// Start is called at the root to start the execution of the key switching.
func (p *KeySwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No ciphertext given as key switching target.")
	}

	if p.TargetPublicKey == nil {
		return errors.New("No new public key to be switched on provided.")
	}

	log.Lvl1(p.ServerIdentity(), "started a Key Switching Protocol")

	// Creates initialize the target cipher text and extract the original ephemeral keys.
	initialMap := make(map[libmedco.TempID]libmedco.CipherVector, len(*p.TargetOfSwitch))
	p.originalEphemKeys = make(map[libmedco.TempID][]abstract.Point, len(*p.TargetOfSwitch))
	for k := range *p.TargetOfSwitch {
		initialCipherVector := *libmedco.NewCipherVector(len((*p.TargetOfSwitch)[k]))
		p.originalEphemKeys[k] = make([]abstract.Point, len((*p.TargetOfSwitch)[k]))
		for i, c := range (*p.TargetOfSwitch)[k] {
			initialCipherVector[i].C = c.C
			p.originalEphemKeys[k][i] = c.K
		}
		initialMap[k] = initialCipherVector
	}

	p.sendToNext(&KeySwitchedCipherMessage{
		initialMap,
		*p.TargetPublicKey,
		p.originalEphemKeys})

	return nil
}

// Dispatch is called on each node. It waits for incoming messages and handle them.
func (p *KeySwitchingProtocol) Dispatch() error {

	keySwitchingTarget := <-p.PreviousNodeInPathChannel

	for k, v := range keySwitchingTarget.Data {
		origEphemKeys := keySwitchingTarget.OriginalEphemeralKeys[k]
		v.KeySwitching(&v, &origEphemKeys, keySwitchingTarget.NewKey, p.Private())
		keySwitchingTarget.Data[k] = v
	}

	// If the tree node is the root then protocol returns.
	if p.IsRoot() {
		log.Lvl1(p.ServerIdentity(), "completed key switching.")
		p.FeedbackChannel <- keySwitchingTarget.Data
	} else {
		log.Lvl1(p.ServerIdentity(), "carried on key switching.")
		p.sendToNext(&keySwitchingTarget.KeySwitchedCipherMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *KeySwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		log.Lvl1("Had an error sending a message: ", err)
	}
}
