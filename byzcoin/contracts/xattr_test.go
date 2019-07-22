package contracts

import (
	"errors"
	"fmt"
	"net/url"
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
	err := byzcoin.RegisterGlobalContract(xattrValueID, contractXattrValueFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}

const xattrValueID = "xattr_value"

func contractXattrValueFromBytes(in []byte) (byzcoin.Contract, error) {
	return contractXattrValue{value: in}, nil
}

type contractXattrValue struct {
	value []byte
}

func notImpl(what string) error { return fmt.Errorf("this contract does not implement %v", what) }

func (c contractXattrValue) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, inst.DeriveID(""),
			xattrValueID, inst.Spawn.Args.Search("value"), darcID),
	}
	return
}

func (c contractXattrValue) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
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
				xattrValueID, inst.Invoke.Args.Search("value"), darcID),
		}
		return
	default:
		return nil, nil, errors.New("Value contract can only update")
	}
}

func (c contractXattrValue) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	evalXattr := func(name, xattr string) error {
		if name != xattrValueID {
			return errors.New("invalid xattr " + name)
		}

		vals, err := url.ParseQuery(xattr)
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
	return inst.VerifyWithOption(rst, ctxHash, &byzcoin.VerificationOptions{EvalXattr: evalXattr})
}

func (c contractXattrValue) VerifyDeferredInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	return notImpl("VerifyDeferredInstruction")
}

func (c contractXattrValue) Delete(byzcoin.ReadOnlyStateTrie, byzcoin.Instruction, []byzcoin.Coin) (sc []byzcoin.StateChange, cs []byzcoin.Coin, err error) {
	err = notImpl("Delete")
	return
}

func (c contractXattrValue) FormatMethod(inst byzcoin.Instruction) string {
	return "not implemented"
}

// This test uses the same code as the Spawn one but then performs an update
// on the value contract.
// Use the value contract but verify the xattr on the DARCs
func TestXattrValue(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:" + xattrValueID}, signer.Identity())
	require.Nil(t, err)

	gDarc := &genesisMsg.GenesisDarc
	// We are only allowed to invoke when the value contains a certain prefix and suffix
	gDarc.Rules.AddRule("invoke:"+xattrValueID+".update", []byte(signer.Identity().String()+" & xattr:"+xattrValueID+":prefix=abc&suffix=xyz"))
	genesisMsg.BlockInterval = time.Second

	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	myvalue := []byte("abcdefgxyz")
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: xattrValueID,
				Args: []byzcoin.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.NoError(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	myID := ctx.Instructions[0].DeriveID("")
	local.WaitDone(genesisMsg.BlockInterval)

	// Invoke ok - the existing value matches the xattr requirement
	myvalue = []byte("abcd5678")
	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: myID,
			Invoke: &byzcoin.Invoke{
				ContractID: xattrValueID,
				Command:    "update",
				Args: []byzcoin.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
			SignerCounter: []uint64{2},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)
	local.WaitDone(genesisMsg.BlockInterval)

	// Invoke fail - the new value does not match the xattr requirement
	myvalue = []byte("abcdefxzy")
	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: myID,
			Invoke: &byzcoin.Invoke{
				ContractID: xattrValueID,
				Command:    "update",
				Args: []byzcoin.Argument{{
					Name:  "value",
					Value: myvalue,
				}},
			},
			SignerCounter: []uint64{3},
		}},
	}
	require.Nil(t, ctx.FillSignersAndSignWith(signer))

	resp, err := cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)
	require.Contains(t, resp.Error, "wrong suffix")
	local.WaitDone(genesisMsg.BlockInterval)
}
