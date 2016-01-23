package sda

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// Export some private functions of Host for testing

func (h *Host) SendSDAData(id *network.Entity, msg *SDAData) error {
	return h.sendSDAData(id, msg)
}

func (h *Host) Receive() network.NetworkMessage {
	return h.receive()
}

func (h *Host) StartNewNode(protocolID uuid.UUID, tree *Tree) (*Node, error) {
	return h.overlay.StartNewNode(protocolID, tree)
}

func (h *Host) StartNewNodeName(name string, tree *Tree) (*Node, error) {
	return h.overlay.StartNewNodeName(name, tree)
}

func (h *Host) AddEntityList(el *EntityList) {
	h.overlay.RegisterEntityList(el)
}

func (h *Host) EntityList(id uuid.UUID) (*EntityList, bool) {
	el := h.overlay.EntityList(id)
	return el, el != nil
}

func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

func (h *Host) GetTree(id uuid.UUID) (*Tree, bool) {
	t := h.overlay.Tree(id)
	return t, t != nil
}

func (h *Host) SendToTreeNode(from *Token, to *TreeNode, msg network.ProtocolMessage) error {
	return h.overlay.SendToTreeNode(from, to, msg)
}

func (h *Host) Overlay() *Overlay {
	return h.overlay
}

func (n *Node) Aggregate(sdaMsg *SDAData) (uuid.UUID, []*SDAData, bool) {
	return n.aggregate(sdaMsg)
}

func (o *Overlay) TokenToNode(tok *Token) (*Node, bool) {
	v, ok := o.nodes[tok.Id()]
	return v, ok
}

func (n *Node) Token() *Token {
	return n.token
}
