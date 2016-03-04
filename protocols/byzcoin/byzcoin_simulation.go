package byzcoin

import (
	"errors"
	"fmt"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/broadcast"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.SimulationRegister("ByzCoinSimulation", NewSimulation)
	sda.ProtocolRegisterName("ByzCoin", func(n *sda.Node) (sda.ProtocolInstance, error) {
		return NewByzCoinProtocol(n)
	})
}

// Simulation implements da.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	SimulationConfig
}

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
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		dbg.Fatal("Couldn't get block:", err)
	}
	sc := &sda.SimulationConfig{}
	e.CreateEntityList(sc, hosts, 2000)
	err = e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

type monitorMut struct {
	*monitor.Measure
	sync.Mutex
}

func (m *monitorMut) NewMeasure(id string) {
	m.Lock()
	defer m.Unlock()
	m.Measure = monitor.NewMeasure(id)
}
func (m *monitorMut) MeasureAndReset() {
	m.Lock()
	defer m.Unlock()
	m.Measure = nil
}

// Run implements sda.Simulation interface
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	dbg.Lvl1("Simulation starting with:  Rounds=", e.Rounds)
	server := NewByzCoinServer(e.Blocksize, e.TimeoutMs, e.Fail)

	node, _ := sdaConf.Overlay.NewNodeEmptyName("Broadcast", sdaConf.Tree)
	proto, _ := broadcast.NewBroadcastRootProtocol(node)
	node.SetProtocolInstance(proto)
	// channel to notify we are done
	broadDone := make(chan bool)
	proto.RegisterOnDone(func() {
		broadDone <- true
	})
	proto.Start()
	// wait
	<-broadDone

	for round := 0; round < e.Rounds; round++ {
		client := NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			dbg.Error("Error in ClientSimulation:", err)
			return err
		}

		dbg.Lvl1("Starting round", round)
		// create an empty node
		node, err := sdaConf.Overlay.NewNodeEmptyName("ByzCoin", sdaConf.Tree)
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewMeasure("round")
		pi, err := server.Instantiate(node)
		if err != nil {
			return err
		}

		bz := pi.(*ByzCoin)
		// Register callback for the generation of the signature !
		bz.RegisterOnSignatureDone(func(sig *BlockSignature) {
			rComplete.Measure()
			if err := verifyBlockSignature(node.Suite(), node.EntityList().Aggregate, sig); err != nil {
				dbg.Error("Round", round, "failed:", err)
			} else {
				dbg.Lvl1("Round", round, "success")
			}
		})

		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		bz.RegisterOnDone(func() {
			done <- true
		})
		if e.Fail > 0 {
			go bz.startAnnouncementPrepare()
			// do not run bz.startAnnouncementCommit()
		} else {
			go bz.Start()
		}
		// wait for the end
		<-done
		dbg.Lvl3("Round", round, "finished")

	}
	return nil
}

func verifyBlockSignature(suite abstract.Suite, aggregate abstract.Point, sig *BlockSignature) error {
	if sig == nil || sig.Sig == nil || sig.Block == nil {
		return errors.New("Empty block signature")
	}
	marshalled, err := sig.Block.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Marshalling of block did not work: %v", err)
	}
	return cosi.VerifySignatureWithException(suite, aggregate, marshalled, sig.Sig.Challenge, sig.Sig.Response, sig.Exceptions)
}
