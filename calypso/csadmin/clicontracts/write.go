package clicontracts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"go.dedis.ch/onet/v3/log"

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
// by the DKG. The secret that will be encrypted under the collective public key
// is provided as a string with --secret and then converted as a slice of bytes.
// This secret has a maximum size depending on the suite used (29 bits for
// ed25519). Another field, filled with --data or from STDIN with the --readin
// option, can contain unlimited sized data. The data however won't be
// automatically encrypted. If everything goes well, it prints the instance id
// of the newly spawned Write instance. With the --export option, the instance
// id is sent to STDOUT.
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

	var dataBuf []byte
	if c.Bool("readin") {
		dataBuf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return errors.New("failed to read from stdin: " + err.Error())
		}
		// We found out that a newline is automatically added when using pipes
		dataBuf = bytes.TrimRight(dataBuf, "\n")
	} else {
		dataBuf = []byte(c.String("data"))
	}

	secret := c.String("secret")
	if secret == "" {
		return errors.New("please provide secret with --secret")
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
		[]byte(secret))
	if write == nil {
		return errors.New("got a nil write, this is due to a key that is " +
			"too long to be embeded")
	}
	write.Data = dataBuf
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

	err = lib.WaitPropagation(c, cl)
	if err != nil {
		return err
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

	log.Infof("Spawned a new write instance. "+
		"Its instance id is:\n%s", iidStr)

	return nil
}

// WriteGet checks the proof and prints the content of the Write contract.
func WriteGet(c *cli.Context) error {

	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	instID := c.String("instid")
	if instID == "" {
		return errors.New("--instid flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string")
	}

	pr, err := cl.GetProofFromLatest(instIDBuf)
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

	var write calypso.Write
	err = proof.VerifyAndDecode(cothority.Suite, calypso.ContractWriteID, &write)
	if err != nil {
		return errors.New("didn't get a write instance: " + err.Error())
	}

	log.Infof("%s", write)

	return nil
}
