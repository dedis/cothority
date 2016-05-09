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
	dbg.TestOutput(testing.Verbose(), 4)
	var timeout = time.Second * 1
	local := sda.NewLocalTest()
	// generate 2 hosts
	// they connect, process messages,
	// and they register the tree or entitylist
	_, el, _ := local.GenTree(2, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	dbg.Lvl1("Sending Sealing to service...")
	res1, err := client.Seal(el, timeout)
	assert.Nil(t, err)

	dbg.Lvl1("Sending second sealing to service...")
	res2, err2 := client.Seal(el, timeout)
	assert.Nil(t, err2)
	assert.NotEqual(t, res1.ID, res2.ID)
	assert.False(t, res2.Key.Equal(res1.Key))

	time.Sleep(timeout)
	// simulate 2 client requests in parallel:

	go func() {
		op1, err := client.Open(el, res1.ID)
		assert.Nil(t, err)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, network.Suite.Point().Mul(nil, op1.Private).Equal(res1.Key))
		assert.Equal(t, op1.ID, res1.ID, "SID of req and response not equal")
		dbg.Print("Waiting for finish 1")
		time.Sleep(time.Millisecond)
	}()

	go func() {
		op2, err := client.Open(el, res2.ID)
		assert.Nil(t, err)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, op2.ID, res2.ID, "SID of req and response not equal")
		assert.True(t, network.Suite.Point().Mul(nil, op2.Private).Equal(res2.Key))
		dbg.Print("Waiting for finish 2")
		time.Sleep(time.Millisecond)
	}()
	time.Sleep(time.Second)

	dbg.Print("DONE")

}
