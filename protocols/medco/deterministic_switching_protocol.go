// The deterministic switching protocol permits to switch a ciphertext encrypted by using
// an El-Gamal encryption (probabilistic) to a Pohlig-Hellman deterministic encrypted ciphertext.
// The El-Gamal ciphertext should be encrypted by the collective public key of the cothority. In that case,
// each cothority server (node) can remove his El-Gamal secret contribution and add a new Pohlig-Hellman
// secret contribution. By doing that the ciphertext is never decrypted.
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

// DeterministicSwitchingProtocolName is the registered name for the deterministic switching protocol.
const DeterministicSwitchingProtocolName = "DeterministicSwitching"

func init() {
	network.RegisterMessageType(DeterministicSwitchedMessage{})
	network.RegisterMessageType(libmedco.CipherText{})
	network.RegisterMessageType(libmedco.CipherVector{})
	sda.ProtocolRegisterName(DeterministicSwitchingProtocolName, NewDeterministSwitchingProtocol)
}

// DeterministicSwitchedMessage represents a deterministic switching message containing the processed cipher vectors,
// their original ephemeral keys.
type DeterministicSwitchedMessage struct {
	Data                  map[libmedco.TempID]libmedco.CipherVector
	OriginalEphemeralKeys map[libmedco.TempID][]abstract.Point
}

type deterministicSwitchedStruct struct {
	*sda.TreeNode
	DeterministicSwitchedMessage
}

// DeterministicSwitchingProtocol hold the state of a deterministic switching protocol instance.
type DeterministicSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan map[libmedco.TempID]libmedco.DeterministCipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan deterministicSwitchedStruct

	// Protocol state data
	nextNodeInCircuit *sda.TreeNode
	TargetOfSwitch    *map[libmedco.TempID]libmedco.CipherVector
	SurveyPHKey       *abstract.Scalar
	originalEphemKeys map[libmedco.TempID][]abstract.Point
}

// NewDeterministSwitchingProtocol constructs deterministic switching protocol instances.
func NewDeterministSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	dsp := &DeterministicSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan map[libmedco.TempID]libmedco.DeterministCipherVector),
	}

	if err := dsp.RegisterChannel(&dsp.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			dsp.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return dsp, nil
}

// Start is called at the root node and starts the execution of the protocol.
func (p *DeterministicSwitchingProtocol) Start() error {
	if p.TargetOfSwitch == nil {
		return errors.New("No map given as deterministic switching target.")
	}
	if p.SurveyPHKey == nil {
		return errors.New("No PH key given.")
	}

	log.Lvl1(p.Name(), "started a Deterministic Switching Protocol (", len(*p.TargetOfSwitch), "rows )")

	p.originalEphemKeys = make(map[libmedco.TempID][]abstract.Point, len(*p.TargetOfSwitch))

	for k := range *p.TargetOfSwitch {
		p.originalEphemKeys[k] = make([]abstract.Point, len((*p.TargetOfSwitch)[k]))
		for i, c := range (*p.TargetOfSwitch)[k] {
			p.originalEphemKeys[k][i] = c.K
		}
	}

	p.sendToNext(&DeterministicSwitchedMessage{*p.TargetOfSwitch,
		p.originalEphemKeys})

	return nil
}

// Dispatch is called on each tree node. It waits for incoming messages and handle them.
func (p *DeterministicSwitchingProtocol) Dispatch() error {

	deterministicSwitchingTarget := <-p.PreviousNodeInPathChannel

	// Each node should use one different PH contribution per survey.
	phContrib := p.Suite().Point().Mul(p.Suite().Point().Base(), *p.SurveyPHKey)
	for k, v := range deterministicSwitchingTarget.Data {
		v.DeterministicSwitching(&v, p.Private(), phContrib)
		deterministicSwitchingTarget.Data[k] = v
	}

	// If this tree node is the root, then protocol reached the end.
	if p.IsRoot() {
		deterministicSwitchedData := make(map[libmedco.TempID]libmedco.DeterministCipherVector, len(deterministicSwitchingTarget.Data))
		for k, v := range deterministicSwitchingTarget.Data {
			deterministicSwitchedData[k] = make(libmedco.DeterministCipherVector, len(v))
			for i, c := range v {
				deterministicSwitchedData[k][i] = libmedco.DeterministCipherText{Point: c.C}
			}
		}
		log.Lvl1(p.ServerIdentity(), "completed deterministic switching (", len(deterministicSwitchedData), "row )")
		p.FeedbackChannel <- deterministicSwitchedData
	} else { // Forward switched message.
		log.Lvl1(p.ServerIdentity(), "carried on deterministic switching.")
		p.sendToNext(&deterministicSwitchingTarget.DeterministicSwitchedMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *DeterministicSwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		log.Lvl1("Had an error sending a message: ", err)
	}
}
