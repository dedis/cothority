// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
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
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
)

func main() {
	dbg.SetDebugVisible(1)
	app := cli.NewApp()
	app.Name = "Cosi signer and verifier"
	app.Usage = "Collectively sign a file or a message and verify it"
	app.Commands = []cli.Command{
		{
			Name:    "sign",
			Aliases: []string{"s"},
			Usage:   "collectively sign",
			Subcommands: []cli.Command{
				{
					Name:   "file",
					Usage:  "file to sign",
					Action: signFile,
				}, {
					Name:   "msg",
					Usage:  "message to sign",
					Action: signString,
				},
			},
		},
		{
			Name:    "verify",
			Aliases: []string{"s"},
			Usage:   "collectively sign",
			Subcommands: []cli.Command{
				{
					Name:   "file",
					Usage:  "file to verify",
					Action: verifyFile,
				}, {
					Name:   "msg",
					Usage:  "message to verify",
					Action: verifyString,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "signature, sig",
							Usage: "signature-file",
						},
					},
				},
			},
		},
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "check the server-file",
			Action:  checkConfig,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "servers, s",
			Value: "servers.toml",
			Usage: "server-list for collective signature",
		},
	}
	app.Run(os.Args)
}

func checkConfig(c *cli.Context) {
	_ := c.GlobalString("servers")
}

func signFile(c *cli.Context) {
	fileName := c.Args().First()
	groupToml := c.GlobalString("servers")
	dbg.Lvl1(fileName, groupToml)
	file, err := os.Open(fileName)
	if err != nil {
		dbg.Error("Couldn't read file to be signed:", err)
	}
	sig, err := sign(file, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	dbg.Lvl1(sig)
	sigFileName := fileName + ".sig"
	outFile, err := os.Create(sigFileName)
	handleErrorAndExit("Couldn't create signature file", err)
	writeSigAsJSON(sig, outFile)
	dbg.Lvl1("Signature written to: " + sigFileName)
}

func signString(c *cli.Context) {
	msg := strings.NewReader(c.Args().First())
	groupToml := c.GlobalString("servers")
	sig, err := sign(msg, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	dbg.Lvl1(sig)
	writeSigAsJSON(sig, os.Stdout)
}

func verifyFile(c *cli.Context) {
	verify(c.Args().First(), c.GlobalString("servers"))
}

func verifyString(c *cli.Context) {
	f, err := ioutil.TempFile("", "cosi")
	handleErrorAndExit("Couldn't create temp file", err)
	f.Write([]byte(c.Args().First()))
	f.Close()
	sigfile := f.Name() + ".sig"
	sig, err := ioutil.ReadFile(c.String("signature"))
	handleErrorAndExit("Couldn't read signature", err)
	err = ioutil.WriteFile(sigfile, sig, 0444)
	handleErrorAndExit("Couldn't write tmp-signature", err)
	verify(f.Name(), c.GlobalString("servers"))
	os.Remove(f.Name())
	os.Remove(sigfile)
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func signStatement(r io.Reader,
	el *sda.EntityList) (*sda.CosiResponse, error) {

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
	dbg.Lvl5("Response:", ok, response)
	if !ok {
		return nil, errors.New("Invalid repsonse: Could not cast the " +
			"received response to the right type")
	}
	err = cosi.VerifySignature(network.Suite, msgB, el.Aggregate,
		response.Challenge, response.Response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result
func verify(fileName, groupToml string) error {
	err := verifySignature(fileName, groupToml)
	verifyPrintResult(err)
	return err
}

// verifySignature checks whether the signature is valid
func verifySignature(fileName, groupToml string) error {
	// if the file hash matches the one in the signature
	suite := network.Suite
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(f)
	hash := suite.Hash()
	if n, err := hash.Write(b); n != len(b) || err != nil {
		return errors.New("Couldn't hash file")
	}
	fHash := hash.Sum(nil)
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
	if !bytes.Equal(sig.Sum, fHash) {
		return errors.New("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}
	el, err := app.ReadGroupToml(fGroup)
	if err != nil {
		return err
	}
	if err := cosi.VerifySignature(network.Suite, b, el.Aggregate, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return err
}

// verifyPrintResult prints out OK or what failed.
func verifyPrintResult(err error) {
	if err == nil {
		dbg.Print("OK: Signature is valid.")
	} else {
		dbg.Print("Invalid: Signature verification failed:", err)
	}
}

// sign takes a stream and a toml file defining the servers
func sign(r io.Reader, tomlFileName string) (*sda.CosiResponse, error) {
	dbg.Lvl3("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := app.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	dbg.Lvl2("Sending signature to", el)
	res, err := signStatement(r, el)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// handleErrorAndExit is a shortcut for all those pesky err-checks
func handleErrorAndExit(msg string, e error) {
	if e != nil {
		dbg.Fatal(os.Stderr, msg+e.Error())
	}
}

// writeSigAsJSON - writes the JSON out to a file
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
	outW.Write([]byte("\n"))
}
