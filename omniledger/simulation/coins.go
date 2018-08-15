package main

import (
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

func init() {
	onet.SimulationRegister("TransferCoins", NewSimulationService)
}

// SimulationService holds the state of the simulation.
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

	// Set block interval from the simulation config.
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return errors.New("parse duration of BlockInterval failed: " + err.Error())
	}
	gm.BlockInterval = blockInterval

	// Create the OmniLedger instance.
	_, err = c.CreateGenesisBlock(gm)
	if err != nil {
		return errors.New("couldn't create genesis block: " + err.Error())
	}

	// Create two accounts and mint 'Transaction' coins on first account.
	coins := make([]byte, 8)
	coins[7] = byte(1)
	tx := service.ClientTransaction{
		Instructions: []service.Instruction{
			{
				InstanceID: service.NewInstanceID(gm.GenesisDarc.GetBaseID()),
				Nonce:      service.GenNonce(),
				Index:      0,
				Length:     2,
				Spawn: &service.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
			{
				InstanceID: service.NewInstanceID(gm.GenesisDarc.GetBaseID()),
				Nonce:      service.GenNonce(),
				Index:      1,
				Length:     2,
				Spawn: &service.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
		},
	}

	// The first instruction will create an account with the InstanceID equal to the
	// hash of the first instruction.
	coinAddr1 := service.NewInstanceID(tx.Instructions[0].Hash())

	// We'll also want to remember this addr so that we can monitor
	// it for coins arriving.
	coinAddr2 := service.NewInstanceID(tx.Instructions[1].Hash())

	// Now sign all the instructions
	for i := range tx.Instructions {
		if err = service.SignInstruction(&tx.Instructions[i], gm.GenesisDarc.GetBaseID(), signer); err != nil {
			return errors.New("signing of instruction failed: " + err.Error())
		}
	}

	// And send the instructions to omniledger
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't initialize accounts: " + err.Error())
	}

	// Because of issue #1379, we need to do this in a separate tx, once we know
	// the spawn is done.
	tx = service.ClientTransaction{
		Instructions: []service.Instruction{
			{
				InstanceID: coinAddr1,
				Nonce:      service.GenNonce(),
				Index:      0,
				Length:     1,
				Invoke: &service.Invoke{
					Command: "mint",
					Args: service.Arguments{{
						Name:  "coins",
						Value: coins}},
				},
			},
		},
	}
	if err = service.SignInstruction(&tx.Instructions[0], gm.GenesisDarc.GetBaseID(), signer); err != nil {
		return errors.New("signing of instruction failed: " + err.Error())
	}
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't mint coin: " + err.Error())
	}

	coinOne := make([]byte, 8)
	coinOne[0] = byte(1)

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
				tx.Instructions = append(tx.Instructions, service.Instruction{
					InstanceID: coinAddr1,
					Nonce:      service.GenNonce(),
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
				err = service.SignInstruction(&tx.Instructions[i], gm.GenesisDarc.GetBaseID(), signer)
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

		// This sleep is needed to wait for the propagation to finish
		// on all the nodes. Otherwise the simulation manager
		// (runsimul.go in onet) might close some nodes and cause
		// skipblock propagation to fail.
		time.Sleep(blockInterval)
	}
	return nil
}
