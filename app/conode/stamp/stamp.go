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
	"errors"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/app/conode/defs"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
	"strings"
)

// Flag-variables
var stamp = ""
var server = "localhost"
var check = ""

// Default config file
const defaultConfigFile = "config.toml"

// Default port where conodes listens
const defaultPort = "2001"

// extension given to a signature file
const sigExtension = ".sig"

// Helper ffunction to get the signature file from a file name
func signatureFile(file string) string {
	return file + sigExtension
}

func init() {
	//flag.StringVar(&stamp, "stamp", stamp, "Stamp that file")
	//flag.StringVar(&check, "verify", check, "Verify that a signature-file contains a valid signature")
	//flag.StringVar(&server, "server", server, "The server to connect to [localhost]")
	//flag.StringVar(&configFile, "config", configFile, "Configuration file of the tree used")
}

// For the file-output we want a structure with base64-encoded strings, so it can be
// easily copy/pasted
type SignatureFile struct {
	// name of the file
	Name string
	// hash of our file
	Hash string
	// the root of the merkle tree
	Root string
	// the inclusion-proof from root to the hash'd file
	Proof string
	// The signature challenge
	Challenge string
	// The signature response
	Response string
}

// Our crypto-suite used in the program
var suite abstract.Suite

// the configuration file of the cothority tree used
var conf *app.ConfigConode

// Actual definition of the Command Line Interface
func constructCli() *cli.App {
	stamp := cli.NewApp()
	stamp.Name = "collective"
	stamp.Usage = "Used to sign files to a cothority tree and to verify issued signatures"
	stamp.Version = "0.0.1"
	stamp.Authors = []cli.Author{
		{
			Name:  "ineiti",
			Email: "",
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
				server = c.String("server")
				if !strings.Contains(server, ":") {
					server += ":" + defaultPort
				}
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
				VerifySignature(c.Args().First(), c.String("sig"))
			},
		},
	}
	stamp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: defaultConfigFile,
			Usage: "Configuration file of the cothority tree we are using.",
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
		return err
	}
	return stamp
}

// If the server is only given with it's hostname, it supposes that the stamp
// server is run on port 2001. Else you will have to add the port yourself.
func main() {
	app := constructCli()
	app.Run(os.Args)
}

// Takes a 'file' to hash and being stamped at the 'server'. The output of the
// signing will be written to 'file'.sig
func StampFile(file, server string) {
	// Create the hash of the file and send it over the net
	myHash := hashFile(file)

	// First get a connection
	dbg.Lvl1("Connecting to", server)
	conn := coconet.NewTCPConn(server)
	err := conn.Connect()
	if err != nil {
		dbg.Fatal("Error when getting the connection to the host:", err)
	}

	msg := &defs.TimeStampMessage{
		Type:  defs.StampRequestType,
		ReqNo: 0,
		Sreq:  &defs.StampRequest{Val: myHash}}

	err = conn.Put(msg)
	if err != nil {
		dbg.Fatal("Couldn't send hash-message to server: ", err)
	}

	// Wait for the signed message
	tsm := &defs.TimeStampMessage{}
	tsm.Srep = &defs.StampReply{}
	tsm.Srep.Suite = suite
	err = conn.Get(tsm)
	if err != nil {
		dbg.Fatal("Error while receiving signature:", err)
	}
	dbg.Printf("%+v", tsm.Srep)

	// Asking to close the connection
	err = conn.Put(&defs.TimeStampMessage{
		ReqNo: 1,
		Type:  defs.StampClose,
	})
	conn.Close()

	// Verify if what we received is correct
	if !verifySignature(myHash, tsm.Srep) {
		dbg.Fatal("Verification of signature failed")
	}

	// Write the signature to the file
	err = WriteSignatureFile(file+".sig", stamp, myHash, tsm.Srep)
	if err != nil {
		dbg.Fatal("Couldn't write file", err)
	}

	dbg.Print("All done - file is written")
}

// Verify signature takes a file name and the name of the signature file
// if signature file is empty ( sigFile == ""), then the signature file is
// simply the name of the file appended with ".sig" extension.
func VerifySignature(file, sigFile string) bool {
	if file == "" {
		dbg.Fatal("Can not verify anything with an empty file name !")
	}

	// by default
	if sigFile == "" {
		sigFile = signatureFile(file)
	}
	// read the sig
	hashOrig, reply, err := ReadSignatureFile(sigFile)
	if err != nil {
		dbg.Fatal("Couldn't read signature-file", sigFile, " : ", err)
	}
	// compute the hash again to verify if hash is good
	hash := hashFile(file)
	if bytes.Compare(hash, hashOrig) == 0 {
		dbg.Lvl1("Hash-check: OK")
	} else {
		dbg.Lvl1("Hash-check: FAILED")
		return false
	}
	// Then verify the proper signature
	return verifySignature(hashOrig, reply)
}

// Verifies that the 'message' is included in the signature and that it
// is correct.
// Message is your own hash, and reply contains the inclusion proof + signature
// on the aggregated message
func verifySignature(message hashid.HashId, reply *defs.StampReply) bool {
	sig := defs.BasicSignature{
		Chall: reply.SigBroad.C,
		Resp:  reply.SigBroad.R0_hat,
	}
	public, _ := cliutils.ReadPub64(strings.NewReader(conf.AggPubKey), suite)
	if err := SchnorrVerify(suite, reply.MerkleRoot, public, sig); err != nil {
		dbg.Lvl1("Signature-check : FAILED (", err, ")")
		return false
	}
	dbg.Lvl1("Signature-check : OK")

	// Verify inclusion proof
	if !proof.CheckProof(suite.Hash, reply.MerkleRoot, message, reply.Prf) {
		dbg.Lvl1("Inclusion-check : FAILED")
		return false
	}
	dbg.Lvl1("Inclusion-check : OK")
	return true
}

// A simple verification of a schnorr signature given the message
//TAKEN FROM SIG_TEST from abstract
func SchnorrVerify(suite abstract.Suite, message []byte, publicKey abstract.Point, sig defs.BasicSignature) error {
	r := sig.Resp
	c := sig.Chall

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := suite.Point()
	V_clean := suite.Point()
	V_clean.Add(V_clean.Mul(nil, r), Aux.Mul(publicKey, c))
	// T is the recreated V_hat
	T := suite.Point().Null()
	T.Add(T, V_clean)

	//// Compute base**(r + x*c) == T
	//var P, T abstract.Point
	//P = suite.Point()
	//T = suite.Point()
	//T.Add(T.Mul(nil, r), P.Mul(publicKey, c))

	// Verify that the hash based on the message and T
	// matches the challange c from the signature
	// copy of hashSchnorr
	bufPoint, _ := T.MarshalBinary()
	cipher := suite.Cipher(bufPoint)
	cipher.Message(nil, nil, message)
	hash := suite.Secret().Pick(cipher)
	if !hash.Equal(sig.Chall) {
		return errors.New("invalid signature")
	}
	return nil
}

// Takes the different part of the signature and writes them to a toml-
// file in copy/pastable base64
func WriteSignatureFile(nameSig, file string, hash []byte, stamp *defs.StampReply) error {
	p := ""
	dbg.Printf("%+v", stamp.Prf)
	for _, pr := range stamp.Prf {
		p += base64.StdEncoding.EncodeToString(pr) + " "
	}
	// Write challenge and response part
	var bufChall bytes.Buffer
	var bufResp bytes.Buffer
	if err := cliutils.WriteSecret64(&bufChall, suite, stamp.SigBroad.C); err != nil {
		dbg.Fatal("Could not write secret challenge :", err)
	}
	if err := cliutils.WriteSecret64(&bufResp, suite, stamp.SigBroad.R0_hat); err != nil {
		dbg.Fatal("Could not write secret response : ", err)
	}
	// Signature file struct containing everything needed
	sigStr := &SignatureFile{
		Name:      file,
		Hash:      base64.StdEncoding.EncodeToString(hash),
		Proof:     base64.StdEncoding.EncodeToString([]byte(p)),
		Root:      base64.StdEncoding.EncodeToString(stamp.MerkleRoot),
		Challenge: bufChall.String(),
		Response:  bufResp.String(),
	}

	// Print to the screen, and write to file
	dbg.Printf("Signature-file will be:\n%+v", sigStr)

	app.WriteTomlConfig(sigStr, nameSig)
	return nil
}

// The inverse of 'WriteSignatureFile' where each field of the toml-file is
// decoded and put back in a 'StampReply'-structure
// Returns the hash of the file, the signature itself with all informations +
// error if any
func ReadSignatureFile(name string) ([]byte, *defs.StampReply, error) {
	// Read in the toml-file
	sigStr := &SignatureFile{}
	err := app.ReadTomlConfig(sigStr, name)
	if err != nil {
		return nil, nil, nil
	}

	reply := &defs.StampReply{}
	reply.SigBroad = sign.SignatureBroadcastMessage{}
	// Convert fields from Base64 to binary
	hash, err := base64.StdEncoding.DecodeString(sigStr.Hash)
	for _, pr := range strings.Fields(sigStr.Proof) {
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
	reply.SigBroad.R0_hat, err = cliutils.ReadSecretHex(suite, sigStr.Challenge)
	if err != nil {
		dbg.Fatal("Could not read secret challenge : ", err)
	}
	reply.SigBroad.C, err = cliutils.ReadSecretHex(suite, sigStr.Response)
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
