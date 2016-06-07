package medco

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)


func init() {
	network.RegisterMessageType(DeterministicSwitchedMessage{})
	sda.ProtocolRegisterName("DeterministSwitching", NewDeterministSwitchingProtocol)
}

type DeterministicSwitchedMessage struct {
	Data map[uuid.UUID]CipherVector
}

type DeterministicSwitchedStruct struct {
	*sda.TreeNode
	DeterministicSwitchedMessage
}

type DeterministSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel           chan map[uuid.UUID]CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan DeterministicSwitchedStruct

	// Protocol state data
	nextNodeInCircuit         *sda.TreeNode
	TargetOfSwitch            *map[uuid.UUID]CipherVector
	SurveyPHKey		  *abstract.Secret
}

func NewDeterministSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	deterministicSwitchingProtocol := &DeterministSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel: make(chan map[uuid.UUID]CipherVector),
	}

	if err := deterministicSwitchingProtocol.RegisterChannel(&deterministicSwitchingProtocol.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			deterministicSwitchingProtocol.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return deterministicSwitchingProtocol, nil
}

// Starts the protocol
func (p *DeterministSwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No map given as deterministic switching target.")
	}

	dbg.Lvl1(p.Entity(),"started a Deterministic Switching Protocol")

	p.sendToNext(&DeterministicSwitchedMessage{*p.TargetOfSwitch})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *DeterministSwitchingProtocol) Dispatch() error {

	deterministicSwitchingTarget := <- p.PreviousNodeInPathChannel

	for k,_ := range *p.TargetOfSwitch {
		(*p.TargetOfSwitch)[k].SwitchToDeterministic(p.Suite(), p.Private(), p.SurveyPHKey)
	}


	if p.IsRoot() {
		dbg.Lvl1(p.Entity(), "completed deterministic switching.")
		p.FeedbackChannel <- deterministicSwitchingTarget.Data
	} else {
		dbg.Lvl1(p.Entity(), "carried on deterministic switching.")
		p.sendToNext(&deterministicSwitchingTarget.DeterministicSwitchedMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *DeterministSwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		dbg.Lvl1("Had an error sending a message: ", err)
	}
}