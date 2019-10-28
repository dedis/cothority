package blscosi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

func TestClient_SignatureRequest(t *testing.T) {
	local := onet.NewLocalTest(serviceTestBuilder)
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

		publics := newRoster.PublicKeys(ServiceName)

		// verify the response still
		pubkey, err := testSuite.AggregatePublicKeys(publics, reply.Signature)
		require.NoError(t, err)
		require.NoError(t, testSuite.Verify(pubkey, reply.Signature, msg))
	}
}
