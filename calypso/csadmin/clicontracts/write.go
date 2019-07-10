package clicontracts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// WriteSpawn creates a new instance of a write contract. It expects a public
// key point in hex string format, which is the collective public key generated
// by the DKG. The data that will be encrypted under the collective public key
// is provided as a string and then converted as a slice of bytes. If everything
// goes well, it prints the instance id of the newly spawned Write instance.
// With the --export option, the instance id is sent to STDOUT.
func WriteSpawn(c *cli.Context) error {
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

	data := c.String("data")
	if data == "" {
		return errors.New("please provide data with --data")
	}

	instidstr := c.String("instid")
	if instidstr == "" {
		return errors.New("please provide the LTS instance ID with --instid")
	}
	instidbuf, err := hex.DecodeString(instidstr)
	if err != nil {
		return errors.New("failed to decode instance id: " + err.Error())
	}

	keyStr := c.String("key")
	if keyStr == "" {
		return errors.New("please provide the hex string public key with --key")
	}
	keyBuf, err := hex.DecodeString(keyStr)
	if err != nil {
		return errors.New("failed to decode hex string key: " + err.Error())
	}
	p := cothority.Suite.Point()
	err = p.UnmarshalBinary(keyBuf)
	if err != nil {
		return errors.New("failed to unmarshal key: " + err.Error())
	}

	instid := byzcoin.NewInstanceID(instidbuf)

	reply := &calypso.WriteReply{}

	write := calypso.NewWrite(cothority.Suite, instid, d.GetBaseID(), p,
		[]byte(data))
	if write == nil {
		return errors.New("got a nil write, this is due to a key that is " +
			"too long to be embeded")
	}
	writeBuf, err := protobuf.Encode(write)
	if err != nil {
		return errors.New("failed to encode Write struct: " + err.Error())
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

	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(d.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: calypso.ContractWriteID,
				Args: byzcoin.Arguments{{
					Name: "write", Value: writeBuf}},
			},
			SignerCounter: []uint64{counters.Counters[0] + 1},
		}},
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return errors.New("failed to sign transaction: " + err.Error())
	}

	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	reply.AddTxResponse, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("failed to send transaction: " + err.Error())
	}

	iidStr := hex.EncodeToString(reply.InstanceID.Slice())
	if c.Bool("export") {
		reader := bytes.NewReader([]byte(iidStr))
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return errors.New("failed to copy to stdout: " + err.Error())
		}
		return nil
	}

	fmt.Fprintf(c.App.Writer, "Spawned a new write instance. "+
		"Its instance id is:\n%s\n", iidStr)

	return nil
}
