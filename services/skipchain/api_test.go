package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func TestActiveAdd(t *testing.T) {
	l := sda.NewLocalTest()
	_, _, tree := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	aar, err := c.AddSkipBlock("", tree)
	dbg.ErrFatal(err)
	if aar == nil {
		t.Fatal("Returned SkipBlock is nil")
	}
}
