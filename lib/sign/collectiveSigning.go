package sign

import (
	"errors"
	"io"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"golang.org/x/net/context"
)

// Collective Signing via ElGamal
// 1. Announcement
// 2. Commitment
// 3. Challenge
// 4. Response

// Get multiplexes all messages from TCPHost using application logic
func (sn *Node) ProcessMessages() error {
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
				dbg.Lvl3(sn.Name(), "got announcement")
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
				dbg.Lvl3(sn.Name(), "got commitment")
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
				dbg.Lvl3(sn.Name(), "got challenge")
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
				dbg.Lvl3(sn.Name(), "received response from", sm.From)
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
				dbg.Lvl3(sn.Name(), "received SignatureBroadcast", sm.From)
				sn.ReceivedHeartbeat(sm.View)
				err = sn.SignatureBroadcast(sm.View, sm.SBm, 0)
			case StatusReturn:
				sn.StatusReturn(sm.View, sm.SRm)
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

	// Collect number of messages of children peers
	// signingmessage nil <=> we are a leaf
	if sm != nil {
		round.Commits = append(round.Commits, sm)
		dbg.Lvl3(sn.Name(), ": Found", sm.Com.Messages, "messages in com-msg")
		sn.Messages += sm.Com.Messages
	}

	dbg.Lvl3("Got", len(round.Commits), "of", len(sn.Children(view)), "commits")
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
		dbg.Lvl3("Number of messages in comMsg:", sn.Messages)
		com := &CommitmentMessage{
			V:             round.Log.V,
			V_hat:         round.Log.V_hat,
			X_hat:         round.X_hat,
			MTRoot:        round.MTRoot,
			ExceptionList: round.ExceptionList,
			Vote:          round.Vote,
			RoundNbr:         roundNbr,
			Messages: sn.Messages}
		sn.Messages = 0

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

	if err := round.SendChildrenChallengesProofs(chm); err != nil {
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
	sn.PeerStatus = StatusReturnMessage{1, len(sn.Children(view))}

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


func (sn *Node) StatusConnections(view int, am *AnnouncementMessage) error {
	dbg.Lvl3(sn.Name(), "StatusConnected", view)

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
	return nil
}

// This will broadcast the final signature to give to client
// it contins the global Response adn global challenge
func (sn *Node) SignatureBroadcast(view int, sb *SignatureBroadcastMessage, round int) error {
	dbg.Lvl3(sn.Name(), "received SignatureBroadcast on", view)
	sn.PeerStatusRcvd = 0
	// Root is creating the sig broadcast
	if sb == nil {
		r := sn.Rounds[round]
		if sn.IsRoot(view) {
			dbg.Lvl2(sn.Name(), ": sending number of messages:", sn.Messages)
			sb = &SignatureBroadcastMessage{
				R0_hat: r.R_hat,
				C:      r.C,
				X0_hat: r.X_hat,
				V0_hat: r.Log.V_hat,
				Messages: sn.Messages,
			}
		}
	} else {
		dbg.Lvl2(sn.Name(), ": sbm tells number of messages is:", sb.Messages)
		sn.Messages = sb.Messages
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
		dbg.Lvl3(sn.Name(), "in SignatureBroadcast is calling", len(sn.Children(view)), "children")
		ctx := context.TODO()
		if err := sn.PutDown(ctx, view, messgs); err != nil {
			return err
		}
	} else {
		dbg.Lvl2(sn.Name(), "sending StatusReturn")
		return sn.StatusReturn(view, &StatusReturnMessage{})
	}
	return nil
}

// StatusReturn just adds up all children and sends the result to
// the parent
func (sn *Node) StatusReturn(view int, sr *StatusReturnMessage) error {
	sn.PeerStatusRcvd += 1
	sn.PeerStatus.Responders += sr.Responders
	sn.PeerStatus.Peers += sr.Peers

	// Wait for other children before propagating the message
	if sn.PeerStatusRcvd < len(sn.Children(view)) {
		dbg.Lvl3(sn.Name(), "Waiting for other children")
		return nil
	}

	var err error = nil
	if sn.IsRoot(view) {
		// Add the root-node
		sn.PeerStatus.Peers += 1
		dbg.Lvl2("We got", sn.PeerStatus.Responders, "responses from", sn.PeerStatus.Peers, "peers.")
	} else {
		dbg.Lvl4(sn.Name(), "puts up statusReturn for", sn.PeerStatus)
		ctx := context.TODO()
		err = sn.PutUp(ctx, view, &SigningMessage{
			View:         view,
			Type:         StatusReturn,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			SRm: &sn.PeerStatus, })
	}
	return err
}
