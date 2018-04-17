// Package main is an app to interact with a lleap service. It can set up
// a new skipchain, store key/value pairs and retrieve values given a key.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

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
	txStr, err := ioutil.ReadFile(c.Args().Get(1))
	if err != nil {
		return errors.New("couldn't read transaction-file: " + err.Error())
	}
	tx := &service.Transaction{}
	err = json.Unmarshal([]byte(txStr), tx)
	if err != nil {
		return errors.New("couldn't decode transaction-file: " + err.Error())
	}
	resp, err := client.CreateSkipchain(group.Roster, *tx)
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
	tx := service.Transaction{
		Kind:  []byte(kind),
		Key:   []byte(key),
		Value: []byte(value),
	}
	resp, err := service.NewClient().SetKeyValue(group.Roster, scid, tx)
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
