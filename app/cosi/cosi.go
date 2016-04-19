// Cosi takes a file or a message and signs it collectively.
// For usage, see README.md
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"errors"
	"time"

	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/config"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	s "github.com/dedis/cothority/services/cosi"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// RequestTimeOut defines when the client stops waiting for the CoSi group to
// reply
const RequestTimeOut = time.Second * 10

func main() {
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
			Name:  "servers, s", // FIXME doesn't work
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
	el, err := config.ReadGroupToml(f)
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
	sig, err := SignStatement(strings.NewReader(msg), list)
	if err != nil {
		dbg.Error("When contacting servers", serverStr, err)
	} else {
		err := VerifySignatureHash([]byte(msg), sig, list)
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
	sig, err := Sign(file, groupToml)
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
	sig, err := Sign(msg, groupToml)
	handleErrorAndExit("Couldn't create signature", err)
	writeSigAsJSON(sig, os.Stdout)
}

func verifyFile(c *cli.Context) {
	dbg.SetDebugVisible(c.GlobalInt("debug"))
	err := Verify(c.Args().First(), c.GlobalString("servers"))
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
	err = Verify(f.Name(), c.GlobalString("servers"))
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

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *SignResponse, outW io.Writer) {
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

// handleErrorAndExit is a shortcut for all those pesky err-checks
func handleErrorAndExit(msg string, e error) {
	if e != nil {
		dbg.Fatal(os.Stderr, msg+": "+e.Error())
	}
}

// SignResponse contains the signature out of the CoSi system.
// It can be verified using the lib/cosi package.
// NOTE: the `suite` field is absent here because this struct is a temporary
// hack and we only supports one suite for the moment,i.e. ed25519.
type SignResponse struct {
	// The hash of the signed statement
	Sum []byte
	// The Challenge out a of the Multi Schnorr signature
	Challenge abstract.Secret
	// the SignResponse out of the Multi Schnorr Signature
	Response abstract.Secret
}

// MarshalJSON implements golang's JSON marshal interface
func (s *SignResponse) MarshalJSON() ([]byte, error) {
	cw := new(bytes.Buffer)
	rw := new(bytes.Buffer)

	err := crypto.WriteSecret64(network.Suite, cw, s.Challenge)
	if err != nil {
		return nil, err
	}
	err = crypto.WriteSecret64(network.Suite, rw, s.Response)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Sum       string
		Challenge string
		Response  string
	}{
		Sum:       base64.StdEncoding.EncodeToString(s.Sum),
		Challenge: cw.String(),
		Response:  rw.String(),
	})
}

// UnmarshalJSON implements golang's JSON unmarshal interface
func (s *SignResponse) UnmarshalJSON(data []byte) error {
	type Aux struct {
		Sum       string
		Challenge string
		Response  string
	}
	aux := &Aux{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var err error
	if s.Sum, err = base64.StdEncoding.DecodeString(aux.Sum); err != nil {
		return err
	}
	suite := network.Suite
	cr := strings.NewReader(aux.Challenge)
	if s.Challenge, err = crypto.ReadSecret64(suite, cr); err != nil {
		return err
	}
	rr := strings.NewReader(aux.Response)
	if s.Response, err = crypto.ReadSecret64(suite, rr); err != nil {
		return err
	}
	return nil
}

// sign takes a stream and a toml file defining the servers
func Sign(r io.Reader, tomlFileName string) (*SignResponse, error) {
	dbg.Lvl3("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := config.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	dbg.Lvl2("Sending signature to", el)
	res, err := SignStatement(r, el)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(read io.Reader, el *sda.EntityList) (*SignResponse, error) {

	// create a throw-away key pair:
	priv, pub := sda.PrivPub()
	// create a throw-away entity with an empty  address:
	e := network.NewEntity(pub)

	client := network.NewSecureTCPHost(priv, e)
	msg, _ := crypto.HashStream(network.Suite.Hash(), read)
	// connect to the root:
	host := el.List[0]
	dbg.Lvl3("Opening connection to", host.First(), host.Public)

	// create request
	r := &s.ServiceRequest{
		Message:    msg,
		EntityList: el,
	}
	req, err := sda.CreateServiceRequest("Cosi", r)
	if err != nil {
		errors.New("could not create service request")
	}

	con, err := client.Open(host)
	defer client.Close()
	if err != nil {
		return nil, fmt.Errorf("Client could not connect to service %s: %s",
			host.First(), err.Error())
	}

	dbg.Lvl3("Sending sign SignRequest")
	pchan := make(chan s.ServiceResponse)
	go func() {
		// send the SignRequest
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
		pchan <- packet.Msg.(s.ServiceResponse)
	}()

	select {
	case response, ok := <-pchan:
		dbg.Lvl5("Response:", ok, response)
		if !ok {
			return nil, errors.New("Invalid repsonse: Could not " +
				"cast the received response to the right type")
		}
		err = cosi.VerifySignature(network.Suite, msg, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
		return &SignResponse{
			Sum:       msg,
			Challenge: response.Challenge,
			Response:  response.Response,
		}, nil
	case <-time.After(RequestTimeOut):
		return nil, errors.New("Timeout on signing.")
	}
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result
func Verify(fileName, groupToml string) error {
	// if the file hash matches the one in the signature
	dbg.Lvl4("Reading file " + fileName)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	// Read the JSON signature file
	dbg.Lvl4("Reading signature")
	sb, err := ioutil.ReadFile(fileName + ".sig")
	if err != nil {
		return err
	}
	sig := &SignResponse{}
	dbg.Lvl4("Unmarshalling signature ")
	if err := json.Unmarshal(sb, sig); err != nil {
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
	err = VerifySignatureHash(b, sig, el)
	return err
}

// verifySignature checks whether the signature is valid
func VerifySignatureHash(b []byte, sig *SignResponse, el *sda.EntityList) error {
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
