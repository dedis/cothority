package cosi

import (
	"encoding/json"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"testing"
)

func TestServiceCosi(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()
	// creating the hosts automatically registers the cosi service with them
	// (you still have to do a
	// sda.RegisterNewService("Cosi", newCosiService)
	// beforehand). The client (see below) can contact each host to get a
	// CoSi signature on a message.

	// Send a request to the service
	var msg = []byte("hello cosi service")
	req := Request{
		EntityList: el,
		Message:    msg,
	}

	buffRequest, err := network.MarshalRegisteredType(&req)
	assert.Nil(t, err)

	re := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("Cosi"),
		Type:    CosiRequestType,
		Data:    json.RawMessage(buffRequest),
	}
	// fake a client: it sends a request (here, a message to be signed)
	private, public := sda.PrivPub()
	client := network.NewSecureTCPHost(private, network.NewEntity(public, ""))
	defer client.Close()
	dbg.Lvl1("Client connecting to host")
	var c network.SecureConn
	if c, err = client.Open(hosts[0].Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	assert.Nil(t, c.Send(context.TODO(), re))

	// receive the response
	var nm network.Message
	var resp Response
	nm, err = c.Receive(context.TODO())
	assert.Nil(t, err)
	assert.Equal(t, nm.MsgType, CosiResponseType)

	resp = nm.Msg.(Response)
	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), msg, el.Aggregate, resp.Challenge, resp.Response))
}
