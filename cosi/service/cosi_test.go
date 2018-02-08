package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var tSuite = cothority.Suite

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceCosi(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service to all hosts
	client := NewClient()
	msg := []byte("hello cosi service")
	serviceReq := &SignatureRequest{
		Roster:  el,
		Message: msg,
	}
	for _, dst := range el.List {
		reply := &SignatureResponse{}
		log.Lvl1("Sending request to service...")
		err := client.SendProtobuf(dst, serviceReq, reply)
		require.Nil(t, err, "Couldn't send")

		// verify the response still
		suite := suites.MustFind(hosts[0].Suite().String())
		require.Nil(t, cosi.Verify(suite, el.Publics(), msg, reply.Signature, cosi.CompletePolicy{}))
	}
}

func TestCreateAggregate(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	msg := []byte("hello cosi service")
	log.Lvl1("Sending request to service...")

	el1 := &onet.Roster{}
	_, err := client.SignatureRequest(el1, msg)
	require.NotNil(t, err)
	// Create a roster with a missing aggregate and ID.
	el2 := &onet.Roster{List: el.List}
	res, err := client.SignatureRequest(el2, msg)
	require.Nil(t, err, "Couldn't send")

	// verify the response still
	suite := suites.MustFind(hosts[0].Suite().String())
	require.Nil(t, cosi.Verify(suite, el.Publics(), msg, res.Signature, cosi.CompletePolicy{}))
}
