package sign

import (
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"errors"
)

/*
NOT WORKING - consensus code for voting - should
be implemented as a roundType
 */

func (sn *Node) TryViewChange(view int) error {
	dbg.Lvl4(sn.Name(), "TRY VIEW CHANGE on", view, "with last view", sn.ViewNo)
	// should ideally be compare and swap
	sn.viewmu.Lock()
	if view <= sn.ViewNo {
		sn.viewmu.Unlock()
		return errors.New("trying to view change on previous/ current view")
	}
	if sn.ChangingView {
		sn.viewmu.Unlock()
		return ChangingViewError
	}
	sn.ChangingView = true
	sn.viewmu.Unlock()

	// take action if new view root
	if sn.Name() == sn.RootFor(view) {
		dbg.Fatal(sn.Name(), "Initiating view change for view:", view, "BTH")
		/*
		go func() {
			err := sn.StartVotingRound(
				&Vote{
					View: view,
					Type: ViewChangeVT,
					Vcv: &ViewChangeVote{
						View: view,
						Root: sn.Name()}})
			if err != nil {
				dbg.Lvl2(sn.Name(), "Try view change failed:", err)
			}
		}()
		*/
	}
	return nil
}

func (sn *Node) TimeForViewChange() bool {
	if sn.RoundsPerView == 0 {
		// No view change asked
		return false
	}
	sn.roundmu.Lock()
	defer sn.roundmu.Unlock()

	// if this round is last one for this view
	if sn.LastSeenRound % sn.RoundsPerView == 0 {
		// dbg.Lvl4(sn.Name(), "TIME FOR VIEWCHANGE:", lsr, rpv)
		return true
	}
	return false
}


func (sn *Node) SetupProposal(view int, am *AnnouncementMessage, from string) error {
	dbg.Fatal("SetupProposal not implemented anymore")
	return nil
}

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
