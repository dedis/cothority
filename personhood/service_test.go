package personhood

import (
	"encoding/binary"
	"testing"
	"time"

	"go.dedis.ch/cothority/v3/calypso"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Stores and loads a personhood data.
func TestService_SaveLoad(t *testing.T) {
	// Creates a party and links it, then verifies the account exists.
	s := newS(t)
	defer s.Close()
	s.createParty(t, len(s.servers), 3)

	s.phs[0].save()
	require.Nil(t, s.phs[0].tryLoad())
}

type sStruct struct {
	local       *onet.LocalTest
	cl          *byzcoin.Client
	servers     []*onet.Server
	roster      *onet.Roster
	services    []onet.Service
	phs         []*Service
	genesisDarc *darc.Darc
	party       FinalStatement
	orgs        []*key.Pair
	attendees   []*key.Pair
	attCoin     []byzcoin.InstanceID
	attDarc     []*darc.Darc
	attSig      []darc.Signer
	service     *key.Pair
	serDarc     *darc.Darc
	serCoin     byzcoin.InstanceID
	serSig      darc.Signer
	ols         *byzcoin.Service
	olID        skipchain.SkipBlockID
	signer      darc.Signer
	gMsg        *byzcoin.CreateGenesisBlock
	popI        byzcoin.InstanceID
	counter     uint64
	t           *testing.T
}

func newS(t *testing.T) (s *sStruct) {
	s = &sStruct{t: t}
	s.local = onet.NewTCPTest(tSuite)
	s.servers, s.roster, _ = s.local.GenTree(5, true)

	s.services = s.local.GetServices(s.servers, templateID)
	for _, p := range s.services {
		s.phs = append(s.phs, p.(*Service))
	}

	// Create the ledger
	s.ols = s.local.Services[s.roster.List[0].ID][onet.ServiceFactory.ServiceID(byzcoin.ServiceName)].(*byzcoin.Service)
	s.signer = darc.NewSignerEd25519(nil, nil)
	var err error
	s.gMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:popParty", "invoke:popParty.finalize", "invoke:popParty.barrier",
			"invoke:popParty.mine",
			"spawn:" + calypso.ContractWriteID,
			"spawn:" + calypso.ContractReadID,
			"spawn:" + calypso.ContractLongTermSecretID,
			"invoke:" + calypso.ContractLongTermSecretID + ".reshare",
			"spawn:" + contracts.ContractCoinID,
			"invoke:" + contracts.ContractCoinID + ".mint",
			"invoke:" + contracts.ContractCoinID + ".fetch",
			"invoke:" + contracts.ContractCoinID + ".transfer",
			"spawn:" + ContractSpawnerID,
			"invoke:" + ContractSpawnerID + ".update",
			"spawn:ropasci", "invoke:ropasci.second", "invoke:ropasci.confirm"}, s.signer.Identity())
	require.Nil(t, err)
	s.gMsg.BlockInterval = 500 * time.Millisecond

	resp, err := s.ols.CreateGenesisBlock(s.gMsg)
	s.genesisDarc = &s.gMsg.GenesisDarc
	require.Nil(t, err)
	s.olID = resp.Skipblock.SkipChainID()
	s.cl = byzcoin.NewClient(s.olID, *s.roster)
	s.cl.Genesis = resp.Skipblock
	s.counter = uint64(0)
	return
}

func (s *sStruct) Close() {
	s.local.CloseAll()
}

func (s *sStruct) coinSpawn(coin byzcoin.Coin) byzcoin.InstanceID {
	// Create a coin
	_, id := s.spawn(contracts.ContractCoinID,
		byzcoin.Arguments{{Name: "type", Value: coin.Name.Slice()}})
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, coin.Value)
	s.invoke(id, contracts.ContractCoinID, "mint", byzcoin.Arguments{
		{Name: "coins", Value: coinsBuf},
	})
	return id
}

func (s *sStruct) coinGet(t *testing.T, inst byzcoin.InstanceID) (ci byzcoin.Coin) {
	gpr, err := s.ols.GetProof(&byzcoin.GetProof{
		Version: byzcoin.CurrentVersion,
		Key:     inst.Slice(),
		ID:      s.olID,
	})
	require.Nil(t, err)
	require.True(t, gpr.Proof.InclusionProof.Match(inst.Slice()))
	_, v0, cid, _, err := gpr.Proof.KeyValue()
	require.Nil(t, err)
	require.Equal(t, contracts.ContractCoinID, cid)
	err = protobuf.Decode(v0, &ci)
	require.Nil(t, err)
	return
}

func (s *sStruct) coinTransfer(t *testing.T, from, to byzcoin.InstanceID, coins uint64, d *darc.Darc, sig darc.Signer) {
	signerCtrs, err := s.ols.GetSignerCounters(&byzcoin.GetSignerCounters{
		SignerIDs:   []string{sig.Identity().String()},
		SkipchainID: s.olID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(signerCtrs.Counters))

	var cBuf = make([]byte, 8)
	binary.LittleEndian.PutUint64(cBuf, coins)
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: from,
			Invoke: &byzcoin.Invoke{
				ContractID: contracts.ContractCoinID,
				Command:    "transfer",
				Args: []byzcoin.Argument{{
					Name:  "coins",
					Value: cBuf,
				},
					{
						Name:  "destination",
						Value: to.Slice(),
					}},
			},
			SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(sig))
	_, err = s.ols.AddTransaction(&byzcoin.AddTxRequest{
		Version:       byzcoin.CurrentVersion,
		SkipchainID:   s.olID,
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.Nil(t, err)
}

func (s *sStruct) spawn(cid string, args byzcoin.Arguments) (*byzcoin.AddTxResponse, byzcoin.InstanceID) {
	tx, txReply := s.addTx(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(s.genesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: cid,
			Args:       args,
		},
	})
	return txReply, tx.Instructions[0].DeriveID("")
}

func (s *sStruct) invoke(id byzcoin.InstanceID, cid, cmd string, args byzcoin.Arguments) *byzcoin.AddTxResponse {
	_, txReply := s.addTx(byzcoin.Instruction{
		InstanceID: id,
		Invoke: &byzcoin.Invoke{
			ContractID: cid,
			Command:    cmd,
			Args:       args,
		},
	})
	return txReply
}

func (s *sStruct) addTx(instr ...byzcoin.Instruction) (byzcoin.ClientTransaction, *byzcoin.AddTxResponse) {
	for i := range instr {
		s.counter++
		instr[i].SignerCounter = []uint64{s.counter}
	}
	tx, err := s.cl.CreateTransaction(instr...)
	require.NoError(s.t, err)
	err = tx.FillSignersAndSignWith(s.signer)
	require.NoError(s.t, err)
	txReply, err := s.cl.AddTransactionAndWait(tx, 10)
	require.NoError(s.t, err)
	return tx, txReply
}

type sCoin struct {
	coin byzcoin.Coin
	id   byzcoin.InstanceID
}

func (s *sStruct) newCoin(name string, value uint64) (sc sCoin) {
	sc.coin = byzcoin.Coin{Name: byzcoin.NewInstanceID([]byte(name)), Value: value}
	sc.id = s.coinSpawn(sc.coin)
	return
}
