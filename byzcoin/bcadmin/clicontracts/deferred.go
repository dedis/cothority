package clicontracts

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// DeferredSpawn is used to spawn a new deferred contract. It expects stdin to
// contain the proposed transaction.
func DeferredSpawn(c *cli.Context) error {
	// Here is what this function does:
	//   1. Parses the stdin in order to get the proposed transaction
	//   2. Fires a spawn instruction for the deferred contract
	//   3. Gets the response back

	// ---
	// 1.
	// ---
	proposedTransactionBuf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return errors.New("failed to read from stding: " + err.Error())
	}

	proposedTransaction := byzcoin.ClientTransaction{}
	err = protobuf.Decode(proposedTransactionBuf, &proposedTransaction)
	if err != nil {
		return errors.New("failed to decode transaction, did you use --redirect ? " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "Here is the proposed Tx: \n%s\n", proposedTransaction)

	// ---
	// 2.
	// ---
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	spawn := byzcoin.Spawn{
		ContractID: byzcoin.ContractDeferredID,
		Args: []byzcoin.Argument{
			{
				Name:  "proposedTransaction",
				Value: proposedTransactionBuf,
			},
		},
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    byzcoin.NewInstanceID(d.GetBaseID()),
				Spawn:         &spawn,
				SignerCounter: []uint64{counters.Counters[0] + 1},
			},
		},
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	instID := ctx.Instructions[0].DeriveID("").Slice()
	_, err = fmt.Fprintf(c.App.Writer, "Spawned new deferred contract, its instance id is: \n%x\n", instID)
	if err != nil {
		return err
	}

	// ---
	// 3.
	// ---
	pr, err := cl.GetProof(instID)
	if err != nil {
		return errors.New("couldn't get proof for admin-darc: " + err.Error())
	}
	proof := pr.Proof

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	result := byzcoin.DeferredData{}
	err = protobuf.Decode(resultBuf, &result)
	if err != nil {
		return errors.New("couldn't decode the result: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "Here is the deferred data: \n%s\n", result)

	return nil
}

// DeferredInvokeAddProof is used to add the proof of a proposed transaction's
// instruction. The proof is computed on the given --hash and based on the
// identity provided by --sign or, by default, the admin.
func DeferredInvokeAddProof(c *cli.Context) error {
	// Here is what this function does:
	//   1. Parses the inoput arguments
	//   2. Computes the signature based on the identity (--sign), the
	//      instruction id (--instrIdx), and the hash (--hash)
	//   3. Sends the addProof transaction
	//   4. Reads the transaction return value (deferred data)

	// ---
	// 1.
	// ---
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	hashStr := c.String("hash")
	if hashStr == "" {
		return errors.New("--hash not found")
	}
	hash, err := hex.DecodeString(hashStr)
	if err != nil {
		return errors.New("coulndn't decode the hash string: " + err.Error())
	}

	instID := c.String("instID")
	if instID == "" {
		return errors.New("--instID flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string: " + err.Error())
	}

	instrIdx := c.Uint("instrIdx")
	index := uint32(instrIdx)
	indexBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBuf, uint32(index))

	// ---
	// 2.
	// ---
	identity := signer.Identity()
	identityBuf, err := protobuf.Encode(&identity)
	if err != nil {
		return errors.New("coulndn't encode the identity: " + err.Error())
	}

	signature, err := signer.Sign(hash)
	if err != nil {
		return errors.New("couldn't sign the hash: " + err.Error())
	}

	// ---
	// 3.
	// ---
	counters, err := cl.GetSignerCounters(signer.Identity().String())

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(instIDBuf),
			Invoke: &byzcoin.Invoke{
				ContractID: byzcoin.ContractDeferredID,
				Command:    "addProof",
				Args: []byzcoin.Argument{
					{
						Name:  "identity",
						Value: identityBuf,
					},
					{
						Name:  "signature",
						Value: signature,
					},
					{
						Name:  "index",
						Value: indexBuf,
					},
				},
			},
			SignerCounter: []uint64{counters.Counters[0] + 1},
		}},
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	// ---
	// 4.
	// ---
	pr, err := cl.GetProof(instIDBuf)
	if err != nil {
		return errors.New("couldn't get proof for admin-darc: " + err.Error())
	}
	proof := pr.Proof

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	result := byzcoin.DeferredData{}
	err = protobuf.Decode(resultBuf, &result)
	if err != nil {
		return errors.New("couldn't decode the result: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "Here is the deferred data: \n%s\n", result)

	return nil
}

// ExecProposedTx is used to execute the proposed transaction if all the
// instructions are correctly signed.
func ExecProposedTx(c *cli.Context) error {
	// Here is what this function does:
	//   1. Parses the input argument
	//   2. Sends an "execProposedTx" transaction
	//   3. Reads the return back and prints it

	// ---
	// 1.
	// ---
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	instID := c.String("instID")
	if instID == "" {
		return errors.New("--instID flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string")
	}

	// ---
	// 2.
	// ---
	counters, err := cl.GetSignerCounters(signer.Identity().String())

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(instIDBuf),
			Invoke: &byzcoin.Invoke{
				ContractID: byzcoin.ContractDeferredID,
				Command:    "execProposedTx",
			},
			SignerCounter: []uint64{counters.Counters[0] + 1},
		}},
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	// ---
	// 3.
	// ---
	pr, err := cl.GetProof(instIDBuf)
	if err != nil {
		return errors.New("couldn't get proof for admin-darc: " + err.Error())
	}
	proof := pr.Proof

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	result := byzcoin.DeferredData{}
	err = protobuf.Decode(resultBuf, &result)
	if err != nil {
		return errors.New("couldn't decode the result: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "Here is the deferred data: \n%s\n", result)

	return nil
}

// DeferredGet checks the proof and retrieves the value of a deferred contract.
func DeferredGet(c *cli.Context) error {

	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	instID := c.String("instID")
	if instID == "" {
		return errors.New("--instID flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string")
	}

	pr, err := cl.GetProof(instIDBuf)
	if err != nil {
		return errors.New("couldn't get proof: " + err.Error())
	}
	proof := pr.Proof

	exist, err := proof.InclusionProof.Exists(instIDBuf)
	if err != nil {
		return errors.New("error while checking if proof exist: " + err.Error())
	}
	if !exist {
		return errors.New("proof not found")
	}

	match := proof.InclusionProof.Match(instIDBuf)
	if !match {
		return errors.New("proof does not match")
	}

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	result := byzcoin.DeferredData{}
	err = protobuf.Decode(resultBuf, &result)
	if err != nil {
		return errors.New("Failed to decode the result: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "%s\n", result)

	return nil
}

// DeferredDelete delete the deferred instance
func DeferredDelete(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	instID := c.String("instID")
	if instID == "" {
		return errors.New("--instID flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	delete := byzcoin.Delete{
		ContractID: byzcoin.ContractDeferredID,
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    byzcoin.NewInstanceID([]byte(instIDBuf)),
				Delete:        &delete,
				SignerCounter: []uint64{counters.Counters[0] + 1},
			},
		},
	}
	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	newInstID := ctx.Instructions[0].DeriveID("").Slice()
	_, err = fmt.Fprintf(c.App.Writer, "Deferred contract deleted! (instance ID is %x)\n", newInstID)
	if err != nil {
		return err
	}

	return nil
}
