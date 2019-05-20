package clicontracts

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// ConfigInvokeUpdateConfig perform an Invoke:update_config on a config contract
func ConfigInvokeUpdateConfig(c *cli.Context) error {
	// Here is what this function does:
	//   1. Get the current config and parse the arguments
	//   2. Build the transaction and send it to stdout if --redirect is given
	//   3. Get the result back and output it

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

	// Get the latest chain config
	pr, err := cl.GetProof(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for chainConfig: " + err.Error())
	}
	proof := pr.Proof
	err = proof.Verify(cl.ID)
	if err != nil {
		return errors.New("failed to verify proof: " + err.Error())
	}

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}
	config := byzcoin.ChainConfig{}
	err = protobuf.DecodeWithConstructors(value, &config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return errors.New("couldn't decode chainConfig: " + err.Error())
	}

	// BlockInterval
	blockInterval := c.String("blockInterval")

	if blockInterval != "" {
		duration, err := time.ParseDuration(blockInterval)
		if err != nil {
			return errors.New("couldn't parse blockInterval: " + err.Error())
		}
		config.BlockInterval = duration
	}

	// MaxBlockSize
	maxBlockSize := c.Int("maxBlockSize")
	if maxBlockSize > 0 {
		if maxBlockSize < 16000 && maxBlockSize > 8e6 {
			return errors.New("maxBlockSize out of bounds: must be between 16e3 and 8e6")
		}
		config.MaxBlockSize = maxBlockSize
	}

	// DarcContractIDs
	// we need the IDs to be separated by commas
	darcContractIDs := c.String("darcContractIDs")
	if darcContractIDs != "" {
		darcContractIDsSlice := strings.Split(darcContractIDs, ",")
		config.DarcContractIDs = darcContractIDsSlice
	}

	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return errors.New("failed to encode config: " + err.Error())
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("couldn't get counters: " + err.Error())
	}

	invoke := byzcoin.Invoke{
		ContractID: byzcoin.ContractConfigID,
		Command:    "update_config",
		Args: []byzcoin.Argument{
			{
				Name:  "config",
				Value: configBuf,
			},
		},
	}

	// ---
	// 2.
	// ---
	redirect := c.Bool("redirect")
	// In the case the --redirect flag is provided, the transaction is not
	// applied but sent to stdout.
	if redirect {
		proposedTransaction := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{
				byzcoin.Instruction{
					InstanceID: byzcoin.ConfigInstanceID,
					Invoke:     &invoke,
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
		Instructions: []byzcoin.Instruction{{
			InstanceID:    byzcoin.ConfigInstanceID,
			Invoke:        &invoke,
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
	pr, err = cl.GetProof(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for config: " + err.Error())
	}
	proof = pr.Proof
	err = proof.Verify(cl.ID)
	if err != nil {
		return errors.New("failed to verify proof: " + err.Error())
	}

	_, resultBuf, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}

	contractConfig := byzcoin.ChainConfig{}
	err = protobuf.Decode(resultBuf, &contractConfig)
	if err != nil {
		return errors.New("failed to decode contractConfig: " + err.Error())
	}

	newInstID := ctx.Instructions[0].DeriveID("").Slice()
	_, err = fmt.Fprintf(c.App.Writer, "Config contract updated! (instance ID is %x)\n", newInstID)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Here is the config data: \n%s", contractConfig)

	return nil
}

// ConfigGet displays the latest chain config contract instance
func ConfigGet(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	// Get the latest chain config
	pr, err := cl.GetProof(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for chainConfig: " + err.Error())
	}
	proof := pr.Proof
	err = proof.Verify(cl.ID)
	if err != nil {
		return errors.New("failed to verify proof: " + err.Error())
	}

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}
	config := byzcoin.ChainConfig{}
	err = protobuf.DecodeWithConstructors(value, &config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return errors.New("couldn't decode chainConfig: " + err.Error())
	}

	fmt.Fprintf(c.App.Writer, "Here is the config data: \n%s", config)

	return nil
}
