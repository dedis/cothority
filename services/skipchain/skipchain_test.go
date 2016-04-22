package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestService(t *testing.T) {
	local := sda.NewLocalTest()

	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	client := NewClient()
	_, err := client.RequestNewBlock("", el)
	dbg.ErrFatal(err)
}
