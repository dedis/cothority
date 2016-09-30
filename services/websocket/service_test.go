package websocket

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/status"
	_ "github.com/dedis/cothority/services/status"
	"golang.org/x/net/websocket"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceTemplate(t *testing.T) {
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
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
	websocket.Message.Send(ws, buf)
}
