package medco

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/random"
)

type KeySwitchable interface {
	SwitchForKey(public abstract.Point)
}


func init() {
	network.RegisterMessageType(KeySwitchedCipherMessage{})
	sda.ProtocolRegisterName("KeySwitching", NewKeySwitchingProtocol)
}

type KeySwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel           chan CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan KeySwitchedCipherStruct

	// Protocol state data
	nextNodeInCircuit         *sda.TreeNode
	TargetOfSwitch            *CipherVector
	TargetPublicKey           *abstract.Point
	originalEphemKeys         []abstract.Point
}

func NewKeySwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	keySwitchingProtocol := &KeySwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel: make(chan CipherVector),
	}

	if err := keySwitchingProtocol.RegisterChannel(&keySwitchingProtocol.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			keySwitchingProtocol.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return keySwitchingProtocol, nil
}

// Starts the protocol
func (p *KeySwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No ciphertext given as key switching target.")
	}

	if p.TargetPublicKey == nil {
		return errors.New("No new public key to be switched on provided.")
	}

	dbg.Lvl1(p.Name(),"started a Key Switching Protocol")

	initialCipherVector := *InitCipherVector(p.Suite(), len(*p.TargetOfSwitch))
	p.originalEphemKeys = make([]abstract.Point, len(*p.TargetOfSwitch))
	for i, c := range *p.TargetOfSwitch {
		initialCipherVector[i].C = c.C
		p.originalEphemKeys[i] = c.K
	}

	p.sendToNext(&KeySwitchedCipherMessage{
		VisitorMessage{0},
		initialCipherVector,
		*p.TargetPublicKey,
		p.originalEphemKeys})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *KeySwitchingProtocol) Dispatch() error {

	keySwitchingTarget := <- p.PreviousNodeInPathChannel

	randomnessContrib := p.Suite().Secret().Pick(random.Stream)

	keySwitchingTarget.Vect.SwitchForKey(p.Suite(), p.Private(), keySwitchingTarget.OriginalEphemeralKeys, keySwitchingTarget.NewKey, randomnessContrib)

	if p.IsRoot() {
		p.FeedbackChannel <- keySwitchingTarget.Vect
	} else {
		p.sendToNext(&keySwitchingTarget.KeySwitchedCipherMessage)
	}

	dbg.Lvl1(p.Name(), "completed key switching.")

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *KeySwitchingProtocol) sendToNext(msg VisitorMessageI) bool {
	if !msg.AlreadyVisited(p.nextNodeInCircuit, p.Tree()) {
		err := p.SendTo(p.nextNodeInCircuit, msg)
		if err != nil {
			dbg.Lvl1("Had an error sending a message: ", err)
		}
		return true;
	}
	return false
}