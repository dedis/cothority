package sda

import "github.com/dedis/cothority/network"

// Export some private functions of Host for testing

func (c *Conode) SendSDAData(id *network.ServerIdentity, msg *ProtocolMsg) error {
	return c.overlay.sendSDAData(id, msg)
}

func (c *Conode) CreateProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return c.overlay.CreateProtocolSDA(name, t)
}

func (c *Conode) StartProtocol(name string, t *Tree) (ProtocolInstance, error) {
	return c.overlay.StartProtocol(t, name)
}

func (c *Conode) Roster(id RosterID) (*Roster, bool) {
	el := c.overlay.Roster(id)
	return el, el != nil
}

func (c *Conode) GetTree(id TreeID) (*Tree, bool) {
	t := c.overlay.Tree(id)
	return t, t != nil
}

func (c *Conode) SendToTreeNode(from *Token, to *TreeNode, msg network.Body) error {
	return c.overlay.SendToTreeNode(from, to, msg)
}

func (c *Conode) Overlay() *Overlay {
	return c.overlay
}

func (o *Overlay) TokenToNode(tok *Token) (*TreeNodeInstance, bool) {
	tni, ok := o.instances[tok.ID()]
	return tni, ok
}

// AddTree registers the given Tree struct in the underlying overlay.
// Useful for unit-testing only.
func (c *Conode) AddTree(t *Tree) {
	c.overlay.RegisterTree(t)
}

// AddRoster registers the given Roster in the underlying overlay.
// Useful for unit-testing only.
func (c *Conode) AddRoster(el *Roster) {
	c.overlay.RegisterRoster(el)
}
