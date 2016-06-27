package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/random"
	. "github.com/dedis/cothority/services/medco/libmedco"
)
const NUM_ATTR_PROB = 2
const NUM_VECT_PROB = 10

// we have 5 clients
func init() {
	sda.SimulationRegister("ProbabilisticSwitching", NewProbabilisticSwitchingSimulation)
	sda.ProtocolRegisterName("ProbabilisticSwitchingSimul", NewProbabilisticSwitchingSimul)

}

type ProbabilisticSwitchingSimulation struct {
	sda.SimulationBFTree
}

func NewProbabilisticSwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &ProbabilisticSwitchingSimulation{}
	_,err := toml.Decode(config, sim)

	if err != nil {
		return nil,err
	}
	return sim, nil
}

func (sim *ProbabilisticSwitchingSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateRoster(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}

	log.Lvl1("Begin test encrypted data generation")

	return sc, nil
}

func (sim *ProbabilisticSwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA( config.Tree,"ProbabilisticSwitchingSimul")
		if err != nil {
			return err
		}

		root := rooti.(*ProbabilisticSwitchingProtocol)

		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")
		root.StartProtocol()
		/*result*/_= <-root.ProtocolInstance().(*ProbabilisticSwitchingProtocol).FeedbackChannel
		round.Record()
		/*output := true
		for i,v := range result {
			if int(i) < (len(result)-1) {
				if !reflect.DeepEqual(v, result[i+1]) { //both are equals but deepEqual says not ?
					dbg.Errorf("Not expected results, got ", v, " & ", result[i+1])
					output = false
				}
			}
		}
		if output{
			log.LLvl1("Test passed")
		}
		//log.Lvl1("Got result", DecryptIntVector(suite, clientSecret, result[TempID(0)]))*/

	}

	return nil
}


func NewProbabilisticSwitchingSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewProbabilisticSwitchingProtocol(tni)
	pap := protocol.(*ProbabilisticSwitchingProtocol)

	if (tni.Index() == 0){
		clientSecret  := suite.Scalar().Pick(random.Stream)
		clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)

		aggregateKey := pap.Roster().Aggregate

		ciphertexts := make(map[TempID]DeterministCipherVector)

		var tab []int64
		for i := 0; i < NUM_ATTR_PROB; i++{
			if i == 0{
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < NUM_VECT_PROB; i++{
			encrypted := *EncryptIntVector(aggregateKey, tab)
			for ind,v := range encrypted {
				if ind == 0{
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