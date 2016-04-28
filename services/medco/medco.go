package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
)

const MEDCO_SERVICE_NAME = "MedCo"

func init() {
	sda.RegisterNewService(MEDCO_SERVICE_NAME, NewMedcoService)
	network.RegisterMessageType(&SurveyResponseData{})
	network.RegisterMessageType(&SurveyResultsQuery{})
	network.RegisterMessageType(&SurveyCreationQuery{})
	network.RegisterMessageType(&SurveyResultResponse{})
	network.RegisterMessageType(&ServiceResponse{})
}

type MedcoService struct {
	*sda.ServiceProcessor
	homePath string

	aggregateProtocol *medco.PrivateAggregateProtocol
	keySwitchProtocol *medco.KeySwitchingProtocol

	localResult *medco.CipherVector
	entityList *sda.EntityList
	aggregatedResults *medco.CipherVector
}

func NewMedcoService(c sda.Context, path string) sda.Service {
	newMedCoInstance := &MedcoService{
		ServiceProcessor: 	sda.NewServiceProcessor(c),
		homePath:		path,
	}

	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResponseData)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyResultsQuery)
	newMedCoInstance.RegisterMessage(newMedCoInstance.HandleSurveyCreationQuery)
	return newMedCoInstance
}

func (mcs *MedcoService) HandleSurveyCreationQuery(e *network.Entity, recq *SurveyCreationQuery) (network.ProtocolMessage, error) {

	mcs.entityList = &recq.EntityList
	if recq.EntityList.List[0].Equal(e) {
		tree := recq.EntityList.GenerateBinaryTree()
		treeNodeInst := mcs.NewTreeNodeInstance(tree, tree.Root)
		treeNodeInst.SendToChildren(recq)
		dbg.Lvl1(e," initiated the survey as the root.")
	} else {
		dbg.Lvl1(e," created the survey, root is : ",recq.EntityList.List[0])
	}
	return &ServiceResponse{int16(201), "Created"}, nil
}

func (mcs *MedcoService) HandleSurveyResponseData(e *network.Entity, resp *SurveyResponseData) (network.ProtocolMessage, error) {

	if mcs.localResult == nil {
		mcs.localResult = &resp.CipherVector
	} else {
		err := mcs.localResult.Add(*mcs.localResult, resp.CipherVector)
		if err != nil {
			dbg.Lvl1("Got error when aggregating survey response.")
			return 500, err
		}
	}
	return 200, nil
}

func (mcs *MedcoService) HandleSurveyResultsQuery(e *network.Entity, resq *SurveyResultsQuery) (network.ProtocolMessage, error) {

	tree := mcs.entityList.GenerateBinaryTree()
	treeNodeInst := mcs.NewTreeNodeInstance(tree, tree.Root)
	pi1,err1 := medco.NewPrivateAggregate(treeNodeInst)
	pi2, err2 := medco.NewKeySwitchingProtocol(treeNodeInst)
	if err1 != nil || err2 != nil{
		return nil, errors.New("Could not instanciate the required protocols")
	}
	mcs.RegisterProtocolInstance(pi1)
	mcs.RegisterProtocolInstance(pi2)
	mcs.aggregateProtocol = pi1.(*medco.PrivateAggregateProtocol)
	mcs.keySwitchProtocol = pi2.(*medco.KeySwitchingProtocol)

	ref := medco.DataRef(mcs.localResult)
	mcs.aggregateProtocol.DataReference = &ref
	go mcs.aggregateProtocol.Dispatch()
	go mcs.aggregateProtocol.Start()
	res := <- mcs.aggregateProtocol.FeedbackChannel
	mcs.aggregatedResults = &res

	mcs.keySwitchProtocol.TargetOfSwitch = mcs.aggregatedResults
	mcs.keySwitchProtocol.TargetPublicKey = &resq.ClientPublic
	go mcs.keySwitchProtocol.Dispatch()
	go mcs.keySwitchProtocol.Start()
	keySwitchedResult := <- mcs.keySwitchProtocol.FeedbackChannel

	return &SurveyResultResponse{keySwitchedResult}, nil
}

func (mcs *MedcoService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	var pi sda.ProtocolInstance
	var err error
	if mcs.aggregateProtocol == nil {
		pi, err = medco.NewPrivateAggregate(tn)
		ref := medco.DataRef(mcs.localResult)
		pi.(*medco.PrivateAggregateProtocol).DataReference = &ref
	} else if mcs.keySwitchProtocol == nil {
		pi, err = medco.NewKeySwitchingProtocol(tn)
		pi.(*medco.KeySwitchingProtocol).TargetOfSwitch = mcs.aggregatedResults
	} else {
		pi = nil
		err = errors.New("Recieved an unexpected NewProtocol event.")
	}
	return pi, err
}