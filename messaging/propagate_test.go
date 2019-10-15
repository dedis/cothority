package messaging

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

type propagateMsg struct {
	Data []byte
}

func init() {
	network.RegisterMessage(propagateMsg{})
}

func TestPropagation(t *testing.T) {
	propagate(t,
		[]int{3, 10, 14, 4, 8, 8},
		[]int{0, 0, 0, 1, 3, 6})
}

// Tests an n-node system
func propagate(t *testing.T, nbrNodes, nbrFailures []int) {
	for i, n := range nbrNodes {
		local := onet.NewLocalTest(tSuite)
		servers, el, _ := local.GenTree(n, true)
		var recvCount int
		var iMut sync.Mutex
		msg := &propagateMsg{[]byte("propagate")}
		propFuncs := make([]PropagationFunc, n)

		// setup the servers
		var err error
		for n, server := range servers {
			pc := &PC{server, local.Overlays[server.ServerIdentity.ID]}
			propFuncs[n], err = NewPropagationFunc(pc,
				"Propagate",
				func(m network.Message) error {
					if bytes.Equal(msg.Data, m.(*propagateMsg).Data) {
						iMut.Lock()
						recvCount++
						iMut.Unlock()
						return nil
					}

					t.Error("Didn't receive correct data")
					return errors.New("Didn't receive correct data")
				}, nbrFailures[i])
			log.ErrFatal(err)
		}

		// shut down some servers to simulate failure
		for k := 0; k < nbrFailures[i]; k++ {
			err = servers[len(servers)-1-k].Close()
			log.ErrFatal(err)
		}

		// start the propagation
		log.Lvl2("Starting to propagate", reflect.TypeOf(msg))
		children, err := propFuncs[0](el, msg, 1*time.Second)
		log.ErrFatal(err)
		if recvCount+nbrFailures[i] != n {
			t.Fatal("Didn't get data-request")
		}
		if children+nbrFailures[i] != n {
			t.Fatal("Not all nodes replied")
		}

		local.CloseAll()
		log.AfterTest(t)
	}
}

type PC struct {
	C *onet.Server
	O *onet.Overlay
}

func (pc *PC) ProtocolRegister(name string, protocol onet.NewProtocol) (onet.ProtocolID, error) {
	return pc.C.ProtocolRegister(name, protocol)
}
func (pc *PC) ServerIdentity() *network.ServerIdentity {
	return pc.C.ServerIdentity

}
func (pc *PC) CreateProtocol(name string, t *onet.Tree) (onet.ProtocolInstance, error) {
	return pc.O.CreateProtocol(name, t, onet.NilServiceID)
}
