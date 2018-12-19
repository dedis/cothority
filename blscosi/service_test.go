package blscosi

import (
	"testing"

	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var testSuite = pairing.NewSuiteBn256()

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_SignatureRequest(t *testing.T) {
	local := onet.NewTCPTest(testSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(10, false)
	defer local.CloseAll()

	service := hosts[0].Service(ServiceName).(*Service)

	// Send a request to the service
	msg := []byte("hello blscosi service")
	log.Lvl1("Sending request to service...")

	// empty roster should fail
	ro1 := &onet.Roster{}
	_, err := service.SignatureRequest(&SignatureRequest{
		Roster:  ro1,
		Message: msg,
	})
	require.Contains(t, err.Error(), "we're not in the roster")

	// wrong subtree number should fail
	ro2 := roster
	service.NSubtrees = len(roster.List) + 1
	_, err = service.SignatureRequest(&SignatureRequest{
		Roster:  ro2,
		Message: msg,
	})
	require.NotNil(t, err)

	// missing message should fail
	service.NSubtrees = 3
	service.Threshold = 1
	_, err = service.SignatureRequest(&SignatureRequest{
		Roster:  ro2,
		Message: nil,
	})
	require.NotNil(t, err)

	// correct request
	buf, err := service.SignatureRequest(&SignatureRequest{
		Roster:  ro2,
		Message: msg,
	})
	require.Nil(t, err, "Couldn't send")

	publics := roster.ServicePublics(ServiceName)
	res := buf.(*SignatureResponse)

	// verify the response still
	require.Nil(t, res.Signature.VerifyWithPolicy(testSuite, msg, publics, cosi.NewThresholdPolicy(1)))
}
