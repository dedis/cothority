// Package main is an app to interact with a lleap service. It can set up
// a new skipchain, store key/value pairs and retrieve values given a key.
package main

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/dedis/student_18_omniledger/omniledger/service"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/kyber.v2/util/encoding"
	"gopkg.in/dedis/kyber.v2/util/key"
	"gopkg.in/dedis/onet.v2/app"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "Omniledger app"
	cliApp.Usage = "Key/value storage for Omniledger"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		{
			Name:      "create",
			Usage:     "creates a new skipchain",
			Aliases:   []string{"c"},
			ArgsUsage: "group.toml public.key",
			Action:    create,
		},
		{
			Name:    "set",
			Usage:   "sets a key/value pair",
			Aliases: []string{"s"},
			Action:  set,
		},
		{
			Name:    "get",
			Usage:   "gets a value",
			Aliases: []string{"g"},
			Action:  get,
		},
		{
			Name:   "keypair",
			Usage:  "generate a key pair",
			Action: keypair,
		},
	}
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		return nil
	}
	log.ErrFatal(cliApp.Run(os.Args))
}

// Creates a new skipchain
func create(c *cli.Context) error {
	log.Info("Create a new skipchain")

	if c.NArg() != 3 {
		return errors.New("please give: group.toml public-key-in-hex private-key-in-hex")
	}
	group := readGroup(c)
	pkReader := strings.NewReader(c.Args().Get(1))
	pk, err := encoding.ReadHexPoint(cothority.Suite, pkReader)
	if err != nil {
		return err
	}
	skReader := strings.NewReader(c.Args().Get(2))
	sk, err := encoding.ReadHexScalar(cothority.Suite, skReader)
	if err != nil {
		return err
	}
	client := service.NewClient()
	signer := darc.NewSignerEd25519(pk, sk)
	msg, err := service.DefaultGenesisMsg(service.CurrentVersion, group.Roster, signer.Identity())
	if err != nil {
		return err
	}
	resp, err := client.CreateGenesisBlock(group.Roster, msg)
	if err != nil {
		return errors.New("during creation of skipchain: " + err.Error())
	}
	log.Infof("Created new skipchain on roster %s with ID: %x", group.Roster.List, resp.Skipblock.Hash)
	return nil
}

// set stores a key/value pair on the given skipchain.
func set(c *cli.Context) error {
	log.Error("Not tested! Will not work!")
	if c.NArg() != 6 {
		return errors.New("please give: group.toml skipchain-ID darc" +
			" kind key value")
	}
	group := readGroup(c)
	scid, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	// TODO: parse darc from c.Args().Get(2) and sign tx with it

	kind := c.Args().Get(3)
	key := c.Args().Get(4)
	value := c.Args().Get(5)
	tx := service.ClientTransaction{
		Instructions: []service.Instruction{{
			DarcID: darc.ID(key[0:32]),
			Nonce:  []byte(key[32:]),
			Kind:   kind,
			Data:   []byte(value),
		}},
	}
	_, err = service.NewClient().SetKeyValue(group.Roster, scid, tx)
	if err != nil {
		return errors.New("couldn't set new key/value pair: " + err.Error())
	}
	log.Info("Submitted new key/value")
	return nil
}

// get returns the value of the key but doesn't verify against the public
// key.
func get(c *cli.Context) error {
	log.Info("Get value")

	if c.NArg() != 3 {
		return errors.New("please give: group.toml skipchain-ID key")
	}
	group := readGroup(c)
	scid, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	key := c.Args().Get(2)
	resp, err := service.NewClient().GetProof(group.Roster, scid, []byte(key))
	if err != nil {
		return errors.New("couldn't get value: " + err.Error())
	}
	_, vs, err := resp.Proof.KeyValue()
	if err != nil {
		return err
	}
	log.Infof("Read value: %x = %x", key, vs[0])
	return nil
}

// readGroup decodes the group given in the file with the name in the
// first argument of the cli.Context.
func readGroup(c *cli.Context) *app.Group {
	name := c.Args().First()
	f, err := os.Open(name)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			name)
	}
	return group
}

// keypair generates a keypair.
// TODO we should include the option to store it in a file.
func keypair(c *cli.Context) error {
	kp := key.NewKeyPair(cothority.Suite)

	secStr, err := encoding.ScalarToStringHex(nil, kp.Private)
	if err != nil {
		return err
	}
	pubStr, err := encoding.PointToStringHex(nil, kp.Public)
	if err != nil {
		return err
	}
	log.Infof("Private: %s\nPublic: %s", secStr, pubStr)
	return nil
}
