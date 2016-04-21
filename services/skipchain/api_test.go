package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestActiveAdd(t *testing.T) {
	l := sda.NewLocalTest()
	_, _, tree := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	sb0 := NewSkipBlock(tree)
	aar, err := c.AddSkipBlock(nil, sb0)
	dbg.ErrFatal(err)
	if aar == nil {
		t.Fatal("Returned SkipBlock is nil")
	}
}
