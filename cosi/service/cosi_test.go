package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/dedis/crypto.v0/cosi"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

func NewTestClient(l *onet.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

func TestServiceCosi(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	msg := []byte("hello cosi service")
	log.Lvl1("Sending request to service...")
	res, err := client.SignatureRequest(el, msg)
	log.ErrFatal(err, "Couldn't send")

	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), el.Publics(),
		msg, res.Signature))
}

func TestServiceCosi2(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service to the second host
	client := NewTestClient(local)
	msg := []byte("hello cosi service")
	serviceReq := &SignatureRequest{
		Roster:  el,
		Message: msg,
	}
	dst := el.List[1]
	reply := &SignatureResponse{}
	log.Lvl1("Sending request to service...")
	cerr := client.SendProtobuf(dst, serviceReq, reply)
	log.ErrFatal(cerr, "Couldn't send")

	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), el.Publics(),
		msg, reply.Signature))
}

func TestCreateAggregate(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	msg := []byte("hello cosi service")
	log.Lvl1("Sending request to service...")

	// Create a roster with a missing aggregate and ID.
	el2 := &onet.Roster{List: el.List}
	res, err := client.SignatureRequest(el2, msg)
	log.ErrFatal(err, "Couldn't send")

	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), el.Publics(),
		msg, res.Signature))
}
