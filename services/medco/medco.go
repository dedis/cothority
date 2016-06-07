package medco_service

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/services/medco/store"
	"github.com/satori/go.uuid"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/random"
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

	entityList *sda.EntityList
	tree *sda.Tree
	store *store.Survey
	surveyPHKey abstract.Secret

}

func NewMedcoService(c sda.Context, path string) sda.Service {
	newMedCoInstance := &MedcoService{
		ServiceProcessor:        sda.NewServiceProcessor(c),
		homePath:                path,
	}
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResponseData)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResultsQuery)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyCreationQuery)
	return newMedCoInstance
}

func (mcs *MedcoService) HandleSurveyCreationQuery(e *network.Entity, recq *SurveyCreationQuery) (network.ProtocolMessage, error) {
	// Future: should initialise a survey store
	mcs.entityList = &recq.EntityList
	mcs.store = store.NewSurvey()
	mcs.surveyPHKey = network.Suite.Secret().Pick(random.Stream)

	if mcs.Entity().Equal(mcs.entityList.List[0]) {
		msg, _ := sda.CreateServiceMessage(MEDCO_SERVICE_NAME, recq)
		// No easy way to get our TreeNode object from the Tree + cannot send ServiceMessage w/ SendToChildren: use SendRaw
		for _,e := range mcs.entityList.List {
			if !e.Equal(mcs.Context.Entity()) {
				mcs.SendRaw(e, msg)
			}
		}
		dbg.Lvl1(mcs.Entity()," initiated the survey as the root.")
	} else {
		dbg.Lvl1(mcs.Entity()," created the survey, root is : ",mcs.entityList.List[0])
	}

	return &ServiceResponse{int32(1)}, nil
}

func (mcs *MedcoService) HandleSurveyResponseData(e *network.Entity, resp *ClientResponse) (network.ProtocolMessage, error) {
	// Future: insert a new row in the CollectedData table of the survey store. Potentially trigger a flush in pipeline

	mcs.store.InsertClientResponse(resp)


	dbg.Lvl1(mcs.Entity(), "recieved survey response data from ", e)
	return &ServiceResponse{int32(1)}, nil
}



func (mcs *MedcoService) HandleSurveyResultsQuery(e *network.Entity, resq *SurveyResultsQuery) (network.ProtocolMessage, error) {
	// Future: flushes every tables in the pipeline order. Answers the request.


	dbg.Lvl1(mcs.Entity(), "recieved a survey result query from", e)
	mcs.tree = mcs.entityList.GenerateBinaryTree()








	// This should go in flushGroupedData ===>

	// <===

	// This should go in flush aggregated ===>

	// <===


	dbg.Lvl1(mcs.Entity(), "completed the query processing...")
	return &SurveyResultResponse{keySwitchedResult}, nil
}

func (mcs *MedcoService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	// Observation : which data we operate the protocol on is important only for aggreg and there is no ambiguity
	// for those data (we aggregate everything that is ready to be aggregated). For key switching, this is a
	// problem as we need to know from which key to which key we switch. The current best solution seems to be make
	// two versions of the key switching protocol because it also solves the interface unmarshalling problem.
	var pi sda.ProtocolInstance
	var err error
	if mcs.aggregateProtocol == nil {
		pi, err = medco.NewPrivateAggregate(tn)
		ref := medco.DataRef(mcs.localResult)
		pi.(*medco.PrivateAggregateProtocol).DataReference = &ref
		mcs.aggregateProtocol = pi.(*medco.PrivateAggregateProtocol)
	} else if mcs.keySwitchProtocol == nil {
		pi, err = medco.NewKeySwitchingProtocol(tn)
		//pi.(*medco.KeySwitchingProtocol).TargetOfSwitch = mcs.aggregatedResults
		mcs.keySwitchProtocol = pi.(*medco.KeySwitchingProtocol)
	} else {
		pi = nil
		err = errors.New("Recieved an unexpected NewProtocol event.")
	}

	if err != nil {
		dbg.Error(err)
	}
	go pi.Dispatch()
	return pi, err
}

// Pipeline steps forward operations

// Performs the private grouping on the currently collected data
func (mcs *MedcoService) flushCollectedData() error {

	var probabilisticGroupingAttributes *map[uuid.UUID]medco.CipherVector

	probabilisticGroupingAttributes = mcs.store.PollProbabilisticGroupingAttributes()

	tni := mcs.NewTreeNodeInstance(mcs.tree, mcs.tree.Root)
	pi, err := medco.NewDeterministSwitchingProtocol(tni)
	if err != nil {
		return nil, errors.New("Could not instanciate the required protocols")
	}
	mcs.RegisterProtocolInstance(pi)
	protocol := pi.(*medco.DeterministSwitchingProtocol)
	protocol.TargetOfSwitch = probabilisticGroupingAttributes
	protocol.SurveyPHKey = &mcs.surveyPHKey
	go protocol.Dispatch()
	go protocol.Start()

	deterministicSwitchedResult := <- protocol.FeedbackChannel

	mcs.store.PushDeterministicGroupingAttributes(deterministicSwitchedResult)

	return nil
}

// Performs the per-group aggregation on the currently grouped data
func (mcs *MedcoService) flushGroupedData() error {

	var groupedData *map[store.GroupingAttributes]medco.CipherVector

	groupedData = mcs.store.PollLocallyAggregatedResponses()
	treeNodeInst := mcs.NewTreeNodeInstance(mcs.tree, mcs.tree.Root)
	pi,err := medco.NewPrivateAggregate(treeNodeInst)
	if err != nil {
		return nil, errors.New("Could not instanciate the required protocols")
	}
	mcs.RegisterProtocolInstance(pi)
	aggregateProtocol := pi.(*medco.PrivateAggregateProtocol)
	aggregateProtocol.DataReference = groupedData
	go aggregateProtocol.Dispatch()
	go aggregateProtocol.Start()
	cothorityAggregatedData := <- aggregateProtocol.FeedbackChannel

	mcs.store.PushCothorityAggregatedGroups(cothorityAggregatedData)

	return nil
}

// Perform the switch to data querier key on the currently aggregated data
func (mcs *MedcoService) flushAggregatedData(querierKey abstract.Point) error {

	var aggregatedGroups *map[uuid.UUID]store.GroupingAttributes
	var aggregatedAttributes *map[uuid.UUID]medco.CipherVector

	aggregatedGroups, aggregatedAttributes = mcs.store.PollCothorityAggregatedGroups()

	treeNodeIKeySwitch := mcs.NewTreeNodeInstance(mcs.tree, mcs.tree.Root)
	piKeySwitch, err := medco.NewKeySwitchingProtocol(treeNodeIKeySwitch)
	if err != nil {
		return nil, errors.New("Could not instanciate the required protocols")
	}
	mcs.RegisterProtocolInstance(piKeySwitch)
	keySwitchProtocol := piKeySwitch.(*medco.KeySwitchingProtocol)
	keySwitchProtocol.TargetOfSwitch = aggregatedAttributes
	keySwitchProtocol.TargetPublicKey = querierKey
	go keySwitchProtocol.Dispatch()
	go keySwitchProtocol.Start()
	keySwitchedAggregatedAttributes := <- keySwitchProtocol.FeedbackChannel


	treeNodeISchemeSwitch := mcs.NewTreeNodeInstance(mcs.tree, mcs.tree.Root)
	piProbSwitch, err2 := medco.NewProbabilisticSwitchingProtocol(treeNodeISchemeSwitch)
	if err2 != nil {
		return nil, errors.New("Could not instanciate the required protocols")
	}
	mcs.RegisterProtocolInstance(piProbSwitch)
	probabilisticSwitchProtocol := piProbSwitch.(*medco.KeySwitchingProtocol)
	probabilisticSwitchProtocol.TargetOfSwitch = aggregatedGroups
	probabilisticSwitchProtocol.TargetPublicKey = querierKey
	go probabilisticSwitchProtocol.Dispatch()
	go probabilisticSwitchProtocol.Start()
	keySwitchedAggregatedGroups := <- probabilisticSwitchProtocol.FeedbackChannel

	mcs.store.PushQuerierKeyEncryptedData(keySwitchedAggregatedGroups, keySwitchedAggregatedAttributes)

	return nil
}