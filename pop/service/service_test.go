package service

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func NewTestClient(lt *onet.LocalTest) *Client {
	return &Client{Client: lt.NewClient(Name)}
}

func TestServiceTemplate(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	log.Lvl1("Sending request to service...")
	duration, err := client.Clock(el)
	log.ErrFatal(err, "Couldn't send")
	log.Lvl1("It took", duration, "to go through the tree.")
}
