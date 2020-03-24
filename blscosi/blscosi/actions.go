package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/blscosi"
	"go.dedis.ch/cothority/v3/blscosi/blscosi/check"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
)

type sigHex struct {
	Hash      string
	Signature string
}

// check contacts all servers and verifies if it receives a valid
// signature from each.
func checkRequest(c *cli.Context) error {
	tomlFileName := c.String(optionGroup)

	log.Info("Checking the availability and responsiveness of the servers in the group...")
	return check.CothorityCheck(tomlFileName, c.Bool("detail"))
}

// signFile will search for the file and sign it
// it always returns nil as an error
func signFile(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("Please give the file to sign")
	}
	fileName := c.Args().First()
	groupToml := c.String(optionGroup)
	msg, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.New("Couldn't read file to be signed:" + err.Error())
	}

	sig, err := sign(msg, groupToml)
	if err != nil {
		return fmt.Errorf("Couldn't create signature: %s", err.Error())
	}

	var outFile *os.File
	outFileName := c.String("out")
	if outFileName != "" {
		outFile, err = os.Create(outFileName)
		if err != nil {
			return fmt.Errorf("Couldn't create signature file: %s", err.Error())
		}
	} else {
		outFile = os.Stdout
	}

	err = writeSigAsJSON(sig, outFile)
	if err != nil {
		return err
	}

	if outFileName != "" {
		log.Lvlf2("Signature written to: %s", outFile.Name())
	} // else keep the Stdout empty
	return nil
}

func verifyFile(c *cli.Context) error {
	if len(c.Args().First()) == 0 {
		return errors.New("Please give the 'msgFile'")
	}

	sigOrEmpty := c.String("signature")
	err := verify(c.Args().First(), sigOrEmpty, c.String(optionGroup))
	if err != nil {
		return fmt.Errorf("Invalid: Signature verification failed: %s", err.Error())
	}

	fmt.Fprintln(c.App.Writer, "[+] OK: Signature is valid.")
	return nil
}

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *blscosi.SignatureResponse, outW io.Writer) error {
	b, err := json.Marshal(sigHex{
		Hash:      hex.EncodeToString(res.Hash),
		Signature: hex.EncodeToString(res.Signature)},
	)

	if err != nil {
		return fmt.Errorf("Couldn't encode signature: %s", err.Error())
	}

	var out bytes.Buffer
	err = json.Indent(&out, b, "", "\t")
	if err != nil {
		return err
	}

	_, err = out.WriteTo(outW)
	if err != nil {
		return fmt.Errorf("Couldn't write signature: %s", err.Error())
	}

	_, err = outW.Write([]byte("\n"))
	return err
}

// sign takes a stream and a toml file defining the servers
func sign(msg []byte, tomlFileName string) (*blscosi.SignatureResponse, error) {
	log.Lvl2("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	g, err := app.ReadGroupDescToml(f)
	if err != nil {
		return nil, err
	}
	if len(g.Roster.List) <= 0 {
		return nil, fmt.Errorf("Empty or invalid blscosi group file: %s", tomlFileName)
	}

	log.Lvl2("Sending signature to", g.Roster)
	return check.SignStatement(msg, g.Roster)
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result. If sigFileName is empty it
// assumes to find the standard signature in fileName.sig
func verify(fileName, sigFileName, groupToml string) error {
	// if the file hash matches the one in the signature
	log.Lvl4("Reading file " + fileName)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.New("Couldn't open msgFile: " + err.Error())
	}

	// Read the JSON signature file
	log.Lvl4("Reading signature")
	var sigBytes []byte
	if sigFileName == "" {
		log.Print("[+] Reading signature from standard input ...")
		sigBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		sigBytes, err = ioutil.ReadFile(sigFileName)
	}
	if err != nil {
		return err
	}

	log.Lvl4("Unmarshalling signature ")
	sigStr := &sigHex{}
	if err = json.Unmarshal(sigBytes, sigStr); err != nil {
		return err
	}

	sig := &blscosi.SignatureResponse{}
	sig.Hash, err = hex.DecodeString(sigStr.Hash)
	if err != nil {
		return err
	}
	sig.Signature, err = hex.DecodeString(sigStr.Signature)
	if err != nil {
		return err
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}

	log.Lvl4("Reading group definition")
	g, err := app.ReadGroupDescToml(fGroup)
	if err != nil {
		return err
	}

	log.Lvlf4("Verifying signature %x %x", b, sig.Signature)
	return check.VerifySignatureHash(b, sig, g.Roster)
}
