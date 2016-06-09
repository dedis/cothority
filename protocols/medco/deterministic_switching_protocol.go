package medco

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	."github.com/dedis/cothority/services/medco/structs"
)

const DETERMINISTIC_SWITCHING_PROTOCOL_NAME = "DeterministicSwitching"

func init() {
	network.RegisterMessageType(DeterministicSwitchedMessage{})
	network.RegisterMessageType(CipherText{})
	network.RegisterMessageType(CipherVector{})
	sda.ProtocolRegisterName(DETERMINISTIC_SWITCHING_PROTOCOL_NAME, NewDeterministSwitchingProtocol)
}

type DeterministicSwitchedMessage struct {
	Data []KeyValCV
}

type DeterministicSwitchedStruct struct {
	*sda.TreeNode
	DeterministicSwitchedMessage
}

type DeterministicSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel           chan map[TempID]DeterministCipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan DeterministicSwitchedStruct

	// Protocol state data
	nextNodeInCircuit         *sda.TreeNode
	TargetOfSwitch            *map[TempID]CipherVector
	SurveyPHKey		  *abstract.Secret
}

func NewDeterministSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	deterministicSwitchingProtocol := &DeterministicSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel: make(chan map[TempID]DeterministCipherVector),
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
func (p *DeterministicSwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No map given as deterministic switching target.")
	}

	dbg.Lvl1(p.Entity(),"started a Deterministic Switching Protocol")

	targetSliced := MapToSliceCV(*p.TargetOfSwitch)

	p.sendToNext(&DeterministicSwitchedMessage{targetSliced})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *DeterministicSwitchingProtocol) Dispatch() error {

	deterministicSwitchingTarget := <- p.PreviousNodeInPathChannel

	for _,kv := range deterministicSwitchingTarget.Data {
		kv.Val.SwitchToDeterministic(p.Suite(), p.Private(), *p.SurveyPHKey)
	}

	if p.IsRoot() {
		dbg.Lvl1(p.Entity(), "completed deterministic switching.")
		deterministicSwitchedData := make(map[TempID]DeterministCipherVector, len(deterministicSwitchingTarget.Data))
		for _,kv := range deterministicSwitchingTarget.Data {
			deterministicSwitchedData[kv.Key] = make(DeterministCipherVector, len(kv.Val))
			for i, c := range kv.Val {
				deterministicSwitchedData[kv.Key][i] = DeterministCipherText{c.C}
			}
		}
		p.FeedbackChannel <- deterministicSwitchedData
	} else {
		dbg.Lvl1(p.Entity(), "carried on deterministic switching.")
		p.sendToNext(&deterministicSwitchingTarget.DeterministicSwitchedMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *DeterministicSwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		dbg.Lvl1("Had an error sending a message: ", err)
	}
}