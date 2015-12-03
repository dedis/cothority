package main

import (
	"bytes"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"net"
)

// This file handles the checking of a host who wants to join the cothority
// tree
// Basically, it will contact the host, waiting for its message containing some
// basics information about its system, and the signature associated

func init() {
	command := cli.Command{
		Name:        "check",
		Aliases:     []string{"c"},
		Usage:       "Checks if a given CoNode is valid in order to be incorporated into a cothority tree",
		Description: "It checks the public key given and the availability of the server. It will be contacted multiple times a day during 24 hours",
		ArgsUsage:   "Public-key-file : file where reside the public key of the host to check",
		Subcommands: []cli.Command{
			{
				Name:  "exit",
				Usage: "Asks the remote node to exit",
				Action: func(c *cli.Context) {
					if c.Args().First() == "" {
						dbg.Fatal("No public key file given for exit.")
					}
					CheckExit(c.Args().First())
				},
			},
		},
		Action: func(c *cli.Context) {
			if c.Args().First() == "" {
				dbg.Fatal("No public key file given for check.")
			}
			Check(c.Args().First())
		},
	}
	registerCommand(command)
}

// Main entry point for the check mode
func Check(pubKeyFile string) {
	// Verifies the remote host and returns the status
	conn, ack := verifyHost(pubKeyFile)
	defer conn.Close()

	if err := suite.Write(conn, ack); err != nil {
		dbg.Fatal("Error writing back the ACK : ", err)
	}
}

// Main entry point for the check mode
func CheckExit(pubKeyFile string) {
	// Verifies the remote host and returns the status
	conn, ack := verifyHost(pubKeyFile)
	defer conn.Close()

	// We only ask the node to exit if everything is OK
	if ack.Code != SYS_OK {
		dbg.Fatal("Not correct key-file?")
	} else {
		ack.Code = SYS_EXIT
	}

	// Finally send the message
	dbg.Lvl1("Sending exit to node")
	if err := suite.Write(conn, ack); err != nil {
		dbg.Fatal("Error asking for exit : ", err)
	}
}

// verifyHost will anaylze the systempacket information and verify the signature
// It will return a ACK properly initialized with the right codes in it.
func verifyHost(pubKeyFile string) (net.Conn, Ack) {
	//  get the right public key
	pub, host, err := cliutils.ReadPubKey(suite, pubKeyFile)
	if err != nil {
		dbg.Fatal("Could not read the public key from the file : ", err)
	}
	dbg.Lvl1("Public key file read")

	// Then get a connection
	conn, err := net.Dial("tcp", host)
	if err != nil {
		dbg.Fatal("Error when getting the connection to the host : ", err)
	}

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

	// First, encode the sys packet
	var b bytes.Buffer
	if err := suite.Write(&b, sys); err != nil {
		dbg.Fatal("Error when encoding the syspacket to be verified : ", err)
	}
	X := make([]abstract.Point, 1)
	X[0] = pub

	// Verify signature
	var ack Ack
	ack.Type = TYPE_SYS
	ack.Code = SYS_EXIT
	if _, err := anon.Verify(suite, b.Bytes(), anon.Set(X), nil, sig); err != nil {
		// Wrong signature
		ack.Code = SYS_WRONG_SIG
		dbg.Lvl1("WARNING : signature provided is wrong.")
	} else {
		// verfiy SystemPacket itself
		ack.Code = SYS_OK
		dbg.Lvl1("Host's signature verified and system seems healty. OK")
	}

	return conn, ack
}
