package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority/blsftcosi"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

type sigHex struct {
	Hash      string
	Signature string
}

// check contacts all servers and verifies if it receives a valid
// signature from each.
func check(c *cli.Context) error {
	tomlFileName := c.String(optionGroup)

	f, err := os.Open(tomlFileName)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := app.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s", tomlFileName)
	}
	log.Info("Checking the availability and responsiveness of the servers in the group...")
	return checkCothority(group, c.Bool("detail"))
}

// Servers contacts all servers in the entity-list and then makes checks
// on each pair. If server-descriptions are available, it will print them
// along with the IP-address of the server.
// In case a server doesn't reply in time or there is an error in the
// signature, an error is returned.
func checkCothority(g *app.Group, detail bool) error {
	log.Lvlf3("Checking roster %v", g.Roster.List)
	totalSuccess := true
	// First check all servers individually and write the working servers
	// in a list
	working := []*network.ServerIdentity{}
	for _, e := range g.Roster.List {
		desc := []string{"none", "none"}
		if d := g.GetDescription(e); d != "" {
			desc = []string{d, d}
		}
		el := onet.NewRoster([]*network.ServerIdentity{e})
		err := checkRoster(el, desc, true)
		if err == nil {
			working = append(working, e)
		} else {
			log.Error(err)
			totalSuccess = false
		}
	}
	wn := len(working)
	if wn > 1 {
		// Check one big roster sqrt(len(working)) times.
		descriptions := make([]string, wn)
		rand.Seed(int64(time.Now().Nanosecond()))
		for j := 0; j <= int(math.Sqrt(float64(wn))); j++ {
			permutation := rand.Perm(wn)
			for i, si := range working {
				descriptions[permutation[i]] = g.GetDescription(si)
			}
			totalSuccess = checkRoster(onet.NewRoster(working), descriptions, detail) == nil && totalSuccess
		}

		// Then check pairs of servers if we want to have detail
		if detail {
			for i, first := range working {
				for _, second := range working[i+1:] {
					log.Lvl3("Testing connection between", first, second)
					desc := []string{"none", "none"}
					if d1 := g.GetDescription(first); d1 != "" {
						desc = []string{d1, g.GetDescription(second)}
					}
					es := []*network.ServerIdentity{first, second}
					totalSuccess = checkRoster(onet.NewRoster(es), desc, detail) == nil && totalSuccess
					es[0], es[1] = es[1], es[0]
					desc[0], desc[1] = desc[1], desc[0]
					totalSuccess = checkRoster(onet.NewRoster(es), desc, detail) == nil && totalSuccess
				}
			}
		}
	}
	if !totalSuccess {
		return errors.New("At least one of the tests failed")
	}
	return nil
}

// checkList sends a message to the cothority defined by list and
// waits for the reply.
// If the reply doesn't arrive in time, it will return an
// error.
func checkRoster(list *onet.Roster, descs []string, detail bool) error {
	serverStr := ""
	for i, s := range list.List {
		name := strings.Split(descs[i], " ")[0]
		if detail {
			serverStr += s.Address.NetworkAddress() + "_"
		}
		serverStr += name + " "
	}
	log.Lvl3("Sending message to: " + serverStr)
	msg := []byte("verification")
	fmt.Printf("Checking %d server(s) %s: ", len(list.List), serverStr)
	sig, err := signStatement(msg, list)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = verifySignatureHash(msg, sig, list)
	if err != nil {
		fmt.Printf("Invalid signature: %s\n", err.Error())
		return err
	}
	fmt.Println("Success")
	return nil
}

// signFile will search for the file and sign it
// it always returns nil as an error
func signFile(c *cli.Context) error {
	if c.Args().First() == "" {
		log.Fatal("Please give the file to sign", 1)
	}
	fileName := c.Args().First()
	groupToml := c.String(optionGroup)
	msg, err := ioutil.ReadFile(fileName)
	log.ErrFatal(err, "Couldn't read file to be signed:")

	sig, err := sign(msg, groupToml)
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
		log.Lvlf2("Signature written to: %s", outFile.Name())
	} // else keep the Stdout empty
	return nil
}

func verifyFile(c *cli.Context) error {
	if len(c.Args().First()) == 0 {
		log.Fatal("Please give the 'msgFile'", 1)
	}
	sigOrEmpty := c.String("signature")
	err := verify(c.Args().First(), sigOrEmpty, c.String(optionGroup))
	log.ErrFatal(err, "Invalid: Signature verification failed")

	log.Print("[+] OK: Signature is valid.")
	return nil
}

// writeSigAsJSON - writes the JSON out to a file
func writeSigAsJSON(res *blsftcosi.SignatureResponse, outW io.Writer) {
	b, err := json.Marshal(sigHex{
		Hash:      hex.EncodeToString(res.Hash),
		Signature: hex.EncodeToString(res.Signature)},
	)
	log.ErrFatal(err, "Couldn't encode signature:")

	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	outW.Write([]byte("\n"))
	_, err = out.WriteTo(outW)
	log.ErrFatal(err, "Couldn't write signature:")

	outW.Write([]byte("\n"))
}

// sign takes a stream and a toml file defining the servers
func sign(msg []byte, tomlFileName string) (*blsftcosi.SignatureResponse, error) {
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
		return nil, errors.New("Empty or invalid blsftcosi group file:" +
			tomlFileName)
	}
	log.Lvl2("Sending signature to", g.Roster)
	res, err := signStatement(msg, g.Roster)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func signStatement(msg []byte, ro *onet.Roster) (*blsftcosi.SignatureResponse, error) {
	client := blsftcosi.NewClient()
	publics := ro.ServicePublics(blsftcosi.ServiceName)

	log.Lvlf4("Signing message %x", msg)

	pchan := make(chan *blsftcosi.SignatureResponse, 1)
	echan := make(chan error, 1)
	go func() {
		log.Lvl3("Waiting for the response on SignRequest")
		response, err := client.SignatureRequest(ro, msg[:])
		if err != nil {
			echan <- err
			return
		}
		pchan <- response
	}()

	select {
	case err := <-echan:
		return nil, err
	case response := <-pchan:
		log.Lvlf5("Response: %x", response.Signature)

		err := response.Signature.Verify(client.PairingSuite(), msg[:], publics)
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
	sig := &blsftcosi.SignatureResponse{}
	sig.Hash, err = hex.DecodeString(sigStr.Hash)
	sig.Signature, err = hex.DecodeString(sigStr.Signature)
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
	err = verifySignatureHash(b, sig, g.Roster)
	return err
}

func verifySignatureHash(b []byte, sig *blsftcosi.SignatureResponse, ro *onet.Roster) error {
	suite := blsftcosi.NewClient().Suite().(*pairing.SuiteBn256)

	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	publics := ro.ServicePublics(blsftcosi.ServiceName)
	h := suite.Hash()
	h.Write(b)
	fHash := h.Sum(nil)
	if !bytes.Equal(fHash, sig.Hash) {
		return errors.New("You are trying to verify a signature " +
			"belonging to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}

	if err := sig.Signature.Verify(suite, b, publics); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
