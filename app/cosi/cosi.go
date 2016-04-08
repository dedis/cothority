// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"bytes"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
)

func main() {
	app := cli.NewApp()
	app.Name = "Cosi signer and verifier"
	app.Usage = "Collectively sign a file or a message and verify it"
	app.Version = "1.0"
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
	app.Before = func(c *cli.Context) error {
		dbg.SetDebugVisible(c.GlobalInt("debug"))
		return nil
	}
	app.Run(os.Args)
}

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) {
	tomlFileName := c.GlobalString("servers")
	f, err := os.Open(tomlFileName)
	handleErrorAndExit("Couldn't open server-file", err)
	el, err := cosi.ReadGroupToml(f)
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
	sig, err := cosi.SignStatement(strings.NewReader(msg), list)
	if err != nil {
		dbg.Error("When contacting servers", serverStr, err)
	} else {
		err := cosi.VerifySignatureHash([]byte(msg), sig, list)
		if err != nil {
			dbg.Error("Signature was invalid:", err)
		}
		dbg.Print("Received signature successfully")
	}
}

// signFile will search for the file and sign it
func signFile(c *cli.Context) {
	fileName := c.Args().First()
	groupToml := c.GlobalString("servers")
	file, err := os.Open(fileName)
	if err != nil {
		handleErrorAndExit("Couldn't read file to be signed: ", err)
	}
	sig, err := cosi.Sign(file, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	dbg.Lvl1(sig)
	sigFileName := fileName + ".sig"
	outFile, err := os.Create(sigFileName)
	handleErrorAndExit("Couldn't create signature file: ", err)
	writeSigAsJSON(sig, outFile)
	dbg.Lvl1("Signature written to: " + sigFileName)
}

func signString(c *cli.Context) {
	msg := strings.NewReader(c.Args().First())
	groupToml := c.GlobalString("servers")
	sig, err := cosi.Sign(msg, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	writeSigAsJSON(sig, os.Stdout)
}

func verifyFile(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	err := cosi.Verify(c.Args().First(), c.GlobalString("servers"))
	verifyPrintResult(err)
}

func verifyString(c *cli.Context) {
	f, err := ioutil.TempFile("", "cosi")
	handleErrorAndExit("Couldn't create temp file", err)
	f.Write([]byte(c.Args().First()))
	f.Close()
	sigfile := f.Name() + ".sig"
	sig, err := ioutil.ReadFile(c.String("signature"))
	handleErrorAndExit("Couldn't read signature: ", err)
	err = ioutil.WriteFile(sigfile, sig, 0444)
	handleErrorAndExit("Couldn't write tmp-signature", err)
	err = cosi.Verify(f.Name(), c.GlobalString("servers"))
	verifyPrintResult(err)
	os.Remove(f.Name())
	os.Remove(sigfile)
	if err != nil {
		os.Exit(1)
	}
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
		dbg.Fatal(os.Stderr, msg+": "+e.Error())
	}
}

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *cosi.SignResponse, outW io.Writer) {
	b, err := json.Marshal(res)
	if err != nil {
		handleErrorAndExit("Couldn't encode signature: ", err)
	}
	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	outW.Write([]byte("\n"))
	if _, err := out.WriteTo(outW); err != nil {
		handleErrorAndExit("Couldn't write signature", err)
	}
	outW.Write([]byte("\n"))
}
