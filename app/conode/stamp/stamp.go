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
	"flag"
	"github.com/dedis/cothority/app/conode/defs"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
	"strings"
)

// Flag-variables
var stamp = ""
var server = "localhost"
var check = ""
var configFile = "config.toml"

func init() {
	flag.StringVar(&stamp, "stamp", stamp, "Stamp that file")
	flag.StringVar(&check, "verify", check, "Verify that a signature-file contains a valid signature")
	flag.StringVar(&server, "server", server, "The server to connect to [localhost]")
	flag.StringVar(&configFile, "config", configFile, "Configuration file of the tree used")
}

// For the file-output we want a structure with base64-encoded strings, so it can be
// easily copy/pasted
type SignatureFile struct {
	// name of the file
	Name string
	// hash of our file
	Hash string
	// the inclusion-proof
	Proof string
	// signature returned by the root-node
	Signature string
}

// Our crypto-suite used in the program
var suite abstract.Suite

// the configuration file of the cothority tree used
var conf *app.ConfigConode

// If the server is only given with it's hostname, it supposes that the stamp
// server is run on port 2001. Else you will have to add the port yourself.
func main() {
	flag.Parse()
	if !strings.Contains(server, ":") {
		server += ":2001"
	}
	conf = new(app.ConfigConode)
	app.ReadTomlConfig(conf, configFile)
	suite = app.GetSuite(conf.Suite)
	dbg.Printf("Suite used is : %v", suite)
	switch {
	case stamp != "":
		StampFile(stamp, server)
	case check != "":
		VerifySignature(check)
	}

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
		dbg.Fatal("Error while receiving signature")
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

// Takes a signature-file and checks the information therein. If the file referenced
// in 'sigFile' is available, the hash is also checkd.
func VerifySignature(sigFile string) bool {
	err, file, hashOrig, reply := ReadSignatureFile(sigFile)
	if err != nil {
		dbg.Fatal("Couldn't read signature-file", sigFile)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		dbg.Lvl1("Didn't find the signed file in our directory:", file)
	} else {
		hash := hashFile(file)
		if bytes.Compare(hash, hashOrig) == 0 {
			dbg.Lvl1("Hash-check: passed")
		} else {
			dbg.Lvl1("Hash-check: FAILED")
			dbg.Lvl1("If you want to check the correctness of the signature, please\n"+
				"remove the file", file)
			return false
		}
	}
	return verifySignature(hashOrig, reply)
}

// Verifies that the 'message' is included in the signature and that it
// is correct.
func verifySignature(message hashid.HashId, reply *defs.StampReply) bool {
	dbg.Lvl1("Checking signature")
	pub, err := cliutils.ReadPub64(strings.NewReader(conf.AggPubKey), suite)
	if err != nil {
		dbg.Fatal("Could not read aggregate public key from config file")
	}
	sig := defs.BasicSignature{
		Chall: reply.SigBroad.C,
		Resp:  reply.SigBroad.R0_hat,
	}
	if err := SchnorrVerify(suite, []byte(message), pub, sig); err != nil {
		dbg.Lvl1("Schnorr verification failed. ", err)
		return false
	}
	dbg.Lvl1("Schnorr verification succeeded !")
	return true
}

//TAKEN FROM SIG_TEST from abstract
func SchnorrVerify(suite abstract.Suite, message []byte, publicKey abstract.Point, sig defs.BasicSignature) error {

	r := sig.Resp
	c := sig.Chall

	// Compute base**(r + x*c) == T
	var P, T abstract.Point
	P = suite.Point()
	T = suite.Point()
	T.Add(T.Mul(nil, r), P.Mul(publicKey, c))

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
	sigStr := &SignatureFile{
		Name:      file,
		Hash:      base64.StdEncoding.EncodeToString(hash),
		Proof:     base64.StdEncoding.EncodeToString([]byte(p)),
		Signature: base64.StdEncoding.EncodeToString(stamp.I0),
	}

	// Print to the screen, and write to file
	dbg.Printf("Signature-file will be:\n%+v", sigStr)

	app.WriteTomlConfig(sigStr, nameSig)
	return nil
}

// The inverse of 'WriteSignatureFile' where each field of the toml-file is
// decoded and put back in a 'StampReply'-structure
func ReadSignatureFile(name string) (error, string, []byte, *defs.StampReply) {
	// Read in the toml-file
	sigStr := &SignatureFile{}
	err := app.ReadTomlConfig(sigStr, name)
	if err != nil {
		return err, "", nil, nil
	}

	reply := &defs.StampReply{}
	// Convert fields from Base64 to binary
	hash, err := base64.StdEncoding.DecodeString(sigStr.Hash)
	for _, pr := range strings.Fields(sigStr.Proof) {
		pro, err := base64.StdEncoding.DecodeString(pr)
		if err != nil {
			dbg.Lvl1("Couldn't decode proof:", pr)
			return err, "", nil, nil
		}
		reply.Prf = append(reply.Prf, pro)
	}
	reply.I0, err = base64.StdEncoding.DecodeString(sigStr.Signature)
	return nil, sigStr.Name, hash, reply
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
