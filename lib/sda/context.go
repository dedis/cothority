package sda

import (
	"errors"
)

// Context is created for each Service
type Context struct {
	o *Overlay
}

func (c *Context) NewTreeNodeInstance(tn *TreeNode) *TreeNodeInstance {
	return &TreeNodeInstance{}
}

func (c *Context) RegisterProtocolInstance(pi ProtocolInstance) error {
	return errors.New("Not implemented")
}
