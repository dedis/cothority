package sda

import (
	"testing"

	"fmt"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

func init() {
	RegisterNewService(serviceWebSocket, newServiceWebSocket)
}

func TestNewWebSocket(t *testing.T) {
	c := NewTCPConode(0)
	defer c.Close()
	require.Equal(t, len(c.serviceManager.services), len(c.websocket.services))
	require.NotEmpty(t, c.websocket.services[serviceWebSocket])
	url, err := getWebHost(c.ServerIdentity)
	log.ErrFatal(err)
	ws, err := websocket.Dial(fmt.Sprintf("ws://%s/WebSocket/SimpleResponse", url),
		"", "http://localhost/")
	log.ErrFatal(err)
	req := &SimpleResponse{}
	log.Lvlf1("Sending message Request: %x", uuid.UUID(network.TypeFromData(req)).Bytes())
	buf, err := protobuf.Encode(req)
	log.ErrFatal(err)
	log.ErrFatal(websocket.Message.Send(ws, buf))

	log.Lvl1("Waiting for reply")
	var rcv []byte
	log.ErrFatal(websocket.Message.Receive(ws, &rcv))
	log.Lvlf1("Received reply: %x", rcv)
	rcvMsg := &SimpleResponse{}
	log.ErrFatal(protobuf.Decode(rcv, rcvMsg))
	assert.Equal(t, 1, rcvMsg.Val)
}

const serviceWebSocket = "WebSocket"

type ServiceWebSocket struct {
	*ServiceProcessor
}

func (i *ServiceWebSocket) SimpleResponse(msg *SimpleResponse) (network.Body, int) {
	return &SimpleResponse{msg.Val + 1}, 0
}

func newServiceWebSocket(c *Context, path string) Service {
	s := &ServiceWebSocket{
		ServiceProcessor: NewServiceProcessor(c),
	}
	log.ErrFatal(s.RegisterMessage(s.SimpleResponse))
	return s
}
