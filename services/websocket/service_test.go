package websocket

import (
	"testing"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
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
	time.Sleep(2 * time.Second)

	url, err := getWebHost(el.List[0])
	log.ErrFatal(err)
	ws, err := websocket.Dial("ws://"+url+"/root", "", "http://localhost/")
	log.ErrFatal(err)
	log.Print("Sending message")
	websocket.Message.Send(ws, "hi there")
}
