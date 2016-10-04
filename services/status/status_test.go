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
func NewTestClient(l *sda.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

func TestServiceStatus(t *testing.T) {
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, tr := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	log.Lvl1("Sending request to service...")
	stat, err := client.GetStatus(el.List[0])
	log.Lvl1(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	assert.NotEmpty(t, stat.Msg["Status"]["Available_Services"])
	pi, err := local.CreateProtocol("ExampleChannels", tr)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	<-pi.(*channels.ProtocolExampleChannels).ChildCount
	stat, err = client.GetStatus(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	assert.NotEmpty(t, stat.Msg["Status"]["Available_Services"])
}
