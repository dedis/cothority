package sda

import "github.com/dedis/cothority/network"

// Export some private functions of Host for testing

func (h *Conode) SendSDAData(id *network.ServerIdentity, msg *ProtocolMsg) error {
	return h.overlay.sendSDAData(id, msg)
}

func (h *Conode) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.CreateProtocolSDA(name, t)
}

func (h *Conode) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return h.overlay.StartProtocol(t, name)
}

func (h *Conode) Roster(id RosterID) (*Roster, bool) {
	el := h.overlay.Roster(id)
	return el, el != nil
}

func (h *Conode) GetTree(id TreeID) (*Tree, bool) {
	t := h.overlay.Tree(id)
	return t, t != nil
}

func (h *Conode) SendToTreeNode(from *Token, to *TreeNode, msg network.Body) error {
	return h.overlay.SendToTreeNode(from, to, msg)
}

func (h *Conode) Overlay() *Overlay {
	return h.overlay
}

func (o *Overlay) TokenToNode(tok *Token) (*TreeNodeInstance, bool) {
	tni, ok := o.instances[tok.ID()]
	return tni, ok
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
func (h *Conode) AddTree(t *Tree) {
	h.overlay.RegisterTree(t)
}

// AddRoster registers the given Roster in the underlying overlay.
// Useful for unit-testing only.
func (h *Conode) AddRoster(el *Roster) {
	h.overlay.RegisterRoster(el)
}
