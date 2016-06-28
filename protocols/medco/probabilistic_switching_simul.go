package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
	. "github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/random"
)

//number of attributes to be switched in each vector
const NUM_ATTR_PROB = 2
//number of vectors to be switched
const NUM_VECT_PROB = 10

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

		ciphertexts := make(map[TempID]DeterministCipherVector)

		//create dummy data
		var tab []int64
		for i := 0; i < NUM_ATTR_PROB; i++ {
			if i == 0 {
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < NUM_VECT_PROB; i++ {
			encrypted := *EncryptIntVector(aggregateKey, tab)
			for ind, v := range encrypted {
				if ind == 0 {
					ciphertexts[TempID(i)] = DeterministCipherVector{DeterministCipherText{v.C}}
				} else {
					ciphertexts[TempID(i)] = append(ciphertexts[TempID(i)], DeterministCipherText{v.C})
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
