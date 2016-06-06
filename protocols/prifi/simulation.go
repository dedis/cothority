package prifi

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"

	prifi_lib "github.com/dedis/cothority/lib/prifi"
)

/*
This is a simple ExampleHandlers-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	sda.SimulationRegister("PriFi", NewSimulation)
}

// Simulation implements sda.Simulation.
type Simulation struct {
	sda.SimulationBFTree
	SimulationConfig
}

// SimulationConfig is the config used by the simulation for byzcoin
type SimulationConfig struct {
	NClients              int
	NTrustees             int
	CellSizeUp            int
	CellSizeDown          int
	RelayWindowSize       int
	RelayUseDummyDataDown bool
	RelayReportingLimit   int
	UseUDP                bool
	DoLatencyTests        bool
}

var tomlConfig SimulationConfig

// NewSimulation is used internally to register the simulation (see the init()
// function above).
func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	tomlConfig = es.SimulationConfig
	tomlConfig.NClients = es.SimulationBFTree.Hosts - tomlConfig.NTrustees - 1

	return es, nil
}

// Setup implements sda.Simulation.
func (e *Simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation.
func (e *Simulation) Run(config *sda.SimulationConfig) error {

	var prifiConfig = &prifi_lib.ALL_ALL_PARAMETERS{
		DoLatencyTests:          tomlConfig.DoLatencyTests,
		DownCellSize:            tomlConfig.CellSizeDown,
		NClients:                tomlConfig.NClients,
		NextFreeClientId:        0,
		NextFreeTrusteeId:       0,
		NTrustees:               tomlConfig.NTrustees,
		RelayReportingLimit:     tomlConfig.RelayReportingLimit,
		RelayUseDummyDataDown:   tomlConfig.RelayUseDummyDataDown,
		RelayWindowSize:         tomlConfig.RelayWindowSize,
		StartNow:                true,
		UpCellSize:              tomlConfig.CellSizeUp,
		UseUDP:                  tomlConfig.UseUDP,
		ClientDataOutputEnabled: false,
		RelayDataOutputEnabled:  false,
		ForceParams:             true,
	}

	dbg.Lvl2("NClients is:", tomlConfig.NClients, ", NTrustees is:", tomlConfig.NTrustees, ", rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocol(config.Tree, "PriFi-SDA-Wrapper")
		p.(*PriFiSDAWrapper).SetConfig(*prifiConfig)
		if err != nil {
			return err
		}
		dbg.Print("Protocol created")
		go p.Start()

		_ = <-p.(*PriFiSDAWrapper).DoneChannel
		round.Record()
	}
	return nil
}
