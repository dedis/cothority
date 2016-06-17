package medco

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/abstract"
)

const DETERMINISTIC_SWITCHING_PROTOCOL_NAME = "DeterministicSwitching"

func init() {
	network.RegisterMessageType(DeterministicSwitchedMessage{})
	network.RegisterMessageType(CipherText{})
	network.RegisterMessageType(CipherVector{})
	sda.ProtocolRegisterName(DETERMINISTIC_SWITCHING_PROTOCOL_NAME, NewDeterministSwitchingProtocol)
}

type DeterministicSwitchedMessage struct {
	Data                  map[TempID]CipherVector
	OriginalEphemeralKeys map[TempID][]abstract.Point
	Proof                 map[TempID][]CompleteProof
}

type DeterministicSwitchedStruct struct {
	*sda.TreeNode
	DeterministicSwitchedMessage
}

type DeterministicSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan map[TempID]DeterministCipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan DeterministicSwitchedStruct

	// Protocol state data
	nextNodeInCircuit *sda.TreeNode
	TargetOfSwitch    *map[TempID]CipherVector
	SurveyPHKey       *abstract.Secret
	originalEphemKeys map[TempID][]abstract.Point
}

func NewDeterministSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	deterministicSwitchingProtocol := &DeterministicSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan map[TempID]DeterministCipherVector),
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
	if p.SurveyPHKey == nil {
		return errors.New("No PH key given.")
	}

	dbg.Lvl1(p.Entity(), "started a Deterministic Switching Protocol (", len(*p.TargetOfSwitch), "rows )")

	p.originalEphemKeys = make(map[TempID][]abstract.Point, len(*p.TargetOfSwitch))

	for k := range *p.TargetOfSwitch {
		p.originalEphemKeys[k] = make([]abstract.Point, len((*p.TargetOfSwitch)[k]))
		for i, c := range (*p.TargetOfSwitch)[k] {
			p.originalEphemKeys[k][i] = c.K
		}
	}
	p.sendToNext(&DeterministicSwitchedMessage{*p.TargetOfSwitch,
		p.originalEphemKeys,
		map[TempID][]CompleteProof{}})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *DeterministicSwitchingProtocol) Dispatch() error {

	deterministicSwitchingTarget := <-p.PreviousNodeInPathChannel

	origEphemKeys := deterministicSwitchingTarget.OriginalEphemeralKeys

	length := len(deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof)
	newProofs := map[TempID][]CompleteProof{}
	for k, v := range deterministicSwitchingTarget.Data {
		if PROOF {
			if length != 0 {
				SwitchCheckMapProofs(deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof)
			}
		}
		//dbg.LLvl1(*p.SurveyPHKey)
		schemeSwitchNewVec := v.SwitchToDeterministicNoReplace(p.Suite(), p.Private(), *p.SurveyPHKey)
		if PROOF {
			dbg.LLvl1("proofs creation")
			newProofs[k] = VectSwitchSchemeProof(p.Suite(), p.Private(), *p.SurveyPHKey, origEphemKeys[k], v, schemeSwitchNewVec)

		}
		deterministicSwitchingTarget.Data[k] = schemeSwitchNewVec
	}

	deterministicSwitchingTarget.Proof = newProofs

	if p.IsRoot() {
		deterministicSwitchedData := make(map[TempID]DeterministCipherVector, len(deterministicSwitchingTarget.Data))
		for k, v := range deterministicSwitchingTarget.Data {
			deterministicSwitchedData[k] = make(DeterministCipherVector, len(v))
			for i, c := range v {
				deterministicSwitchedData[k][i] = DeterministCipherText{c.C}
			}
		}
		dbg.Lvl1(p.Entity(), "completed deterministic switching (", len(deterministicSwitchedData),"row )")
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
