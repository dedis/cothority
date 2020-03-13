package did

import (
	"fmt"
	"testing"
	"time"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/protobuf"
)

type ts struct {
	local      *onet.LocalTest
	servers    []*onet.Server
	roster     *onet.Roster
	signer     darc.Signer
	cl         *byzcoin.Client
	gDarc      *darc.Darc
	genesisMsg *byzcoin.CreateGenesisBlock
	gbReply    *byzcoin.CreateGenesisBlockResponse
}

func newTS(t *testing.T, nodes int) ts {
	s := ts{}
	s.local = onet.NewLocalTestT(cothority.Suite, t)

	s.servers, s.roster, _ = s.local.GenTree(nodes, true)

	// Create the skipchain
	s.signer = darc.NewSignerEd25519(nil, nil)
	s.createGenesis(t)
	return s
}

func (s *ts) createGenesis(t *testing.T) {
	var err error
	s.genesisMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, s.roster,
		[]string{"spawn:" + ContractSovrinDIDID,
			"invoke:" + ContractSovrinDIDID + ".set"},
		s.signer.Identity())
	require.NoError(t, err)
	s.gDarc = &s.genesisMsg.GenesisDarc
	s.genesisMsg.BlockInterval = time.Second

	s.cl, s.gbReply, err = byzcoin.NewLedger(s.genesisMsg, false)
	require.NoError(t, err)
}

func TestContractDID_Sovrin(t *testing.T) {
	s := newTS(t, 3)

	// Spawn a new SovrinDID contract
	sovrinBuf, err := protobuf.Encode(&Sovrin{
		Pool: SovrinPool{
			Name:       "TestPool",
			GenesisTxn: "test",
		},
	})

	tx, err := s.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSovrinDIDID,
			Args: []byzcoin.Argument{
				{
					Name:  "sovrin",
					Value: sovrinBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)

	require.NoError(t, tx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransactionAndWait(tx, 4)
	require.NoError(t, err)

	_, err = s.cl.WaitProof(tx.Instructions[0].DeriveID(""), s.genesisMsg.BlockInterval, nil)
	require.NoError(t, err)

	// Invoke the contract and add a DID
	sk := cothority.Suite.Scalar().Pick(cothority.Suite.RandomStream())
	pk := cothority.Suite.Point().Mul(sk, nil)
	pkBuf, err := pk.MarshalBinary()
	require.NoError(t, err)

	did := base58.Encode(pkBuf[:16])
	sovrinDIDProps := &SovrinDIDProps{
		DID: did,
		Transaction: GetNymTransaction{
			Data: fmt.Sprintf("{\"verkey\": \"~%s\"}", base58.Encode(pkBuf[16:])),
			Dest: did,
		},
	}

	sovrinDIDPropsBuf, err := protobuf.Encode(sovrinDIDProps)
	require.NoError(t, err)

	tx, err = s.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: tx.Instructions[0].DeriveID(""),
		Invoke: &byzcoin.Invoke{
			ContractID: ContractSovrinDIDID,
			Command:    "set",
			Args: []byzcoin.Argument{
				{
					Name:  "sovrinDIDProps",
					Value: sovrinDIDPropsBuf,
				},
			},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)

	require.NoError(t, tx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransactionAndWait(tx, 4)
	require.NoError(t, err)

	didSigner := darc.NewSignerDID(pk, sk, did, "sov")
	key, err := didSigner.Identity().DID.GetStateTrieKey()
	require.NoError(t, err)

	_, err = s.cl.WaitProof(byzcoin.NewInstanceID(key), s.genesisMsg.BlockInterval, nil)
	require.NoError(t, err)

	// Ensure an instruction can be verified with a DID as a signer
	ids := []darc.Identity{didSigner.Identity()}

	tx, err = s.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
	})
	newDarc := darc.NewDarc(darc.InitRules(ids, ids), []byte("DARC owned by a DID"))
	newDarc.Rules.AddRule(darc.Action("spawn:"+ContractSovrinDIDID), expression.InitAndExpr(ids[0].String()))
	darcBuf, err := newDarc.ToProto()
	require.NoError(t, err)

	// Spawn a new DARC with a DID Identity
	tx, err = s.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractDarcID,
			Args: []byzcoin.Argument{
				{
					Name:  "darc",
					Value: darcBuf,
				},
			},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)

	require.NoError(t, tx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransactionAndWait(tx, 4)
	require.NoError(t, err)

	// Try to Spawn a new ContractSovrinDIDID
	// It should work because `newDarc` above has
	// spawn:sovrinDid added
	tx, err = s.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(newDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: ContractSovrinDIDID,
			Args: []byzcoin.Argument{
				{
					Name:  "sovrin",
					Value: sovrinBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)

	require.NoError(t, tx.FillSignersAndSignWith(didSigner))
	_, err = s.cl.AddTransactionAndWait(tx, 4)
	require.NoError(t, err)

	_, err = s.cl.WaitProof(tx.Instructions[0].DeriveID(""), s.genesisMsg.BlockInterval, nil)
	require.NoError(t, err)
}
