package sda

import (
	"gopkg.in/dedis/cothority.v0/lib/dbg"
	"gopkg.in/dedis/cothority.v0/lib/network"
)

// Export some private functions of Host for testing

func (h *Host) SendSDAData(id *network.Entity, msg *Data) error {
	return h.sendSDAData(id, msg)
}

func (h *Host) Receive() network.Message {
	data := <-h.networkChan
	dbg.Lvl5("Got message", data)
	return data
}

func (h *Host) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.CreateProtocol(t, name)
}

func (h *Host) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.StartProtocol(t, name)
}

func (h *Host) EntityList(id EntityListID) (*EntityList, bool) {
	el := h.overlay.EntityList(id)
	return el, el != nil
}

func (h *Host) GetTree(id TreeID) (*Tree, bool) {
	t := h.overlay.Tree(id)
	return t, t != nil
}

func (h *Host) SendToTreeNode(from *Token, to *TreeNode, msg network.ProtocolMessage) error {
	return h.overlay.SendToTreeNode(from, to, msg)
}

func (h *Host) Overlay() *Overlay {
	return h.overlay
}

func (o *Overlay) TokenToNode(tok *Token) (*TreeNodeInstance, bool) {
	tni, ok := o.instances[tok.Id()]
	return tni, ok
}

func (h *Host) AbortConnections() error {
	h.closeConnections()
	close(h.ProcessMessagesQuit)
	return h.host.Close()
}

func (h *Host) CloseConnections() error {
	return h.closeConnections()
}

func (h *Host) RegisterConnection(e *network.Entity, c network.SecureConn) {
	h.networkLock.Lock()
	defer h.networkLock.Unlock()
	h.connections[e.ID] = c
}

func (h *Host) Connection(e *network.Entity) network.SecureConn {
	h.networkLock.RLock()
	defer h.networkLock.RUnlock()
	c, _ := h.connections[e.ID]
	return c
}
