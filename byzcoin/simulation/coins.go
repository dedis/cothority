package main

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/BurntSushi/toml"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul/monitor"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

func init() {
	onet.SimulationRegister("TransferCoins", NewSimulationService)
}

var txNonce uint64

// SimulationService holds the state of the simulation.
type SimulationService struct {
	onet.SimulationBFTree
	Transactions  int
	BlockInterval string
	BatchSize     int
	Keep          bool
	Delay         int
	Accounts      int
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

//SpawnAccounts creates the specified number of accounts and returns the associated transaction
func (s *SimulationService) SpawnAccounts(c *byzcoin.Client, gm *byzcoin.CreateGenesisBlock, signer darc.Signer) (byzcoin.ClientTransaction, error) {
	instr := make([]byzcoin.Instruction, 0)
	for i := 0; i < s.Accounts; i++ {
		inst := byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(gm.GenesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractCoinID,
			},
			SignerIdentities: []darc.Identity{signer.Identity()},
			SignerCounter:    []uint64{txNonce},
		}
		txNonce++
		instr = append(instr, inst)
	}

	txAccounts, err := c.CreateTransaction(instr...)
	if err != nil {
		return byzcoin.ClientTransaction{}, err
	}

	// Now sign all the instructions
	if err = txAccounts.FillSignersAndSignWith(signer); err != nil {
		return byzcoin.ClientTransaction{}, xerrors.Errorf("signing of instruction failed: %v", err)
	}

	// Send the instructions.
	_, err = c.AddTransactionAndWait(txAccounts, 2)
	if err != nil {
		return byzcoin.ClientTransaction{}, xerrors.Errorf("couldn't initialize accounts: %v", err)
	}

	return txAccounts, nil

}

//ViewBalance returns the current balance of the specified account
func (s *SimulationService) ViewBalance(account1 byzcoin.InstanceID, c *byzcoin.Client) (uint64, error) {
	proof, err := c.GetProof(account1.Slice())
	if err != nil {
		return 0, xerrors.Errorf("couldn't get proof for transaction: %v", err)
	}
	_, v0, _, _, err := proof.Proof.KeyValue()
	if err != nil {
		return 0, xerrors.Errorf("proof doesn't hold transaction: %v", err)
	}
	var account byzcoin.Coin
	err = protobuf.Decode(v0, &account)
	if err != nil {
		return 0, xerrors.Errorf("couldn't decode account: %v", err)
	}
	return account.Value, nil
}

//Credit credits the specified number of coins to the specified account
func (s *SimulationService) Credit(account byzcoin.InstanceID, c *byzcoin.Client, value uint64, signer darc.Signer) error {
	quant := make([]byte, 8)
	binary.LittleEndian.PutUint64(quant, value)
	inst := byzcoin.Instruction{
		InstanceID: account,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCoinID,
			Command:    "mint",
			Args: byzcoin.Arguments{{
				Name:  "coins",
				Value: quant}},
		},
		SignerIdentities: []darc.Identity{signer.Identity()},
		SignerCounter:    []uint64{txNonce},
	}
	tx, err := c.CreateTransaction(inst)
	if err != nil {
		return err
	}
	if err = tx.FillSignersAndSignWith(signer); err != nil {
		return xerrors.Errorf("signing of instruction failed: %v", err)
	}
	_, err = c.AddTransactionAndWait(tx, 2)
	if err != nil {
		return xerrors.Errorf("couldn't mint coin: %v", err)
	}
	txNonce++
	return nil
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
		return xerrors.Errorf("couldn't setup genesis message: %v", err)
	}

	// Set block interval from the simulation config.
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return xerrors.Errorf("parse duration of BlockInterval failed: %v", err)
	}
	gm.BlockInterval = blockInterval

	c, _, err := byzcoin.NewLedger(gm, s.Keep)
	if err != nil {
		return xerrors.Errorf("couldn't create genesis block: %v", err)
	}
	if err = c.UseNode(0); err != nil {
		return err
	}

	balance := make(map[string]uint64)

	// Create number of specified accounts and mint 'Transaction' coins on first account.
	txNonce = uint64(1)

	txAccounts, err := s.SpawnAccounts(c, gm, signer)
	if err != nil {
		xerrors.Errorf("Cannot spawn accounts")
	}

	account1 := txAccounts.Instructions[0].DeriveID("")

	for k := range txAccounts.Instructions {
		log.LLvl1("Created account", txAccounts.Instructions[k].DeriveID("").String())
	}

	coins := make([]byte, 8)
	coins[2] = byte(1)

	// Because of issue #1379, we need to do this in a separate tx, once we know
	// the spawn is done.`
	err = s.Credit(account1, c, binary.LittleEndian.Uint64(coins), signer)
	if err != nil {
		xerrors.Errorf("Error crediting", err)
	}

	balance[account1.String()] = binary.LittleEndian.Uint64(coins)

	//Check the balance of account1
	s.ViewBalance(account1, c)

	coinOne := make([]byte, 8)
	coinOne[0] = byte(1)

	rand.Seed(time.Now().UnixNano())
	min := 1
	max := s.Accounts - 1

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
					return xerrors.Errorf("couldn't add transfer transaction: %v", err)
				}
				send.Record()
				tx.Instructions = byzcoin.Instructions{}
			}

			prepare := monitor.NewTimeMeasure("prepare")
			for i := 0; i < insts; i++ {
				randomAccountNumber := rand.Intn(max-min+1) + min
				rAccount := txAccounts.Instructions[randomAccountNumber].DeriveID("")
				instrs := append(tx.Instructions, byzcoin.Instruction{
					InstanceID: account1,
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
								Value: rAccount.Slice(),
							}},
					},
					SignerIdentities: []darc.Identity{signer.Identity()},
					SignerCounter:    []uint64{txNonce},
				})

				txNonce++
				balance[rAccount.String()] = balance[rAccount.String()] + binary.LittleEndian.Uint64(coinOne)
				balance[account1.String()] = balance[account1.String()] - 1

				tx, err = c.CreateTransaction(instrs...)
				if err != nil {
					return err
				}
				err = tx.FillSignersAndSignWith(signer)
				if err != nil {
					return xerrors.Errorf("signature error: %v", err)
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
			return xerrors.Errorf("while adding transaction and waiting: %v", err)
		}

		// The AddTransactionAndWait returns as soon as the transaction is included in the node, but
		// it doesn't wait until the transaction is included in all nodes. Thus this wait for
		// the new block to be propagated.
		time.Sleep(time.Second)
		confirm.Record()
		roundM.Record()

		// This sleep is needed to wait for the propagation to finish
		// on all the nodes. Otherwise the simulation manager
		// (runsimul.go in onet) might close some nodes and cause
		// skipblock propagation to fail.
		time.Sleep(blockInterval)
	}

	log.LLvl1("Check all balances")
	for k, v := range txAccounts.Instructions {
		account := txAccounts.Instructions[k].DeriveID("").String()
		ledgerBalance, err := s.ViewBalance(v.DeriveID(""), c)
		if err != nil {
			return xerrors.Errorf("check balance error: %v", err)
		}

		txs := balance[account]
		if int(txs) != int(ledgerBalance) {
			log.LLvl1(ledgerBalance, int(txs), "account has wrong amount")
			return xerrors.New("account has wrong amount")
		}
		log.Lvlf1("Account %s has %d - total should be: %d", account, ledgerBalance, int(txs))
	}

	// We wait a bit before closing because c.GetProof is sent to the
	// leader, but at this point some of the children might still be doing
	// updateCollection. If we stop the simulation immediately, then the
	// database gets closed and updateCollection on the children fails to
	// complete.
	time.Sleep(time.Second)
	return nil
}
