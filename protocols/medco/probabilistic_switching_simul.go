package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
)

const probabilisticSwitchedAttributeCount = 2

const probabilisticSwitchedVectorCount = 10

func init() {
	sda.SimulationRegister("ProbabilisticSwitching", NewProbabilisticSwitchingSimulation)
	sda.ProtocolRegisterName("ProbabilisticSwitchingSimul", NewProbabilisticSwitchingSimul)

}

//ProbabilisticSwitchingSimulation contains simulation tree
type ProbabilisticSwitchingSimulation struct {
	sda.SimulationBFTree
}

//NewProbabilisticSwitchingSimulation simulation constructor
func NewProbabilisticSwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &ProbabilisticSwitchingSimulation{}
	_, err := toml.Decode(config, sim)

	if err != nil {
		return nil, err
	}
	return sim, nil
}

//Setup initializes servers tree to do the simulation
func (sim *ProbabilisticSwitchingSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateRoster(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}

	log.Lvl1("Setup done")

	return sc, nil
}

//Run starts the protocol simulation
func (sim *ProbabilisticSwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA(config.Tree, "ProbabilisticSwitchingSimul")
		if err != nil {
			return err
		}

		root := rooti.(*ProbabilisticSwitchingProtocol)

		//runtime measurement
		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")
		root.StartProtocol()
		<-root.ProtocolInstance().(*ProbabilisticSwitchingProtocol).FeedbackChannel
		round.Record()
	}

	return nil
}

//NewProbabilisticSwitchingSimul default constructor used by each node to init its parameters
func NewProbabilisticSwitchingSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewProbabilisticSwitchingProtocol(tni)
	pap := protocol.(*ProbabilisticSwitchingProtocol)

	if tni.Index() == 0 { //root
		clientSecret := suite.Scalar().Pick(random.Stream)
		clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)

		aggregateKey := pap.Roster().Aggregate

		ciphertexts := make(map[libmedco.TempID]libmedco.DeterministCipherVector)

		//create dummy data
		var tab []int64
		for i := 0; i < probabilisticSwitchedAttributeCount; i++ {
			if i == 0 {
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < probabilisticSwitchedVectorCount; i++ {
			encrypted := *libmedco.EncryptIntVector(aggregateKey, tab)
			for ind, v := range encrypted {
				if ind == 0 {
					ciphertexts[libmedco.TempID(i)] = libmedco.DeterministCipherVector{libmedco.DeterministCipherText{v.C}}
				} else {
					ciphertexts[libmedco.TempID(i)] = append(ciphertexts[libmedco.TempID(i)], libmedco.DeterministCipherText{v.C})
				}
			}
		}

		pap.TargetOfSwitch = &ciphertexts
		pap.TargetPublicKey = &clientPublic
	}
	tempKey := suite.Scalar().Pick(random.Stream)
	pap.SurveyPHKey = &tempKey

	return protocol, err
}
