package clicontracts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// ValueSpawn is used to spawn a new contract.
func ValueSpawn(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	value := c.String("value")
	if value == "" {
		return errors.New("--value flag is required")
	}
	valueBuf := []byte(value)

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
		ContractID: contracts.ContractValueID,
		Args: []byzcoin.Argument{
			{
				Name:  "value",
				Value: valueBuf,
			},
		},
	}

	redirect := c.Bool("redirect")
	// In the case the --redirect flag is provided, the transaction is not
	// applied but sent to stdout.
	if redirect {
		proposedTransaction := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{
				byzcoin.Instruction{
					InstanceID: byzcoin.NewInstanceID(d.GetBaseID()),
					Spawn:      &spawn,
				},
			},
		}
		proposedTransactionBuf, err := protobuf.Encode(&proposedTransaction)
		if err != nil {
			return errors.New("couldn't encode the transaction: " + err.Error())
		}
		reader := bytes.NewReader(proposedTransactionBuf)
		_, err = io.Copy(c.App.Writer, reader)
		if err != nil {
			return errors.New("failed to copy to stdout: " + err.Error())
		}
		return nil
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
	_, err = fmt.Fprintf(c.App.Writer, "Spawned new value contract. Instance id is: \n%x\n", instID)
	if err != nil {
		return err
	}

	return nil
}

// ValueInvokeUpdate is able to update the value of a value contract
func ValueInvokeUpdate(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	value := c.String("value")
	if value == "" {
		return errors.New("--value flag is required")
	}
	valueBuf := []byte(value)

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

	invoke := byzcoin.Invoke{
		ContractID: contracts.ContractValueID,
		Command:    "update",
		Args: []byzcoin.Argument{
			{
				Name:  "value",
				Value: valueBuf,
			},
		},
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{
			{
				InstanceID:    byzcoin.NewInstanceID([]byte(instIDBuf)),
				Invoke:        &invoke,
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
	_, err = fmt.Fprintf(c.App.Writer, "Value contract updated! (instance ID is %x)\n", newInstID)
	if err != nil {
		return err
	}

	return nil
}

// ValueGet checks the proof and retrieves the value of a value contract.
func ValueGet(c *cli.Context) error {

	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	instID := c.String("iid")
	if instID == "" {
		return errors.New("--iid flag is required")
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
	match := proof.InclusionProof.Match(instIDBuf)
	if !match {
		return errors.New("proof does not match")
	}

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "%s\n", resultBuf)

	return nil
}
