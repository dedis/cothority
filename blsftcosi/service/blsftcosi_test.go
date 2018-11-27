package service

import (
	"testing"

	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var testSuite = bn256.NewSuiteG2()

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceCosi(t *testing.T) {
	local := onet.NewTCPTest(testSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(10, false)
	defer local.CloseAll()

	// Send a request to the service to all hosts
	client := NewClient()
	msg := []byte("hello blsftcosi service")
	serviceReq := &SignatureRequest{
		Roster:  roster,
		Message: msg,
	}

	for _, dst := range roster.List {
		reply := &SignatureResponse{}
		log.Lvl1("Sending request to service...")
		err := client.SendProtobuf(dst, serviceReq, reply)
		require.Nil(t, err, "Couldn't send")

		// verify the response still
		require.Nil(t, reply.Signature.Verify(testSuite, msg, roster.Publics(), cosi.CompletePolicy{}))

	}
}

func TestCreateAggregate(t *testing.T) {
	local := onet.NewTCPTest(testSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(10, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	msg := []byte("hello blsftcosi service")
	log.Lvl1("Sending request to service...")

	el1 := &onet.Roster{}
	_, err := client.SignatureRequest(el1, msg)
	require.NotNil(t, err)
	// Create a roster with a missing aggregate and ID.
	el2 := &onet.Roster{List: roster.List}
	res, err := client.SignatureRequest(el2, msg)
	require.Nil(t, err, "Couldn't send")

	// verify the response still
	require.Nil(t, res.Signature.Verify(testSuite, msg, roster.Publics(), cosi.CompletePolicy{}))
}
