package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	defer r.Stop()

	proc := NewSendBackProc(t, r)
	r.RegisterProcessor(proc, SimpleMessageType)

	client := cl()
	nm, err := client.Send(r.id, &SimpleMessage{3})
	assert.Nil(t, err)
	assert.Equal(t, 3, nm.Msg.(SimpleMessage).I)
}

type SendBackProc struct {
	r *Router
	t *testing.T
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
	err := sbp.r.Send(msg.ServerIdentity, &simple)
	assert.Nil(sbp.t, err)
}
