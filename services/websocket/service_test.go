package websocket

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"golang.org/x/net/websocket"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceTemplate(t *testing.T) {
	defer log.AfterTest(t)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	url, err := getWebHost(el.List[0])
	log.ErrFatal(err)
	ws, err := websocket.Dial(url, "/root", "hi")
	log.ErrFatal(err)
	websocket.Message.Send(ws, "hi there")
}
