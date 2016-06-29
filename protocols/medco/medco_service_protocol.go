package medco

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	. "github.com/dedis/cothority/services/medco/libmedco"
)

// MedcoServiceProtocolName is the registered name for the medco service protocol
const MedcoServiceProtocolName = "MedcoServiceProtocol"

func init() {
	sda.ProtocolRegisterName(MedcoServiceProtocolName, NewPipelineProcotol)
	network.RegisterMessageType(TriggerFlushCollectedDataMessage{})
	network.RegisterMessageType(DoneFlushCollectedDataMessage{})
}

//ServiceInterface defines the 3 phases of a medco protocol. The service implements this interface so
// the protocol can trigger them.
type ServiceInterface interface {
	DeterministicSwitchingPhase(SurveyID) error
	AggregationPhase(SurveyID) error
	KeySwitchingPhase(SurveyID) error
}

//triggerFlushCollectedDataMessage is a message trigger the Map phase at all node
type TriggerFlushCollectedDataMessage struct {
	SurveyID SurveyID // Currently unused
}

//doneFlushCollectedDataMessage is a message reporting the Map phase completion
type DoneFlushCollectedDataMessage struct{}

//doneProcessingMessage is a message indicating that pipeline execution complete
type DoneProcessingMessage struct{}

type flushCollectedDataStruct struct {
	*sda.TreeNode
	TriggerFlushCollectedDataMessage
}

type doneFlushCollectedDataStruct struct {
	*sda.TreeNode
	DoneFlushCollectedDataMessage
}

//PipelineProtocol contains elements of a service protocol
type PipelineProtocol struct {
	*sda.TreeNodeInstance

	TriggerFlushCollectedData chan flushCollectedDataStruct
	DoneFlushCollectedData    chan []doneFlushCollectedDataStruct

	FeedbackChannel chan DoneProcessingMessage

	MedcoServiceInstance ServiceInterface
	TargetSurvey         *Survey
}

//NewPipelineProcotol constructor of a pipeline protocol
func NewPipelineProcotol(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol := &PipelineProtocol{TreeNodeInstance: tni,
		FeedbackChannel: make(chan DoneProcessingMessage)}

	chans := []interface{}{&protocol.TriggerFlushCollectedData, &protocol.DoneFlushCollectedData}
	for _, c := range chans {
		if err := protocol.RegisterChannel(c); err != nil {
			return nil, errors.New("couldn't register data reference channel: " + err.Error())
		}
	}
	return protocol, nil
}

// Start is called at the root the start the execution of the protocol
func (p *PipelineProtocol) Start() error {

	if p.MedcoServiceInstance == nil {
		return errors.New("No Medco Service pointer provided.")
	}
	if p.TargetSurvey == nil {
		return errors.New("No Target Survey ID pointer provided")
	}

	log.Lvl1(p.ServerIdentity(), "started a Medco Service Protocol.")
	p.Broadcast(&TriggerFlushCollectedDataMessage{p.TargetSurvey.ID})

	return nil
}

// Dispatch is called at all node and handle the incoming messages
func (p *PipelineProtocol) Dispatch() error {

	// 1st phase (optional) : Grouping
	if p.TargetSurvey.SurveyDescription.GroupingAttributesCount > 0 {
		if p.IsRoot() {
			p.MedcoServiceInstance.DeterministicSwitchingPhase(p.TargetSurvey.ID)
			<-p.DoneFlushCollectedData
		} else {
			msg := <-p.TriggerFlushCollectedData
			p.MedcoServiceInstance.DeterministicSwitchingPhase(msg.SurveyID)
			if !p.IsLeaf() {
				<-p.DoneFlushCollectedData
			}
			p.SendToParent(&DoneFlushCollectedDataMessage{})
		}
	}

	// 2nd phase: Aggregating
	if p.IsRoot() {
		p.MedcoServiceInstance.AggregationPhase(p.TargetSurvey.ID)
	}

	// 4rd phase: Key Switching
	if p.IsRoot() {
		p.MedcoServiceInstance.KeySwitchingPhase(p.TargetSurvey.ID)
		p.FeedbackChannel <- DoneProcessingMessage{}
	}

	return nil
}
