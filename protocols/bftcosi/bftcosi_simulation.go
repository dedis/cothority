package bftcosi

import (
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
)

// FIXME everything ... reimplement
func init() {
	sda.SimulationRegister("ByzCoin", NewSimulation)
	sda.ProtocolRegisterName("ByzCoin", func(n *sda.Node) (sda.ProtocolInstance, error) {
		return NewBFTCoSiProtocol(n)
	})
}

// Simulation implements da.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	SimulationConfig
}

// SimulationConfig is the config used by the simulation for bftcosi
type SimulationConfig struct {
	// Blocksize is the number of transactions in one block:
	Blocksize int
	// timeout the leader after TimeoutMs milliseconds
	TimeoutMs uint64
	// Fail:
	// 0  do not fail
	// 1 fail by doing nothing
	// 2 fail by sending wrong blocks
	Fail uint
}

// NewSimulation returns a fresh bftcosi simulation out of the toml config
func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface. It checks on the availability
// of the block-file and downloads it if missing. Then the block-file will be
// copied to the simulation-directory
func (e *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

type monitorMut struct {
	*monitor.TimeMeasure
	sync.Mutex
}

func (m *monitorMut) NewMeasure(id string) {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure = monitor.NewTimeMeasure(id)
}
func (m *monitorMut) Record() {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure.Record()
	m.TimeMeasure = nil
}

// Run implements sda.Simulation interface
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl2("Simulation will start later with: Rounds=", e.Rounds)
	return nil
}
