package service

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/cosi"
	"github.com/stretchr/testify/assert"
)

func NewTestClient(l *sda.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

func TestServiceCosi(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	msg := []byte("hello cosi service")
	log.Lvl1("Sending request to service...")
	res, err := client.SignMsg(el, msg)
	log.ErrFatal(err, "Couldn't send")

	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), el.Publics(),
		msg, res.Signature))
}
