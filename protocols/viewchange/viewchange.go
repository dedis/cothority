package bizcoin

type ViewChange struct {
	// the node we are
	*sda.Node
	// the new entitylist we wish to change to
	el *sda.EntityList
	// the new tree we wish to change to
	tree *sda.Tree
}

// NewViewChange is the simple function needed to register to SDA.
func NewViewChange(node *sda.Node) (*ViewChange, error) {
	vcp := &ViewChange{
		Node: node,
	}
	return vcp, nil
}

// SetupViewChange the function that a protocol can call when it
// needs to operate a view change. You must supply here the new entityList + new
// Tree you wish to apply to the tree.
func Propagate(entityList *sda.EntityList, tree *sda.Tree) (*ViewChange, error) {
	vcp := NewViewChange(node)
	vcp.el = entityList
	vcp.tree = tree
}

func (vcp *ViewChange) Start() error {
}

func (vcp *ViewChange) Dispatch() error {

}

// waitAgreement will wait until it receis 2/3 of the peers.
func (vcp *ViewChange) waitAgreement() {

}
