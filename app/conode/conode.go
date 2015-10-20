package main

import (
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/app/conode/defs"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"os"
)

// Which suite to use
const suiteStr string = "ed25519"

var suite abstract.Suite = edwards.NewAES128SHA256Ed25519(true)

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

// Create the CLI of conode
func NewCli() *cli.App {
	conode := cli.NewApp()
	conode.Name = "Conode"
	conode.Usage = "Run a cothority server and contacts others conodes to form a cothority tree"
	conode.Version = "0.0.1"
	conode.Authors = []cli.Author{
		{
			Name:  "Linus Gasser",
			Email: "linus.gasser@epfl.ch",
		},
		{
			Name:  "nikkolasg",
			Email: "not provided yet",
		},
	}
	// already create the key gen command
	keyGen := cli.Command{
		Name:      "keygen",
		Aliases:   []string{"k"},
		Usage:     "Create a new key pair and binding the public part to your address. ",
		ArgsUsage: "ADRESS[:PORT] will be the address binded to the generated public key",
		Action: func(c *cli.Context) {
			if c.String("key") != "" {
				KeyGeneration(c.String("key"), c.Args().First())
			} else {
				KeyGeneration(defaultKeyFile, c.Args().First())
			}
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
	conode.Commands = commands
	conode.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Usage: "debug level from 1 (only major operations) to 5 (very noisy text)",
			Value: 1,
		},
	}
	// sets the right debug options
	conode.Before = func(c *cli.Context) error {
		dbg.DebugVisible = c.GlobalInt("debug")
		dbg.Print("Suite : ", suite.String())
		return nil
	}
	return conode
}

func main() {
	conode := NewCli()
	conode.Run(os.Args)
}

// KeyGeneration will generate a fresh public / private key pair
// and write those down into two separate files
func KeyGeneration(key, address string) {
	if address == "" {
		dbg.Fatal("You must call keygen with ipadress !")
	}
	address, err := cliutils.UpsertPort(address, defs.DefaultPort)
	if err != nil {
		dbg.Fatal(err)
	}
	// gen keypair
	kp := cliutils.KeyPair(suite)
	// Write private
	if err := cliutils.WritePrivKey(kp.Secret, suite, namePriv(key)); err != nil {
		dbg.Fatal("Error writing private key file : ", err)
	}

	// Write public
	if err := cliutils.WritePubKey(kp.Public, suite, namePub(key), address); err != nil {
		dbg.Fatal("Error writing public key file : ", err)
	}

	dbg.Lvl1("Keypair generated and written to ", namePriv(key), " / ", namePub(key))
}
