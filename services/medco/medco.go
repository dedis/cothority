package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"reflect"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
)

const MEDCO_SERVICE_NAME = "MedCo"

func init() {
	sda.RegisterNewService(MEDCO_SERVICE_NAME, NewMedcoService)
	network.RegisterMessageType(SurveyResponseData{})
	network.RegisterMessageType(SurveyResultsQuery{})
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
	return newMedCoInstance
}

func (mcs *MedcoService) HandleSurveyCreationQuery(e *network.Entity, recq *SurveyCreationQuery) (network.ProtocolMessage, error) {
	mcs.entityList = &recq.EntityList
	if recq.EntityList.List[0].Equal(e) {
		tree := recq.EntityList.GenerateBinaryTree()
		treeNodeInst := mcs.NewTreeNodeInstance(tree, e)
		treeNodeInst.SendToChildren(recq)
		dbg.Lvl1(e.String()," initiated the survey.")
	} else {
		dbg.Lvl1(e.String()," created the survey.")
	}
	return 200, nil
}

func (mcs *MedcoService) HandleSurveyResponseData(e *network.Entity, resp *SurveyResponseData) (network.ProtocolMessage, error) {

	if mcs.localResult == nil {
		mcs.localResult = make(medco.CipherVector, len(resp))
		reflect.Copy(mcs.localResult, resp)
	} else {
		err := mcs.localResult.Add(mcs.localResult, resp)
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

	mcs.aggregateProtocol.DataReference = mcs.localResult
	go mcs.aggregateProtocol.Dispatch()
	go mcs.aggregateProtocol.Start()
	mcs.aggregatedResults = <- mcs.aggregateProtocol.FeedbackChannel

	mcs.keySwitchProtocol.TargetOfSwitch = &mcs.aggregatedResults
	mcs.keySwitchProtocol.TargetPublicKey = &resq.ClientPublic
	go mcs.keySwitchProtocol.Dispatch()
	go mcs.keySwitchProtocol.Start()
	keySwitchedResult := <- mcs.keySwitchProtocol.FeedbackChannel

	return &SurveyResultResponse{keySwitchedResult}, nil
}

func (mcs *MedcoService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	var pi *sda.ProtocolInstance
	var err error
	if mcs.aggregateProtocol == nil {
		pi, err = medco.NewPrivateAggregate(tn)
		pi.(*medco.PrivateAggregateProtocol).DataReference = mcs.localResult
	} else if mcs.keySwitchProtocol == nil {
		pi, err = medco.NewKeySwitchingProtocol(tn)
		pi.(*medco.KeySwitchingProtocol).TargetOfSwitch = mcs.aggregatedResults
	} else {
		pi = nil
		err = errors.New("Recieved an unexpected NewProtocol event.")
	}
	return pi, err
}