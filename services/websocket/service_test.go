package websocket

import (
	"testing"

	"encoding/binary"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/status"
	_ "github.com/dedis/cothority/services/status"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestStatus(t *testing.T) {
	local := sda.NewLocalTest()
	_, el, _ := local.GenTree(2, false, true, false)
	defer local.CloseAll()

	url, err := getWebHost(el.List[0])
	log.ErrFatal(err)
	ws, err := websocket.Dial("ws://"+url+"/status", "", "http://localhost/")
	log.ErrFatal(err)
	req := &status.Request{}
	log.Print("Sending message")
	buf, err := network.MarshalRegisteredType(req)
	log.ErrFatal(err)
	size := make([]byte, 2)
	binary.LittleEndian.PutUint16(size, uint16(len(buf)))
	err = websocket.Message.Send(ws, size)
	log.ErrFatal(err)
	err = websocket.Message.Send(ws, buf)
	log.ErrFatal(err)

	log.Lvl1("Waiting for reply")
	var rcv []byte
	err = websocket.Message.Receive(ws, &rcv)
	log.ErrFatal(err)
	log.Lvlf1("Received reply: %x", rcv)
	_, stat, err := network.UnmarshalRegistered(rcv)
	status, ok := stat.(*status.Response)
	require.True(t, ok)
	log.Lvl1("Received correct status-reply:", status)
}

func TestPong(t *testing.T) {
	local := sda.NewLocalTest()
	_, el, _ := local.GenTree(2, false, true, false)
	defer local.CloseAll()

	url, err := getWebHost(el.List[0])
	log.ErrFatal(err)
	ws, err := websocket.Dial("ws://"+url+"/ping", "", "http://localhost/")
	log.ErrFatal(err)
	err = websocket.Message.Send(ws, "ping")
	var rcv []byte
	err = websocket.Message.Receive(ws, &rcv)
	log.ErrFatal(err)
	log.Lvlf1("Received reply: %s", rcv)
}
