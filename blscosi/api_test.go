package blscosi

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestClient_SignatureRequest(t *testing.T) {
	local := onet.NewTCPTest(testSuite)
	_, roster, _ := local.GenTree(10, false)
	defer local.CloseAll()

	// Send a request to the service to all hosts
	client := NewClient()
	msg := []byte("hello blscosi service")

	_, err := client.SignatureRequest(&onet.Roster{}, msg)
	require.Error(t, err)

	for _, dst := range roster.List {
		newRoster := roster.NewRosterWithRoot(dst)
		log.Lvlf1("Sending request to service... %v", dst)
		reply, err := client.SignatureRequest(newRoster, msg)
		require.Nil(t, err, "Couldn't send")

		publics := newRoster.ServicePublics(ServiceName)

		// verify the response still
		require.Nil(t, reply.Signature.Verify(testSuite, msg, publics))
	}
}
