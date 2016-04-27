package timevault

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/stretchr/testify/assert"
)

func TestSealOpen(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 3)
	var timeout = time.Second * 1
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(2, false, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	dbg.Lvl1("Sending Sealing to service...")
	res, err := client.Seal(el, timeout)
	assert.Nil(t, err)

	/* dbg.Lvl1("Sending second sealing to service...")*/
	//res2, err2 := client.Seal(el, time.Second*5)
	//assert.Nil(t, err2)

	//assert.NotEqual(t, res.ID, res2.ID)
	//assert.False(t, res2.Key.Equal(res.Key))

	time.Sleep(timeout)

	op1, err := client.Open(el, res.ID)
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, network.Suite.Point().Mul(nil, op1.Private).Equal(res.Key))
}
