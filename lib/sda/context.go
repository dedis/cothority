package sda

import (
	"github.com/dedis/cothority/lib/network"
)

type Context interface {
	NewTreeNodeInstance(*Tree, *TreeNode) *TreeNodeInstance
	RegisterProtocolInstance(ProtocolInstance) error
	SendRaw(*network.Entity, interface{}) error
}

// defaultContext is the implementation of the Context interface. It is
// instantiated for each Service.
type defaultContext struct {
	*Overlay
	*Host
	servID ServiceID
}

func newDefaultContext(h *Host, o *Overlay, servID ServiceID) *defaultContext {
	return &defaultContext{
		Overlay: o,
		Host:    h,
		servID:  servID,
	}
}

// NewTreeNodeInstance implements the Context interface method
func (dc *defaultContext) NewTreeNodeInstance(t *Tree, tn *TreeNode) *TreeNodeInstance {
	return dc.Overlay.newTreeNodeInstance(t, tn, dc.servID)
}

func (dc *defaultContext) SendRaw(e *network.Entity, msg interface{}) error {
	return dc.Host.SendRaw(e, msg)
}

//
// DESIGN IDEA ... why not !
//

/*type Router interface {*/
//Send(network.Entity,msg interface{})
//RegisterProcessor(msg interface{},p Processor)
//ActiveConnections() []Conn
// // new and dropped connection event and status of connections as handlers
//RegisterNetworkEvent(e *Event)
//}

//type Processor interface {
//ProcessMessage(msg interface{})
//}

//type ProcessHandler struct {

//}

//func (ProcessHandler) RegisterHandler() {

//}

//func (PH) Process(msg interface{}) {
//// Dispatch to handlers
//}

//type ProtocolInstance interface {
//Processor
//Shutdown()
//Token() *Token
//}

//type Service interface {
//Processor

/*}*/
