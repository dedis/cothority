package service

import (
	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	network.RegisterMessage(&PropagateIdentity{})
	network.RegisterMessage(&UpdateSkipBlock{})
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	NewBlock *skipchain.SkipBlock
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID     skipchain.SkipBlockID
	Latest *skipchain.SkipBlock
}
