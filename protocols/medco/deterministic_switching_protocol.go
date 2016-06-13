package medco

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	."github.com/dedis/cothority/services/medco/structs"
		//"github.com/dedis/crypto/random"
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
	OriginalEphemeralKeys []KeyValSPoint
	Proof [][]CompleteProof
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
	originalEphemKeys         map[TempID][]abstract.Point
}

var nilPH *abstract.Secret

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
	nilPH = p.SurveyPHKey
	if p.SurveyPHKey == nil {
		return errors.New("No PH key given.")
	}

	dbg.Lvl1(p.Entity(),"started a Deterministic Switching Protocol")

	targetSliced := MapToSliceCV(*p.TargetOfSwitch)
	
	p.originalEphemKeys = make(map[TempID][]abstract.Point, len(*p.TargetOfSwitch))
	
	for k := range *p.TargetOfSwitch {
		p.originalEphemKeys[k] = make([]abstract.Point, len((*p.TargetOfSwitch)[k]))
		for i, c := range (*p.TargetOfSwitch)[k] {
			p.originalEphemKeys[k][i] = c.K
		}
	}
	p.sendToNext(&DeterministicSwitchedMessage{targetSliced,
		MapToSliceSPoint(p.originalEphemKeys),
		[][]CompleteProof{}})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *DeterministicSwitchingProtocol) Dispatch() error {

	deterministicSwitchingTarget := <- p.PreviousNodeInPathChannel
	
	origEphemKeys := SliceToMapSPoint(deterministicSwitchingTarget.OriginalEphemeralKeys)
	
	if p.SurveyPHKey == nil {
		p.SurveyPHKey = nilPH
	}
	
	length := len(deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof)
	newProofs :=  [][]CompleteProof{}
	for i,kv := range deterministicSwitchingTarget.Data {
		if PROOF{
			if length != 0 {
				for u:=0; u < len(kv.Val); u++ {
					if !VectSwitchCheckProof(deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof[0]){
						
						dbg.Errorf("ATTENTION, false proof detected")
					}
					deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof = deterministicSwitchingTarget.DeterministicSwitchedMessage.Proof[1:]
				}
			}
		}
		
		//kv.Val.SwitchToDeterministic(p.Suite(), p.Private(), *p.SurveyPHKey)
		schemeSwitchNewVec := SwitchToDeterministic2(kv.Val, p.Suite(), p.Private(), *p.SurveyPHKey)
		dbg.LLvl1(schemeSwitchNewVec)
		if PROOF{
			dbg.LLvl1("proofs creation")
			if len(newProofs) == 0 {
				newProofs = [][]CompleteProof{[]CompleteProof{}}
			} else {
				newProofs = append(newProofs, []CompleteProof{})
			}
				//dbg.LLvl1(i)
				//suite abstract.Suite, k abstract.Secret, s abstract.Secret, Rjs []abstract.Point, C1 CipherVector, C2 CipherVector
				newProofs[i] = VectSwitchSchemeProof(p.Suite(), p.Private(), *p.SurveyPHKey, origEphemKeys[kv.Key], kv.Val, schemeSwitchNewVec)
				//dbg.LLvl1(newProofs[i])
		}
		deterministicSwitchingTarget.Data[i].Val = schemeSwitchNewVec
		
	}
	
	deterministicSwitchingTarget.Proof = newProofs
	

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