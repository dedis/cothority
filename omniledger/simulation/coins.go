package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/omniledger/contracts"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul/monitor"
)

/*
 * Defines the simulation for the service-template
 */

func init() {
	onet.SimulationRegister("TransferCoins", NewSimulationService)
}

// SimulationService only holds the BFTree simulation
type SimulationService struct {
	onet.SimulationBFTree
	Transactions  int
	BlockInterval string
	BatchSize     int
	Keep          bool
	Delay         int
}

// NewSimulationService returns the new simulation, where all fields are
// initialised using the config-file
func NewSimulationService(config string) (onet.Simulation, error) {
	es := &SimulationService{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (s *SimulationService) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2000)
	err := s.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Node can be used to initialize each node before it will be run
// by the server. Here we call the 'Node'-method of the
// SimulationBFTree structure which will load the roster- and the
// tree-structure to speed up the first round.
func (s *SimulationService) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

// Run is used on the destination machines and runs a number of
// rounds
func (s *SimulationService) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", s.Rounds, "transactions:", s.Transactions)
	var c *service.Client
	if s.Keep {
		c = service.NewClientKeep()
	} else {
		c = service.NewClient()
	}
	signer := darc.NewSignerEd25519(nil, nil)

	// Create omniledger
	gm, err := service.DefaultGenesisMsg(service.CurrentVersion, config.Roster,
		[]string{"spawn:coin", "invoke:mint", "invoke:transfer"}, signer.Identity())
	if err != nil {
		return errors.New("couldn't setup genesis message: " + err.Error())
	}
	// gm.BlockInterval = time.Second
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return errors.New("parse duration of BlockInterval failed: " + err.Error())
	}
	gm.BlockInterval = blockInterval
	rep, err := c.CreateGenesisBlock(gm)
	if err != nil {
		return errors.New("couldn't craete genesis block: " + err.Error())
	}
	c.ID = rep.Skipblock.SkipChainID()
	c.Roster = config.Roster

	// Create two accounts and mint 'Transaction' coins on first account.
	coins := make([]byte, 8)
	coins[7] = byte(1)
	coinOne := make([]byte, 8)
	coinOne[0] = byte(1)
	tx := service.ClientTransaction{
		Instructions: []service.Instruction{
			{
				InstanceID: service.InstanceID{
					DarcID: gm.GenesisDarc.GetID(),
					SubID:  service.SubID{},
				},
				Nonce:  service.Nonce{},
				Index:  0,
				Length: 3,
				Spawn: &service.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
			{
				InstanceID: service.InstanceID{
					DarcID: gm.GenesisDarc.GetID(),
					SubID:  service.SubID{},
				},
				Nonce:  service.Nonce{},
				Index:  1,
				Length: 3,
				Spawn: &service.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
			{
				Nonce:  service.Nonce{},
				Index:  2,
				Length: 3,
				Invoke: &service.Invoke{
					Command: "mint",
					Args: service.Arguments{{
						Name:  "coins",
						Value: coins}},
				},
			},
		},
	}

	// The first instruction will create an account with the SubID equal to the
	// hash of the first instruction. So we can mint directly on the hash of this
	// instruction. Theoretically...
	coinAddr1 := service.InstanceID{
		DarcID: gm.GenesisDarc.GetBaseID(),
		SubID:  service.NewSubID(tx.Instructions[0].Hash()),
	}
	coinAddr2 := service.InstanceID{
		DarcID: gm.GenesisDarc.GetBaseID(),
		SubID:  service.NewSubID(tx.Instructions[1].Hash()),
	}
	tx.Instructions[2].InstanceID = coinAddr1

	// Now sign all the instructions
	for i := range tx.Instructions {
		if err = service.SignInstruction(&tx.Instructions[i], signer); err != nil {
			return errors.New("signing of instruction failed: " + err.Error())
		}
	}

	// And send the instructions to omniledger
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't add transaction: " + err.Error())
	}

	for round := 0; round < s.Rounds; round++ {
		log.Lvl1("Starting round", round)
		roundM := monitor.NewTimeMeasure("round")

		txs := s.Transactions / s.BatchSize
		insts := s.BatchSize
		log.Lvlf2("Sending %d transactions with %d instructions each", txs, insts)
		for t := 0; t < txs; t++ {
			tx := service.ClientTransaction{}
			prepare := monitor.NewTimeMeasure("prepare")
			for i := 0; i < insts; i++ {
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, i+1)
				tx.Instructions = append(tx.Instructions, service.Instruction{
					InstanceID: coinAddr1,
					Nonce:      service.NewNonce(buf.Bytes()),
					Index:      i,
					Length:     insts,
					Invoke: &service.Invoke{
						Command: "transfer",
						Args: service.Arguments{
							{
								Name:  "coins",
								Value: coinOne,
							},
							{
								Name:  "destination",
								Value: coinAddr2.Slice(),
							}},
					},
				})
				err = service.SignInstruction(&tx.Instructions[i], signer)
				if err != nil {
					return errors.New("signature error: " + err.Error())
				}
			}
			prepare.Record()
			send := monitor.NewTimeMeasure("send")
			_, err = c.AddTransaction(tx)
			if err != nil {
				return errors.New("couldn't add transfer transaction: " + err.Error())
			}
			send.Record()
		}
		confirm := monitor.NewTimeMeasure("confirm")
		var i int
		for {
			proof, err := c.GetProof(coinAddr2.Slice())
			if err != nil {
				return errors.New("couldn't get proof for transaction: " + err.Error())
			}
			_, v, err := proof.Proof.KeyValue()
			if err != nil {
				return errors.New("proof doesn't hold transaction: " + err.Error())
			}
			account := int(binary.LittleEndian.Uint64(v[0]))
			log.Lvlf1("[%03d] account has %d", i, account)
			if account == s.Transactions*(round+1) {
				break
			}
			time.Sleep(time.Second / 10)
			i++
		}
		confirm.Record()
		roundM.Record()
	}
	// We wait a bit before closing because c.GetProof is sent to the
	// leader, but at this point some of the children might still be doing
	// updateCollection. If we stop the simulation immediately, then the
	// database gets closed and updateCollection on the children fails to
	// complete.
	time.Sleep(time.Second)
	return nil
}
