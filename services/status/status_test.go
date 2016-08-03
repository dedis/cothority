package status

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/protocols/example/channels"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// NewClient makes a new Client
func NewTestClient() *Client {
	return &Client{Client: sda.NewLocalClient(ServiceName)}
}
func TestServiceStatus(t *testing.T) {
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist.
	// Branching factor of 2 by default
	_, el, tr := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient()
	stat, err := client.GetStatus(el.List[0])
	log.Lvl1(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	// 1 connection from host to client. Before it was 2 because of the host
	// connecting to itself...
	assert.Equal(t, "1", stat.Msg["Status"]["Total"])
	pi, err := local.CreateProtocol(tr, "ExampleChannels")
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	<-pi.(*channels.ProtocolExampleChannels).ChildCount
	stat, err = client.GetStatus(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	// 3 connections : 1 host<->client, and 3 between host(root) and the
	// children node in the tree which has a BF of 2
	// before it was 4 because of the connection to itself...

	assert.Equal(t, "3", stat.Msg["Status"]["Total"])
}
