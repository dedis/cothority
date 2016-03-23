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

	// XXX print some very clear instructions:
	fmt.Fprintf(os.Stderr, `Usage:
	cosi -m “<Message to be signed>” -c my-cosi-group.toml
	cosi -f <file-to-be-signed> -c my-cosi-group.toml`)
	os.Exit(1)
}

var f *flag.FlagSet
var m *flag.FlagSet

func init() {
	// XXX use flagsets as we soon might add different flags for each case
	// might be obsolete
	f = flag.NewFlagSet("f", flag.ContinueOnError)
	m = flag.NewFlagSet("m", flag.ContinueOnError)
}

func main() {
	if !(len(os.Args) == 5) {
		printUsageAndExit("Not enough arguments provided.\n")
	}
	switch os.Args[1] {
	case "-f":
		strOrFilename := f.String("f", "",
			"Filename of the file to be signed.")
		groupToml := f.String("c", "",
			"Toml file containing the list of CoSi nodes.")

		if err := f.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing file. " +
				"Couldn't parse arguments:" + err.Error())
		}
		sig, err := signFile(*strOrFilename, *groupToml)
		handleErrorAndExit(err)
		printSigAsJSON(sig)
	case "-m":
		strOrFilename := m.String("m", "", "Message to be signed.")
		groupToml := m.String("c", "", "Toml file containing the list "+
			"of CoSi nodes.")
		if err := m.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing message" +
				"Couldn't parse arguments:" + err.Error())
		}
		sig, err := signString(*strOrFilename, *groupToml)
		handleErrorAndExit(err)
		printSigAsJSON(sig)
	default:
		printUsageAndExit("")
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

func handleErrorAndExit(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create signature"+e.Error())
		os.Exit(1)
	}
}

func printSigAsJSON(res *sda.CosiResponse) {
	b, err := json.Marshal(res)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println("JSON encoded signature:")
	os.Stdout.Write(b)
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
	dbg.Lvl3("Recieved response")
	response, ok := packet.Msg.(sda.CosiResponse)
	if !ok {
		return nil, errors.New("Invalid repsonse: Could not cast the " +
			"received response to the right type")
	}

	if verify { // verify signature
		err := cosi.VerifySignature(network.Suite, msgB, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
	}
	return &response, nil
}
