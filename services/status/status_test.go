package status

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/example/channels"
	"github.com/stretchr/testify/assert"
)

func TestServiceStatus(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, tr := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	dbg.Lvl1("Sending request to service...")
	stat, err := client.GetStatus(el.List[0])
	dbg.ErrFatal(err)
	dbg.Lvl1(stat)
	assert.Equal(t, 2, stat.Connections)
	pi, err := local.CreateProtocol(tr, "ExampleChannels")
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	<-pi.(*channels.ProtocolExampleChannels).ChildCount
	stat, err = client.GetStatus(el.List[0])
	dbg.ErrFatal(err)
	dbg.Lvl1(stat)
	assert.Equal(t, 4, stat.Connections)
}
