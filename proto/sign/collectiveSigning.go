package sign

import (
	"errors"
	"io"
	"strconv"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// Collective Signing via ElGamal
// 1. Announcement
// 2. Commitment
// 3. Challenge
// 4. Response

// Get multiplexes all messages from TCPHost using application logic
func (sn *Node) getMessages() error {
	dbg.Lvl4(sn.Name(), "getting")
	defer dbg.Lvl4(sn.Name(), "done getting")

	sn.UpdateTimeout()
	dbg.Lvl4("Going to get", sn.Name())
	msgchan := sn.Host.GetNetworkMessg()
	// heartbeat for intiating viewChanges, allows intial 500s setup time
	/* sn.hbLock.Lock()
	sn.heartbeat = time.NewTimer(500 * time.Second)
	sn.hbLock.Unlock() */

	// as votes get approved they are streamed in ApplyVotes
	voteChan := sn.VoteLog.Stream()
	sn.ApplyVotes(voteChan)

	// gossip to make sure we are up to date
	sn.StartGossip()

	for {
		select {
		case <-sn.closed:
			sn.StopHeartbeat()
			return nil
		default:
			dbg.Lvl4(sn.Name(), "waiting for message")
			nm, ok := <-msgchan
			err := nm.Err

		// TODO: graceful shutdown voting
			if !ok || err == coconet.ErrClosed || err == io.EOF {
				dbg.Lvl3(sn.Name(), " getting from closed host")
				sn.Close()
				return coconet.ErrClosed
			}

		// if it is a non-fatal error try again
			if err != nil {
				log.Errorln(sn.Name(), " error getting message (still continuing) ", err)
				continue
			}
		// interpret network message as Signing Message
		//log.Printf("got message: %#v with error %v\n", sm, err)
			sm := nm.Data.(*SigningMessage)
			sm.From = nm.From
			dbg.Lvl4(sn.Name(), "received message:", sm.Type)

		// don't act on future view if not caught up, must be done after updating vote index
			sn.viewmu.Lock()
			if sm.View > sn.ViewNo {
				if atomic.LoadInt64(&sn.LastSeenVote) != atomic.LoadInt64(&sn.LastAppliedVote) {
					log.Warnln(sn.Name(), "not caught up for view change", sn.LastSeenVote, sn.LastAppliedVote)
					return errors.New("not caught up for view change")
				}
			}
			sn.viewmu.Unlock()
			sn.updateLastSeenVote(sm.LastSeenVote, sm.From)

			switch sm.Type {
			// if it is a bad message just ignore it
			default:
				continue
			case Announcement:
				dbg.Lvl2(sn.Name(), "got announcement")
				sn.ReceivedHeartbeat(sm.View)

				var err error
				if sm.Am.Vote != nil {
					err = sn.Propose(sm.View, sm.Am, sm.From)
					dbg.Lvl4(sn.Name(), "done proposing")
				} else {
					if !sn.IsParent(sm.View, sm.From) {
						log.Fatalln(sn.Name(), "received announcement from non-parent on view", sm.View)
						continue
					}
					err = sn.Announce(sm.View, sm.Am)
				}
				if err != nil {
					log.Errorln(sn.Name(), "announce error:", err)
				}

			// if it is a commitment or response it is from the child
			case Commitment:
				dbg.Lvl4(sn.Name(), "got commitment")
				if !sn.IsChild(sm.View, sm.From) {
					log.Fatalln(sn.Name(), "received commitment from non-child on view", sm.View)
					continue
				}

				var err error
				if sm.Com.Vote != nil {
					err = sn.Promise(sm.View, sm.Com.RoundNbr, sm)
				} else {
					err = sn.Commit(sm.View, sm.Com.RoundNbr, sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "commit error:", err)
				}
			case Challenge:
				dbg.Lvl4(sn.Name(), "got challenge")
				if !sn.IsParent(sm.View, sm.From) {
					log.Fatalln(sn.Name(), "received challenge from non-parent on view", sm.View)
					continue
				}
				sn.ReceivedHeartbeat(sm.View)

				var err error
				if sm.Chm.Vote != nil {
					err = sn.Accept(sm.View, sm.Chm)
				} else {
					err = sn.Challenge(sm.View, sm.Chm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "challenge error:", err)
				}
			case Response:
				dbg.Lvl4(sn.Name(), "received response from", sm.From)
				if !sn.IsChild(sm.View, sm.From) {
					log.Fatalln(sn.Name(), "received response from non-child on view", sm.View)
					continue
				}

				var err error
				if sm.Rm.Vote != nil {
					err = sn.Accepted(sm.View, sm.Rm.RoundNbr, sm)
				} else {
					err = sn.Respond(sm.View, sm.Rm.RoundNbr, sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "response error:", err)
				}
			case SignatureBroadcast:
				sn.ReceivedHeartbeat(sm.View)
				err = sn.SignatureBroadcast(sm.View, sm.SBm, 0)
			case CatchUpReq:
				v := sn.VoteLog.Get(sm.Cureq.Index)
				ctx := context.TODO()
				sn.PutTo(ctx, sm.From,
					&SigningMessage{
						From:         sn.Name(),
						Type:         CatchUpResp,
						LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
						Curesp:       &CatchUpResponse{Vote: v}})
			case CatchUpResp:
				if sm.Curesp.Vote == nil || sn.VoteLog.Get(sm.Curesp.Vote.Index) != nil {
					continue
				}
				vi := sm.Curesp.Vote.Index
				// put in votelog to be streamed and applied
				sn.VoteLog.Put(vi, sm.Curesp.Vote)
				// continue catching up
				sn.CatchUp(vi + 1, sm.From)
			case GroupChange:
				if sm.View == -1 {
					sm.View = sn.ViewNo
					if sm.Vrm.Vote.Type == AddVT {
						sn.AddPeerToPending(sm.From)
					}
				}
				// TODO sanity checks: check if view is == sn.ViewNo
				if sn.RootFor(sm.View) == sn.Name() {
					go sn.StartVotingRound(sm.Vrm.Vote)
					continue
				}
				sn.PutUp(context.TODO(), sm.View, sm)
			case GroupChanged:
				if !sm.Gcm.V.Confirmed {
					dbg.Lvl4(sn.Name(), " received attempt to group change not confirmed")
					continue
				}
				if sm.Gcm.V.Type == RemoveVT {
					dbg.Lvl4(sn.Name(), " received removal notice")
				} else if sm.Gcm.V.Type == AddVT {
					dbg.Lvl4(sn.Name(), " received addition notice")
					sn.NewView(sm.View, sm.From, nil, sm.Gcm.HostList)
				} else {
					log.Errorln(sn.Name(), "received GroupChanged for unacceptable action")
				}
			case StatusConnections:
				sn.ReceivedHeartbeat(sm.View)
				err = sn.StatusConnections(sm.View, sm.Am)
			case CloseAll:
				sn.ReceivedHeartbeat(sm.View)
				err = sn.CloseAll(sm.View)
				return nil
			case Error:
				dbg.Lvl4("Received Error Message:", ErrUnknownMessageType, sm, sm.Err)
			}
		}
	}

}

func (sn *Node) Announce(view int, am *AnnouncementMessage) error {
	dbg.Lvl4(sn.Name(), "received announcement on", view)

	msgs, err := sn.Callbacks.Announcement(sn, am)
	if err != nil {
		return err
	}

	if len(sn.Children(view)) == 0 {
		// If we are a leaf, start the commit phase process
		sn.Commit(view, am.RoundNbr, nil)
	} else {
		// Transform the AnnouncementMessages to SigningMessages to send to the
		// Children
		msgs_bm := make([]coconet.BinaryMarshaler, sn.NChildren(sn.ViewNo))
		for i := range msgs {
			sm := SigningMessage{
				Type:         Announcement,
				View:         sn.ViewNo,
				LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
				Am:           msgs[i]}
			msgs_bm[i] = &sm
		}

		// And sending to all our children-nodes
		dbg.Lvl4(sn.Name(), "sending to all children")
		ctx := context.TODO()
		if err := sn.PutDown(ctx, view, msgs_bm); err != nil {
			return err
		}
	}

	return nil
}

func (sn *Node) Commit(view, roundNbr int, sm *SigningMessage) error {
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()

	round := sn.Rounds[roundNbr]
	if round == nil {
		// was not announced of this round, should retreat
		return nil
	}

	// signingmessage nil <=> we are a leaf
	if sm != nil {
		round.Commits = append(round.Commits, sm)
	}

	if len(round.Commits) != len(sn.Children(view)) {
		return nil
	}

	// TODO - there should be a real passage of values and not just
	// a filled-up round
	sn.Callbacks.Commitment(nil)

	var err error
	if round.IsRoot() {
		dbg.Lvl3("Commit root : Aggregate Public Key :", round.X_hat)
		sn.commitsDone <- roundNbr
		err = sn.Challenge(view, &ChallengeMessage{
			C:      round.C,
			MTRoot: round.MTRoot,
			Proof:  round.Proof,
			RoundNbr:  roundNbr,
			Vote:   round.Vote})
	} else {
		// create and putup own commit message
		com := &CommitmentMessage{
			V:             round.Log.V,
			V_hat:         round.Log.V_hat,
			X_hat:         round.X_hat,
			MTRoot:        round.MTRoot,
			ExceptionList: round.ExceptionList,
			Vote:          round.Vote,
			RoundNbr:         roundNbr}

		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		dbg.Lvl4(sn.Name(), "puts up commit")
		ctx := context.TODO()
		err = sn.PutUp(ctx, view, &SigningMessage{
			View:         view,
			Type:         Commitment,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Com:          com})
	}
	return err
}

// initiated by root, propagated by all others
func (sn *Node) Challenge(view int, chm *ChallengeMessage) error {
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, chm.RoundNbr)
	sn.roundmu.Unlock()

	round := sn.Rounds[chm.RoundNbr]
	if round == nil {
		return nil
	}

	err := sn.Callbacks.Challenge(chm)
	if err != nil {
		return err
	}

	if err := sn.SendChildrenChallengesProofs(view, chm); err != nil {
		return err
	}

	// if we are a leaf, send the respond up
	if len(sn.Children(view)) == 0 {
		sn.Respond(view, chm.RoundNbr, nil)
	}
	// dbg.Lvl4(sn.Name(), "Done handling challenge message")
	return nil
}

// Respond send the response UP from leaf to parent
// called initially by the all the bottom leaves
func (sn *Node) Respond(view, roundNbr int, sm *SigningMessage) error {
	dbg.Lvl4(sn.Name(), "couting response on view, round", view, roundNbr, "Nchildren", len(sn.Children(view)))
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()

	Round := sn.Rounds[roundNbr]
	if Round == nil || Round.Log.v == nil {
		// If I was not announced of this round, or I failed to commit
		return nil
	}

	// Check if we have all replies from the children
	if sm != nil {
		Round.Responses = append(Round.Responses, sm)
	}
	if len(Round.Responses) != len(sn.Children(view)) {
		return nil
	}

	err := sn.Callbacks.Response(Round.FillInWithDefaultMessages())
	isroot := Round.IsRoot()
	// if error put it up if parent exists
	if err != nil && !isroot {
		sn.PutUpError(view, err)
		return err
	}

	// if no error send up own response
	if err == nil && !isroot {
		if Round.Log.Getv() == nil && sn.ShouldIFail("response") {
			dbg.Lvl4(Round.Name, "failing on response")
			return nil
		}

		// create and putup own response message
		rm := &ResponseMessage{
			R_hat:          Round.R_hat,
			ExceptionList:  Round.ExceptionList,
			ExceptionV_hat: Round.ExceptionV_hat,
			ExceptionX_hat: Round.ExceptionX_hat,
			RoundNbr:          roundNbr}

		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		ctx := context.TODO()
		dbg.Lvl4(sn.Name(), "put up response to", sn.Parent(view))
		err = sn.PutUp(ctx, view, &SigningMessage{
			Type:         Response,
			View:         view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Rm:           rm})
	} else {
		dbg.Lvl4("Root received response")
	}

	if sn.TimeForViewChange() {
		dbg.Lvl4("acting on responses: trying viewchanges")
		err := sn.TryViewChange(view + 1)
		if err != nil {
			dbg.Lvl3(err)
		}
	}

	// root reports round is done
	// Sends the final signature to every one
	if isroot {
		sn.SignatureBroadcast(view, nil, roundNbr)
		sn.done <- roundNbr
	}

	return err
}

func (sn *Node) SignatureBroadcast(view int, sb *SignatureBroadcastMessage, round int) error {
	dbg.Lvl2(sn.Name(), "received SignatureBroadcast on", view)
	// Root is creating the sig broadcast
	if sb == nil {
		r := sn.Rounds[round]
		if sn.IsRoot(view) {
			sb = &SignatureBroadcastMessage{
				R0_hat: r.R_hat,
				C:      r.C,
				X0_hat: r.X_hat,
				V0_hat: r.Log.V_hat,
			}
		}
	}

	sn.Callbacks.SignatureBroadcast(sb)

	// Inform all children of announcement
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
	for i := range messgs {
		sm := SigningMessage{
			Type:         SignatureBroadcast,
			View:         view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			SBm:          sb,
		}
		messgs[i] = &sm
	}

	if len(sn.Children(view)) > 0 {
		dbg.Lvl2(sn.Name(), "in SignatureBroadcast is calling", len(sn.Children(view)), "children")
		ctx := context.TODO()
		if err := sn.PutDown(ctx, view, messgs); err != nil {
			return err
		}
	}
	return nil
}

func (sn *Node) StatusConnections(view int, am *AnnouncementMessage) error {
	dbg.Lvl2(sn.Name(), "StatusConnected", view)

	// Ask connection-count on all connected children
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
	for i := range messgs {
		sm := SigningMessage{
			Type:         StatusConnections,
			View:         view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Am:           am}
		messgs[i] = &sm
	}

	ctx := context.TODO()
	if err := sn.PutDown(ctx, view, messgs); err != nil {
		return err
	}

	if len(sn.Children(view)) == 0 {
		sn.Commit(view, am.RoundNbr, nil)
	}
	return nil
}

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
		dbg.Lvl4(sn.Name(), "INITIATING VIEW CHANGE FOR VIEW:", view)
		go func() {
			err := sn.StartVotingRound(
				&Vote{
					View: view,
					Type: ViewChangeVT,
					Vcv: &ViewChangeVote{
						View: view,
						Root: sn.Name()}})
			if err != nil {
				log.Errorln(sn.Name(), "TRY VIEW CHANGE FAILED: ", err)
			}
		}()
	}
	return nil
}

// Called by every node after receiving aggregate responses from descendants
func (sn *Node) VerifyResponses(view, roundNbr int) error {
	round := sn.Rounds[roundNbr]

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := sn.suite.Point()
	V_clean := sn.suite.Point()
	V_clean.Add(V_clean.Mul(nil, round.R_hat), Aux.Mul(round.X_hat, round.C))
	// T is the recreated V_hat
	T := sn.suite.Point().Null()
	T.Add(T, V_clean)
	T.Add(T, round.ExceptionV_hat)

	var c2 abstract.Secret
	isroot := sn.IsRoot(view)
	if isroot {
		// round challenge must be recomputed given potential
		// exception list
		if sn.Type == PubKey {
			round.C = HashElGamal(sn.suite, sn.Message, round.Log.V_hat)
			c2 = HashElGamal(sn.suite, sn.Message, T)
		} else {
			msg := round.Msg
			msg = append(msg, []byte(round.MTRoot)...)
			round.C = HashElGamal(sn.suite, msg, round.Log.V_hat)
			c2 = HashElGamal(sn.suite, msg, T)
		}
	}

	// intermediary nodes check partial responses aginst their partial keys
	// the root node is also able to check against the challenge it emitted
	if !T.Equal(round.Log.V_hat) || (isroot && !round.C.Equal(c2)) {
		return errors.New("Verifying ElGamal Collective Signature failed in " + sn.Name() + "for round " + strconv.Itoa(roundNbr))
	} else if isroot {
		dbg.Lvl4(sn.Name(), "reports ElGamal Collective Signature succeeded for round", roundNbr, "view", view)
		/*
			nel := len(round.ExceptionList)
			nhl := len(sn.HostListOn(view))
			p := strconv.FormatFloat(float64(nel) / float64(nhl), 'f', 6, 64)
			log.Infoln(sn.Name(), "reports", nel, "out of", nhl, "percentage", p, "failed in round", Round)
		*/
		// dbg.Lvl4(round.MTRoot)
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

func (sn *Node) CloseAll(view int) error {
	dbg.Lvl2(sn.Name(), "received CloseAll on", view)

	// At the leaves
	if len(sn.Children(view)) == 0 {
		dbg.Lvl2(sn.Name(), "in CloseAll is root leaf")
	} else {
		dbg.Lvl2(sn.Name(), "in CloseAll is calling", len(sn.Children(view)), "children")

		// Inform all children of announcement
		messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
		for i := range messgs {
			sm := SigningMessage{
				Type:         CloseAll,
				View:         view,
				LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			}
			messgs[i] = &sm
		}
		ctx := context.TODO()
		if err := sn.PutDown(ctx, view, messgs); err != nil {
			return err
		}
	}

	sn.Close()
	dbg.Lvl3("Closing down shop", sn.Isclosed)
	return nil
}

func (sn *Node) PutUpError(view int, err error) {
	// dbg.Lvl4(sn.Name(), "put up response with err", err)
	// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	ctx := context.TODO()
	sn.PutUp(ctx, view, &SigningMessage{
		Type:         Error,
		View:         view,
		LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
		Err:          &ErrorMessage{Err: err.Error()}})
}

// Getting actual View
func (sn *Node) GetView() int {
	return sn.ViewNo
}
