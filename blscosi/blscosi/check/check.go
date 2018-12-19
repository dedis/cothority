package check

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/dedis/cothority/blscosi"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// RequestTimeOut defines when the client stops waiting for the CoSi group to
// reply
const RequestTimeOut = time.Second * 10

// CothorityCheck contacts all servers in the entity-list and then makes checks
// on each pair. If server-descriptions are available, it will print them
// along with the IP-address of the server.
// In case a server doesn't reply in time or there is an error in the
// signature, an error is returned.
func CothorityCheck(tomlFileName string, detail bool) error {
	f, err := os.Open(tomlFileName)
	if err != nil {
		return fmt.Errorf("Couldn't open group definition file: %s", err.Error())
	}

	group, err := app.ReadGroupDescToml(f)
	if err != nil {
		return fmt.Errorf("Error while reading group definition file: %s", err.Error())
	}

	if group.Roster == nil || len(group.Roster.List) == 0 {
		return fmt.Errorf("Empty roster or invalid group defintion in: %s", tomlFileName)
	}

	log.Lvlf3("Checking roster %v", group.Roster.List)
	var checkErr error
	// First check all servers individually and write the working servers
	// in a list
	working := []*network.ServerIdentity{}
	for _, e := range group.Roster.List {
		desc := []string{"none", "none"}
		if d := group.GetDescription(e); d != "" {
			desc = []string{d, d}
		}
		ro := onet.NewRoster([]*network.ServerIdentity{e})
		err := checkRoster(ro, desc, true)
		if err == nil {
			working = append(working, e)
		} else {
			checkErr = err
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
				descriptions[permutation[i]] = group.GetDescription(si)
			}
			err = checkRoster(onet.NewRoster(working), descriptions, detail)
			if err != nil {
				checkErr = err
			}
		}

		// Then check pairs of servers if we want to have detail
		if detail {
			for i, first := range working {
				for _, second := range working[i+1:] {
					log.Lvl3("Testing connection between", first, second)
					desc := []string{"none", "none"}
					if d1 := group.GetDescription(first); d1 != "" {
						desc = []string{d1, group.GetDescription(second)}
					}
					es := []*network.ServerIdentity{first, second}
					err = checkRoster(onet.NewRoster(es), desc, detail)
					if err != nil {
						checkErr = err
					}
					es[0], es[1] = es[1], es[0]
					desc[0], desc[1] = desc[1], desc[0]
					err = checkRoster(onet.NewRoster(es), desc, detail)
					if err != nil {
						checkErr = err
					}
				}
			}
		}
	}
	return checkErr
}

// checkList sends a message to the cothority defined by list and
// waits for the reply.
// If the reply doesn't arrive in time, it will return an
// error.
func checkRoster(ro *onet.Roster, descs []string, detail bool) error {
	serverStr := ""
	for i, s := range ro.List {
		name := strings.Split(descs[i], " ")[0]
		if detail {
			serverStr += s.Address.NetworkAddress() + "_"
		}
		serverStr += name + " "
	}
	log.Lvl3("Sending message to: " + serverStr)
	log.Lvlf3("Checking %d server(s) %s: ", len(ro.List), serverStr)
	msg := []byte("verification")
	sig, err := SignStatement(msg, ro)
	if err != nil {
		return err
	}
	err = VerifySignatureHash(msg, sig, ro)
	if err != nil {
		return fmt.Errorf("Invalid signature: %s", err.Error())
	}

	return nil
}

// SignStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(msg []byte, ro *onet.Roster) (*blscosi.SignatureResponse, error) {
	client := blscosi.NewClient()
	publics := ro.ServicePublics(blscosi.ServiceName)

	log.Lvlf4("Signing message %x", msg)

	pchan := make(chan *blscosi.SignatureResponse, 1)
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

		err := response.Signature.Verify(client.Suite().(*pairing.SuiteBn256), msg[:], publics)
		if err != nil {
			return nil, err
		}
		return response, nil
	case <-time.After(RequestTimeOut):
		return nil, errors.New("timeout on signing request")
	}
}

// VerifySignatureHash checks that the signature is correct
func VerifySignatureHash(b []byte, sig *blscosi.SignatureResponse, ro *onet.Roster) error {
	suite := blscosi.NewClient().Suite().(*pairing.SuiteBn256)
	publics := ro.ServicePublics(blscosi.ServiceName)

	h := suite.Hash()
	_, err := h.Write(b)
	if err != nil {
		return err
	}

	hash := h.Sum(nil)
	if !bytes.Equal(hash, sig.Hash) {
		return errors.New("You are trying to verify a signature " +
			"belonging to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}

	if err := sig.Signature.Verify(suite, b, publics); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
