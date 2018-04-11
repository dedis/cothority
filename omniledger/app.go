// Package main is an app to interact with a lleap service. It can set up
// a new skipchain, store key/value pairs and retrieve values given a key.
package main

import (
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dedis/student_18_omniledger/omniledger/service"
	"gopkg.in/dedis/onet.v2/app"

	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "LLEAP kv"
	cliApp.Usage = "Key/value storage for LLEAP project"
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

	if c.NArg() != 2 {
		return errors.New("please give: group.toml public.key")
	}
	group := readGroup(c)
	client := service.NewClient()
	keyStr, err := ioutil.ReadFile(c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't read key-file: " + err.Error())
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(keyStr)))
	if err != nil {
		return errors.New("couldn't decode key-file: " + err.Error())
	}
	resp, err := client.CreateSkipchain(group.Roster, key)
	if err != nil {
		return errors.New("during creation of skipchain: " + err.Error())
	}
	log.Infof("Created new skipchain on roster %s with ID: %x", group.Roster.List, resp.Skipblock.Hash)
	return nil
}

// set stores a key/value pair on the given skipchain.
func set(c *cli.Context) error {
	log.Error("Not tested! Will not work!")
	if c.NArg() != 5 {
		return errors.New("please give: group.toml skipchain-ID private.key key value")
	}
	group := readGroup(c)
	scid, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return err
	}
	privStr, err := ioutil.ReadFile(c.Args().Get(2))
	if err != nil {
		return errors.New("couldn't read file of private key: " + err.Error())
	}
	privTrimmed := strings.TrimSpace(string(privStr))
	privByte, err := hex.DecodeString(privTrimmed)
	if err != nil {
		return errors.New("couldn't decode private key: " + err.Error())
	}
	priv, err := x509.ParsePKCS1PrivateKey(privByte)
	if err != nil {
		return errors.New("couldn't parse private key: " + err.Error())
	}

	key := c.Args().Get(3)
	value := c.Args().Get(4)
	resp, err := service.NewClient().SetKeyValue(group.Roster, scid, priv,
		[]byte(key), []byte(value))
	if err != nil {
		return errors.New("couldn't set new key/value pair: " + err.Error())
	}
	log.Infof("Successfully set new key/value pair in block: %x", resp.SkipblockID)
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
	resp, err := service.NewClient().GetValue(group.Roster, scid, []byte(key))
	if err != nil {
		return errors.New("couldn't get value: " + err.Error())
	}
	log.Infof("Read value: %x = %x", key, *resp.Value)
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
