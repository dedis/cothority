package sda

import "github.com/dedis/cothority/network"

// Export some private functions of Host for testing

func (h *Host) SendSDAData(id *network.ServerIdentity, msg *ProtocolMsg) error {
	return h.overlay.sendSDAData(id, msg)
}

func (h *Host) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.CreateProtocolSDA(name, t)
}

func (h *Host) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.StartProtocol(t, name)
}

func (h *Host) Roster(id RosterID) (*Roster, bool) {
	el := h.overlay.Roster(id)
	return el, el != nil
}

func (h *Host) GetTree(id TreeID) (*Tree, bool) {
	t := h.overlay.Tree(id)
	return t, t != nil
}

func (h *Host) SendToTreeNode(from *Token, to *TreeNode, msg network.Body) error {
	return h.overlay.SendToTreeNode(from, to, msg)
}

func (h *Host) Overlay() *Overlay {
	return h.overlay
}

func (o *Overlay) TokenToNode(tok *Token) (*TreeNodeInstance, bool) {
	tni, ok := o.instances[tok.ID()]
	return tni, ok
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
func (h *Host) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

// AddRoster registers the given Roster in the underlying overlay.
// Useful for unit-testing only.
func (h *Host) AddRoster(el *Roster) {
	h.overlay.RegisterRoster(el)
}
