package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// Which suite to use
var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)
var suiteStr string = suite.String()

// where to write the key file .priv + .pub
var defaultKeyFile string = "key"

// Returns the name of the file for the private key
func namePriv(key string) string {
	return key + ".priv"
}

// Returns the name of the file for the public key
func namePub(key string) string {
	return key + ".pub"
}

// config file by default
const defaultConfigFile string = "config.toml"

///////////////////////
// will sotre each files / packages commands before creating the cli
var commands []cli.Command = make([]cli.Command, 0)

// register a new command to be added to the cli
func registerCommand(com cli.Command) {
	commands = append(commands, com)
}

func main() {
	coApp := cli.NewApp()
	coApp.Name = "CoNode"
	coApp.Usage = "Runs a cothority node and contacts others CoNodes to form a cothority tree"
	coApp.Version = "0.1.0"
	coApp.Authors = []cli.Author{
		{
			Name:  "Linus Gasser",
			Email: "linus.gasser@epfl.ch",
		},
		{
			Name:  "Nicolas Gailly",
			Email: "not specified",
		},
	}
	// already create the key gen command
	keyGen := cli.Command{
		Name:      "keygen",
		Aliases:   []string{"k"},
		Usage:     "Creates a new key pair and binds the public part to the specified IPv4 address and port",
		ArgsUsage: "ADRESS[:PORT] is the address (and port) bound to the generated public key.",
		Action: func(c *cli.Context) {
			KeyGeneration(c.String("key"), c.Args().First())
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: "key, k",
				Usage: "Basename of the files where reside the keys. If key = 'key'," +
				"then conode will search through 'key.pub' and 'key.priv'",
				Value: defaultKeyFile,
			},
		},
	}
	commands = append(commands, keyGen)
	coApp.Commands = commands
	coApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Usage: "debug level from 1 (only major operations) to 5 (very noisy text)",
			Value: 1,
		},
	}
	// sets the right debug options
	coApp.Before = func(c *cli.Context) error {
		dbg.DebugVisible = c.GlobalInt("debug")
		return nil
	}

	coApp.Run(os.Args)
}

// KeyGeneration will generate a fresh public / private key pair
// and write those down into two separate files
func KeyGeneration(key, address string) {
	if address == "" {
		dbg.Fatal("You must call keygen with ipadress !")
	}
	address, err := cliutils.VerifyPort(address, conode.DefaultPort)
	dbg.Lvl1("Address is", address)
	if err != nil {
		dbg.Fatal(err)
	}
	// gen keypair
	kp := cliutils.KeyPair(suite)
	// Write private
	if err := cliutils.WritePrivKey(suite, namePriv(key), kp.Secret); err != nil {
		dbg.Fatal("Error writing private key file : ", err)
	}

	// Write public
	if err := cliutils.WritePubKey(suite, namePub(key), kp.Public, address); err != nil {
		dbg.Fatal("Error writing public key file : ", err)
	}

	dbg.Lvl1("Keypair generated and written to ", namePriv(key), " / ", namePub(key))
}
