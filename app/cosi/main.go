package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"bytes"
	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
)

func printUsageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, "%s", msg)
	}

	fmt.Fprintf(os.Stderr, `Usage:
First make sure that you have a valid group toml file. Either create a
local one by running several instances of cosid (follow the instructions when
running cosid the first time) or use a group toml of an existing CoSi group.

To collectively sign a text message run:
	cosi -m “<Message to be signed>” -c <cosi-group.toml>
If you would instead like to sign a message contained in a file you can the following command:
	cosi -f <file-to-be-signed> -c <cosi-group.toml>
If will create a file file-to-be-signed.sig containing the hash of the file and the signature.

To verify the signature on a file add the verify flag (-v):
	cosi -f <file-to-be-signed> -c <cosi-group.toml> -v
This command opens the corresnponding .sig file and validates the contained signature.

Example usuage (create a file my-local-group.toml using cosid first):
cosi -m "Hello CoSi" -c my-local-group.toml`)
	os.Exit(1)
}

var f *flag.FlagSet
var m *flag.FlagSet

func init() {
	f = flag.NewFlagSet("f", flag.ContinueOnError)
	m = flag.NewFlagSet("m", flag.ContinueOnError)
	//dbg.SetDebugVisible(3)
}

func main() {
	lenArgs := len(os.Args)
	if !(5 <= lenArgs && lenArgs <= 6) {
		printUsageAndExit("Not enough arguments provided.\n")
	}
	switch os.Args[1] {
	case "-f":
		strOrFilename := f.String("f", "",
			"Filename of the file to be signed.")
		groupToml := f.String("c", "",
			"Toml file containing the list of CoSi nodes.")
		verify := f.Bool("v", false, "Verify the files signature.")
		if err := f.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing file. " +
				"Couldn't parse arguments:" + err.Error())
		}
		if *verify {
			err := verifyFileSig(*strOrFilename, *groupToml)
			printVerificationResult(err)
		} else {
			sig, err := signFile(*strOrFilename, *groupToml)
			handleErrorAndExit("Couldn't create signature", err)
			sigFileName := *strOrFilename + ".sig"
			outFile, err := os.Create(sigFileName)
			handleErrorAndExit("Couldn't create signature file", err)
			writeSigAsJSON(sig, outFile)
			fmt.Println("Signature written to: " + sigFileName)
		}
	case "-m":
		strOrFilename := m.String("m", "", "Message to be signed.")
		groupToml := m.String("c", "", "Toml file containing the list "+
			"of CoSi nodes.")
		if err := m.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing message" +
				"Couldn't parse arguments:" + err.Error())
		}
		sig, err := signString(*strOrFilename, *groupToml)
		handleErrorAndExit("Couldn't create signature", err)
		writeSigAsJSON(sig, os.Stdout)
	default:
		printUsageAndExit("")
	}
}

// SignStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(r io.Reader,
	el *sda.EntityList,
	verify bool) (*sda.CosiResponse, error) {

	msgB, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	e := network.NewEntity(kp.Public, "")
	client := network.NewSecureTcpHost(kp.Secret, e)
	req := &sda.CosiRequest{
		EntityList: el,
		Message:    msgB,
	}

	// Connect to the root
	host := el.List[0]
	dbg.Lvl3("Opening connection")
	con, err := client.Open(host)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	dbg.Lvl3("Sending sign request")
	// send the request
	if err := con.Send(context.TODO(), req); err != nil {
		return nil, err
	}
	dbg.Lvl3("Waiting for the response")
	// wait for the response
	packet, err := con.Receive(context.TODO())
	if err != nil {
		return nil, err
	}
	response, ok := packet.Msg.(sda.CosiResponse)
	if !ok {
		return nil, errors.New("Invalid repsonse: Could not cast the " +
			"received response to the right type")
	}
	dbg.Lvl3("Response:", response)
	if verify && false { // verify signature
		err := cosi.VerifySignature(network.Suite, msgB, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
	}
	return &response, nil
}

func verifyFileSig(fileName, groupToml string) error {
	// if the file hash matches the one in the signature
	// iff yes -> return nil; else error
	suite := network.Suite
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(f)
	fHash := suite.Hash().Sum(b)
	// Read the JSON signature file
	sf, err := os.Open(fileName + ".sig")
	if err != nil {
		return err
	}
	sb, err := ioutil.ReadAll(sf)
	if err != nil {
		return err
	}
	sig := &sda.CosiResponse{}
	if err := json.Unmarshal(sb, sig); err != nil {
		return err
	}
	if bytes.Equal(sig.Sum, fHash) {

	} else {
		return fmt.Errorf("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with with the hash of the file.")
	}

	return nil
}

func printVerificationResult(err error) {
	if err == nil {
		fmt.Println("OK: Signature is valid.")
	} else {
		fmt.Println("Invalid: Signature verification failed.")
		dbg.Lvl2("Details:", err)
	}

}

func signFile(fileName, groupToml string) (*sda.CosiResponse, error) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read file to be signed: %s",
			err)
	}
	return sign(file, groupToml)
}

func signString(statement, groupToml string) (*sda.CosiResponse, error) {
	msgR := strings.NewReader(statement)
	return sign(msgR, groupToml)
}

func sign(r io.Reader, tomlFileName string) (*sda.CosiResponse, error) {
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := app.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	res, err := SignStatement(r, el, true)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func handleErrorAndExit(msg string, e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, msg+e.Error())
		os.Exit(1)
	}
}

func writeSigAsJSON(res *sda.CosiResponse, outW io.Writer) {
	b, err := json.Marshal(res)
	if err != nil {
		fmt.Println("error:", err)
	}
	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	if _, err := out.WriteTo(outW); err != nil {
		handleErrorAndExit("Couldn't write signature", err)
	}
}
