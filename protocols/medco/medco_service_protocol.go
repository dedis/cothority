package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/services/medco/structs"
)

const MEDCO_SERVICE_PROTOCOL_NAME = "MedcoServiceProtocol"

func init() {
	sda.ProtocolRegisterName(MEDCO_SERVICE_PROTOCOL_NAME, NewMedcoServiceProcotol)
	network.RegisterMessageType(TriggerFlushCollectedDataMessage{})
	network.RegisterMessageType(DoneFlushCollectedDataMessage{})
}

type MedcoServiceInterface interface {
	FlushCollectedData(medco_structs.SurveyID) error
	FlushGroupedData(medco_structs.SurveyID) error
	FlushAggregatedData(medco_structs.SurveyID) error
}

type TriggerFlushCollectedDataMessage struct {
	SurveyID medco_structs.SurveyID
}
type DoneFlushCollectedDataMessage struct {}
type DoneProcessingMessage struct {}

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
	DoneFlushCollectedData chan []DoneFlushCollectedDataStruct

	FeedbackChannel chan DoneProcessingMessage

	MedcoServiceInstance MedcoServiceInterface
	TargetSurvey *medco_structs.SurveyID
}

func NewMedcoServiceProcotol(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol := &MedcoServiceProtocol{TreeNodeInstance: tni,
		FeedbackChannel: make(chan DoneProcessingMessage)}

	chans := []interface{}{&protocol.TriggerFlushCollectedData, &protocol.DoneFlushCollectedData}
	for _,c := range chans {
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

	dbg.Lvl1(p.Entity(), "started a Medco Service Protocol.")
	p.Broadcast(&TriggerFlushCollectedDataMessage{*p.TargetSurvey})

	return nil
}

func (p *MedcoServiceProtocol) Dispatch() error {

	if !p.IsRoot() {
		msg := <- p.TriggerFlushCollectedData
		p.MedcoServiceInstance.FlushCollectedData(msg.SurveyID)
		p.SendTo(p.Root(), &DoneFlushCollectedDataMessage{})
	} else {
		p.MedcoServiceInstance.FlushCollectedData(*p.TargetSurvey)
		<- p.DoneFlushCollectedData
		p.MedcoServiceInstance.FlushGroupedData(*p.TargetSurvey)
		p.MedcoServiceInstance.FlushAggregatedData(*p.TargetSurvey)
		p.FeedbackChannel <- DoneProcessingMessage{}
	}

	return nil
}
