package skipchain

import (
	"testing"

	"github.com/dedis/cothority/lib/sda"
)

func TestClientGenesis(t *testing.T) {
	l := sda.NewLocalTest()
	l.GenTree(5, true, true, true)
	defer l.CloseAll()

	c := NewClient()
	c.ProposeSkipBlock(nil, nil)
}

func TestClient_ProposeSkipBlock(t *testing.T) {

}

func TestClient_GetUpdateChain(t *testing.T) {

}
