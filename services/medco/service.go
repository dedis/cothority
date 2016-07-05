package medco

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"github.com/satori/go.uuid"
)

// ServiceName is the registered name for the medco service.
const ServiceName = "MedCo"

func init() {
	sda.RegisterNewService(ServiceName, NewService)
	network.RegisterMessageType(&libmedco.ClientResponse{})
	network.RegisterMessageType(&SurveyResultsQuery{})
	network.RegisterMessageType(&SurveyCreationQuery{})
	network.RegisterMessageType(&SurveyResultResponse{})
	network.RegisterMessageType(&ServiceResponse{})
}

// SurveyCreationQuery is used to trigger the creation of a survey.
type SurveyCreationQuery struct {
	SurveyID *libmedco.SurveyID
	sda.Roster
	libmedco.SurveyDescription
}

// SurveyResponseQuery is used to ask a client for its response to a survey.
type SurveyResponseQuery struct {
	SurveyID libmedco.SurveyID
	libmedco.ClientResponse
}

// SurveyResultsQuery is used by querier to ask for the response of the survey.
type SurveyResultsQuery struct {
	SurveyID     libmedco.SurveyID
	ClientPublic abstract.Point
}

// ServiceResponse represents the service "state".
type ServiceResponse struct {
	SurveyID libmedco.SurveyID
}

// SurveyResultResponse will contain final results of a survey and be sent to querier.
type SurveyResultResponse struct {
	Results []libmedco.SurveyResult
}

// Service defines a service in medco case with a survey.
type Service struct {
	*sda.ServiceProcessor
	homePath string

	survey libmedco.Survey // For now, server only handles one survey.
}

// NewService constructor which registers the needed messages.
func NewService(c *sda.Context, path string) sda.Service {
	newMedCoInstance := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		homePath:         path,
	}
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResponseData)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResultsQuery)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyCreationQuery)
	return newMedCoInstance
}

// Queries Handlers definitions

// HandleSurveyCreationQuery handles the reception of a survey creation query by instantiating the corresponding survey.
func (mcs *Service) HandleSurveyCreationQuery(e *network.ServerIdentity, recq *SurveyCreationQuery) (network.Body, error) {
	log.Lvl1(mcs.ServerIdentity(), "received a Survey Creation Query")
	if recq.SurveyID == nil {
		newID := libmedco.SurveyID(uuid.NewV4().String())
		recq.SurveyID = &newID

		mcs.SendISMOthers(&recq.Roster, recq)

		log.Lvl1(mcs.ServerIdentity(), "initiated the survey", newID)
	}

	mcs.survey = libmedco.Survey{
		SurveyStore:       libmedco.NewSurveyStore(),
		ID:                *recq.SurveyID,
		Roster:            recq.Roster,
		SurveyPHKey:       network.Suite.Scalar().Pick(random.Stream),
		ClientPublic:      nil,
		SurveyDescription: recq.SurveyDescription,
	}
	log.Lvl1(mcs.ServerIdentity(), "created the survey", *recq.SurveyID)

	return &ServiceResponse{*recq.SurveyID}, nil
}

// HandleSurveyResponseData handles a survey answers submission by a subject.
func (mcs *Service) HandleSurveyResponseData(e *network.ServerIdentity, resp *SurveyResponseQuery) (network.Body, error) {
	log.Lvl1(mcs.ServerIdentity(), "recieved response data for survey ", resp.SurveyID)
	if mcs.survey.ID == resp.SurveyID {
		mcs.survey.InsertClientResponse(resp.ClientResponse)
		return &ServiceResponse{"1"}, nil
	}
	log.Lvl1(mcs.ServerIdentity(), "does not know about this survey!")
	return &ServiceResponse{resp.SurveyID}, nil
}

// HandleSurveyResultsQuery handles the survey result query by the surveyor.
func (mcs *Service) HandleSurveyResultsQuery(e *network.ServerIdentity, resq *SurveyResultsQuery) (network.Body, error) {

	log.Lvl1(mcs.ServerIdentity(), "recieved a survey result query from", e)
	mcs.survey.ClientPublic = resq.ClientPublic
	pi, _ := mcs.startProtocol(medco.MedcoServiceProtocolName, resq.SurveyID)

	<-pi.(*medco.PipelineProtocol).FeedbackChannel
	log.Lvl1(mcs.ServerIdentity(), "completed the query processing...")
	return &SurveyResultResponse{mcs.survey.PollDeliverableResults()}, nil
}

// NewProtocol handles the creation of the right protocol parameters.
func (mcs *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {

	var pi sda.ProtocolInstance
	var err error
	switch tn.ProtocolName() {
	case medco.MedcoServiceProtocolName:
		pi, err = medco.NewPipelineProcotol(tn)
		medcoServ := pi.(*medco.PipelineProtocol)
		medcoServ.MedcoServiceInstance = mcs
		medcoServ.TargetSurvey = &mcs.survey
	case medco.DeterministicSwitchingProtocolName:
		pi, err = medco.NewDeterministSwitchingProtocol(tn)
		detSwitch := pi.(*medco.DeterministicSwitchingProtocol)
		detSwitch.SurveyPHKey = &mcs.survey.SurveyPHKey
		if tn.IsRoot() {
			groupingAttr := mcs.survey.PollProbabilisticGroupingAttributes()
			detSwitch.TargetOfSwitch = &groupingAttr
		}
	case medco.PrivateAggregateProtocolName:
		pi, err = medco.NewPrivateAggregate(tn)
		groups, groupedData := mcs.survey.PollLocallyAggregatedResponses()
		pi.(*medco.PrivateAggregateProtocol).GroupedData = &groupedData
		pi.(*medco.PrivateAggregateProtocol).Groups = &groups
	case medco.ProbabilisticSwitchingProtocolName:
		pi, err = medco.NewProbabilisticSwitchingProtocol(tn)
		probSwitch := pi.(*medco.ProbabilisticSwitchingProtocol)
		probSwitch.SurveyPHKey = &mcs.survey.SurveyPHKey
		if tn.IsRoot() {
			groups := mcs.survey.PollCothorityAggregatedGroupsID()
			probSwitch.TargetOfSwitch = libmedco.GroupingAttributesToDeterministicCipherVector(&groups)
			probSwitch.TargetPublicKey = &mcs.survey.ClientPublic
		}
	case medco.KeySwitchingProtocolName:
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

func (mcs *Service) startProtocol(name string, targetSurvey libmedco.SurveyID) (sda.ProtocolInstance, error) {
	tree := mcs.survey.Roster.GenerateNaryTreeWithRoot(2, mcs.ServerIdentity())
	tni := mcs.NewTreeNodeInstance(tree, tree.Root, name)
	pi, err := mcs.NewProtocol(tni, nil)
	mcs.RegisterProtocolInstance(pi)
	go pi.Dispatch()
	go pi.Start()
	return pi, err
}

// Pipeline steps forward operations

// DeterministicSwitchingPhase performs the private grouping on the currently collected data.
func (mcs *Service) DeterministicSwitchingPhase(targetSurvey libmedco.SurveyID) error {

	pi, err := mcs.startProtocol(medco.DeterministicSwitchingProtocolName, targetSurvey)
	if err != nil {
		return err
	}
	deterministicSwitchedResult := <-pi.(*medco.DeterministicSwitchingProtocol).FeedbackChannel
	mcs.survey.PushDeterministicGroupingAttributes(*libmedco.DeterministicCipherVectorToGroupingAttributes(&deterministicSwitchedResult))
	return err
}

// AggregationPhase performs the per-group aggregation on the currently grouped data.
func (mcs *Service) AggregationPhase(targetSurvey libmedco.SurveyID) error {

	pi, err := mcs.startProtocol(medco.PrivateAggregateProtocolName, targetSurvey)
	if err != nil {
		return err
	}
	cothorityAggregatedData := <-pi.(*medco.PrivateAggregateProtocol).FeedbackChannel

	mcs.survey.PushCothorityAggregatedGroups(cothorityAggregatedData.Groups, cothorityAggregatedData.GroupedData)

	return err
}

// KeySwitchingPhase performs the switch to data querier key on the currently aggregated data.
func (mcs *Service) KeySwitchingPhase(targetSurvey libmedco.SurveyID) error {

	pi, err := mcs.startProtocol(medco.KeySwitchingProtocolName, targetSurvey)
	if err != nil {
		return err
	}
	keySwitchedAggregatedAttributes := <-pi.(*medco.KeySwitchingProtocol).FeedbackChannel

	//TODO: extract this subphase because it is optional
	keySwitchedAggregatedGroups := make(map[libmedco.TempID]libmedco.CipherVector)
	if mcs.survey.SurveyDescription.GroupingAttributesCount > 0 {
		pi, err = mcs.startProtocol(medco.ProbabilisticSwitchingProtocolName, targetSurvey)
		if err != nil {
			return err
		}
		keySwitchedAggregatedGroups = <-pi.(*medco.ProbabilisticSwitchingProtocol).FeedbackChannel
	}

	mcs.survey.PushQuerierKeyEncryptedData(keySwitchedAggregatedGroups, keySwitchedAggregatedAttributes)

	return err
}
