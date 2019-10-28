package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/cothority/v4/calypso"
	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/app"
	"golang.org/x/xerrors"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v4/byzcoin/bcadmin/lib"
	"go.dedis.ch/onet/v4/cfgpath"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

func init() {
	cliApp.Name = "csadmin"
	cliApp.Usage = "Handle the calypso service"
	cliApp.Version = gitTag
	cliApp.Commands = cmds // stored in "commands.go"
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:   "config, c",
			EnvVar: "BC_CONFIG",
			// We use the bcadmin config folder because the bcadmin utiliy is
			// the prefered way to generate the config files. And this is where
			// bcadmin will put them.
			Value: getDataPath(lib.BcaName),
			Usage: "path to configuration-directory",
		},
		cli.BoolFlag{
			Name:   "wait, w",
			EnvVar: "BC_WAIT",
			Usage:  "wait for transaction available in all nodes",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		lib.ConfigPath = c.String("config")
		return nil
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	err := cliApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func authorize(c *cli.Context) error {
	if c.NArg() < 2 {
		return xerrors.New("please give: private.toml byzcoin-id")
	}

	cfg, err := app.LoadCothority(c.Args().First())
	if err != nil {
		return xerrors.Errorf("loading cothority: %v", err)
	}
	si, err := cfg.GetServerIdentity()
	if err != nil {
		return xerrors.Errorf("getting server identity: %v", err)
	}

	bc, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return xerrors.Errorf("decoding byzcoin-id: %v", err)
	}
	log.Infof("Contacting %s to authorize byzcoin %x", si.Address, bc)
	cl := calypso.NewClient(nil)
	return cl.Authorize(si, bc)
}

// Runs a Distributed Key Generation, which is based on an LTS instance. If the
// --export option is provided, the hexadecimal string representation of the
// public key X is redirected to STDOUT.
func dkgStart(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return xerrors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.Errorf("failed to load config: %v", err)
	}

	instidstr := c.String("instid")
	if instidstr == "" {
		return xerrors.New("please provide an LTS instance ID with --instid")
	}

	instid, err := hex.DecodeString(instidstr)
	if err != nil {
		return xerrors.Errorf("failed to decode LTS instance id: %v", err)
	}

	resp, err := cl.GetProof(instid)
	if err != nil {
		return xerrors.Errorf("failed to get proof: %v", err)
	}
	exist, err := resp.Proof.InclusionProof.Exists(instid)
	if err != nil {
		return xerrors.Errorf("failed to get inclusion proof: %v", err)
	}
	if !exist {
		return xerrors.New("proof for the given \"--instid\" not found")
	}

	reply := &calypso.CreateLTSReply{}
	oc := onet.NewClient(cothority.Suite, calypso.ServiceName)
	err = oc.SendProtobuf(cl.Roster.List[0], &calypso.CreateLTS{
		Proof: resp.Proof,
	}, reply)
	if err != nil {
		return xerrors.Errorf("failed to send create LTS protobof: %v", err)
	}

	// Get the public key (X) as a string
	keyBuf, err := reply.X.MarshalBinary()
	if err != nil {
		return xerrors.Errorf("failed to marshal X: %v", err)
	}
	keyStr := hex.EncodeToString(keyBuf)

	err = lib.WaitPropagation(c, cl)
	if err != nil {
		return xerrors.Errorf("waiting for blocks to be propagated: %v", err)
	}

	if c.Bool("export") {
		reader := bytes.NewReader([]byte(keyStr))
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return xerrors.Errorf("failed to copy to stdout: %v", err)
		}
		return nil
	}

	log.Infof("LTS created:\n"+
		"- ByzcoinID: %x\n- InstanceID: %x\n- X: %s",
		reply.ByzCoinID, reply.InstanceID.Slice(), keyStr)

	return nil
}

// dkgInfo - prints information about the lts stored in the given instance
func dkgInfo(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return xerrors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.New("failed to load config: " + err.Error())
	}

	instidstr := c.String("instid")
	if instidstr == "" {
		return xerrors.New("please provide an LTS instance ID with --instid")
	}

	instid, err := hex.DecodeString(instidstr)
	if err != nil {
		return xerrors.New("failed to decode LTS instance id: " + err.Error())
	}

	resp, err := cl.GetProof(instid)
	if err != nil {
		return xerrors.New("failed to get proof: " + err.Error())
	}
	exist, err := resp.Proof.InclusionProof.Exists(instid)
	if err != nil {
		return xerrors.New("failed to get inclusion proof: " + err.Error())
	}
	if !exist {
		return xerrors.New("proof for the given \"--instid\" not found")
	}
	val, cid, _, err := resp.Proof.Get(instid)
	if err != nil {
		return xerrors.New("couldn't get values: " + err.Error())
	}
	if cid != calypso.ContractLongTermSecretID {
		return xerrors.New("given instanceID is not from an LTS contract")
	}
	var ltsInfo calypso.LtsInstanceInfo
	err = protobuf.Decode(val, &ltsInfo)
	if err != nil {
		return xerrors.New("couldn't decode info: " + err.Error())
	}
	log.Info("lts-roster is: ", ltsInfo.Roster.List)
	return nil
}

// reencrypt decrypts the encrypted secret of a write instance and re-encrypts
// it under the specified key of the write instance. If the proofs of the write
// and read instances are correct, it then outputs a DecryptKeyReply. With the
// --export option, the reply is protobuf encoded and sent to STDOUT.
func reencrypt(c *cli.Context) error {

	bcArg := c.String("bc")
	if bcArg == "" {
		return xerrors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.Errorf("failed to load config: %v", err)
	}

	// needed to get the block interval for WaitProof
	chainConfig, err := cl.GetChainConfig()
	if err != nil {
		return xerrors.Errorf("failed to get chain config: %v", err)
	}

	// Get and check the proof of an instance given an argument's name that
	// contains the instance id as hexadecimal string.
	getProof := func(iidArgs string) (*byzcoin.Proof, error) {
		iidStr := c.String(iidArgs)
		if iidStr == "" {
			return nil, fmt.Errorf("please provide the "+
				"instance id with --%s", iidArgs)
		}
		iid, err := hex.DecodeString(iidStr)
		if err != nil {
			return nil, xerrors.Errorf("failed to decode instance id: %v", err)
		}
		proof, err := cl.WaitProof(byzcoin.NewInstanceID(iid),
			chainConfig.BlockInterval*10, nil)
		if err != nil {
			return nil, xerrors.Errorf("couldn't get proof: %v", err)
		}
		exist, err := proof.InclusionProof.Exists(iid)
		if err != nil {
			return nil,
				xerrors.Errorf("error while checking if proof exist: %v", err)
		}
		if !exist {
			return nil, xerrors.New("proof not found")
		}
		match := proof.InclusionProof.Match(iid)
		if !match {
			return nil, xerrors.New("proof does not match")
		}

		return proof, nil
	}

	writeProof, err := getProof("writeid")
	if err != nil {
		return xerrors.Errorf("failed to get write proof: %v", err)
	}
	readProof, err := getProof("readid")
	if err != nil {
		return xerrors.Errorf("failed to get read proof: %v", err)
	}

	decryptKey := &calypso.DecryptKey{Write: *writeProof, Read: *readProof}

	reply := &calypso.DecryptKeyReply{}
	oc := onet.NewClient(cothority.Suite, calypso.ServiceName)
	err = oc.SendProtobuf(cl.Roster.List[0], decryptKey, reply)
	if err != nil {
		return xerrors.Errorf("failed to send protobuf decryptkey: %v", err)
	}

	if c.Bool("export") {
		// In case the --export option is provided, the DecryptKeyReply is
		// encoded and sent to STDOUT.
		buf, err := protobuf.Encode(reply)
		if err != nil {
			return xerrors.Errorf("failed to encode reply: %v", err)
		}
		reader := bytes.NewReader(buf)
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return xerrors.Errorf("failed to copy to stdout: %v", err)
		}
		return nil
	}

	log.Infof("Got decrypt reply:\n"+
		"- C: %s\n"+
		"- xHat: %s\n"+
		"- X: %s", reply.C, reply.XhatEnc, reply.X)

	return nil
}

// decrypt decrypts a re-encrypted secret stored in a DecryptKeyReply. It
// expects the DecryptKeyReply to be protobuf encoded and passed in STDIN. With
// the --export option, the recovered secret is sent to STDOUT.
func decrypt(c *cli.Context) error {
	decryptKeyReplyBuf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return xerrors.Errorf("failed to read from stdin: %v", err)
	}

	dkr := calypso.DecryptKeyReply{}
	err = protobuf.Decode(decryptKeyReplyBuf, &dkr)
	if err != nil {
		return xerrors.Errorf("failed to decode decryptKeyReply: %v", err)
	}

	bcArg := c.String("bc")
	if bcArg == "" {
		return xerrors.New("--bc flag is required")
	}

	cfg, _, err := lib.LoadConfig(bcArg)
	if err != nil {
		return xerrors.Errorf("loading byzcoin config: %v", err)
	}

	keyPath := c.String("key")
	var signer *darc.Signer
	if keyPath == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadSigner(keyPath)
	}
	if err != nil {
		return xerrors.Errorf("failed to load key file: %v", err)
	}

	xc, err := signer.GetPrivate()
	if err != nil {
		return xerrors.Errorf("failed to get private key: %v", err)
	}

	key, err := dkr.RecoverKey(xc)
	if err != nil {
		return xerrors.Errorf("failed to recover the key: %v", err)
	}

	if c.Bool("export") {
		reader := bytes.NewReader(key)
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return xerrors.Errorf("failed to copy to stdout: %v", err)
		}
		return nil
	}

	log.Infof("Key decrypted:\n%x", key)

	return nil
}
