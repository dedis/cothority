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

// DeterministicSwitchingSimulation is the structure holding the state of the simulation.
type DeterministicSwitchingSimulation struct {
	sda.SimulationBFTree
}

// NewDeterministicSwitchingSimulation is a constructor for the simulation.
func NewDeterministicSwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &DeterministicSwitchingSimulation{}
	_, err := toml.Decode(config, sim)

	if err != nil {
		return nil, err
	}
	return sim, nil
}

// Setup initializes a simulation.
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

// Run starts the simulation.
func (sim *DeterministicSwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA("DeterministicSwitchingSimul", config.Tree)
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

// NewDeterministicSwitchingSimul is a custom protocol constructor specific for simulation purposes.
func NewDeterministicSwitchingSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewDeterministSwitchingProtocol(tni)
	pap := protocol.(*DeterministicSwitchingProtocol)

	if tni.IsRoot() {
		aggregateKey := pap.Roster().Aggregate

		// Creates dummy data...
		ciphertexts := make(map[libmedco.TempID]libmedco.CipherVector)
		tab := make([]int64, deterministicSwitchedAttributesCount)
		for i := 0; i < deterministicSwitchedAttributesCount; i++ {
			tab[i] = int64(1)
		}

		for i := 0; i < deterministicSwitchedVectorCount; i++ {
			ciphertexts[libmedco.TempID(i)] = *libmedco.EncryptIntVector(aggregateKey, tab)
		}
		pap.TargetOfSwitch = &ciphertexts
	}
	tempKey := network.Suite.Scalar().Pick(random.Stream)
	pap.SurveyPHKey = &tempKey

	return pap, err
}
