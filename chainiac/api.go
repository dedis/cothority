package chainiac

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

// CreateRootControl is a convenience function and creates two Skipchains:
// a root SkipChain with maximumHeight of maxHRoot and a control SkipChain with
// maximumHeight of maxHControl. It connects both chains for later
// reference. The root-chain will use `VerificationRoot` and the config-chain
// will use `VerificationConfig`.
//
// A slice of verification-functions is given for the root and the control
// service.
func (c *Client) CreateRootControl(elRoot, elControl *onet.Roster,
	keys []abstract.Point, baseHeight,
	maxHRoot, maxHControl int) (root, control *SkipBlock, cerr onet.ClientError) {
	log.Lvl2("Creating root roster", elRoot)
	root, cerr = c.CreateGenesis(elRoot, baseHeight, maxHRoot,
		VerificationRoot, nil, nil)
	if cerr != nil {
		return
	}
	log.Lvl2("Creating control roster", elControl)
	control, cerr = c.CreateGenesis(elControl, baseHeight, maxHControl,
		VerificationControl, nil, root.Hash)
	if cerr != nil {
		return
	}
	return root, control, cerr
}
