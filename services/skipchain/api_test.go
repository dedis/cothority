package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
)

func TestClient_RequestNewBlock(t *testing.T) {
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	sb, err := c.RequestNewBlock("", nil, el)
	if err == nil {
		t.Fatal("The block should be rejected")
	}

	sb, err = c.RequestNewBlock("accept", nil, el)
	dbg.ErrFatal(err)
	if sb == nil {
		t.Fatal("Returned SkipBlock is nil")
	}
	if sb.Index != 1 {
		t.Fatal("Root-block should be 1")
	}
}
