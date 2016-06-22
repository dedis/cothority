package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	. "github.com/dedis/cothority/services/medco/structs"
)


var grpattr = DeterministCipherText{suite.Point().Base()}
var clientPrivate = suite.Secret().One() //one -> to have the same for each node
var clientPublic = suite.Point().Mul(suite.Point().Base(), clientPrivate)

func CreateDataSet(numberGroups int, numberAttributes int) (map[GroupingKey]GroupingAttributes, map[GroupingKey]CipherVector){
	testGAMap := make(map[GroupingKey]GroupingAttributes)
	testCVMap := make(map[GroupingKey]CipherVector)

	for i := 0; i < numberGroups; i++ {
		newGrpattr := grpattr
		(DeterministCipherText(newGrpattr).Point).Add(DeterministCipherText(newGrpattr).Point,DeterministCipherText(newGrpattr).Point)
		groupAttributes := GroupingAttributes{grpattr, newGrpattr}

		grpattr = newGrpattr


		var tab []int64
		for i := 0; i < numberAttributes; i++{
			if i == 0{
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}
		cipherVect := *EncryptIntVector(clientPublic, tab)

		testGAMap[groupAttributes.Key()] = groupAttributes
		testCVMap[groupAttributes.Key()] = cipherVect
	}
	return testGAMap, testCVMap
}

func init() {
	sda.SimulationRegister("PrivateAggregate", NewPrivateAggregateSimulation)
	sda.ProtocolRegisterName("PrivateAggregateSimul", NewAggregationProtocolSimul)
}

type PrivateAggregateSimulation struct {
	sda.SimulationBFTree
}

func NewPrivateAggregateSimulation(config string) (sda.Simulation, error) {
	sim := &PrivateAggregateSimulation{}
	_,err := toml.Decode(config, sim)
	if err != nil {
		return nil,err
	}
	return sim, nil
}


func (sim *PrivateAggregateSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateEntityList(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}

	dbg.Lvl1("Begin simulation")

	return sc, nil

}


func (sim *PrivateAggregateSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA( config.Tree,"PrivateAggregateSimul")
		if err != nil {
			return err
		}


		root := rooti.(*PrivateAggregateProtocol)

		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")

		root.StartProtocol()

		result := <-root.ProtocolInstance().(*PrivateAggregateProtocol).FeedbackChannel
		round.Record()

		dbg.LLvl1("RESULT SIZE: ", len(result.GroupedData))
		for i,v := range result.GroupedData{
			dbg.Lvl1(i, " ", DecryptIntVector(clientPrivate, &v))
		}
	}

	return nil
}



func NewAggregationProtocolSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewPrivateAggregate(tni)
	pap := protocol.(*PrivateAggregateProtocol)

	groupMap := make(map[GroupingKey]GroupingAttributes)
	attribMap := make(map[GroupingKey]CipherVector)

	switch tni.Index() {
	/*case 0:
		// Generate test data
		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1, 2, 3, 4, 5})
		testGAMap[groupingAttrB.Key()] = groupingAttrB
		testCVMap[groupingAttrB.Key()] = *EncryptIntArray(suite, clientPublic, []int64{0, 0, 0, 0, 0})
	case 1:
		testGAMap[groupingAttrB.Key()] = groupingAttrB
		testCVMap[groupingAttrB.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1, 2, 3, 4, 5})
	case 2:
		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1, 1, 1, 1, 1})
	case 3:
		testGAMap[groupingAttrC.Key()] = groupingAttrC
		testCVMap[groupingAttrC.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1, 0, 1, 0, 1})
		testGAMap[groupingAttrA.Key()] = groupingAttrA
		testCVMap[groupingAttrA.Key()] = *EncryptIntArray(suite, clientPublic, []int64{1, 2, 3, 4, 5})
	case 4:
		testGAMap[groupingAttrC.Key()] = groupingAttrC
		testCVMap[groupingAttrC.Key()] = *EncryptIntArray(suite, clientPublic, []int64{0, 1, 0, 1, 0})*/
	default:
		groupMap, attribMap = CreateDataSet(10,100)
	}
	pap.Groups = &groupMap
	pap.GroupedData = &attribMap

	return protocol, err
}

