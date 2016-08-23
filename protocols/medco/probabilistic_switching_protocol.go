// Package medco contains the probabilistic switching protocol which permits to switch a ciphertext encrypted
// under a deterministic Pohlig-Hellman encryption to a probabilistic El-Gamal encryption.
// Each cothority server (node) removes his Pohlig-Hellman secret contribution and adds a new
// El-Gamal secret contribution. By doing that the ciphertext is never decrypted.
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

// ProbabilisticSwitchingProtocolName is the registered name for the probabilistic switching protocol.
const ProbabilisticSwitchingProtocolName = "ProbabilisticSwitching"

func init() {
	network.RegisterPacketType(ProbabilisticSwitchedMessage{})
	sda.ProtocolRegisterName(ProbabilisticSwitchingProtocolName, NewProbabilisticSwitchingProtocol)
}

// ProbabilisticSwitchedMessage contains switched vector and data used in protocol.
type ProbabilisticSwitchedMessage struct {
	Data            map[libmedco.TempID]libmedco.CipherVector
	TargetPublicKey abstract.Point
}

type probabilisticSwitchedStruct struct {
	*sda.TreeNode
	ProbabilisticSwitchedMessage
}

// ProbabilisticSwitchingProtocol is a struct holding the state of a protocol instance.
type ProbabilisticSwitchingProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan map[libmedco.TempID]libmedco.CipherVector

	// Protocol communication channels
	PreviousNodeInPathChannel chan probabilisticSwitchedStruct

	// Protocol state data
	nextNodeInCircuit *sda.TreeNode
	TargetOfSwitch    *map[libmedco.TempID]libmedco.DeterministCipherVector
	SurveyPHKey       *abstract.Scalar
	TargetPublicKey   *abstract.Point
}

// NewProbabilisticSwitchingProtocol is the protocol instance constructor.
func NewProbabilisticSwitchingProtocol(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	psp := &ProbabilisticSwitchingProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan map[libmedco.TempID]libmedco.CipherVector),
	}

	if err := psp.RegisterChannel(&psp.PreviousNodeInPathChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			psp.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return psp, nil
}

// Start is called at the root to start the execution of the protocol.
func (p *ProbabilisticSwitchingProtocol) Start() error {

	if p.TargetOfSwitch == nil {
		return errors.New("No map given as probabilistic switching target.")
	}
	if p.TargetPublicKey == nil {
		return errors.New("No map given as target public key.")
	}
	if p.SurveyPHKey == nil {
		return errors.New("No PH key given.")
	}

	log.Lvl1(p.ServerIdentity(), "started a Probabilistic Switching Protocol")

	targetOfSwitch := make(map[libmedco.TempID]libmedco.CipherVector, len(*p.TargetOfSwitch))
	for k := range *p.TargetOfSwitch {
		targetOfSwitch[k] = make(libmedco.CipherVector, len((*p.TargetOfSwitch)[k]))
		for i, dc := range (*p.TargetOfSwitch)[k] {
			var pc libmedco.CipherText
			pc.K = network.Suite.Point().Null()
			pc.C = dc.Point
			targetOfSwitch[k][i] = pc
		}
	}
	p.sendToNext(&ProbabilisticSwitchedMessage{targetOfSwitch, *p.TargetPublicKey})

	return nil
}

// Dispatch handles messages from channels.
func (p *ProbabilisticSwitchingProtocol) Dispatch() error {

	probabilisticSwitchingTarget := <-p.PreviousNodeInPathChannel

	phContrib := suite.Point().Mul(suite.Point().Base(), *p.SurveyPHKey)
	for k, v := range probabilisticSwitchingTarget.Data {
		v.ProbabilisticSwitching(&v, phContrib, probabilisticSwitchingTarget.TargetPublicKey)
		probabilisticSwitchingTarget.Data[k] = v
	}

	if p.IsRoot() {
		log.Lvl1(p.ServerIdentity(), "completed probabilistic switching.")
		p.FeedbackChannel <- probabilisticSwitchingTarget.Data
	} else {
		log.Lvl1(p.ServerIdentity(), "carried on probabilistic switching.")
		p.sendToNext(&probabilisticSwitchingTarget.ProbabilisticSwitchedMessage)
	}

	return nil
}

// Sends the message msg to the next node in the circuit based on the next TreeNode in Tree.List() If not visited yet.
// If the message already visited the next node, doesn't send and returns false. Otherwise, return true.
func (p *ProbabilisticSwitchingProtocol) sendToNext(msg interface{}) {
	err := p.SendTo(p.nextNodeInCircuit, msg)
	if err != nil {
		log.Lvl1("Had an error sending a message: ", err)
	}
}
