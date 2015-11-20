package sign

import (
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

func (sn *Node) SetupProposal(view int, am *AnnouncementMessage, from string) error {
	dbg.Fatal("SetupProposal not implemented anymore")
	return nil
}

// A propose for a view change would come on current view + sth
// when we receive view change  message on a future view,
// we must be caught up, create that view  and apply actions on it
func (sn *Node) Propose(view int, RoundNbr int, am *AnnouncementMessage, from string) error {
	dbg.Fatal("Propose not implemented anymore")
	return nil
}

func (sn *Node) Promise(view, Round int, sm *SigningMessage) error {
	dbg.Fatal("Promise not implemented anymore")
	return nil
}

func (sn *Node) Accept(view, RoundNbr int, chm *ChallengeMessage) error {
	dbg.Fatal("Accept not implemented anymore")
	return nil
}

func (sn *Node) Accepted(view, Round int, sm *SigningMessage) error {
	dbg.Fatal("Accepted not implemented anymore")
	return nil
}
