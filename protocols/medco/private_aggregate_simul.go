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
var grpattr = libmedco.DeterministCipherText{Point: suite.Point().Base()}
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

		tab := make([]int64, numberAttributes)
		for i := 0; i < numberAttributes; i++ {
			tab[i] = int64(1)
		}
		cipherVect := *libmedco.EncryptIntVector(clientPublic, tab)

		testGAMap[groupAttributes.Key()] = groupAttributes
		testCVMap[groupAttributes.Key()] = cipherVect
	}
	return testGAMap, testCVMap
}

func init() {
	sda.SimulationRegister("PrivateAggregate", NewPrivateAggregateSimulation)
	sda.ProtocolRegisterName("PrivateAggregateSimul", NewAggregationProtocolSimul)
}

// PrivateAggregateSimulation holds the state of a simulation.
type PrivateAggregateSimulation struct {
	sda.SimulationBFTree
}

// NewPrivateAggregateSimulation is the simulation instance constructor.
func NewPrivateAggregateSimulation(config string) (sda.Simulation, error) {
	sim := &PrivateAggregateSimulation{}
	_, err := toml.Decode(config, sim)
	if err != nil {
		return nil, err
	}
	return sim, nil
}

// Setup initializes the simulation.
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

// Run starts the simulation of the protocol and measures its runtime.
func (sim *PrivateAggregateSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA("PrivateAggregateSimul", config.Tree)
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

// NewAggregationProtocolSimul is a simulation specific protocol instance constructor that injects test data.
func NewAggregationProtocolSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewPrivateAggregate(tni)
	pap := protocol.(*PrivateAggregateProtocol)

	groupMap := make(map[libmedco.GroupingKey]libmedco.GroupingAttributes)
	attribMap := make(map[libmedco.GroupingKey]libmedco.CipherVector)

	switch tni.Index() {
	// Put special cases you want to simulate here...
	default:
		groupMap, attribMap = createDataSet(10, 100)
	}
	pap.Groups = &groupMap
	pap.GroupedData = &attribMap

	return protocol, err
}
