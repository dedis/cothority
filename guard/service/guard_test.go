package guard

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.M) {
	log.MainTest(t)
}

func NewLocalTestClient(l *onet.LocalTest) *Client {
	return &Client{l.NewClient(ServiceName)}
}

func TestServiceGuard(t *testing.T) {
	local := onet.NewTCPTest()
	_, el, _ := local.GenTree(5, true)
	defer local.CloseAll()

	// Send a request to the service
	client := NewLocalTestClient(local)
	log.Lvl1("Sending request to service...")
	UID := []byte("USER")
	Epoch := []byte("EPOCH")
	msg := network.Suite.Point()

	Hzi, _ := client.SendToGuard(el.List[0], UID, Epoch, msg)
	// We send the message twice to see that the key did not change for the
	//same epoch.
	Hz2, _ := client.SendToGuard(el.List[0], UID, Epoch, msg)
	assert.Equal(t, Hzi, Hz2)

}
