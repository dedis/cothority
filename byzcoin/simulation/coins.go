package main

import (
	"errors"
	"time"

	"github.com/BurntSushi/toml"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul/monitor"
	"go.dedis.ch/protobuf"
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
	signer := darc.NewSignerEd25519(nil, nil)

	// Create the ledger
	gm, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, config.Roster,
		[]string{"spawn:" + contracts.ContractCoinID, "invoke:" + contracts.ContractCoinID + ".mint", "invoke:" + contracts.ContractCoinID + ".transfer"}, signer.Identity())
	if err != nil {
		return errors.New("couldn't setup genesis message: " + err.Error())
	}

	// Set block interval from the simulation config.
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return errors.New("parse duration of BlockInterval failed: " + err.Error())
	}
	gm.BlockInterval = blockInterval

	c, _, err := byzcoin.NewLedger(gm, s.Keep)
	if err != nil {
		return errors.New("couldn't create genesis block: " + err.Error())
	}
	if err = c.UseNode(0); err != nil {
		return err
	}

	// Create two accounts and mint 'Transaction' coins on first account.
	coins := make([]byte, 8)
	coins[7] = byte(1)
	tx, err := c.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gm.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
		},
		SignerIdentities: []darc.Identity{signer.Identity()},
		SignerCounter:    []uint64{1},
	}, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gm.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCoinID,
		},
		SignerIdentities: []darc.Identity{signer.Identity()},
		SignerCounter:    []uint64{2},
	})
	if err != nil {
		return err
	}

	// Now sign all the instructions
	if err = tx.FillSignersAndSignWith(signer); err != nil {
		return errors.New("signing of instruction failed: " + err.Error())
	}
	coinAddr1 := tx.Instructions[0].DeriveID("")
	coinAddr2 := tx.Instructions[1].DeriveID("")

	// Send the instructions.
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't initialize accounts: " + err.Error())
	}

	// Because of issue #1379, we need to do this in a separate tx, once we know
	// the spawn is done.
	tx, err = c.CreateTransaction(byzcoin.Instruction{
		InstanceID: coinAddr1,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCoinID,
			Command:    "mint",
			Args: byzcoin.Arguments{{
				Name:  "coins",
				Value: coins}},
		},
		SignerIdentities: []darc.Identity{signer.Identity()},
		SignerCounter:    []uint64{3},
	})
	if err != nil {
		return err
	}
	if err = tx.FillSignersAndSignWith(signer); err != nil {
		return errors.New("signing of instruction failed: " + err.Error())
	}
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return errors.New("couldn't mint coin: " + err.Error())
	}

	coinOne := make([]byte, 8)
	coinOne[0] = byte(1)

	var signatureCtr uint64 = 4 // we finished at 3, so start here at 4
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
		tx := byzcoin.ClientTransaction{}
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
				tx.Instructions = byzcoin.Instructions{}
			}

			prepare := monitor.NewTimeMeasure("prepare")
			for i := 0; i < insts; i++ {
				instrs := append(tx.Instructions, byzcoin.Instruction{
					InstanceID: coinAddr1,
					Invoke: &byzcoin.Invoke{
						ContractID: contracts.ContractCoinID,
						Command:    "transfer",
						Args: byzcoin.Arguments{
							{
								Name:  "coins",
								Value: coinOne,
							},
							{
								Name:  "destination",
								Value: coinAddr2.Slice(),
							}},
					},
					SignerIdentities: []darc.Identity{signer.Identity()},
					SignerCounter:    []uint64{signatureCtr},
				})
				tx, err = c.CreateTransaction(instrs...)
				if err != nil {
					return err
				}
				signatureCtr++
				err = tx.FillSignersAndSignWith(signer)
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

		// The AddTransactionAndWait returns as soon as the transaction is included in the node, but
		// it doesn't wait until the transaction is included in all nodes. Thus this wait for
		// the new block to be propagated.
		time.Sleep(time.Second)
		proof, err := c.GetProof(coinAddr2.Slice())
		if err != nil {
			return errors.New("couldn't get proof for transaction: " + err.Error())
		}
		_, v0, _, _, err := proof.Proof.KeyValue()
		if err != nil {
			return errors.New("proof doesn't hold transaction: " + err.Error())
		}
		var account byzcoin.Coin
		err = protobuf.Decode(v0, &account)
		if err != nil {
			return errors.New("couldn't decode account: " + err.Error())
		}
		log.Lvlf1("Account has %d - total should be: %d", account.Value, s.Transactions*(round+1))
		if account.Value != uint64(s.Transactions*(round+1)) {
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
