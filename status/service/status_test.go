package status

import (
	"testing"

	"github.com/dedis/cothority/example/channels"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}
func NewTestClient(l *onet.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

func TestServiceStatus(t *testing.T) {
	local := onet.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, tr := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	log.Lvl1("Sending request to service...")
	stat, cerr := client.Request(el.List[0])
	log.Lvl1(el.List[0])
	log.ErrFatal(cerr)
	log.Lvl1(stat)
	assert.NotEmpty(t, stat.Msg["Status"].Field["Available_Services"])
	log.Lvl1(stat.Msg["Status"])
	log.Lvl1(stat.Msg["Status"].Field["Available_Services"])
	pi, err := local.CreateProtocol("ExampleChannels", tr)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	<-pi.(*channels.ProtocolExampleChannels).ChildCount
	stat, cerr = client.Request(el.List[0])
	log.ErrFatal(cerr)
	log.Lvl1(stat)
	assert.NotEmpty(t, stat.Msg["Status"].Field["Available_Services"])
}
