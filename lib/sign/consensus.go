package sign

import (
	"errors"
	"sync/atomic"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

func (sn *Node) SetupProposal(view int, am *AnnouncementMessage, from string) error {
	// if this is for viewchanges: otherwise new views are not allowed
	if am.Vote.Type == ViewChangeVT {
		// viewchange votes must be received from the new parent on the new view
		if view != am.Vote.Vcv.View {
			return errors.New("view change attempt on view != received view")
		}
		// ensure that we are caught up
		if atomic.LoadInt64(&sn.LastSeenVote) != atomic.LoadInt64(&sn.LastAppliedVote) {
			return errors.New("not up to date: need to catch up")
		}
		if sn.RootFor(am.Vote.Vcv.View) != am.Vote.Vcv.Root {
			return errors.New("invalid root for proposed view")
		}

		nextview := sn.ViewNo + 1
		for ; nextview <= view; nextview++ {
			// log.Println(sn.Name(), "CREATING NEXT VIEW", nextview)
			sn.NewViewFromPrev(nextview, from)
			for _, act := range sn.Actions[nextview] {
				sn.ApplyAction(nextview, act)
			}
			for _, act := range sn.Actions[nextview] {
				sn.NotifyOfAction(nextview, act)
			}
		}
		// fmt.Fprintln(os.Stderr, sn.Name(), "setuppropose:", sn.HostListOn(view))
		// fmt.Fprintln(os.Stderr, sn.Name(), "setuppropose:", sn.Parent(view))
	} else {
		if view != sn.ViewNo {
			return errors.New("vote on not-current view")
		}
	}

	if am.Vote.Type == AddVT {
		if am.Vote.Av.View <= sn.ViewNo {
			return errors.New("unable to change past views")
		}
	}
	if am.Vote.Type == RemoveVT {
		if am.Vote.Rv.View <= sn.ViewNo {
			return errors.New("unable to change past views")
		}
	}
	return nil
}

// A propose for a view change would come on current view + sth
// when we receive view change  message on a future view,
// we must be caught up, create that view  and apply actions on it
func (sn *Node) Propose(view int, RoundNbr int, am *AnnouncementMessage, from string) error {
	log.Println(sn.Name(), "GOT ", "Propose", am)
	if err := sn.SetupProposal(view, am, from); err != nil {
		return err
	}

	if err := MerkleSetup(sn, view, RoundNbr, am); err != nil {
		return err
	}
	// log.Println(sn.Name(), "propose on view", view, sn.HostListOn(view))
	sn.RemoveMerkle[RoundNbr].Vote = am.Vote

	// Inform all children of proposal
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
	for i := range messgs {
		sm := SigningMessage{
			Type:         Announcement,
			View:         view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			RoundNbr:     RoundNbr,
			Am:           am}
		messgs[i] = &sm
	}
	ctx := context.TODO()
	//ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	if err := sn.PutDown(ctx, view, messgs); err != nil {
		return err
	}

	if len(sn.Children(view)) == 0 {
		log.Println(sn.Name(), "no children")
		sn.Promise(view, RoundNbr, nil)
	}
	return nil
}

func (sn *Node) Promise(view, Round int, sm *SigningMessage) error {
	log.Println(sn.Name(), "GOT ", "Promise", sm)
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, Round)
	sn.roundmu.Unlock()

	round := sn.RemoveMerkle[Round]
	if round == nil {
		// was not announced of this round, should retreat
		return nil
	}
	if sm != nil {
		round.Commits = append(round.Commits, sm)
	}

	if len(round.Commits) != len(sn.Children(view)) {
		return nil
	}

	// cast own vote
	sn.AddVotes(Round, round.Vote)

	for _, sm := range round.Commits {
		// count children votes
		round.Vote.Count.Responses = append(round.Vote.Count.Responses, sm.Com.Vote.Count.Responses...)
		round.Vote.Count.For += sm.Com.Vote.Count.For
		round.Vote.Count.Against += sm.Com.Vote.Count.Against

	}

	return sn.actOnPromises(view, Round)
}

func (sn *Node) actOnPromises(view, Round int) error {
	round := sn.RemoveMerkle[Round]
	var err error
	dbg.Lvl1("Act on Promise")
	if sn.IsRoot(view) {
		sn.commitsDone <- Round

		var b []byte
		b, err = round.Vote.MarshalBinary()
		if err != nil {
			// log.Fatal("Marshal Binary failed for CountedVotes")
			return err
		}
		round.C = HashElGamal(sn.suite, b, round.Log.V_hat)
		err = sn.Accept(view, Round, &ChallengeMessage{
			C:    round.C,
			Vote: round.Vote})

	} else {
		// create and putup own commit message
		com := &CommitmentMessage{
			Vote: round.Vote,
		}

		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		// log.Println(sn.Name(), "puts up promise on view", view, "to", sn.Parent(view))
		ctx := context.TODO()
		err = sn.PutUp(ctx, view, &SigningMessage{
			View:         view,
			Type:         Commitment,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			RoundNbr:     Round,
			Com:          com})
	}
	return err
}

func (sn *Node) Accept(view, RoundNbr int, chm *ChallengeMessage) error {
	log.Println(sn.Name(), "GOT ", "Accept", chm)
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, RoundNbr)
	sn.roundmu.Unlock()

	round := sn.RemoveMerkle[RoundNbr]
	if round == nil {
		log.Errorln("error round is nil")
		return nil
	}

	// act on decision of aggregated votes
	// log.Println(sn.Name(), chm.Round, round.VoteRequest)
	if round.Vote != nil {
		// append vote to vote log
		// potentially initiates signing node action based on vote
		sn.actOnVotes(view, chm.Vote)
	}
	if err := round.SendChildrenChallenges(chm); err != nil {
		return err
	}

	if len(sn.Children(view)) == 0 {
		sn.Accepted(view, RoundNbr, nil)
	}

	return nil
}

func (sn *Node) Accepted(view, Round int, sm *SigningMessage) error {
	log.Println(sn.Name(), "GOT ", "Accepted")
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, Round)
	sn.roundmu.Unlock()

	round := sn.RemoveMerkle[Round]
	if round == nil {
		// TODO: if combined with cosi pubkey, check for round.Log.v existing needed
		// If I was not announced of this round, or I failed to commit
		return nil
	}

	if sm != nil {
		round.Responses = append(round.Responses, sm)
	}
	if len(round.Responses) != len(sn.Children(view)) {
		return nil
	}
	// TODO: after having a chance to inspect the contents of the challenge
	// nodes can raise an alarm respond by ack/nack

	if sn.IsRoot(view) {
		sn.done <- Round
	} else {
		// create and putup own response message
		rm := &ResponseMessage{
			Vote: round.Vote,
		}

		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		ctx := context.TODO()
		return sn.PutUp(ctx, view, &SigningMessage{
			Type:         Response,
			View:         view,
			RoundNbr:     Round,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Rm:           rm})
	}

	return nil
}

func (sn *Node) actOnVotes(view int, v *Vote) {
	// TODO: percentage of nodes for quorum should be parameter
	// Basic check to validate Vote was Confirmed, can be enhanced
	// TODO: signing node can go through list of votes and verify
	accepted := v.Count.For > 2*len(sn.HostListOn(view))/3

	// Report on vote decision
	if sn.IsRoot(view) {
		abstained := len(sn.HostListOn(view)) - v.Count.For - v.Count.Against
		log.Infoln("Votes FOR:", v.Count.For, "; Votes AGAINST:", v.Count.Against, "; Absteined:", abstained, "Accepted", accepted)
	}

	// Act on vote Decision
	if accepted {
		log.Println(sn.Name(), "actOnVotes: vote", v.Index, " has been accepted")
		sn.VoteLog.Put(v.Index, v)
	} else {
		log.Println(sn.Name(), "actOnVotes: vote", v.Index, " has been rejected")

		// inform node trying to join/remove group of rejection
		gcm := &SigningMessage{
			Type:         GroupChanged,
			From:         sn.Name(),
			View:         view,
			LastSeenVote: int(sn.LastSeenVote),
			Gcm: &GroupChangedMessage{
				V:        &*v, // need copy bcs PutTo on separate thread
				HostList: sn.HostListOn(view)}}

		if v.Type == AddVT && sn.Name() == v.Av.Parent {
			sn.PutTo(context.TODO(), v.Av.Name, gcm)
		} else if v.Type == RemoveVT && sn.Name() == v.Rv.Parent {
			sn.PutTo(context.TODO(), v.Rv.Name, gcm)
		}

		v.Type = NoOpVT
		sn.VoteLog.Put(v.Index, v)

	}
	// List out all votes
	// for _, vote := range round.CountedVotes.Votes {
	// 	log.Infoln(vote.Name, vote.Accepted)
	// }
}
