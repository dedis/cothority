package skeleton_channels

import (
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"strconv"
)

/*
This is a simple SkeletonChannels-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	sda.SimulationRegister("SkeletonChannels", NewSkeletonChannelsSimulation)
}

type SkeletonChannelsSimulation struct {
	sda.SimulationBFTree
}

func NewSkeletonChannelsSimulation(config string) (sda.Simulation, error) {
	es := &SkeletonChannelsSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

func (e *SkeletonChannelsSimulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (e *SkeletonChannelsSimulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	dbg.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		dbg.Lvl1("Starting round", round)
		round := monitor.NewMeasure("round")
		n, err := config.Overlay.StartNewNodeName("SkeletonChannels", config.Tree)
		if err != nil {
			return err
		}
		children := <-n.ProtocolInstance().(*ProtocolSkeletonChannels).ChildCount
		round.Measure()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}
