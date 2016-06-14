package medco

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/abstract"
)

const PROBABILISTIC_SWITCHING_PROTOCOL_NAME = "ProbabilisticSwitching"

func init() {
	network.RegisterMessageType(ProbabilisticSwitchedMessage{})
	sda.ProtocolRegisterName(PROBABILISTIC_SWITCHING_PROTOCOL_NAME, NewProbabilisticSwitchingProtocol)
}

type ProbabilisticSwitchedMessage struct {
	Data            map[TempID]CipherVector
	TargetPublicKey abstract.Point
}

type ProbabilisticSwitchedStruct struct {
	*sda.TreeNode
	ProbabilisticSwitchedMessage
}

type ProbabilisticSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan map[TempID]CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan ProbabilisticSwitchedStruct

	// Protocol state data
	nextNodeInCircuit *sda.TreeNode
	TargetOfSwitch    *map[TempID]DeterministCipherVector
	SurveyPHKey       *abstract.Secret
	TargetPublicKey   *abstract.Point
}

func NewProbabilisticSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	probabilisticSwitchingProtocol := &ProbabilisticSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan map[TempID]CipherVector),
	}

	if err := probabilisticSwitchingProtocol.RegisterChannel(&probabilisticSwitchingProtocol.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			probabilisticSwitchingProtocol.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return probabilisticSwitchingProtocol, nil
}

// Starts the protocol
func (p *ProbabilisticSwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No map given as probabilistic switching target.")
	}
	if p.TargetPublicKey == nil {
		return errors.New("No map given as target public key.")
	}

	dbg.Lvl1(p.Entity(), "started a Probabilistic Switching Protocol")

	targetOfSwitch := make(map[TempID]CipherVector, len(*p.TargetOfSwitch))
	for k := range *p.TargetOfSwitch {
		targetOfSwitch[k] = make(CipherVector, len((*p.TargetOfSwitch)[k]))
		for i, dc := range (*p.TargetOfSwitch)[k] {
			var pc CipherText
			pc.K = network.Suite.Point().Null()
			pc.C = dc.C
			targetOfSwitch[k][i] = pc
		}
	}
	p.sendToNext(&ProbabilisticSwitchedMessage{targetOfSwitch, *p.TargetPublicKey})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *ProbabilisticSwitchingProtocol) Dispatch() error {

	probabilisticSwitchingTarget := <-p.PreviousNodeInPathChannel

	for _, v := range probabilisticSwitchingTarget.Data {
		v.SwitchToProbabilistic(p.Suite(), *p.SurveyPHKey, probabilisticSwitchingTarget.TargetPublicKey)
	}

	if p.IsRoot() {
		dbg.Lvl1(p.Entity(), "completed probabilistic switching.")
		p.FeedbackChannel <- probabilisticSwitchingTarget.Data
	} else {
		dbg.Lvl1(p.Entity(), "carried on probabilistic switching.")
		p.sendToNext(&probabilisticSwitchingTarget.ProbabilisticSwitchedMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *ProbabilisticSwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		dbg.Lvl1("Had an error sending a message: ", err)
	}
}
