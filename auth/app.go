/*
* This is a template for creating an app. It only has one command which
* prints out the name of the app.
 */
package main

import (
	"errors"
	"io/ioutil"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "Authentication"
	cliApp.Usage = "Handle who is allowed to access your conode"
	cliApp.Version = "0.1"
	cliApp.Commands = []cli.Command{
		cli.Command{
			Name:      "status",
			Aliases:   []string{"s"},
			Usage:     "return the policy-status of this conode",
			ArgsUsage: "(public.toml|ip:port)",
			Action:    status,
		},
		cli.Command{
			Name:    "keypair",
			Aliases: []string{"kp"},
			Usage:   "create a new ed25519-keypair",
			Action:  keypair,
		},
		cli.Command{
			Name:    "update",
			Aliases: []string{"u"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "pin,p",
					Usage: "indicate pin read from server",
				},
				cli.BoolFlag{
					Name:  "fetchPin,fp",
					Usage: "fetch pin from server",
				},
				cli.StringFlag{
					Name:  "private,pr",
					Usage: "indicate private.toml file from server",
				},
			},
			Subcommands: cli.Commands{
				cli.Command{
					Name:      "add",
					Usage:     "adds a new key to a darc",
					ArgsUsage: "policy public",
					Action:    updateAdd,
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:  "owner",
							Usage: "adds key as owner",
						},
						cli.BoolFlag{
							Name:  "user",
							Usage: "adds key as user",
						},
						cli.StringFlag{
							Name:  "ed25519,ed",
							Usage: "public key to store",
						},
						cli.StringFlag{
							Name:  "pop",
							Usage: "final_statement.toml of pop-party",
						},
					},
				},
			},
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

func status(c *cli.Context) error {
	if c.NArg() != 1 {
		return errors.New("please give (public.toml|ip:port)")
	}
	si, err := parseSI(c.Args().First())
	if err != nil {
		return err
	}
	log.Print(si)
	return nil
}

func keypair(c *cli.Context) error {
	kp := key.NewKeyPair(cothority.Suite)
	log.Infof("Public key: %x", kp.Public)
	log.Infof("Private key: %x", kp.Private)
	return nil
}

func updateAdd(c *cli.Context) error {
	return nil
}

func parseSI(siStr string) (si *network.ServerIdentity, err error) {
	if _, err = os.Stat(siStr); err == nil {
		var pub []byte
		pub, err = ioutil.ReadFile(siStr)
		if err != nil {
			return
		}
		siToml := &struct{ Servers []*network.ServerIdentityToml }{}
		_, err = toml.Decode(string(pub), siToml)
		if err != nil {
			return
		}
		si = siToml.Servers[0].ServerIdentity(cothority.Suite)
	} else {
		add := siStr
		_, _, err = net.SplitHostPort(add)
		if err != nil {
			return
		}
		si = &network.ServerIdentity{
			Address: network.NewAddress(network.PlainTCP, add),
		}
	}
	return
}
