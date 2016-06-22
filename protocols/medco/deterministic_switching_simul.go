package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/random"
	. "github.com/dedis/cothority/services/medco/structs"

)
const NUM_ATTR_DET = 3
const NUM_VECT_DET = 2

// we have 5 clients
func init() {
	sda.SimulationRegister("DeterministicSwitching", NewDeterministicSwitchingSimulation)
	sda.ProtocolRegisterName("DeterministicSwitchingSimul", NewDeterministicSwitchingSimul)

}

type DeterministicSwitchingSimulation struct {
	sda.SimulationBFTree
}

func NewDeterministicSwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &DeterministicSwitchingSimulation{}
	_,err := toml.Decode(config, sim)

	if err != nil {
		return nil,err
	}
	return sim, nil
}

func (sim *DeterministicSwitchingSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateEntityList(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}

	dbg.Lvl1("Begin test encrypted data generation")

	return sc, nil
}


func (sim *DeterministicSwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA( config.Tree,"DeterministicSwitchingSimul")
		if err != nil {
			return err
		}

		root := rooti.(*DeterministicSwitchingProtocol)

		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")
		root.StartProtocol()        // problem is here!
		/*result*/_= <-root.ProtocolInstance().(*DeterministicSwitchingProtocol).FeedbackChannel
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
			dbg.LLvl1("Test passed")
		}
		//dbg.Lvl1("Got result", DecryptIntVector(suite, clientSecret, result[TempID(0)]))*/

	}

	return nil
}

func NewDeterministicSwitchingSimul(tni *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	protocol, err := NewDeterministSwitchingProtocol(tni)
	pap := protocol.(*DeterministicSwitchingProtocol)

	if (tni.Index() == 0) {
		aggregateKey := pap.EntityList().Aggregate

		ciphertexts := make(map[TempID]CipherVector)

		var tab []int64
		for i := 0; i < NUM_ATTR_DET; i++ {
			if i == 0 {
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < NUM_VECT_DET; i++ {
			ciphertexts[TempID(i)] = *EncryptIntVector(aggregateKey, tab)
		}

		pap.TargetOfSwitch = &ciphertexts
	}

	tempKey := suite.Secret().Pick(random.Stream)
	pap.SurveyPHKey = &tempKey

	return pap, err
	}