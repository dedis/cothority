package clicontracts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"go.dedis.ch/onet/v3/log"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// LTSSpawn spawns a instance of an LTS contract. It prints the instance id,
// which can then be used to stat the DKG. This instance id will also be needed
// to send write requests.
// With the --export option, the instance id is sent to STDOUT.
func LTSSpawn(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return errors.New("failed to load config: " + err.Error())
	}

	dstr := c.String("darc")
	if dstr == "" {
		dstr = cfg.AdminDarc.GetIdentityString()
	}
	d, err := lib.GetDarcByString(cl, dstr)
	if err != nil {
		return errors.New("failed to get darc by string: " + err.Error())
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return errors.New("failed to parse the signer: " + err.Error())
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("failed to get the signer counters: " + err.Error())
	}

	// Make the transaction and get its proof
	ltsInstanceInfo := calypso.LtsInstanceInfo{Roster: cfg.Roster}
	buf, err := protobuf.Encode(&ltsInstanceInfo)
	if err != nil {
		return errors.New("failed to encode instance info: " + err.Error())
	}

	inst := byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(d.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: calypso.ContractLongTermSecretID,
			Args: []byzcoin.Argument{
				{
					Name:  "lts_instance_info",
					Value: buf,
				},
			},
		},
		SignerCounter: []uint64{counters.Counters[0] + 1},
	}

	tx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{inst},
	}

	err = tx.FillSignersAndSignWith(*signer)
	if err != nil {
		return errors.New("failed to fill signers and sign: " + err.Error())
	}

	_, err = cl.AddTransactionAndWait(tx, 10)
	if err != nil {
		return errors.New("failed to add transaction and wait: " + err.Error())
	}

	newInstID := tx.Instructions[0].DeriveID("").Slice()

	err = lib.WaitPropagation(c, cl)
	if err != nil {
		return err
	}

	iidStr := hex.EncodeToString(newInstID)
	if c.Bool("export") {
		reader := bytes.NewReader([]byte(iidStr))
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return errors.New("failed to copy to stdout: " + err.Error())
		}
		return nil
	}

	log.Infof("Spawned a new LTS contract. Its instance id is:\n"+
		"%s", iidStr)

	return nil
}
