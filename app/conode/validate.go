package main

import (
	"bytes"
	"github.com/dedis/cothority/lib/cliutils"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"net"
	"strconv"
)

// This file handles the validation process
// Basically it listen on default port 2000
// When a connection occurs, it will create a message containing some system
// stats such as the soft rlimits,
// Signs it, and returns the message + signature
// Then wait for an ACK or FIN msg. An ACK means all went well
// An FIN msg means something went wrong and you should contact
// the development team about it.

// Main entry point of the validation mode
func Validation() {

	// First, retrieve our public / private key pair
	kp := readKeyPair()
	// Then wait for the connection

	// Accept incoming connections
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(listenPort))
	if err != nil {
		dbg.Fatal("Could not listen for validation : ", err)
	}

	var conn net.Conn
	for ; ; conn.Close() {
		dbg.Lvl1("Will wait for the verifier connection ...")
		// Accept the one
		conn, err = ln.Accept()
		if err != nil {
			dbg.Fatal("Could not accept an input connection : ", err)
		}

		dbg.Lvl1("Verifier connected ! validation in progress...")
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
		if ack.Code == SYS_OK {
			dbg.Lvl1("Validation is done and correct ! You should receive an email from development team soon.")
		} else if ack.Code == SYS_WRONG_HOST {
			dbg.Lvl1(er + "HOSTNAME")
		} else if ack.Code == SYS_WRONG_SOFT {
			dbg.Lvl1(er + "SOFT limits")
		} else if ack.Code == SYS_WRONG_SIG {
			dbg.Lvl1(er + "Wrong signature !")
		} else {
			dbg.Lvl1("Validation received unknown ACK : type = ", ack.Type, " Code = ", ack.Code)
			continue
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
func readKeyPair() config.KeyPair {
	sec, err := cliutils.ReadPrivKey(suite, namePriv())
	if err != nil {
		dbg.Fatal("Could not read private key : ", err)
	}
	pub, _, err := cliutils.ReadPubKey(suite, namePub())
	if err != nil {
		dbg.Fatal("Could not read public key : ", err)
	}
	return config.KeyPair{
		Suite:  suite,
		Secret: sec,
		Public: pub,
	}
}
