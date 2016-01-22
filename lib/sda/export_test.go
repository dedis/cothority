package sda

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

// Export some private functions of Host for testing

func (h *Host) AddPendingTreeMarshal(tm *TreeMarshal) {
	h.addPendingTreeMarshal(tm)
}

func (h *Host) CheckPendingTreeMarshal(el *EntityList) {
	h.checkPendingTreeMarshal(el)
}

func (h *Host) SendSDAData(id *network.Entity, msg *SDAData) error {
	return h.sendSDAData(id, msg)
}

func (h *Host) Receive() network.NetworkMessage {
	return h.receive()
}

func (h *Host) StartNewNode(protocolID uuid.UUID, tree *Tree) (*Node, error) {
	return h.overlay.StartNewNode(protocolID, tree)
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
