package main

import (
	"bytes"
	"flag"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"net"
)

// This file handles the checking of a host who wants to join the cothority
// tree
// Basically, it will contact the host, waiting for its message containing some
// basics information about its system, and the signature associated

// Where to read the file for the host we want to check
var pubKeyFile string = namePub()

// The actual host to check
var host string

func init() {
	flag.StringVar(&pubKeyFile, "public", pubKeyFile, "File where the public key of the host to check resides.")
}

// Main entry point for the check mode
func Check() {
	// First get the host we want to contact
	host = flag.Arg(1)
	if host == "" {
		dbg.Fatal("No host given to check ...")
	}
	//  get the right public key
	pub, err := cliutils.ReadPubKey(suite, pubKeyFile)
	if err != nil {
		dbg.Fatal("Could not read the public key from the file : ", err)
	}

	// Then get a connection
	conn, err := net.Dial("tcp", host)
	if err != nil {
		dbg.Fatal("Error when getting the connection to the host : ", err)
	}
	defer conn.Close()
	dbg.Lvl1("Verifier connected to the host. Validation in progress...")
	// Get the system packet message
	var sys SystemPacket
	if err = suite.Read(conn, &sys); err != nil {
		dbg.Fatal("Error when reading the system packet message from host :", err)
	}
	// Get the signature length first
	var length int
	if err := suite.Read(conn, &length); err != nil {
		dbg.Fatal("Could not read length of the signature ...")
	}
	// Get the signature
	sig := make([]byte, length)
	if err := suite.Read(conn, &sig); err != nil {
		dbg.Fatal("Error reading the signature :", err)
	}

	// analyse the results and send back the corresponding ack
	ack := verifyHost(sys, sig, pub)
	if err = suite.Write(conn, ack); err != nil {
		dbg.Fatal("Error writing back the ACK : ", err)
	}
}

// verifyHost will anaylze the systempacket information and verify the signature
// Of course, it needs the public key to verify it
// It will return a ACK properly initialized with the right codes in it.
func verifyHost(sys SystemPacket, sig []byte, pub abstract.Point) Ack {
	var ack Ack
	ack.Type = TYPE_SYS
	// First, encode the sys packet
	var b bytes.Buffer
	if err := suite.Write(&b, sys); err != nil {
		dbg.Fatal("Error when encoding the syspacket to be verified : ", err)
	}
	X := make([]abstract.Point, 1)
	X[0] = pub
	// Verify signature
	if _, err := anon.Verify(suite, b.Bytes(), anon.Set(X), nil, sig); err != nil {
		// Wrong sig ;(
		ack.Code = SYS_WRONG_SIG
		dbg.Lvl1("WARNING : signature provided is wrong.")
	} else {
		// verfiy SystemPacket itself
		ack.Code = SYS_OK
		dbg.Lvl1("Host's signature verified and system seems healty. OK")
	}

	return ack
}
