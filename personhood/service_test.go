package personhood

import (
	"encoding/binary"
	"testing"
	"time"

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
}

func newS(t *testing.T) (s *sStruct) {
	s = &sStruct{}
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
			"spawn:ropasci", "invoke:ropasci.second", "invoke:ropasci.confirm"}, s.signer.Identity())
	require.Nil(t, err)
	s.gMsg.BlockInterval = 500 * time.Millisecond

	resp, err := s.ols.CreateGenesisBlock(s.gMsg)
	s.genesisDarc = &s.gMsg.GenesisDarc
	require.Nil(t, err)
	s.olID = resp.Skipblock.SkipChainID()
	s.cl = byzcoin.NewClient(s.olID, *s.roster)
	s.cl.UseNode(0)
	return
}

func (s *sStruct) Close() {
	s.local.CloseAll()
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
