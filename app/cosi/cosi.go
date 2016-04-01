// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"encoding/json"
	"errors"
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
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"time"
)

func main() {
	dbg.SetDebugVisible(4)
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
			Aliases: []string{"v"},
			Usage:   "verify collective signature",
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
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
	}
	app.Run(os.Args)
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	tomlFileName := c.GlobalString("servers")
	f, err := os.Open(tomlFileName)
	handleErrorAndExit("Couldn't open server-file", err)
	el, err := app.ReadGroupToml(f)
	handleErrorAndExit("Error while reading server-file", err)
	// First check all servers individually
	for i := range el.List {
		checkList(sda.NewEntityList(el.List[i : i+1]))
	}
	if len(el.List) > 1 {
		// Then check pairs of servers
		for i, first := range el.List {
			for _, second := range el.List[i+1:] {
				es := []*network.Entity{first, second}
				checkList(sda.NewEntityList(es))
				es[0], es[1] = es[1], es[0]
				checkList(sda.NewEntityList(es))
			}
		}
	}
}

// checkList sends a message to the list and waits for the reply
func checkList(list *sda.EntityList) {
	serverStr := ""
	for _, s := range list.List {
		serverStr += s.Addresses[0] + " "
	}
	dbg.Print("Sending message to", serverStr)
	msg := "verification"
	sig, err := signStatement(strings.NewReader(msg), list)
	if err != nil {
		dbg.Error("When contacting servers", serverStr, err)
	} else {
		err := verifySignature([]byte(msg), sig, list)
		if err != nil {
			dbg.Error("Signature was invalid:", err)
		}
		dbg.Print("Received signature successfully")
	}
}

// signFile will search for the file and sign it
func signFile(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	fileName := c.Args().First()
	groupToml := c.GlobalString("servers")
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
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	msg := strings.NewReader(c.Args().First())
	groupToml := c.GlobalString("servers")
	sig, err := sign(msg, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	dbg.Lvl1(sig)
	writeSigAsJSON(sig, os.Stdout)
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

func verifyFile(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	verify(c.Args().First(), c.GlobalString("servers"))
}

func verifyString(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
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
func signStatement(r io.Reader, el *sda.EntityList) (*sda.CosiResponse, error) {

	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	e := network.NewEntity(kp.Public, "")
	client := network.NewSecureTcpHost(kp.Secret, e)
	msg, _ := crypto.HashStream(network.Suite.Hash(), r)
	req := &sda.CosiRequest{
		EntityList: el,
		Message:    msg,
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
	pchan := make(chan sda.CosiResponse)
	go func() {
		// send the request
		if err := con.Send(context.TODO(), req); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl3("Waiting for the response")
		// wait for the response
		packet, err := con.Receive(context.TODO())
		if err != nil {
			close(pchan)
			return
		}
		pchan <- packet.Msg.(sda.CosiResponse)
	}()
	select {
	case response, ok := <-pchan:
		dbg.Lvl5("Response:", ok, response)
		if !ok {
			return nil, errors.New("Invalid repsonse: Could not cast the " +
				"received response to the right type")
		}
		err = cosi.VerifySignature(network.Suite, msg, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
		return &response, nil
	case <-time.After(time.Second * 10):
		return nil, errors.New("Timeout on signing")
	}
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result
func verify(fileName, groupToml string) error {
	// if the file hash matches the one in the signature
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	// Read the JSON signature file
	sb, err := ioutil.ReadFile(fileName + ".sig")
	if err != nil {
		return err
	}
	sig := &sda.CosiResponse{}
	if err := json.Unmarshal(sb, sig); err != nil {
		return err
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}
	el, err := app.ReadGroupToml(fGroup)
	if err != nil {
		return err
	}
	err = verifySignature(b, sig, el)
	verifyPrintResult(err)
	return err
}

// verifySignature checks whether the signature is valid
func verifySignature(b []byte, sig *sda.CosiResponse, el *sda.EntityList) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	if err := cosi.VerifySignature(network.Suite, fHash, el.Aggregate, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}

// verifyPrintResult prints out OK or what failed.
func verifyPrintResult(err error) {
	if err == nil {
		dbg.Print("OK: Signature is valid.")
	} else {
		dbg.Print("Invalid: Signature verification failed:", err)
	}
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
