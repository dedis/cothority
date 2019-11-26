package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"errors"
	"time"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/cosi/check"
	"go.dedis.ch/cothority/v3/cosi/crypto"
	s "go.dedis.ch/cothority/v3/cosi/service"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
)

// checkConfig contacts all servers and verifies if it receives a valid
// signature from each.
func checkConfig(c *cli.Context) error {
	tomlFileName := c.String(optionGroup)
	return check.Config(tomlFileName, c.Bool("detail"))
}

// signFile will search for the file and sign it
// it always returns nil as an error
func signFile(c *cli.Context) error {
	if c.Args().First() == "" {
		log.Fatal("Please give the file to sign", 1)
	}
	fileName := c.Args().First()
	groupToml := c.String(optionGroup)
	file, err := os.Open(fileName)
	log.ErrFatal(err, "Couldn't read file to be signed:")

	sig, err := sign(file, groupToml)
	log.ErrFatal(err, "Couldn't create signature:")

	log.Lvl3(sig)
	var outFile *os.File
	outFileName := c.String("out")
	if outFileName != "" {
		outFile, err = os.Create(outFileName)
		log.ErrFatal(err, "Couldn't create signature file:")
	} else {
		outFile = os.Stdout
	}
	writeSigAsJSON(sig, outFile)
	if outFileName != "" {
		log.Lvl2("Signature written to:", outFile.Name())
	} // else keep the Stdout empty
	return nil
}

func verifyFile(c *cli.Context) error {
	if len(c.Args().First()) == 0 {
		log.Fatal("Please give the 'msgFile'", 1)
	}
	sigOrEmpty := c.String("signature")
	err := verify(c.Args().First(), sigOrEmpty, c.String(optionGroup))
	verifyPrintResult(err)
	return nil
}

// verifyPrintResult prints out OK or what failed.
func verifyPrintResult(err error) {
	log.ErrFatal(err, "Invalid: Signature verification failed:")

	log.Info("[+] OK: Signature is valid.")
}

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *s.SignatureResponse, outW io.Writer) {
	b, err := json.Marshal(res)
	log.ErrFatal(err, "Couldn't encode signature:")

	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	outW.Write([]byte("\n"))
	_, err = out.WriteTo(outW)
	log.ErrFatal(err, "Couldn't write signature:")

	outW.Write([]byte("\n"))
}

// sign takes a stream and a toml file defining the servers
func sign(r io.Reader, tomlFileName string) (*s.SignatureResponse, error) {
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
		return nil, errors.New("Empty or invalid cosi group file:" +
			tomlFileName)
	}
	log.Lvl2("Sending signature to", g.Roster)
	res, err := signStatement(r, g.Roster)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func signStatement(read io.Reader, el *onet.Roster) (*s.SignatureResponse,
	error) {
	publics := entityListToPublics(el)
	client := s.NewClient()

	h := client.Suite().(kyber.HashFactory).Hash()
	io.Copy(h, read)
	msg := h.Sum(nil)

	pchan := make(chan *s.SignatureResponse)
	var err error
	go func() {
		log.Lvl3("Waiting for the response on SignRequest")
		response, e := client.SignatureRequest(el, msg)
		if e != nil {
			err = e
			close(pchan)
			return
		}
		pchan <- response
	}()

	select {
	case response, ok := <-pchan:
		log.Lvl5("Response:", response)
		if !ok || err != nil {
			return nil, errors.New("received an invalid repsonse")
		}

		err = crypto.VerifySignature(client.Suite(), publics, msg, response.Signature)
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
	log.Lvl4("Reading file " + fileName)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.New("Couldn't open msgFile: " + err.Error())
	}
	// Read the JSON signature file
	log.Lvl4("Reading signature")
	var sigBytes []byte
	if sigFileName == "" {
		log.Info("[+] Reading signature from standard input ...")
		sigBytes, err = ioutil.ReadAll(os.Stdin)
	} else {
		sigBytes, err = ioutil.ReadFile(sigFileName)
	}
	if err != nil {
		return err
	}
	sig := &s.SignatureResponse{}
	log.Lvl4("Unmarshalling signature ")
	if err := json.Unmarshal(sigBytes, sig); err != nil {
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
	log.Lvl4("Verfifying signature")
	err = verifySignatureHash(b, sig, g.Roster)
	return err
}

func verifySignatureHash(b []byte, sig *s.SignatureResponse, el *onet.Roster) error {
	localSuite := cothority.Suite.(kyber.HashFactory)

	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	publics := entityListToPublics(el)
	h := localSuite.Hash()
	h.Write(b)
	fHash := h.Sum(nil)
	h.Reset()
	h.Write(fHash)
	hashHash := h.Sum(nil)
	if !bytes.Equal(hashHash, sig.Hash) {
		return errors.New("You are trying to verify a signature " +
			"belonging to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	if err := crypto.VerifySignature(cothority.Suite, publics, fHash, sig.Signature); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
func entityListToPublics(r *onet.Roster) []kyber.Point {
	publics := make([]kyber.Point, len(r.List))
	for i, e := range r.List {
		publics[i] = e.Public
	}
	return publics
}
