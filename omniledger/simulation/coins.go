package main

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/omniledger/contracts"
	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
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
	var c *ol.Client
	if s.Keep {
		c = ol.NewClientKeep()
	} else {
		c = ol.NewClient()
	}
	signer := darc.NewSignerEd25519(nil, nil)

	// Create omniledger
	gm, err := ol.DefaultGenesisMsg(ol.CurrentVersion, config.Roster,
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
	tx := ol.ClientTransaction{
		Instructions: []ol.Instruction{
			{
				InstanceID: ol.NewInstanceID(gm.GenesisDarc.GetBaseID()),
				Nonce:      ol.GenNonce(),
				Index:      0,
				Length:     2,
				Spawn: &ol.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
			{
				InstanceID: ol.NewInstanceID(gm.GenesisDarc.GetBaseID()),
				Nonce:      ol.GenNonce(),
				Index:      1,
				Length:     2,
				Spawn: &ol.Spawn{
					ContractID: contracts.ContractCoinID,
				},
			},
		},
	}

	// Now sign all the instructions
	for i := range tx.Instructions {
		if err = ol.SignInstruction(&tx.Instructions[i], gm.GenesisDarc.GetBaseID(), signer); err != nil {
			return errors.New("signing of instruction failed: " + err.Error())
		}
	}
	coinAddr1 := tx.Instructions[0].DeriveID("")
	coinAddr2 := tx.Instructions[1].DeriveID("")

	// And send the instructions to omniledger
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't initialize accounts: " + err.Error())
	}

	// Because of issue #1379, we need to do this in a separate tx, once we know
	// the spawn is done.
	tx = ol.ClientTransaction{
		Instructions: []ol.Instruction{
			{
				InstanceID: coinAddr1,
				Nonce:      ol.GenNonce(),
				Index:      0,
				Length:     1,
				Invoke: &ol.Invoke{
					Command: "mint",
					Args: ol.Arguments{{
						Name:  "coins",
						Value: coins}},
				},
			},
		},
	}
	if err = ol.SignInstruction(&tx.Instructions[0], gm.GenesisDarc.GetBaseID(), signer); err != nil {
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

		if s.Transactions < 3 {
			log.Warn("The 'send_sum' measurement will be very skewed, as the last transaction")
			log.Warn("is not measured.")
		}

		txs := s.Transactions / s.BatchSize
		insts := s.BatchSize
		log.Lvlf1("Sending %d transactions with %d instructions each", txs, insts)
		tx := ol.ClientTransaction{}
		// Inverse the prepare/send loop, so that the last transaction is not sent,
		// but can be sent in the 'confirm' phase using 'AddTransactionAndWait'.
		for t := 0; t < txs; t++ {
			if len(tx.Instructions) > 0 {
				log.Lvlf1("Sending transaction %d", t)
				send := monitor.NewTimeMeasure("send")
				_, err = c.AddTransaction(tx)
				if err != nil {
					return errors.New("couldn't add transfer transaction: " + err.Error())
				}
				send.Record()
				tx.Instructions = ol.Instructions{}
			}

			prepare := monitor.NewTimeMeasure("prepare")
			for i := 0; i < insts; i++ {
				tx.Instructions = append(tx.Instructions, ol.Instruction{
					InstanceID: coinAddr1,
					Nonce:      ol.GenNonce(),
					Index:      i,
					Length:     insts,
					Invoke: &ol.Invoke{
						Command: "transfer",
						Args: ol.Arguments{
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
				err = ol.SignInstruction(&tx.Instructions[i], gm.GenesisDarc.GetBaseID(), signer)
				if err != nil {
					return errors.New("signature error: " + err.Error())
				}
			}
			prepare.Record()
		}

		// Confirm the transaction by sending the last transaction using
		// AddTransactionAndWait. There is a small error in measurement,
		// as we're missing one of the AddTransaction call in the measurements.
		confirm := monitor.NewTimeMeasure("confirm")
		log.Lvl1("Sending last transaction and waiting")
		_, err = c.AddTransactionAndWait(tx, 20)
		if err != nil {
			return errors.New("while adding transaction and waiting: " + err.Error())
		}
		proof, err := c.GetProof(coinAddr2.Slice())
		if err != nil {
			return errors.New("couldn't get proof for transaction: " + err.Error())
		}
		_, v, err := proof.Proof.KeyValue()
		if err != nil {
			return errors.New("proof doesn't hold transaction: " + err.Error())
		}
		account := int(binary.LittleEndian.Uint64(v[0]))
		log.Lvlf1("Account has %d", account)
		if account != s.Transactions*(round+1) {
			return errors.New("account has wrong amount")
		}
		confirm.Record()
		roundM.Record()

		// This sleep is needed to wait for the propagation to finish
		// on all the nodes. Otherwise the simulation manager
		// (runsimul.go in onet) might close some nodes and cause
		// skipblock propagation to fail.
		time.Sleep(blockInterval)
	}
	// We wait a bit before closing because c.GetProof is sent to the
	// leader, but at this point some of the children might still be doing
	// updateCollection. If we stop the simulation immediately, then the
	// database gets closed and updateCollection on the children fails to
	// complete.
	time.Sleep(time.Second)
	return nil
}
