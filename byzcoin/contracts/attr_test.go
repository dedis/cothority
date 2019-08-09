package contracts

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func init() {
	err := byzcoin.RegisterGlobalContract(contractAttrValueID, contractAttrValueFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

const contractAttrValueID = "attr_value"
const attrAffixID = "affix"
const attrSigSchemeID = "sigscheme"

func contractAttrValueFromBytes(in []byte) (byzcoin.Contract, error) {
	return contractAttrValue{value: in}, nil
}

type contractAttrValue struct {
	byzcoin.BasicContract
	value []byte
}

func notImpl(what string) error { return fmt.Errorf("this contract does not implement %v", what) }

func (c contractAttrValue) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""),
			contractAttrValueID, inst.Spawn.Args.Search("value"), darcID),
	}
	return
}

func (c contractAttrValue) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID

	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "update":
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				contractAttrValueID, inst.Invoke.Args.Search("value"), darcID),
		}
		return
	case "update-v2":
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
				contractAttrValueID, inst.Invoke.Args.Search("value"), darcID),
		}
		return
	default:
		return nil, nil, errors.New("Value contract can only update")
	}
}

func (c contractAttrValue) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	cbAffix := func(attr string) error {
		vals, err := url.ParseQuery(attr)
		if err != nil {
			return err
		}
		prefix := vals.Get("prefix")
		suffix := vals.Get("suffix")

		s := string(c.value)
		if !strings.HasPrefix(s, prefix) {
			return errors.New("wrong prefix")
		}
		if !strings.HasSuffix(s, suffix) {
			return errors.New("wrong suffix")
		}
		return nil
	}
	cbSigScheme := func(attr string) error {
		roSC, ok := rst.(byzcoin.ReadOnlySkipChain)
		if !ok {
			return errors.New("cannot access read only skipchain")
		}
		sb, err := roSC.GetLatest()
		if err != nil {
			return err
		}

		// do some sanity check
		{
			sb0, err := roSC.GetBlockByIndex(0)
			if err != nil {
				return err
			}
			gen, err := roSC.GetGenesisBlock()
			if err != nil {
				return err
			}
			if !sb0.Equal(gen) {
				return errors.New("genesis block is not at index 0")
			}
			sb2, err := roSC.GetBlock(sb.Hash)
			if err != nil {
				return err
			}
			if !sb2.Equal(sb) {
				return errors.New("sb and sb2 should be the same")
			}
		}

		sigSchemeBuf := inst.Invoke.Args.Search("sigscheme")
		if len(sigSchemeBuf) == 0 {
			return errors.New("cannot find sigscheme argument")
		}
		sigScheme, err := strconv.Atoi(string(sigSchemeBuf))
		if err != nil {
			return err
		}
		if int(sb.SignatureScheme) != sigScheme {
			return errors.New("signature scheme did not match")
		}
		return nil
	}
	attrFuncs := c.BasicContract.MakeAttrInterpreters(rst, inst)
	attrFuncs[attrAffixID] = cbAffix
	attrFuncs[attrSigSchemeID] = cbSigScheme
	return inst.VerifyWithOption(rst, ctxHash, &byzcoin.VerificationOptions{EvalAttr: attrFuncs})
}

// Use the new contract and verify the attr on the DARCs. The first attr says
// the user is only allowed to modify the value if the existing value has a
// prefix of "abc" and a suffix of "xyz". It demonstrates how to access data in
// the new instance to do the verification. The second attr says the given
// sigscheme must match the SignatureScheme in the block. It demonstrates how
// to use the skipchain data to do the verification.
func TestAttrCustomRule(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + contractAttrValueID}, signer.Identity())
	require.Nil(t, err)

	gDarc := &genesisMsg.GenesisDarc
	// We are only allowed to invoke when the value contains a certain prefix and suffix
	require.NoError(t, gDarc.Rules.AddRule("invoke:"+contractAttrValueID+".update", []byte(signer.Identity().String()+" & attr:"+attrAffixID+":prefix=abc&suffix=xyz")))
	require.NoError(t, gDarc.Rules.AddRule("invoke:"+contractAttrValueID+".update-v2", []byte(signer.Identity().String()+" & attr:"+attrSigSchemeID+":dummy")))
	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("abcdefgxyz")
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contractAttrValueID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke ok - the existing value matches the attr requirement
	myvalue = []byte("abcd5678")
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke fail - the new value does not match the attr requirement
	myvalue = []byte("abcdefxzy")
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	resp, err := cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "wrong suffix")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke fail - submitting empty sigscheme
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update-v2",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	resp, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "cannot find sigscheme argument")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke fail - submit a wrong sigscheme
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update-v2",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			},
				{
					Name:  "sigscheme",
					Value: []byte("999"),
				}},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	resp, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "signature scheme did not match")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke ok - the correct sigscheme is used
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update-v2",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			},
				{
					Name:  "sigscheme",
					Value: []byte("1"),
				}},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}

func TestAttrBlockIndex(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + contractAttrValueID}, signer.Identity())
	require.Nil(t, err)

	gDarc := &genesisMsg.GenesisDarc
	// We are only allowed to invoke when the value contains a certain prefix and suffix
	require.NoError(t, gDarc.Rules.AddRule("invoke:"+contractAttrValueID+".update", []byte(signer.Identity().String()+" & attr:block:after=0&before=2")))
	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("abcdefgxyz")
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contractAttrValueID,
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke ok - we're within the block interval
	myvalue = []byte("abcde888fgxyz")
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))

	// Invoke fail - we are outside the block interval
	myvalue = []byte("abcde8888fxzy")
	ctx, err = cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: myID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractAttrValueID,
			Command:    "update",
			Args: []byzcoin.Argument{{
				Name:  "value",
				Value: myvalue,
			}},
		},
		SignerCounter: []uint64{3},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	resp, err := cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "does not fit in the interval")
	require.NoError(t, local.WaitDone(genesisMsg.BlockInterval))
}
