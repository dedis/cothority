package cli

import (
	"io"
	"io/ioutil"

	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

func ReadGroupToml(f io.Reader) (*sda.EntityList, error) {
	return nil, nil
}

// SignStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(r io.Reader,
	el *sda.EntityList,
	verify bool) (*sda.CosiResponse, error) {

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
	con, err := client.Open(host)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	// send the request
	if err := con.Send(context.TODO(), req); err != nil {
		return nil, err
	}
	// wait for the response
	packet, err := con.Receive(context.TODO())
	if err != nil {
		return nil, err
	}

	response, ok := packet.Msg.(sda.CosiResponse)
	if !ok {
		return nil, errors.New(`Invalid repsonse: Could not cast the received
		response to the right type`)
	}

	if verify { // verify signature
		err := cosi.VerifySignature(network.Suite, msgB, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
	}

}
