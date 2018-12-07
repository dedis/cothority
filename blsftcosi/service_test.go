package blsftcosi

import (
	"testing"

	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var testSuite = pairing.NewSuiteBn256()

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceCosi(t *testing.T) {
	local := onet.NewTCPTest(testSuite)
	_, roster, _ := local.GenTree(10, false)
	defer local.CloseAll()

	// Send a request to the service to all hosts
	client := NewClient()
	msg := []byte("hello blsftcosi service")
	serviceReq := &SignatureRequest{
		Roster:  roster,
		Message: msg,
	}

	publics := roster.ServicePublics(ServiceName)

	for _, dst := range roster.List {
		reply := &SignatureResponse{}
		log.Lvlf1("Sending request to service... %v", dst)
		err := client.SendProtobuf(dst, serviceReq, reply)
		require.Nil(t, err, "Couldn't send")

		// need to correct the roster as _dst_ becomes the root
		// for the protocol, the order of the public keys is different
		publics = roster.NewRosterWithRoot(dst).ServicePublics(ServiceName)

		// verify the response still
		require.Nil(t, reply.Signature.Verify(testSuite, msg, publics))
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
	el2 := roster
	res, err := client.SignatureRequest(el2, msg)
	require.Nil(t, err, "Couldn't send")

	publics := roster.ServicePublics(ServiceName)

	// verify the response still
	require.Nil(t, res.Signature.Verify(testSuite, msg, publics))
}
