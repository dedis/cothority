/*
* Stamp - works together with a cothority-tree to sign a file. It can also verify
* that a signature is valid.
*
* # Signature
* For use in signature, run
* ```./stamp -stamp <file>```
* It will connect to the stampserver running on the localhost. If you want to
* connect to another stampserver, you can give the address with the ```-server```
* argument.
* At the end a file signature.sig will be generated which holds all necessary
* information necessary to check the signature.
*
* # Verification
* If you want to verify whether a file is correctly signed, you can run
* ```./stamp -verify <file.sig>```
* which will tell whether the signature is valid. If the file referenced in the
* file.sig is in the current directoy, it will also check it's hash.
 */

package main

import (
	"bytes"
	"encoding/base64"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
)

// Default config file
const defaultConfigFile = "config.toml"

// Default port where conodes listens
const defaultPort = "2001"

// extension given to a signature file
const sigExtension = ".sig"

// Our crypto-suite used in the program
var suite abstract.Suite

// the configuration file of the cothority tree used
var conf *app.ConfigConode

// The public aggregate X0
var public_X0 abstract.Point

// If the server is only given with it's hostname, it supposes that the stamp
// server is run on port 2001. Else you will have to add the port yourself.
func main() {
	stamp := cli.NewApp()
	stamp.Name = "collective"
	stamp.Usage = "Used to sign files to a cothority tree and to verify issued signatures"
	stamp.Version = "0.0.1"
	stamp.Authors = []cli.Author{
		{
			Name:  "Linus Gasser",
			Email: "linus.gasser@epfl.ch",
		},
		{
			Name:  "nikkolasg",
			Email: "",
		},
	}
	stamp.Commands = []cli.Command{
		{
			Name:    "sign",
			Aliases: []string{"s"},
			Usage:   "Request a signed time-stamp on a file. Provide with FILE.",
			Action: func(c *cli.Context) {
				dbg.Lvl1("Requesting a timestamp on a cothority tree")
				server := c.String("server")
				StampFile(c.Args().First(), server)
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "server, s",
					Value: "",
					Usage: "Server in the cothority tree we wish to contact. If not given, it will select a random one.",
				},
			},
		},
		{
			Name:      "check",
			Aliases:   []string{"c"},
			Usage:     "Verify a given signature against a file",
			ArgsUsage: "FILE is the name of the file. Signature file should be file.sig otherwise use the sig option",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "sig",
					Value: "",
					Usage: "signature file to verify",
				},
			},
			Action: func(c *cli.Context) {
				sigFile := c.String("sig")
				if sigFile == "" {
					sigFile = c.Args().First() + sigExtension
				}
				if VerifyFileSignature(c.Args().First(), sigFile) {
					dbg.Lvl1("Verification OK")
				} else {
					dbg.Lvl1("Verification of file failed")
				}
			},
		},
	}
	stamp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: defaultConfigFile,
			Usage: "Configuration file of the cothority tree we are using.",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug level from 1 (major operations) to 5 (very noisy text)",
		},
	}
	// Read the config file before
	stamp.Before = func(c *cli.Context) error {
		var cf string = c.String("config")
		if c.String("config") == "" {
			cf = defaultConfigFile
		}
		conf = new(app.ConfigConode)
		err := app.ReadTomlConfig(conf, cf)
		suite = app.GetSuite(conf.Suite)
		pub, _ := base64.StdEncoding.DecodeString(conf.AggPubKey)
		suite.Read(bytes.NewReader(pub), &public_X0)

		// sets the right debug options
		dbg.DebugVisible = c.GlobalInt("debug")
		return err
	}

	stamp.Run(os.Args)
}

// Takes a 'file' to hash and being stamped at the 'server'. The output of the
// signing will be written to 'file'.sig
func StampFile(file, server string) {
	// Create the hash of the file and send it over the net
	myHash := hashFile(file)

	stamper, err := conode.NewStamp("config.toml")
	if err != nil {
		dbg.Fatal("Couldn't setup stamper:", err)
	}
	tsm, err := stamper.GetStamp(myHash, server)
	if err != nil {
		dbg.Fatal("Stamper didn't succeed:", err)
	}

	if err := tsm.Srep.Save(file + sigExtension); err != nil {
		dbg.Fatal("Could not write signature file : ", err)
	}
	dbg.Lvl1("Signature file", file+".sig", "written.")

	dbg.Lvl1("Stamp OK - signature file", file+".sig", "written.")
}

// Verify signature takes a file name and the name of the signature file
// if signature file is empty ( sigFile == ""), then the signature file is
// simply the name of the file appended with ".sig" extension.
func VerifyFileSignature(file, sigFile string) bool {
	if file == "" {
		dbg.Fatal("Can not verify anything with an empty file name !")
	}

	// by default
	if sigFile == "" {
		sigFile = file + sigExtension
	}
	// read the sig
	signature := conode.StampSignature{
		SuiteStr: suite.String(),
	}
	if err := signature.Open(sigFile); err != nil {
		dbg.Fatal("Couldn't read signature-file", sigFile, " : ", err)
	}
	hash := hashFile(file)
	// Then verify the proper signature
	return conode.VerifySignature(suite, &signature, public_X0, hash)
}

// Takes a file to be hashed - reads in chunks of 1MB
func hashFile(name string) []byte {
	hash := suite.Hash()
	file, err := os.Open(name)
	if err != nil {
		dbg.Fatal("Couldn't open file", name)
	}

	buflen := 1024 * 1024
	buf := make([]byte, buflen)
	read := buflen
	for read == buflen {
		read, err = file.Read(buf)
		if err != nil && err != io.EOF {
			dbg.Fatal("Error while reading bytes")
		}
		hash.Write(buf)
	}
	return hash.Sum(nil)
}
