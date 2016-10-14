package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientTCP(t *testing.T) {
	testClient(t, NewTestRouterTCP, NewTCPClient)
}

func TestClientLocal(t *testing.T) {
	testClient(t, NewTestRouterLocal, NewLocalClient)
}

type clientFactory func() *Client

func testClient(t *testing.T, fac routerFactory, cl clientFactory) {
	r, err := fac(2000)
	if err != nil {
		t.Fatal(err)
	}
	go r.Start()

	proc := NewSendBackProc(t, r)
	r.RegisterProcessor(proc, SimpleMessageType)

	client := cl()
	nm, err := client.Send(r.ServerIdentity, &SimpleMessage{3})
	require.Nil(t, err)
	require.Equal(t, 3, nm.Msg.(SimpleMessage).I)

	// client won't have any response
	old := timeoutResponse
	timeoutResponse = 10 * time.Millisecond
	proc.drop = true
	client = cl()
	_, err = client.Send(r.ServerIdentity, &SimpleMessage{3})
	if err == nil {
		t.Fatal("Client should not be able to have a response")
	}
	// client will get an error message
	proc.err = true
	client = cl()
	_, err = client.Send(r.ServerIdentity, &SimpleMessage{3})
	if err == nil {
		t.Fatal("Client should return an error")
	}
	timeoutResponse = old

	// client won't connect
	r.Stop()
	c2 := cl()
	_, err = c2.Send(r.ServerIdentity, &SimpleMessage{3})
	if err == nil {
		t.Fatal("Should not be able to send !!")
	}
}

type SendBackProc struct {
	r *Router
	t *testing.T
	// do we drop the connection or not
	drop bool
	// sendback error
	err bool
}

func NewSendBackProc(t *testing.T, r *Router) *SendBackProc {
	return &SendBackProc{
		r: r,
		t: t,
	}
}

func (sbp *SendBackProc) Process(msg *Packet) {
	simple, ok := msg.Msg.(SimpleMessage)
	if !ok {
		sbp.t.Fatal("Not the message expected")
	}
	if sbp.drop {
		c := sbp.r.connection(msg.ServerIdentity.ID)
		if c == nil {
			sbp.t.Fatal("no connection?")
		}
		if err := c.Close(); err != nil {
			sbp.t.Fatal("closing impossible??")
		}
	} else if sbp.err {
		sbp.r.Send(msg.ServerIdentity, &StatusRet{"Error returning"})
	} else {
		err := sbp.r.Send(msg.ServerIdentity, &simple)
		assert.Nil(sbp.t, err)
	}
}
