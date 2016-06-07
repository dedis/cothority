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
	network.RegisterMessageType(ProbabilisticSwitchedMessage{})
	sda.ProtocolRegisterName("DeterministSwitching", NewProbabilisticSwitchingProtocol)
}

type ProbabilisticSwitchedMessage struct {
	Data map[uuid.UUID]CipherVector
}

type ProbabilisticSwitchedStruct struct {
	*sda.TreeNode
	ProbabilisticSwitchedMessage
}

type ProbabilisticSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel           chan map[uuid.UUID]CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan ProbabilisticSwitchedStruct

	// Protocol state data
	nextNodeInCircuit         *sda.TreeNode
	TargetOfSwitch            *map[uuid.UUID]CipherVector
	SurveyPHKey		  *abstract.Secret
}

func NewProbabilisticSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	probabilisticSwitchingProtocol := &ProbabilisticSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel: make(chan map[uuid.UUID]CipherVector),
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

	dbg.Lvl1(p.Entity(),"started a Probabilistic Switching Protocol")

	p.sendToNext(&ProbabilisticSwitchedMessage{*p.TargetOfSwitch})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *ProbabilisticSwitchingProtocol) Dispatch() error {

	probabilisticSwitchingTarget := <- p.PreviousNodeInPathChannel

	for k,_ := range *p.TargetOfSwitch {
		(*p.TargetOfSwitch)[k].SwitchToProbabilistic(p.Suite(), p.Private(), p.SurveyPHKey)
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