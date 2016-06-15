package medco_service

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/services/medco/store"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/abstract"
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

	surveys  map[string]Survey
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

	if recq.SurveyID == nil {
		newID := SurveyID(uuid.NewV4())
		recq.SurveyID = &newID
		msg, _ := sda.CreateServiceMessage(MEDCO_SERVICE_NAME, recq)
		mcs.SendISMOthers(recq.EntityList, msg)
		dbg.Lvl1(mcs.Entity(), "initiated the survey", newID)
	}

	mcs.surveys[*recq.SurveyID] =  Survey{
		SurveyStore: store.NewSurvey(),
		ID: *recq.SurveyID,
		EntityList: recq.EntityList,
		SurveyPHKey: network.Suite.Secret().Pick(random.Stream),
		ClientPublic: nil,
		SurveyDescription: recq.SurveyDescription,
	}

	dbg.Lvl1(mcs.Entity(), "created the survey", *recq.SurveyID)

	return &ServiceResponse{int32(1)}, nil
}

func (mcs *MedcoService) HandleSurveyResponseData(e *network.Entity, resp *ClientResponse) (network.ProtocolMessage, error) {

	mcs.store.InsertClientResponse(*resp)

	dbg.Lvl1(mcs.Entity(), "recieved survey response data from ", e)
	return &ServiceResponse{int32(1)}, nil
}

func (mcs *MedcoService) HandleSurveyResultsQuery(e *network.Entity, resq *SurveyResultsQuery) (network.ProtocolMessage, error) {

	dbg.Lvl1(mcs.Entity(), "recieved a survey result query from", e)

	mcs.clientPublic = resq.ClientPublic
	pi,_ := mcs.startProtocol(medco.MEDCO_SERVICE_PROTOCOL_NAME)

	<- pi.(*medco.MedcoServiceProtocol).FeedbackChannel
	dbg.Lvl1(mcs.Entity(), "completed the query processing...")
	return &SurveyResultResponse{mcs.store.PollDeliverableResults()}, nil
}

func (mcs *MedcoService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	// Observation : which data we operate the protocol on is important only for aggreg and there is no ambiguity
	// for those data (we aggregate everything that is ready to be aggregated). For key switching, this is a
	// problem as we need to know from which key to which key we switch. The current best solution seems to be make
	// two versions of the key switching protocol because it also solves the interface unmarshalling problem.
	var pi sda.ProtocolInstance
	var err error

	switch tn.ProtocolName() {
	case medco.MEDCO_SERVICE_PROTOCOL_NAME:
		pi, err = medco.NewMedcoServiceProcotol(tn)
		pi.(*medco.MedcoServiceProtocol).MedcoServiceInstance = mcs
	case medco.DETERMINISTIC_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewDeterministSwitchingProtocol(tn)
		detSwitch := pi.(*medco.DeterministicSwitchingProtocol)
		detSwitch.SurveyPHKey = &mcs.surveyPHKey
		if tn.IsRoot() {
			detSwitch.TargetOfSwitch = mcs.store.PollProbabilisticGroupingAttributes()
		}
	case medco.PRIVATE_AGGREGATE_PROTOCOL_NAME:
		pi, err = medco.NewPrivateAggregate(tn)
		groups, groupedData := mcs.store.PollLocallyAggregatedResponses()
		pi.(*medco.PrivateAggregateProtocol).GroupedData = groupedData
		pi.(*medco.PrivateAggregateProtocol).Groups = groups
	case medco.PROBABILISTIC_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewProbabilisticSwitchingProtocol(tn)
		probSwitch := pi.(*medco.ProbabilisticSwitchingProtocol)
		probSwitch.SurveyPHKey = &mcs.surveyPHKey
		if tn.IsRoot() {
			groups,_ := mcs.store.PollCothorityAggregatedGroups()
			probSwitch.TargetOfSwitch  = GroupingAttributesToDeterministicCipherVector(groups)
			probSwitch.TargetPublicKey = &mcs.clientPublic
		}
	case medco.KEY_SWITCHING_PROTOCOL_NAME:
		pi, err = medco.NewKeySwitchingProtocol(tn)
		keySwitch := pi.(*medco.KeySwitchingProtocol)
		if tn.IsRoot() {
			_,keySwitch.TargetOfSwitch = mcs.store.PollCothorityAggregatedGroups()
			keySwitch.TargetPublicKey = &mcs.clientPublic
		}
	default:
		return nil, errors.New("Service attempts to start an unknown protocol: " + tn.ProtocolName() + ".")
	}
	return pi, err
}

func (mcs *MedcoService) startProtocol(name string) (sda.ProtocolInstance, error) {
	tree := mcs.entityList.GenerateNaryTreeWithRoot(2, mcs.Entity())
	tni := mcs.NewTreeNodeInstance(tree, tree.Root, name)
	pi , err := mcs.NewProtocol(tni, nil)
	mcs.RegisterProtocolInstance(pi)
	go pi.Dispatch()
	go pi.Start()
	return pi, err
}

// Pipeline steps forward operations

// Performs the private grouping on the currently collected data
func (mcs *MedcoService) FlushCollectedData() error {

	// TODO: Start only if data
	pi, err := mcs.startProtocol(medco.DETERMINISTIC_SWITCHING_PROTOCOL_NAME)
	if err != nil {
		return err
	}
	deterministicSwitchedResult := <-pi.(*medco.DeterministicSwitchingProtocol).FeedbackChannel

	mcs.store.PushDeterministicGroupingAttributes(*DeterministicCipherVectorToGroupingAttributes(&deterministicSwitchedResult))
	return err
}

// Performs the per-group aggregation on the currently grouped data
func (mcs *MedcoService) FlushGroupedData() error {

	pi, err := mcs.startProtocol(medco.PRIVATE_AGGREGATE_PROTOCOL_NAME)
	if err != nil {
		return err
	}
	cothorityAggregatedData := <-pi.(*medco.PrivateAggregateProtocol).FeedbackChannel

	mcs.store.PushCothorityAggregatedGroups(cothorityAggregatedData.Groups, cothorityAggregatedData.GroupedData)

	return err
}

// Perform the switch to data querier key on the currently aggregated data
func (mcs *MedcoService) FlushAggregatedData() error {

	pi, err := mcs.startProtocol(medco.KEY_SWITCHING_PROTOCOL_NAME)
	if err != nil {
		return err
	}
	keySwitchedAggregatedAttributes := <-pi.(*medco.KeySwitchingProtocol).FeedbackChannel


	pi, err = mcs.startProtocol(medco.PROBABILISTIC_SWITCHING_PROTOCOL_NAME)
	if err != nil {
		return err
	}
	keySwitchedAggregatedGroups := <-pi.(*medco.ProbabilisticSwitchingProtocol).FeedbackChannel

	mcs.store.PushQuerierKeyEncryptedData(keySwitchedAggregatedGroups, keySwitchedAggregatedAttributes)

	return err
}
