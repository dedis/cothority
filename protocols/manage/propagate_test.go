package manage

import (
	"testing"

	"bytes"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

type PropagateMsg struct {
	Data []byte
}

func init() {
	network.RegisterPacketType(PropagateMsg{})
}

// Tests an n-node system
func TestPropagate(t *testing.T) {
	for _, nbrNodes := range []int{3 /*10, 14*/} {
		local := sda.NewLocalTest()
		_, el, _ := local.GenTree(nbrNodes, true)
		o := local.Overlays[el.List[0].ID]

		i := 0
		msg := &PropagateMsg{[]byte("propagate")}

		tree := el.GenerateNaryTreeWithRoot(8, o.ServerIdentity())
		log.Lvl2("Starting to propagate", reflect.TypeOf(msg))
		pi, err := o.CreateProtocolSDA("Propagate", tree)
		log.ErrFatal(err)
		nodes, err := propagateStartAndWait(pi, msg, 1000,
			func(m network.Body) {
				if bytes.Equal(msg.Data, m.(*PropagateMsg).Data) {
					i++
				} else {
					t.Error("Didn't receive correct data")
				}
			})
		log.ErrFatal(err)
		if i != 1 {
			t.Fatal("Didn't get data-request")
		}
		if nodes != nbrNodes {
			t.Fatal("Not all nodes replied")
		}
		local.CloseAll()
		log.AfterTest(t)
	}
}
