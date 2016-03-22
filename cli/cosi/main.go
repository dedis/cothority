package main

import (
	"flag"
	"fmt"
	"github.com/dedis/cothority/cli"
	"github.com/dedis/crypto/abstract"
	"io"
	"os"
	"strings"
)

func printUsageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, "%s", msg)
	}

	// XXX print some very clear instructions:
	fmt.Fprintf(os.Stderr, `usage:
	cosi -m “<Message to be signed>” my-cosi-group.toml
	cosi -f <file-to-be-signed> my-cosi-group.toml`)
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
	if !(len(os.Args) == 3) {
		printUsageAndExit("")
	}
	switch os.Args[1] {
	case "-f":
		strOrFilename := f.String("f", "",
			"Filename of the file to be signed.")
		groupToml := f.String("f", "",
			"Toml file containing the list of CoSi nodes.")

		if err := f.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing file. " +
				"Couldn't parse arguments:" + err.Error())
		}
		sig, err := signFile(*strOrFilename, *groupToml)
		handleErrorAndExit(err)
		fmt.Println("Signature:" + sig.String())

	case "-m":
		strOrFilename := m.String("m", "", "Message to be signed.")
		groupToml := m.String("m", "", "Toml file containing the list of CoSi nodes.")
		if err := m.Parse(os.Args[1:]); err != nil {
			printUsageAndExit("Unable to start signing message" +
				"Couldn't parse arguments:" + err.Error())
		}
		sig, err := signString(*strOrFilename, *groupToml)
		handleErrorAndExit(err)
		fmt.Println("Signature:" + sig.String())
	default:
		printUsageAndExit("")
	}
}

func signFile(fileName, groupToml string) (abstract.Secret, error) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read file to be signed: %s", err)
	}
	return sign(file, groupToml)
}

func signString(statement, groupToml string) (abstract.Secret, error) {
	msgR := strings.NewReader(statement)
	return sign(msgR, groupToml)
}

func sign(r io.Reader, tomlFileName string) (abstract.Secret, error) {
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := cli.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	res, err := cli.SignStatement(r, el, true)
	if err != nil {
		return nil, err
	}
	return res.Response, nil
}

func handleErrorAndExit(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create signature"+e.Error())
	}
	os.Exit(1)
}
