package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/cosi"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

var tSuite = cothority.Suite

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceCosi(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service to all hosts
	client := NewClient()
	msg := []byte("hello ftcosi service")
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
		require.Nil(t, cosi.Verify(tSuite, roster.Publics(), msg, reply.Signature, cosi.CompletePolicy{}))
	}
}

func TestCreateAggregate(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	msg := []byte("hello ftcosi service")
	log.Lvl1("Sending request to service...")

	el1 := &onet.Roster{}
	_, err := client.SignatureRequest(el1, msg)
	require.NotNil(t, err)
	// Create a roster with a missing aggregate and ID.
	el2 := &onet.Roster{List: roster.List}
	res, err := client.SignatureRequest(el2, msg)
	require.Nil(t, err, "Couldn't send")

	// verify the response still
	require.Nil(t, cosi.Verify(tSuite, roster.Publics(), msg, res.Signature, cosi.CompletePolicy{}))
}
