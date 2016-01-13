package sda

// Export some private functions of Host for testing

func (n *Host) AddPendingTreeMarshal(tm *TreeMarshal) {
	n.addPendingTreeMarshal(tm)
}

func (n *Host) CheckPendingTreeMarshal(el *EntityList) {
	n.checkPendingTreeMarshal(el)
}
