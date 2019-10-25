package clicontracts

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"

	"go.dedis.ch/onet/v4/log"
	"golang.org/x/xerrors"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/cothority/v4/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v4/calypso"
	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/protobuf"
)

// WriteSpawn creates a new instance of a write contract. It expects a public
// key point in hex string format, which is the collective public key generated
// by the DKG. The secret that will be encrypted under the collective public key
// is provided as a string with --secret and then converted as a slice of bytes.
// This secret has a maximum size depending on the suite used (29 bits for
// ed25519). Another field, filled with --data or from STDIN with the --readData
// option, can contain unlimited sized data. The data however won't be
// automatically encrypted. An additional field 'extra data' can be set, which
// is useful to store cleartext data, where --data should be used to store
// encrypted data. The 'extra data' field can be filled with either --extraData
// or --readExtra. Both --readExtra and --readData can NOT be used at the same
// time. If everything goes well, it prints the instance id of the newly spawned
// Write instance. With the --export option, the instance id is sent to STDOUT.
func WriteSpawn(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return xerrors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.Errorf("loading configuration: %v", err)
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
	if c.Bool("readData") {
		if c.Bool("readExtra") {
			return xerrors.New("--readData can not be used toghether with" +
				"--readExtra")
		}
		dataBuf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return xerrors.Errorf("failed to read from stdin: %v", err)
		}
	} else {
		dataBuf = []byte(c.String("data"))
	}

	var extraDataBuf []byte
	if c.Bool("readExtra") {
		extraDataBuf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return xerrors.Errorf("failed to read from stdin: %v", err)
		}
		// We found out that a newline is automatically added when using pipes
		extraDataBuf = bytes.TrimRight(extraDataBuf, "\n")
	} else {
		extraDataBuf = []byte(c.String("extraData"))
	}

	secret := c.String("secret")
	if secret == "" {
		return xerrors.New("please provide secret with --secret")
	}
	secretBuf, err := hex.DecodeString(secret)
	if err != nil {
		return xerrors.Errorf("failed to decode secret as hexadecimal: %v", err)
	}

	instidstr := c.String("instid")
	if instidstr == "" {
		return xerrors.New("please provide the LTS instance ID with --instid")
	}
	instidbuf, err := hex.DecodeString(instidstr)
	if err != nil {
		return xerrors.Errorf("failed to decode instance id: %v", err)
	}

	keyStr := c.String("key")
	if keyStr == "" {
		return xerrors.New("please provide the hex string public key with --key")
	}
	keyBuf, err := hex.DecodeString(keyStr)
	if err != nil {
		return xerrors.Errorf("failed to decode hex string key: %v", err)
	}
	p := cothority.Suite.Point()
	err = p.UnmarshalBinary(keyBuf)
	if err != nil {
		return xerrors.Errorf("failed to unmarshal key: %v", err)
	}

	instid := byzcoin.NewInstanceID(instidbuf)

	reply := &calypso.WriteReply{}

	write := calypso.NewWrite(cothority.Suite, instid, d.GetBaseID(), p, secretBuf)
	if write == nil {
		return xerrors.New("got a nil write, this is due to a key that is " +
			"too long to be embeded")
	}
	write.Data = dataBuf
	write.ExtraData = extraDataBuf
	writeBuf, err := protobuf.Encode(write)
	if err != nil {
		return xerrors.Errorf("failed to encode Write struct: %v", err)
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return xerrors.Errorf("failed to parse the signer: %v", err)
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return xerrors.Errorf("getting signer counters: %v", err)
	}

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
		return xerrors.Errorf("failed to sign transaction: %v", err)
	}

	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	reply.AddTxResponse, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return xerrors.Errorf("failed to send transaction: %v", err)
	}

	err = lib.WaitPropagation(c, cl)
	if err != nil {
		return xerrors.Errorf("waiting for block propagation: %v", err)
	}

	iidStr := hex.EncodeToString(reply.InstanceID.Slice())
	if c.Bool("export") {
		reader := bytes.NewReader([]byte(iidStr))
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return xerrors.Errorf("failed to copy to stdout: %v", err)
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
		return xerrors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.Errorf("loading configuration: %v", err)
	}

	instID := c.String("instid")
	if instID == "" {
		return xerrors.New("--instid flag is required")
	}
	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return xerrors.New("failed to decode the instID string")
	}

	pr, err := cl.GetProofFromLatest(instIDBuf)
	if err != nil {
		return xerrors.Errorf("couldn't get proof: %v", err)
	}
	proof := pr.Proof

	exist, err := proof.InclusionProof.Exists(instIDBuf)
	if err != nil {
		return xerrors.Errorf("error while checking if proof exist: %v", err)
	}
	if !exist {
		return xerrors.New("proof not found")
	}

	match := proof.InclusionProof.Match(instIDBuf)
	if !match {
		return xerrors.New("proof does not match")
	}

	var write calypso.Write
	err = proof.VerifyAndDecode(cothority.Suite, calypso.ContractWriteID, &write)
	if err != nil {
		return xerrors.Errorf("didn't get a write instance: %v", err)
	}

	if c.Bool("export") {
		_, buf, _, _, err := proof.KeyValue()
		if err != nil {
			return xerrors.Errorf("failed to get value from proof: %v", err)
		}
		reader := bytes.NewReader(buf)
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return xerrors.Errorf("failed to copy to stdout: %v", err)
		}
		return nil
	}

	log.Infof("%s", write)

	return nil
}
