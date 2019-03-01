package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/unicore"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// creates a new instance that can create contract from a executable file
func create(c *cli.Context) error {
	filepath := c.Args().Get(0)
	bb, err := ioutil.ReadFile(filepath)
	if err != nil {
		return errors.New("missing binary file path")
	}

	client, err := newClient(c)
	if err != nil {
		return err
	}

	return client.Create(bb)
}

func exec(c *cli.Context) error {
	client, err := newClient(c)
	if err != nil {
		return err
	}

	args := []byzcoin.Argument{}
	for i, a := range c.Args() {
		args = append(args, byzcoin.Argument{
			Name:  fmt.Sprintf("%d", i),
			Value: []byte(a),
		})
	}

	return client.Exec(args)
}

func state(c *cli.Context) error {
	client, err := newClient(c)
	if err != nil {
		return err
	}

	state, err := client.GetState(client.Instance)
	if err != nil {
		return err
	}

	fmt.Printf("%x\n", state)
	return nil
}

func newClient(c *cli.Context) (*unicore.Client, error) {
	bc := c.String("bc")
	if bc == "" {
		return nil, errors.New("--bc flag is required")
	}

	cfgBuf, err := ioutil.ReadFile(bc)
	if err != nil {
		return nil, err
	}
	var cfg unicore.BcConfig
	err = protobuf.Decode(cfgBuf, &cfg)
	if err != nil {
		return nil, err
	}

	// Create a client with the existing ByzCoin
	uc := unicore.NewClient(&cfg)

	iid := c.String("instance")
	if iid != "" {
		iidb, err := hex.DecodeString(iid)
		if err != nil {
			return nil, err
		}
		uc.Instance = byzcoin.NewInstanceID(iidb)
	}

	priv := c.String("key")
	if priv == "" {
		return nil, errors.New("--key flag is required")
	}

	privKey, err := ioutil.ReadFile(priv)
	if err != nil {
		return nil, err
	}

	var signer darc.Signer
	err = protobuf.Decode(privKey, &signer)
	if err != nil {
		return nil, err
	}

	// Prepare the signer used to create this byzcoin for further signatures
	err = uc.AddSigner(signer)
	if err != nil {
		return nil, err
	}

	// Get the DARC back
	darc, err := uc.ByzCoinClient.GetGenDarc()
	if err != nil {
		return nil, err
	}

	uc.DarcID = darc.BaseID

	return uc, nil
}
