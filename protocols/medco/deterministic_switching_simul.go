package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
)

const deterministicSwitchedAttributesCount = 3

const deterministicSwitchedVectorCount = 2

func init() {
	sda.SimulationRegister("DeterministicSwitching", NewDeterministicSwitchingSimulation)
	sda.ProtocolRegisterName("DeterministicSwitchingSimul", NewDeterministicSwitchingSimul)

}

//DeterministicSwitchingSimulation contains a simulation tree
type DeterministicSwitchingSimulation struct {
	sda.SimulationBFTree
}

//NewDeterministicSwitchingSimulation constructor for the simulation
func NewDeterministicSwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &DeterministicSwitchingSimulation{}
	_, err := toml.Decode(config, sim)

	if err != nil {
		return nil, err
	}
	return sim, nil
}

//Setup initializes a simulation by creating the servers tree
func (sim *DeterministicSwitchingSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateRoster(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}
	log.Lvl1("Setup done")

	return sc, nil
}

//Run starts the simulation
func (sim *DeterministicSwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA(config.Tree, "DeterministicSwitchingSimul")
		if err != nil {
			return err
		}

		root := rooti.(*DeterministicSwitchingProtocol)

		//complete protocol time measurement
		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")
		root.StartProtocol()
		<-root.ProtocolInstance().(*DeterministicSwitchingProtocol).FeedbackChannel
		round.Record()
	}

	return nil
}

//NewDeterministicSwitchingSimul default simulation constructor used by all nodes to init their params
func NewDeterministicSwitchingSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewDeterministSwitchingProtocol(tni)
	pap := protocol.(*DeterministicSwitchingProtocol)

	if tni.Index() == 0 { //root
		aggregateKey := pap.Roster().Aggregate

		//create dummy data
		ciphertexts := make(map[libmedco.TempID]libmedco.CipherVector)
		var tab []int64
		for i := 0; i < deterministicSwitchedAttributesCount; i++ {
			if i == 0 {
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < deterministicSwitchedVectorCount; i++ {
			ciphertexts[libmedco.TempID(i)] = *libmedco.EncryptIntVector(aggregateKey, tab)
		}

		pap.TargetOfSwitch = &ciphertexts

		//TODO: put it in its own simulation
		//local aggregation time measurement, quick way to measure aggregation time**
		//ciphertext := *EncryptIntArray(suite, aggregateKey, tab)
		//ciphertexts[0] = ciphertext
		//dbg.LLvl1("local aggr")
		//round := monitor.NewTimeMeasure("MEDCO_LOCAGGR")
		//for i := 0; i < NUM_VECT_DET; i++ {
		//	ciphertext.AddNoReplace(ciphertext,ciphertext)
		//}
		//round.Record()
		//**************************************************************************
	}
	tempKey := network.Suite.Scalar().Pick(random.Stream)
	pap.SurveyPHKey = &tempKey

	return pap, err
}
