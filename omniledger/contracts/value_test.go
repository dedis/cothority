package contracts

import (
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestValue_Spawn(t *testing.T) {
	s := newSer(t, 1, time.Second)
	defer s.local.CloseAll()

	darc2 := s.darc.Copy()
	darc2.Rules.AddRule("spawn:value", darc2.Rules.GetSignExpr())
	darc2.BaseID = s.darc.GetBaseID()
	darc2.PrevID = s.darc.GetID()
	darc2.Version++
	ctx := darcToTx(t, *darc2, s.signer)
	s.sendTx(t, ctx)
	for {
		pr := s.waitProof(t, ctx.Instructions[0].InstanceID)
		require.True(t, pr.InclusionProof.Match())
		values, err := pr.InclusionProof.RawValues()
		require.Nil(t, err)
		d, err := darc.NewFromProtobuf(values[0])
		require.Nil(t, err)
		if d.Version == darc2.Version {
			break
		}
		time.Sleep(s.interval)
	}
	log.Lvl1("Updated darc")

	myvalue := []byte("1234")
	ctx = ClientTransaction{
		Instructions: []Instruction{{
			InstanceID: InstanceID{
				DarcID: s.darc.GetBaseID(),
				SubID:  ZeroSubID,
			},
			Nonce:  GenNonce(),
			Index:  0,
			Length: 1,
			Spawn: &Spawn{
				ContractID: ContractValueID,
				Args: []Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
		}},
	}
	require.Nil(t, ctx.Instructions[0].SignBy(s.signer))

	var subID SubID
	copy(subID[:], ctx.Instructions[0].Hash())
	pr := s.sendTxAndWait(t, ctx, &InstanceID{darc2.GetBaseID(), subID})
	require.True(t, pr.InclusionProof.Match())
	values, err := pr.InclusionProof.RawValues()
	require.Nil(t, err)
	require.Equal(t, myvalue, values[0])
}

func newSer(t *testing.T, step int, interval time.Duration) *ser {
	s := &ser{
		local:  onet.NewTCPTest(tSuite),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(2, true)

	for _, sv := range s.local.GetServices(s.hosts, omniledgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}
	registerDummy(s.hosts)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:invalid", "spawn:panic", "spawn:darc"}, s.signer.Identity())
	require.Nil(t, err)
	s.darc = &genesisMsg.GenesisDarc

	genesisMsg.BlockInterval = interval
	s.interval = genesisMsg.BlockInterval

	for i := 0; i < step; i++ {
		switch i {
		case 0:
			resp, err := s.service().CreateGenesisBlock(genesisMsg)
			require.Nil(t, err)
			s.sb = resp.Skipblock
		case 1:
			tx, err := createOneClientTx(s.darc.GetBaseID(), dummyKind, s.value, s.signer)
			require.Nil(t, err)
			s.tx = tx
			_, err = s.service().AddTransaction(&AddTxRequest{
				Version:     CurrentVersion,
				SkipchainID: s.sb.SkipChainID(),
				Transaction: tx,
			})
			require.Nil(t, err)
			time.Sleep(4 * s.interval)
		default:
			require.Fail(t, "no such step")
		}
	}
	return s
}

func darcToTx(t *testing.T, d2 darc.Darc, signer darc.Signer) ClientTransaction {
	d2Buf, err := d2.ToProto()
	require.Nil(t, err)
	invoke := Invoke{
		Command: "evolve",
		Args: []Argument{
			Argument{
				Name:  "darc",
				Value: d2Buf,
			},
		},
	}
	instr := Instruction{
		InstanceID: InstanceID{
			DarcID: d2.GetBaseID(),
			SubID:  ZeroSubID,
		},
		Nonce:  GenNonce(),
		Index:  0,
		Length: 1,
		Invoke: &invoke,
	}
	require.Nil(t, instr.SignBy(signer))
	return ClientTransaction{
		Instructions: []Instruction{instr},
	}
}
