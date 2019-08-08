package clicontracts

import (
	"errors"
	"strings"
	"time"

	"go.dedis.ch/onet/v3/log"

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
	//   2. Build the transaction and send it to stdout if --export is given
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
	pr, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for chainConfig: " + err.Error())
	}
	proof := pr.Proof

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
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID:    byzcoin.ConfigInstanceID,
		Invoke:        &invoke,
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})
	if err != nil {
		return err
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	if lib.FindRecursivefBool("export", c) {
		return lib.ExportTransaction(ctx)
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	// ---
	// 3.
	// ---
	err = lib.WaitPropagation(c, cl)
	if err != nil {
		return err
	}
	pr, err = cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for config: " + err.Error())
	}
	proof = pr.Proof
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
	log.Infof("Config contract updated! (instance ID is %x)", newInstID)
	log.Infof("Here is the config data:\n%s", contractConfig)

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
	pr, err := cl.GetProofFromLatest(byzcoin.ConfigInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for chainConfig: " + err.Error())
	}
	proof := pr.Proof

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}
	config := byzcoin.ChainConfig{}
	err = protobuf.DecodeWithConstructors(value, &config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return errors.New("couldn't decode chainConfig: " + err.Error())
	}

	log.Infof("Here is the config data:\n%s", config)

	return nil
}
