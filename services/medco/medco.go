package medco_service

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/random"
	"github.com/satori/go.uuid"
)

const MEDCO_SERVICE_NAME = "MedCo"

func init() {
	sda.RegisterNewService(MEDCO_SERVICE_NAME, NewMedcoService)
	network.RegisterMessageType(&ClientResponse{})
	network.RegisterMessageType(&SurveyResultsQuery{})
	network.RegisterMessageType(&SurveyCreationQuery{})
	network.RegisterMessageType(&SurveyResultResponse{})
	network.RegisterMessageType(&ServiceResponse{})
}

type MedcoService struct {
	*sda.ServiceProcessor
	homePath string

	survey  Survey
	//currentSurveyID SurveyID
}

func NewMedcoService(c sda.Context, path string) sda.Service {
	newMedCoInstance := &MedcoService{
		ServiceProcessor: sda.NewServiceProcessor(c),
		homePath:         path,
	}
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResponseData)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResultsQuery)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyCreationQuery)
	return newMedCoInstance
}

func (mcs *MedcoService) HandleSurveyCreationQuery(e *network.Entity, recq *SurveyCreationQuery) (network.ProtocolMessage, error) {

	dbg.Lvl1(mcs.Entity(), "received a Survey Creation Query")
	if recq.SurveyID == nil {
		newID := SurveyID(uuid.NewV4().String())
		recq.SurveyID = &newID
		msg, _ := sda.CreateServiceMessage(MEDCO_SERVICE_NAME, recq)
		//mcs.SendISMOthers(&recq.EntityList, msg)
		for _, e := range recq.EntityList.List {
			if !e.Equal(mcs.Entity()) {
				mcs.SendRaw(e, msg)
			}
		}
		dbg.Lvl1(mcs.Entity(), "initiated the survey", newID)
	}

	mcs.survey =  Survey{
		SurveyStore: NewSurveyStore(),
		ID: *recq.SurveyID,
		EntityList: recq.EntityList,
		SurveyPHKey: network.Suite.Secret().Pick(random.Stream),
		ClientPublic: nil,
		SurveyDescription: recq.SurveyDescription,
	}
	dbg.Lvl1(mcs.Entity(), "created the survey", *recq.SurveyID)

	return &ServiceResponse{*recq.SurveyID}, nil
}

func (mcs *MedcoService) HandleSurveyResponseData(e *network.Entity, resp *SurveyResponseQuery) (network.ProtocolMessage, error) {
	dbg.Lvl1(mcs.Entity(), "recieved response data for survey ",resp.SurveyID)
	if mcs.survey.ID == resp.SurveyID {
		mcs.survey.InsertClientResponse(resp.ClientResponse)
		return &ServiceResponse{"1"}, nil
	}
	dbg.Lvl1(mcs.Entity(),"does not know about this survey!")
	return &ServiceResponse{"2"}, nil
}

func (mcs *MedcoService) HandleSurveyResultsQuery(e *network.Entity, resq *SurveyResultsQuery) (network.ProtocolMessage, error) {

	dbg.Lvl1(mcs.Entity(), "recieved a survey result query from", e)
	mcs.survey.ClientPublic = resq.ClientPublic
	pi,_ := mcs.startProtocol(medco.MEDCO_SERVICE_PROTOCOL_NAME, resq.SurveyID)

	<- pi.(*medco.MedcoServiceProtocol).FeedbackChannel
	dbg.Lvl1(mcs.Entity(), "completed the query processing...")
	return &SurveyResultResponse{mcs.survey.PollDeliverableResults()}, nil
}

func (mcs *MedcoService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {

	var pi sda.ProtocolInstance
	var err error
	switch tn.ProtocolName() {
	case medco.MEDCO_SERVICE_PROTOCOL_NAME:
		pi, err = medco.NewMedcoServiceProcotol(tn)
		medcoServ := pi.(*medco.MedcoServiceProtocol)
		medcoServ.MedcoServiceInstance = mcs
		medcoServ.TargetSurvey = &mcs.survey
	case medco.DETERMINISTIC_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewDeterministSwitchingProtocol(tn)
		detSwitch := pi.(*medco.DeterministicSwitchingProtocol)
		detSwitch.SurveyPHKey = &mcs.survey.SurveyPHKey
		if tn.IsRoot() {
			groupingAttr := mcs.survey.PollProbabilisticGroupingAttributes()
			detSwitch.TargetOfSwitch = &groupingAttr
		}
	case medco.PRIVATE_AGGREGATE_PROTOCOL_NAME:
		pi, err = medco.NewPrivateAggregate(tn)
		groups, groupedData := mcs.survey.PollLocallyAggregatedResponses()
		pi.(*medco.PrivateAggregateProtocol).GroupedData = &groupedData
		pi.(*medco.PrivateAggregateProtocol).Groups = &groups
	case medco.PROBABILISTIC_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewProbabilisticSwitchingProtocol(tn)
		probSwitch := pi.(*medco.ProbabilisticSwitchingProtocol)
		probSwitch.SurveyPHKey = &mcs.survey.SurveyPHKey
		if tn.IsRoot() {
			groups := mcs.survey.PollCothorityAggregatedGroupsId()
			probSwitch.TargetOfSwitch  = GroupingAttributesToDeterministicCipherVector(&groups)
			probSwitch.TargetPublicKey = &mcs.survey.ClientPublic
		}
	case medco.KEY_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewKeySwitchingProtocol(tn)
		keySwitch := pi.(*medco.KeySwitchingProtocol)
		if tn.IsRoot() {
			coaggr := mcs.survey.PollCothorityAggregatedGroupsAttr()
			keySwitch.TargetOfSwitch = &coaggr
			keySwitch.TargetPublicKey = &mcs.survey.ClientPublic
		}
	default:
		return nil, errors.New("Service attempts to start an unknown protocol: " + tn.ProtocolName() + ".")
	}
	return pi, err
}

func (mcs *MedcoService) startProtocol(name string, targetSurvey SurveyID) (sda.ProtocolInstance, error) {
	//dbg.Printf("%#v",survey)
	tree := mcs.survey.EntityList.GenerateNaryTreeWithRoot(2, mcs.Entity())
	tni := mcs.NewTreeNodeInstance(tree, tree.Root, name)
	pi , err := mcs.NewProtocol(tni, nil)
	mcs.RegisterProtocolInstance(pi)
	go pi.Dispatch()
	go pi.Start()
	return pi, err
}

// Pipeline steps forward operations

// Performs the private grouping on the currently collected data
func (mcs *MedcoService) DeterministicSwitchingPhase(targetSurvey SurveyID) error {

	pi, err := mcs.startProtocol(medco.DETERMINISTIC_SWITCHING_PROTOCOL_NAME, targetSurvey)
	if err != nil {
		return err
	}
	deterministicSwitchedResult := <-pi.(*medco.DeterministicSwitchingProtocol).FeedbackChannel
	mcs.survey.PushDeterministicGroupingAttributes(*DeterministicCipherVectorToGroupingAttributes(&deterministicSwitchedResult))
	return err
}

// Performs the per-group aggregation on the currently grouped data
func (mcs *MedcoService) AggregationPhase(targetSurvey SurveyID) error {

	pi, err := mcs.startProtocol(medco.PRIVATE_AGGREGATE_PROTOCOL_NAME, targetSurvey)
	if err != nil {
		return err
	}
	cothorityAggregatedData := <-pi.(*medco.PrivateAggregateProtocol).FeedbackChannel

	mcs.survey.PushCothorityAggregatedGroups(cothorityAggregatedData.Groups, cothorityAggregatedData.GroupedData)

	return err
}

// Perform the switch to data querier key on the currently aggregated data
func (mcs *MedcoService) KeySwitchingPhase(targetSurvey SurveyID) error {

	pi, err := mcs.startProtocol(medco.KEY_SWITCHING_PROTOCOL_NAME, targetSurvey)
	if err != nil {
		return err
	}
	keySwitchedAggregatedAttributes := <-pi.(*medco.KeySwitchingProtocol).FeedbackChannel

	//TODO: extract this subphase because it is optional
	keySwitchedAggregatedGroups := make(map[TempID]CipherVector)
	if mcs.survey.SurveyDescription.GroupingAttributesCount > 0 {
		pi, err = mcs.startProtocol(medco.PROBABILISTIC_SWITCHING_PROTOCOL_NAME, targetSurvey)
		if err != nil {
			return err
		}
		keySwitchedAggregatedGroups = <-pi.(*medco.ProbabilisticSwitchingProtocol).FeedbackChannel
	}

	mcs.survey.PushQuerierKeyEncryptedData(keySwitchedAggregatedGroups, keySwitchedAggregatedAttributes)

	return err
}
