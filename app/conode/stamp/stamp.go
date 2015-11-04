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
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
	"strings"
)

// Default config file
const defaultConfigFile = "config.toml"

// Default port where conodes listens
const defaultPort = "2001"

// extension given to a signature file
const sigExtension = ".sig"

// For the file-output we want a structure with base64-encoded strings, so it can be
// easily copy/pasted
type SignatureFile struct {
	// name of the file
	Name       string
	// The time it has been timestamped
	Timestamp  int64
	// hash of our file
	Hash       string
	// the root of the merkle tree
	Root       string
	// the inclusion-proof from root to the hash'd file
	Proof      []string
	// The signature challenge
	Challenge  string
	// The signature response
	Response   string
	// The aggregated commitment used for signing
	Commitment string
}

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
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "Verify a given signature against a file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "sig",
					Value: "",
					Usage: "signature file to verify",
				},
			},
			Action: func(c *cli.Context) {
				dbg.Lvl2("Requesting a verification of a file given its signature. Provide with FILE.")
				if VerifyFileSignature(c.Args().First(), c.String("sig")) {
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

	// Write the signature to the file
	err = writeSignatureFile(file + ".sig", file, myHash, tsm.Srep)
	if err != nil {
		dbg.Fatal("Couldn't write file", err)
	}

	dbg.Lvl1("Stamp OK - signature file", file + ".sig", "written.")
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
	hashOrig, reply, err := readSignatureFile(sigFile)
	if err != nil {
		dbg.Fatal("Couldn't read signature-file", sigFile, " : ", err)
	}
	// compute the hash again to verify if hash is good
	hash := hashFile(file)
	if bytes.Compare(hash, hashOrig) == 0 {
		dbg.Lvl2("Hash-check: OK")
	} else {
		dbg.Lvl2("Hash-check: FAILED")
		return false
	}
	// Then verify the proper signature
	return conode.VerifySignature(suite, reply, public_X0, hash)
}

// Takes the different part of the signature and writes them to a toml-
// file in copy/pastable base64
func writeSignatureFile(nameSig, file string, hash []byte, stamp *conode.StampReply) error {
	var p []string
	for _, pr := range stamp.Prf {
		p = append(p, base64.StdEncoding.EncodeToString(pr))
	}
	// Write challenge and response + commitment part
	var bufChall bytes.Buffer
	var bufResp bytes.Buffer
	var bufCommit bytes.Buffer
	if err := cliutils.WriteSecret64(suite, &bufChall, stamp.SigBroad.C); err != nil {
		dbg.Fatal("Could not write secret challenge :", err)
	}
	if err := cliutils.WriteSecret64(suite, &bufResp, stamp.SigBroad.R0_hat); err != nil {
		dbg.Fatal("Could not write secret response : ", err)
	}
	if err := cliutils.WritePub64(suite, &bufCommit, stamp.SigBroad.V0_hat); err != nil {
		dbg.Fatal("Could not write aggregated commitment : ", err)
	}
	// Signature file struct containing everything needed
	sigStr := &SignatureFile{
		Name:       file,
		Timestamp:  stamp.Timestamp,
		Hash:       base64.StdEncoding.EncodeToString(hash),
		Proof:      p,
		Root:       base64.StdEncoding.EncodeToString(stamp.MerkleRoot),
		Challenge:  bufChall.String(),
		Response:   bufResp.String(),
		Commitment: bufCommit.String(),
	}

	// Print to the screen, and write to file
	dbg.Lvl2("Signature-file will be:\n%+v", sigStr)

	app.WriteTomlConfig(sigStr, nameSig)
	return nil
}

// The inverse of 'WriteSignatureFile' where each field of the toml-file is
// decoded and put back in a 'StampReply'-structure
// Returns the hash of the file, the signature itself with all informations +
// error if any
func readSignatureFile(name string) ([]byte, *conode.StampReply, error) {
	// Read in the toml-file
	sigStr := &SignatureFile{}
	err := app.ReadTomlConfig(sigStr, name)
	if err != nil {
		return nil, nil, nil
	}

	reply := &conode.StampReply{}
	reply.Timestamp = sigStr.Timestamp
	reply.SigBroad = sign.SignatureBroadcastMessage{} // Convert fields from Base64 to binary
	hash, err := base64.StdEncoding.DecodeString(sigStr.Hash)
	for _, pr := range sigStr.Proof {
		pro, err := base64.StdEncoding.DecodeString(pr)
		if err != nil {
			dbg.Lvl1("Couldn't decode proof:", pr)
			return nil, nil, err
		}
		reply.Prf = append(reply.Prf, pro)
	}
	// Read the root, the challenge and response
	reply.MerkleRoot, err = base64.StdEncoding.DecodeString(sigStr.Root)
	if err != nil {
		dbg.Fatal("Could not decode Merkle Root from sig file :", err)
	}
	reply.SigBroad.R0_hat, err = cliutils.ReadSecret64(suite, strings.NewReader(sigStr.Response))
	if err != nil {
		dbg.Fatal("Could not read secret challenge : ", err)
	}
	if reply.SigBroad.C, err = cliutils.ReadSecret64(suite, strings.NewReader(sigStr.Challenge)); err != nil {
		dbg.Fatal("Could not read the aggregate commitment :", err)
	}
	reply.SigBroad.V0_hat, err = cliutils.ReadPub64(suite, strings.NewReader(sigStr.Commitment))

	return hash, reply, err

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
