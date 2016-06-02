// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"errors"
	"time"

	"fmt"
	"sync"

	"github.com/dedis/cothority/lib/config"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	s "github.com/dedis/cothority/services/cosi"
	"gopkg.in/codegangsta/cli.v1"
)

// RequestTimeOut defines when the client stops waiting for the CoSi group to
// reply
const RequestTimeOut = time.Second * 10

const optionGroup = "group"
const optionGroupShort = "g"

func init() {
	dbg.SetDebugVisible(1)
	dbg.SetUseColors(false)
}

func main() {
	app := cli.NewApp()
	app.Name = "Cosi signer and verifier"
	app.Usage = "Collectively sign a file or verify its signature."
	app.Commands = []cli.Command{
		{
			Name:    "sign",
			Aliases: []string{"s"},
			Usage:   "Collectively sign file and write signature to standard output.",
			Action:  signFile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Write signature to `outfile` instead of standard output",
				},
			},
		},
		{
			Name:    "verify",
			Aliases: []string{"v"},
			Usage:   "verify collective signature of a FILE",
			Action:  verifyFile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "signature, s",
					Usage: "use the `SIGNATURE_FILE` containing the signature (instead of reading from standard input)",
				},
			},
		},
		{
			Name:    "check",
			Aliases: []string{"c"},
			Usage:   "check if the servers int the group configuration are up and running",
			Action:  checkConfig,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  optionGroup + " ," + optionGroupShort,
			Value: "group.toml",
			Usage: "Cothority group definition in `FILE.toml`",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
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
func checkConfig(c *cli.Context) error {
	tomlFileName := c.GlobalString(optionGroup)
	f, err := os.Open(tomlFileName)
	printErrAndExit("Couldn't open group definition file: %v", err)
	el, err := config.ReadGroupToml(f)
	printErrAndExit("Error while reading group definition file: %v", err)
	if len(el.List) == 0 {
		printErrAndExit("Empty entity or invalid group defintion in: %s",
			tomlFileName)
	}
	fmt.Print("Will check the availability and responsiveness of the " +
		"servers forming the group and inform you about possible " +
		"problems.\nThis make take some time ...")

	var wg sync.WaitGroup
	wg.Add(len(el.List))
	// First check all servers individually
	for i := range el.List {
		go checkList(sda.NewEntityList(el.List[i:i+1]), &wg)
	}
	if len(el.List) > 1 {
		// Then check pairs of servers
		for i, first := range el.List {
			for _, second := range el.List[i+1:] {
				wg.Add(2)
				es := []*network.Entity{first, second}
				go checkList(sda.NewEntityList(es), &wg)
				es[0], es[1] = es[1], es[0]
				go checkList(sda.NewEntityList(es), &wg)
			}
		}
	}

	wg.Wait()
	return nil
}

// checkList sends a message to the list and waits for the reply
func checkList(list *sda.EntityList, wg *sync.WaitGroup) {
	defer wg.Done()
	serverStr := ""
	for _, s := range list.List {
		serverStr += s.Addresses[0] + " "
	}
	dbg.Lvl3("Sending message to: " + serverStr)
	msg := "verification"
	sig, err := signStatement(strings.NewReader(msg), list)
	if err != nil {
		fmt.Fprintln(os.Stderr,
			fmt.Sprintf("Error '%v' while contacting servers: %s",
				err, serverStr))
	} else {
		err := verifySignatureHash([]byte(msg), sig, list)
		if err != nil {
			fmt.Println(os.Stderr,
				fmt.Sprintf("Received signature was invalid: %v",
					err))
		} else {
			fmt.Println("Received signature successfully")
		}
	}

}

// signFile will search for the file and sign it
// it always returns nil as an error
func signFile(c *cli.Context) error {
	fileName := c.Args().First()
	groupToml := c.GlobalString(optionGroup)
	file, err := os.Open(fileName)
	if err != nil {
		printErrAndExit("Couldn't read file to be signed: %v", err)
	}
	sig, err := sign(file, groupToml)
	printErrAndExit("Couldn't create signature: %v", err)
	dbg.Lvl3(sig)
	var outFile *os.File
	outFileName := c.String("out")
	if outFileName != "" {
		outFile, err = os.Create(outFileName)
		printErrAndExit("Couldn't create signature file: %v", err)
	} else {
		outFile = os.Stdout
	}
	writeSigAsJSON(sig, outFile)
	if outFileName != "" {
		dbg.Lvl2("Signature written to: %s", outFile.Name())
	} // else keep the Stdout empty
	return nil
}

func verifyFile(c *cli.Context) error {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	sigOrEmpty := c.String("signature")
	if err := verify(c.Args().First(), sigOrEmpty, c.GlobalString(optionGroup)); err != nil {
		os.Exit(1)
	}
	return nil
}

// verifyPrintResult prints out OK or what failed.
func verifyPrintResult(err error) {
	if err == nil {
		fmt.Println("OK: Signature is valid.")
	} else {
		fmt.Println("Invalid: Signature verification failed: %v", err)
	}
}

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *s.SignatureResponse, outW io.Writer) {
	b, err := json.Marshal(res)
	if err != nil {
		printErrAndExit("Couldn't encode signature: %v", err)
	}
	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	outW.Write([]byte("\n"))
	if _, err := out.WriteTo(outW); err != nil {
		printErrAndExit("Couldn't write signature: %v", err)
	}
	outW.Write([]byte("\n"))
}

func printErrAndExit(format string, a ...interface{}) {
	if len(a) > 0 && a[0] != nil {
		fmt.Fprintln(os.Stderr, fmt.Sprintf(format, a...))
		os.Exit(1)
	}
}

// sign takes a stream and a toml file defining the servers
func sign(r io.Reader, tomlFileName string) (*s.SignatureResponse, error) {
	dbg.Lvl2("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := config.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	if len(el.List) <= 0 {
		return nil, errors.New("Empty or invalid cosi group file:" +
			tomlFileName)
	}
	dbg.Lvl2("Sending signature to", el)
	res, err := signStatement(r, el)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func signStatement(read io.Reader, el *sda.EntityList) (*s.SignatureResponse,
	error) {

	client := s.NewClient()
	msg, _ := crypto.HashStream(network.Suite.Hash(), read)

	pchan := make(chan *s.SignatureResponse)
	var err error
	go func() {
		dbg.Lvl3("Waiting for the response on SignRequest")
		response, e := client.SignMsg(el, msg)
		if e != nil {
			err = e
			close(pchan)
			return
		}
		pchan <- response
	}()

	select {
	case response, ok := <-pchan:
		dbg.Lvl5("Response:", response)
		if !ok || err != nil {
			return nil, errors.New("Received an invalid repsonse.")
		}
		err = cosi.VerifySignature(network.Suite, msg, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
		return response, nil
	case <-time.After(RequestTimeOut):
		return nil, errors.New("timeout on signing request")
	}
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result. If sigFileName is empty it
// assumes to find the standard signature in fileName.sig
func verify(fileName, sigFileName, groupToml string) error {
	// if the file hash matches the one in the signature
	dbg.Lvl4("Reading file " + fileName)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	// Read the JSON signature file
	dbg.Lvl4("Reading signature")
	var sigBytes []byte
	if sigFileName == "" {
		fmt.Println("Reading signature from standard input ...")
		sigBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		sigBytes, err = ioutil.ReadFile(sigFileName)
	}
	if err != nil {
		return err
	}
	sig := &s.SignatureResponse{}
	dbg.Lvl4("Unmarshalling signature ")
	if err := json.Unmarshal(sigBytes, sig); err != nil {
		return err
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}
	dbg.Lvl4("Reading group definition")
	el, err := config.ReadGroupToml(fGroup)
	if err != nil {
		return err
	}
	dbg.Lvl4("Verfifying signature")
	err = verifySignatureHash(b, sig, el)
	return err
}

func verifySignatureHash(b []byte, sig *s.SignatureResponse, el *sda.EntityList) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	if err := cosi.VerifySignature(network.Suite, fHash, el.Aggregate,
		sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
