package guard

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.M) {
	log.MainTest(t)
}

func TestServiceGuard(t *testing.T) {
	local := sda.NewLocalTest()
	// This statement generates 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist.
	_, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
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
