package main

import (
	"errors"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/blockchain"
	"github.com/dedis/cothority/byzcoin/cosi"
	"github.com/dedis/cothority/messaging"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul/monitor"
)

func init() {
	onet.SimulationRegister("ByzCoin", NewByzCoinSimulation)
	onet.GlobalProtocolRegister("ByzCoin", func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return byzcoin.NewByzCoinProtocol(n)
	})
}

// ByzCoinSimulation implements da.Simulation interface
type ByzCoinSimulation struct {
	// onet fields:
	onet.SimulationBFTree
	// your simulation specific fields:
	ByzCoinSimulationConfig
}

// ByzCoinSimulationConfig is the config used by the simulation for byzcoin
type ByzCoinSimulationConfig struct {
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

// NewByzCoinSimulation returns a fresh byzcoin simulation out of the toml config
func NewByzCoinSimulation(config string) (onet.Simulation, error) {
	es := &ByzCoinSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation interface. It checks on the availability
// of the block-file and downloads it if missing. Then the block-file will be
// copied to the simulation-directory
func (e *ByzCoinSimulation) Setup(dir string, hosts []string) (*onet.SimulationConfig, error) {
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		log.Fatal("Couldn't get block:", err)
	}
	sc := &onet.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err = e.CreateTree(sc)
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

// Run implements onet.Simulation interface
func (e *ByzCoinSimulation) Run(onetConf *onet.SimulationConfig) error {
	log.Lvl2("Simulation starting with: Rounds=", e.Rounds)
	server := byzcoin.NewByzCoinServer(e.Blocksize, e.TimeoutMs, e.Fail)

	pi, err := onetConf.Overlay.CreateProtocol("Broadcast", onetConf.Tree, onet.NilServiceID)
	if err != nil {
		return err
	}
	proto, _ := pi.(*messaging.Broadcast)
	// channel to notify we are done
	broadDone := make(chan bool)
	proto.RegisterOnDone(func() {
		broadDone <- true
	})
	// ignore error on purpose: Broadcast.Start() always returns nil
	_ = proto.Start()
	// wait
	<-broadDone

	for round := 0; round < e.Rounds; round++ {
		client := byzcoin.NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			log.Error("Error in ClientSimulation:", err)
			return err
		}

		log.Lvl1("Starting round", round)
		// create an empty node
		tni := onetConf.Overlay.NewTreeNodeInstanceFromProtoName(onetConf.Tree, "ByzCoin")
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewTimeMeasure("round")
		pi, err := server.Instantiate(tni)
		if err != nil {
			return err
		}
		onetConf.Overlay.RegisterProtocolInstance(pi)

		bz := pi.(*byzcoin.ByzCoin)
		// Register callback for the generation of the signature !
		bz.RegisterOnSignatureDone(func(sig *byzcoin.BlockSignature) {
			rComplete.Record()
			if err := verifyBlockSignature(tni.Suite(), tni.Roster().Aggregate, sig); err != nil {
				log.Error("Round", round, "failed:", err)
			} else {
				log.Lvl2("Round", round, "success")
			}
		})

		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		bz.RegisterOnDone(func() {
			done <- true
		})
		if e.Fail > 0 {
			go func() {
				err := bz.StartAnnouncementPrepare()
				if err != nil {
					log.Error("Error while starting "+
						"announcment prepare:", err)
				}
			}()
			// do not run bz.startAnnouncementCommit()
		} else {
			go func() {
				if err := bz.Start(); err != nil {
					log.Error("Couldn't start protocol",
						err)
				}
			}()
		}
		// wait for the end
		<-done
		log.Lvl3("Round", round, "finished")

	}
	return nil
}

func verifyBlockSignature(suite abstract.Suite, aggregate abstract.Point, sig *byzcoin.BlockSignature) error {
	if sig == nil || sig.Sig == nil || sig.Block == nil {
		return errors.New("Empty block signature")
	}
	marshalled := sig.Block.HashSum()
	return cosi.VerifySignatureWithException(suite, aggregate, marshalled, sig.Sig.Challenge, sig.Sig.Response, sig.Exceptions)
}
