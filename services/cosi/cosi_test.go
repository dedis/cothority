package cosi

import (
	"testing"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
)

func TestServiceCosi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	msg := []byte("hello cosi service")
	dbg.Lvl1("Sending request to service...")
	res, err := client.SignMsg(el, msg)
	dbg.ErrFatal(err, "Couldn't send")

	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), msg, el.Aggregate,
		res.Challenge, res.Response))
}
