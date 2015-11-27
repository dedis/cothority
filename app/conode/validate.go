package main

import (
	"bytes"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"net"
	"os"
)

// This file handles the validation process
// Basically it listen on default port 2000
// When a connection occurs, it will create a message containing some system
// stats such as the soft rlimits,
// Signs it, and returns the message + signature
// Then wait for an ACK or FIN msg. An ACK means all went well
// An FIN msg means something went wrong and you should contact
// the development team about it.

func init() {
	command := cli.Command{
		Name:    "validate",
		Aliases: []string{"v"},
		Usage:   "Starts validation mode of the CoNode",
		Description: "The CoNode will be running for a whole day during which" +
		             "the development team will run repeated checks to verify " +
		             "that your server is eligible for being incorporated in the cothority tree.",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "key, k",
				Usage: "KEY : the basename of where to find the public / private keys of this host to be verified.",
				Value: defaultKeyFile,
			},
		},
		Action: func(c *cli.Context) {
			Validation(c.String("key"))
		},
	}
	registerCommand(command)
}

// Main entry point of the validation mode
func Validation(keyFile string) {

	// First, retrieve our public / private key pair + address for which it has
	// been created
	kp, addr := readKeyFile(keyFile)
	// Then wait for the connection

	// Accept incoming connections
	global, _ := cliutils.GlobalBind(addr)
	ln, err := net.Listen("tcp", global)
	if err != nil {
		dbg.Fatal("Could not listen for validation : ", err)
	}

	var conn net.Conn
	for ;; conn.Close() {
		dbg.Lvl1("Waiting for verifier connection ...")
		// Accept the one
		conn, err = ln.Accept()
		if err != nil {
			dbg.Fatal("Could not accept an input connection : ", err)
		}

		dbg.Lvl1("Verifier connected! Validation in progress...")
		// Craft the message about our system,signs it, and then send the whole
		msg := createSystemPacket()
		signature := signSystemPacket(msg, kp)
		// We also send the size of the signature for the receiver to know how much
		// byte he is expecting
		if err := suite.Write(conn, msg, len(signature), signature); err != nil {
			dbg.Lvl1("Error when writing the system packet to the connection :", err)
			continue
		}

		// Receive the response
		var ack Ack
		if err := suite.Read(conn, &ack); err != nil {
			dbg.Lvl1("Error when reading the response :", err)
		}

		var er string = "Validation is NOT correct, something is wrong about your "
		// All went fine
		dbg.Lvl2("Received code", ack)
		switch ack.Code{
		default:
			dbg.Lvl1("Validation received unknown ACK : type = ", ack.Type, " Code = ", ack.Code)
			continue
		case SYS_OK:
			dbg.Lvl1("Validation finished successfully! You should receive an email from development team soon.")
		case SYS_WRONG_HOST:
			dbg.Lvl1(er + "HOSTNAME")
		case SYS_WRONG_SOFT:
			dbg.Lvl1(er + "SOFT limits")
		case SYS_WRONG_SIG:
			dbg.Lvl1(er + "signature!")
		case SYS_EXIT:
			dbg.Lvl1("Exiting - need to update to get config.toml")
			os.Exit(1)
		}
	}
}

// createSystemMessage will return a packet containing one or many information
// about the system. It is version dependant.
func createSystemPacket() SystemPacket {
	host := "myhostname"
	arr := [maxSize]byte{}
	copy(arr[:], host)
	return SystemPacket{
		Soft:     10000,
		Hostname: arr,
	}
}

// signSystemPacket will sign the packet using the crypto library with  package
// anon. No anonymity set here. Must pass the private / public keys to sign.
func signSystemPacket(sys SystemPacket, kp config.KeyPair) []byte {
	var buf bytes.Buffer
	if err := suite.Write(&buf, sys); err != nil {
		dbg.Fatal("Could not sign the system packet : ", err)
	}
	// setup
	X := make([]abstract.Point, 1)
	mine := 0
	X[mine] = kp.Public
	// The actual signing
	sig := anon.Sign(suite, random.Stream, buf.Bytes(), anon.Set(X), nil, mine, kp.Secret)
	return sig
}

// readKeyPair will read both private and public files
// and returns a keypair containing the respective private and public keys
func readKeyFile(keyFile string) (config.KeyPair, string) {
	sec, err := cliutils.ReadPrivKey(suite, namePriv(keyFile))
	if err != nil {
		dbg.Fatal("Could not read private key : ", err)
	}
	pub, addr, err := cliutils.ReadPubKey(suite, namePub(keyFile))
	if err != nil {
		dbg.Fatal("Could not read public key : ", err)
	}
	return config.KeyPair{
		Suite:  suite,
		Secret: sec,
		Public: pub,
	}, addr
}
