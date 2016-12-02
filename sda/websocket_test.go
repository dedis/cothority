package sda

import (
	"testing"

	"fmt"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"github.com/satori/go.uuid"
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
	log.Printf("Sending message Request: %x", uuid.UUID(network.TypeFromData(req)).Bytes())
	buf, err := protobuf.Encode(req)
	log.ErrFatal(err)
	err = websocket.Message.Send(ws, buf)
	log.ErrFatal(err)

	log.Lvl1("Waiting for reply")
	time.Sleep(time.Second)
	var rcv []byte
	err = websocket.Message.Receive(ws, &rcv)
	log.ErrFatal(err)
	log.Lvlf1("Received reply: %x", rcv)
}

const serviceWebSocket = "WebSocket"

type ServiceWebSocket struct {
	*ServiceProcessor
	GotResponse chan int
}

func (i *ServiceWebSocket) SimpleResponse(msg *SimpleResponse) (network.Body, error) {
	i.GotResponse <- msg.Val
	return nil, nil
}

func (i *ServiceWebSocket) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	return nil, nil
}

func newServiceWebSocket(c *Context, path string) Service {
	s := &ServiceWebSocket{
		ServiceProcessor: NewServiceProcessor(c),
		GotResponse:      make(chan int),
	}
	log.ErrFatal(s.RegisterMessage(s.SimpleResponse))
	return s
}
