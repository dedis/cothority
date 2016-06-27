package medco

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/random"
	. "github.com/dedis/cothority/services/medco/libmedco"

	"reflect"
)
const NUM_ATTR = 100000
const NUM_VECT = 1

// we have 5 clients
func init() {
sda.SimulationRegister("KeySwitching", NewKeySwitchingSimulation)

}

type KeySwitchingSimulation struct {
	sda.SimulationBFTree
}

func NewKeySwitchingSimulation(config string) (sda.Simulation, error) {
	sim := &KeySwitchingSimulation{}
	_,err := toml.Decode(config, sim)

	if err != nil {
		return nil,err
	}
		return sim, nil
}// Send a file to all nodes ? Dir lisible dans Run et Protocol

func (sim *KeySwitchingSimulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	sim.CreateRoster(sc, hosts, 20)
	err := sim.CreateTree(sc)

	if err != nil {
		return nil, err
	}    // global variable?

	log.Lvl1("Begin test encrypted data generation")

	return sc, nil
}// Run when all node ready , run as a node ?


func (sim *KeySwitchingSimulation) Run(config *sda.SimulationConfig) error {
	for round := 0; round < sim.Rounds; round++ {
		log.Lvl1("Starting round", round)
		rooti, err := config.Overlay.CreateProtocolSDA( config.Tree,"KeySwitching")
		if err != nil {
			return err
		}

		root := rooti.(*KeySwitchingProtocol)
		suite := root.Suite()
		aggregateKey := root.Roster().Aggregate

		ciphertexts := make(map[TempID]CipherVector)

		//log.Lvl1("ciphertexts",ciphertexts)

		var tab []int64
		for i := 0; i < NUM_ATTR; i++{
			if i == 0{
				tab = []int64{1}
			} else {
				tab = append(tab, 1)
			}
		}

		for i := 0; i < NUM_VECT; i++{
			ciphertexts[TempID(i)] = *EncryptIntVector(aggregateKey, tab)
		}



		clientSecret  := suite.Scalar().Pick(random.Stream)
		clientPublic := suite.Point().Mul(suite.Point().Base(), clientSecret)

		root.ProtocolInstance().(*KeySwitchingProtocol).TargetPublicKey = &clientPublic
		root.ProtocolInstance().(*KeySwitchingProtocol).TargetOfSwitch = &ciphertexts
		//root.ProtocolInstance().(*KeySwitchingProtocol).originalEphemKeys =

		round := monitor.NewTimeMeasure("MEDCO_PROTOCOL")
		root.StartProtocol()        // problem is here!
		result := <-root.ProtocolInstance().(*KeySwitchingProtocol).FeedbackChannel
		round.Record()
		output := true
		for _,v := range result {
			val1 := DecryptIntVector(clientSecret, &v)
			if !reflect.DeepEqual(val1, tab){
				log.Errorf("Not expected results, got ", val1, " & ", tab)
				output = false
			}
		}
		if output{
			log.LLvl1("Test passed")
		}
		//log.Lvl1("Got result", DecryptIntVector(suite, clientSecret, result[TempID(0)]))

	}

	return nil
}