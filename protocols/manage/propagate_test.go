package manage_test

import (
	"testing"

	"bytes"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/lib/network"
)

type PropagateMsg struct{
	Data []byte
}

func init(){
	network.RegisterMessageType(PropagateMsg{})
}

// Tests an n-node system
func TestPropagate(t *testing.T) {
	//defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 3)
	for _, nbrNodes := range []int{3, 10, 14} {
		local := sda.NewLocalTest()
		_, _, tree := local.GenTree(nbrNodes, false, true, true)

		i := 0
		msg := &PropagateMsg{[]byte("propagate")}
		nodes, err := manage.PropagateStartAndWait(local, tree, msg, 1000,
			func(m network.ProtocolMessage) {
				if bytes.Equal(msg.Data, m.(*PropagateMsg).Data) {
					i++
				} else {
					t.Error("Didn't receive correct data")
				}
			})
		dbg.ErrFatal(err)
		if i != 1 {
			t.Fatal("Didn't get data-request")
		}
		if nodes != nbrNodes {
			t.Fatal("Not all nodes replied")
		}
		local.CloseAll()
	}
}
