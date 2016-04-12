package services

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
	// generate 5 hosts, they dont connect, they process messages and they
	// dont register the tree or entitylist
	hosts, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	var msg = []byte("hello cosi service")
	req := CosiRequest{
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
	// fake a client
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
	var resp CosiResponse
	nm, err = c.Receive(context.TODO())
	assert.Nil(t, err)
	assert.Equal(t, nm.MsgType, CosiResponseType)

	resp = nm.Msg.(CosiResponse)
	// verify the response still
	assert.Nil(t, cosi.VerifySignature(hosts[0].Suite(), msg, el.Aggregate, resp.Challenge, resp.Response))
}
