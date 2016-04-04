package cosi

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"time"
)

// sign takes a stream and a toml file defining the servers
func Sign(r io.Reader, tomlFileName string) (*sda.CosiResponse, error) {
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
	res, err := SignStatement(r, el)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(r io.Reader, el *sda.EntityList) (*sda.CosiResponse, error) {

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
		err = VerifySignature(network.Suite, msg, el.Aggregate,
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
func Verify(fileName, groupToml string) error {
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
	err = VerifySignatureHash(b, sig, el)
	return err
}

// verifySignature checks whether the signature is valid
func VerifySignatureHash(b []byte, sig *sda.CosiResponse, el *sda.EntityList) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	if err := VerifySignature(network.Suite, fHash, el.Aggregate, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
