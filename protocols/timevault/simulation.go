package timevault

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	sda.SimulationRegister("TimeVault", NewSimulation)
}

// Simulation implements a TimeVault simulation.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation creates a TimeVault simulation.
func NewSimulation(config string) (sda.Simulation, error) {
	tvs := &Simulation{}
	_, err := toml.Decode(config, tvs)
	if err != nil {
		return nil, err
	}
	return tvs, nil
}

// Setup configures a TimeVault simulation.
func (tvs *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sim := new(sda.SimulationConfig)
	tvs.CreateEntityList(sim, hosts, 2000)
	err := tvs.CreateTree(sim)
	return sim, err
}

func (tvs *Simulation) Run(config *sda.SimulationConfig) error {
	return nil
}

// Run initiates a TimeVault simulation
//func (tvs *Simulation) Run(config *sda.SimulationConfig) error {
//
//	msg := []byte("Test message for TimeVault simulation")
//
//	p, err := config.Overlay.CreateProtocol(config.Tree, "TimeVault")
//	if err != nil {
//		return err
//	}
//	proto := p.(*TimeVault)
//
//	dbg.Lvl1("Starting setup")
//	proto.Start()
//	dbg.Lvl1("Setup done")
//
//	sid, key, c, err := proto.Seal(msg, time.Second*2)
//	if err != nil {
//		dbg.Fatal(err)
//	}
//	<-time.After(time.Second * 4)
//
//	m, err := proto.Open(sid, key, c)
//	if err != nil {
//		dbg.Fatal(err)
//	}
//	if !bytes.Equal(m, msg) {
//		dbg.Warn(m)
//		dbg.Warn(msg)
//		dbg.Fatal("Error, decryption failed")
//	}
//	//dbg.Lvl1("TimeVault - opening successful")
//
//	return nil
//}
