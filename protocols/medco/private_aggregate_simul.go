package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
)

var suite = network.Suite
var grpattr = libmedco.DeterministCipherText{suite.Point().Base()}
var clientPrivate = suite.Scalar().One() //one -> to have the same for each node
var clientPublic = suite.Point().Mul(suite.Point().Base(), clientPrivate)

func createDataSet(numberGroups int, numberAttributes int) (
	map[libmedco.GroupingKey]libmedco.GroupingAttributes, map[libmedco.GroupingKey]libmedco.CipherVector) {
	testGAMap := make(map[libmedco.GroupingKey]libmedco.GroupingAttributes)
	testCVMap := make(map[libmedco.GroupingKey]libmedco.CipherVector)

	for i := 0; i < numberGroups; i++ {
		newGrpattr := grpattr
		(libmedco.DeterministCipherText(newGrpattr).Point).Add(libmedco.DeterministCipherText(newGrpattr).Point,
									libmedco.DeterministCipherText(newGrpattr).Point)
		groupAttributes := libmedco.GroupingAttributes{grpattr, newGrpattr}

		grpattr = newGrpattr

		var tab []int64
		for i := 0; i < numberAttributes; i++ {
			if i == 0 {
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}
		//round := monitor.NewTimeMeasure("MEDCO_ENCRYPTION")
		cipherVect := *libmedco.EncryptIntVector(clientPublic, tab)
		//round.Record()

		testGAMap[groupAttributes.Key()] = groupAttributes
		testCVMap[groupAttributes.Key()] = cipherVect
	}
	return testGAMap, testCVMap
}

func init() {
	sda.SimulationRegister("PrivateAggregate", NewPrivateAggregateSimulation)
	sda.ProtocolRegisterName("PrivateAggregateSimul", NewAggregationProtocolSimul)
}

//PrivateAggregateSimulation contains simulation tree
type PrivateAggregateSimulation struct {
	sda.SimulationBFTree
}

//NewPrivateAggregateSimulation simultaion constructor
func NewPrivateAggregateSimulation(config string) (sda.Simulation, error) {
	sim := &PrivateAggregateSimulation{}
	_, err := toml.Decode(config, sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

//Setup initializes the servers tree
func (sim *PrivateAggregateSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateRoster(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}

	log.Lvl1("Setup done")

	return sc, nil

}

//Run starts the simulation of the protocol and measures its runtime
func (sim *PrivateAggregateSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA(config.Tree, "PrivateAggregateSimul")
		if err != nil {
			return err
		}

		root := rooti.(*PrivateAggregateProtocol)

		//time measurement
		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")

		root.StartProtocol()

		<-root.ProtocolInstance().(*PrivateAggregateProtocol).FeedbackChannel
		round.Record()
	}

	return nil
}

//NewAggregationProtocolSimul default constructor used by all nodes to init their parameters
func NewAggregationProtocolSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewPrivateAggregate(tni)
	pap := protocol.(*PrivateAggregateProtocol)

	groupMap := make(map[libmedco.GroupingKey]libmedco.GroupingAttributes)
	attribMap := make(map[libmedco.GroupingKey]libmedco.CipherVector)

	switch tni.Index() {
	//if want to study special cases********************************************************************************
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
	//**************************************************************************************************************
	default:
		groupMap, attribMap = createDataSet(10, 100)
	}
	pap.Groups = &groupMap
	pap.GroupedData = &attribMap

	return protocol, err
}
