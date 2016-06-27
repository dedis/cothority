package medco

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	. "github.com/dedis/cothority/services/medco/libmedco"
)

const MEDCO_SERVICE_PROTOCOL_NAME = "MedcoServiceProtocol"

func init() {
	sda.ProtocolRegisterName(MEDCO_SERVICE_PROTOCOL_NAME, NewMedcoServiceProcotol)
	network.RegisterMessageType(TriggerFlushCollectedDataMessage{})
	network.RegisterMessageType(DoneFlushCollectedDataMessage{})
}

type MedcoServiceInterface interface {
	DeterministicSwitchingPhase(SurveyID) error
	AggregationPhase(SurveyID) error
	KeySwitchingPhase(SurveyID) error
}

type TriggerFlushCollectedDataMessage struct {
	SurveyID SurveyID
}
type DoneFlushCollectedDataMessage struct{}
type DoneProcessingMessage struct{}

type FlushCollectedDataStruct struct {
	*sda.TreeNode
	TriggerFlushCollectedDataMessage
}

type DoneFlushCollectedDataStruct struct {
	*sda.TreeNode
	DoneFlushCollectedDataMessage
}

type MedcoServiceProtocol struct {
	*sda.TreeNodeInstance

	TriggerFlushCollectedData chan FlushCollectedDataStruct
	DoneFlushCollectedData    chan []DoneFlushCollectedDataStruct

	FeedbackChannel chan DoneProcessingMessage

	MedcoServiceInstance MedcoServiceInterface
	TargetSurvey         *Survey
}

func NewMedcoServiceProcotol(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol := &MedcoServiceProtocol{TreeNodeInstance: tni,
		FeedbackChannel: make(chan DoneProcessingMessage)}

	chans := []interface{}{&protocol.TriggerFlushCollectedData, &protocol.DoneFlushCollectedData}
	for _, c := range chans {
		if err := protocol.RegisterChannel(c); err != nil {
			return nil, errors.New("couldn't register data reference channel: " + err.Error())
		}
	}
	return protocol, nil
}

func (p *MedcoServiceProtocol) Start() error {

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

func (p *MedcoServiceProtocol) Dispatch() error {

	// 1st phase (optional) : Grouping
	if p.TargetSurvey.SurveyDescription.GroupingAttributesCount > 0 {
		if p.IsRoot() {
			p.MedcoServiceInstance.DeterministicSwitchingPhase(p.TargetSurvey.ID)
			<-p.DoneFlushCollectedData
		} else {
			msg := <-p.TriggerFlushCollectedData
			p.MedcoServiceInstance.DeterministicSwitchingPhase(msg.SurveyID)
			p.SendTo(p.Root(), &DoneFlushCollectedDataMessage{})
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
