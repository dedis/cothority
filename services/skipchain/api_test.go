package skipchain

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/config"
	"testing"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestActiveAdd(t *testing.T) {
	pair := config.NewKeyPair(network.Suite)
	addr := "localhost:2000"
	e := network.NewEntity(pair.Public, addr)
	host := sda.NewHost(e, pair.Secret)
	host.ListenAndBind()
	sb0 := NewSkipBlock()
	aar, err := SendActiveAdd(e, nil, sb0)
	dbg.ErrFatal(err)
	if aar == nil {
		t.Fatal("Returned SkipBlock is nil")
	}
}
