package sign

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/dbg"
	"golang.org/x/net/context"
)

/*
This implements the part of the Node-structure that has to
do with the protocol itself: Announce, Commit, Chalenge and
Response. Two additional steps are done: SignatureBroadcast
to send the final commit to all nodes, and StatusReturn which
allows for collection of statistics.
 */

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

	// gossip to make sure we are up to date
	sn.StartGossip()

	for {
		select {
		case <-sn.closed:
			dbg.LLvl3("Received closed-message through channel")
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
		/*
			sn.viewmu.Lock()
			if sm.ViewNbr > sn.ViewNo {
				if atomic.LoadInt64(&sn.LastSeenVote) != atomic.LoadInt64(&sn.LastAppliedVote) {
					log.Warnln(sn.Name(), "not caught up for view change", sn.LastSeenVote, sn.LastAppliedVote)
					return errors.New("not caught up for view change")
				}
			}
			sn.viewmu.Unlock()
			sn.updateLastSeenVote(sm.LastSeenVote, sm.From)
			*/

			switch sm.Type {
			// if it is a bad message just ignore it
			default:
				continue
			case Announcement:
				dbg.Lvl3(sn.Name(), "got announcement")
				sn.ReceivedHeartbeat(sm.ViewNbr)

				var err error
				if sm.Am.Vote != nil {
					err = sn.Propose(sm.ViewNbr, sm.RoundNbr, sm.Am, sm.From)
					dbg.Lvl4(sn.Name(), "done proposing")
				} else {
					if !sn.IsParent(sm.ViewNbr, sm.From) {
						log.Fatalln(sn.Name(), "received announcement from non-parent on view", sm.ViewNbr)
						continue
					}
					err = sn.Announce(sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "announce error:", err)
				}

			// if it is a commitment or response it is from the child
			case Commitment:
				dbg.Lvl3(sn.Name(), "got commitment")
				if !sn.IsChild(sm.ViewNbr, sm.From) {
					log.Fatalln(sn.Name(), "received commitment from non-child on view", sm.ViewNbr)
					continue
				}

				var err error
				if sm.Com.Vote != nil {
					err = sn.Promise(sm.ViewNbr, sm.RoundNbr, sm)
				} else {
					err = sn.Commit(sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "commit error:", err)
				}
			case Challenge:
				dbg.Lvl3(sn.Name(), "got challenge")
				if !sn.IsParent(sm.ViewNbr, sm.From) {
					log.Fatalln(sn.Name(), "received challenge from non-parent on view", sm.ViewNbr)
					continue
				}
				sn.ReceivedHeartbeat(sm.ViewNbr)

				var err error
				if sm.Chm.Vote != nil {
					err = sn.Accept(sm.ViewNbr, sm.RoundNbr, sm.Chm)
				} else {
					err = sn.Challenge(sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "challenge error:", err)
				}
			case Response:
				dbg.Lvl3(sn.Name(), "received response from", sm.From)
				if !sn.IsChild(sm.ViewNbr, sm.From) {
					log.Fatalln(sn.Name(), "received response from non-child on view", sm.ViewNbr)
					continue
				}

				var err error
				if sm.Rm.Vote != nil {
					err = sn.Accepted(sm.ViewNbr, sm.RoundNbr, sm)
				} else {
					err = sn.Respond(sm)
				}
				if err != nil {
					log.Errorln(sn.Name(), "response error:", err)
				}
			case SignatureBroadcast:
				dbg.Lvl3(sn.Name(), "received SignatureBroadcast", sm.From)
				sn.ReceivedHeartbeat(sm.ViewNbr)
				err = sn.SignatureBroadcast(sm)
			case StatusReturn:
				sn.StatusReturn(sm.ViewNbr, sm.SRm)
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
				if sm.ViewNbr == -1 {
					sm.ViewNbr = sn.ViewNo
					if sm.Vrm.Vote.Type == AddVT {
						sn.AddPeerToPending(sm.From)
					}
				}
				// TODO sanity checks: check if view is == sn.ViewNo
				if sn.RootFor(sm.ViewNbr) == sn.Name() {
					dbg.Fatal("Group change not implementekd. BTH")
					//go sn.StartVotingRound(sm.Vrm.Vote)
					continue
				}
				sn.PutUp(context.TODO(), sm.ViewNbr, sm)
			case GroupChanged:
				if !sm.Gcm.V.Confirmed {
					dbg.Lvl4(sn.Name(), " received attempt to group change not confirmed")
					continue
				}
				if sm.Gcm.V.Type == RemoveVT {
					dbg.Lvl4(sn.Name(), " received removal notice")
				} else if sm.Gcm.V.Type == AddVT {
					dbg.Lvl4(sn.Name(), " received addition notice")
					sn.NewView(sm.ViewNbr, sm.From, nil, sm.Gcm.HostList)
				} else {
					log.Errorln(sn.Name(), "received GroupChanged for unacceptable action")
				}
			case StatusConnections:
				sn.ReceivedHeartbeat(sm.ViewNbr)
				err = sn.StatusConnections(sm.ViewNbr, sm.Am)
			case CloseAll:
				sn.ReceivedHeartbeat(sm.ViewNbr)
				err = sn.CloseAll(sm.ViewNbr)
				return nil
			case Error:
				dbg.Lvl4("Received Error Message:", errors.New("received message of unknown type"), sm, sm.Err)
			}
		}
	}

}

func (sn *Node) Announce(sm *SigningMessage) error {
	view := sm.ViewNbr
	RoundNbr := sm.RoundNbr
	am := sm.Am
	dbg.Lvl4(sn.Name(), "received announcement on", view)
	var round Round
	round = sn.Rounds[RoundNbr]
	if round == nil {
		if am == nil {
			return fmt.Errorf("Got a nil announcement on a non root nde?")
		}

		sn.LastSeenRound = max(sn.LastSeenRound, RoundNbr)
		rtype := am.RoundType
		// create the new round and save it
		r, err := NewRoundFromType(rtype, sn)
		if err != nil {
			dbg.Lvl3(sn.Name(), "Error getting new round in announcement")
			return err
		}
		sn.Rounds[RoundNbr] = r
		round = r
	}

	nChildren := sn.NChildren(view)
	out := make([]*SigningMessage, nChildren)
	for i := range out {
		out[i] = &SigningMessage{
			Type:         Announcement,
			ViewNbr:         sn.ViewNo,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			RoundNbr:     RoundNbr,
			Am: &AnnouncementMessage{},
		}
	}
	err := round.Announcement(view, RoundNbr, sm, out)
	if err != nil {
		dbg.Lvl3(sn.Name(), "Error on announcement", err)
		return err
	}

	if len(sn.Children(view)) == 0 {
		// If we are a leaf, start the commit phase process
		sn.Commit(&SigningMessage{
			Type:     Commitment,
			RoundNbr: RoundNbr,
			ViewNbr:     view,
		})
	} else {
		// Transform the AnnouncementMessages to SigningMessages to send to the
		// Children
		msgs_bm := make([]coconet.BinaryMarshaler, nChildren)
		for i := range msgs_bm {
			msgs_bm[i] = out[i]
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

func (sn *Node) Commit(sm *SigningMessage) error {
	view := sm.ViewNbr
	roundNbr := sm.RoundNbr
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()

	commitList, ok := sn.RoundCommits[roundNbr]
	if !ok {
		// first time we see a commit message for this round
		commitList = make([]*SigningMessage, 0)
		sn.RoundCommits[roundNbr] = commitList
	}
	// Collect number of messages of children peers
	// signingmessage nil <=> we are a leaf
	if sm.Com != nil {
		commitList = append(commitList, sm)
		sn.RoundCommits[roundNbr] = commitList
		dbg.Lvl3(sn.Name(), ": Found", sm.Com.Messages, "messages in com-msg")
		sn.Messages += sm.Com.Messages
	}

	dbg.Lvl3("Got", len(sn.RoundCommits[roundNbr]), "of", len(sn.Children(view)), "commits")
	// not enough commits yet (not all children replied)
	if len(sn.RoundCommits[roundNbr]) != len(sn.Children(view)) {
		dbg.Lvl3(sn.Name(), "Not enough commits received to call the Commit of the round")
		return nil
	}

	ri := sn.Rounds[roundNbr]
	if ri == nil {
		dbg.Lvl3(sn.Name(), "No round interface for commit round number", roundNbr)
		return fmt.Errorf("No Round Interface defined for this round number (commitment)")
	}
	out := &SigningMessage{
		ViewNbr:         view,
		Type:         Commitment,
		LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
		RoundNbr:     roundNbr,
		Com: &CommitmentMessage{},
	}
	err := ri.Commitment(sn.RoundCommits[roundNbr], out)
	// now we can delete the commits for this round
	delete(sn.RoundCommits, roundNbr)

	if err != nil {
		return nil
	}

	if sn.IsRoot(view) {
		sn.commitsDone <- roundNbr
		err = sn.Challenge(&SigningMessage{
			RoundNbr: roundNbr,
			Type:     Challenge,
			ViewNbr:     view,
			Chm: &ChallengeMessage{},
		})
	} else {
		// create and putup own commit message
		dbg.Lvl3("Number of messages in comMsg:", sn.Messages)

		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		dbg.Lvl4(sn.Name(), "puts up commit")
		ctx := context.TODO()
		err = sn.PutUp(ctx, view, out)
	}
	return err
}

// initiated by root, propagated by all others
func (sn *Node) Challenge(sm *SigningMessage) error {
	view := sm.ViewNbr
	RoundNbr := sm.RoundNbr
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, RoundNbr)
	sn.roundmu.Unlock()

	round := sn.Rounds[RoundNbr]
	if round == nil {
		return fmt.Errorf("No Round Interface created for this round")
	}
	challs := make([]*SigningMessage, sn.NChildren(view))
	for i := range challs {
		challs[i] = &SigningMessage{
			ViewNbr: view,
			RoundNbr: RoundNbr,
			Type: Challenge,
			Chm: &ChallengeMessage{}}
	}

	err := round.Challenge(sm, challs)

	if err != nil {
		return err
	}
	// if we are a leaf, send the respond up
	if len(sn.Children(view)) == 0 {
		sn.Respond(&SigningMessage{
			Type:     Response,
			ViewNbr:     view,
			RoundNbr: RoundNbr,
		})
	} else { // otherwise continue to pass down challenge
		// TODO remove this hack of using the first one. Should be separate messages
		// + SendChildrenChallengesProof should be put into roundstamper or
		// round interface
		for _, out := range (challs) {
			conn := sn.Children(view)[out.To]
			conn.PutData(out)
		}
	}
	// dbg.Lvl4(sn.Name(), "Done handling challenge message")
	return nil
}

// Respond send the response UP from leaf to parent
// called initially by the all the bottom leaves
func (sn *Node) Respond(sm *SigningMessage) error {
	view := sm.ViewNbr
	roundNbr := sm.RoundNbr
	dbg.Lvl4(sn.Name(), "couting response on view, round", view, roundNbr, "Nchildren", len(sn.Children(view)))
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()
	sn.PeerStatus = StatusReturnMessage{1, len(sn.Children(view))}

	responseList, ok := sn.RoundResponses[roundNbr]
	if !ok {
		responseList = make([]*SigningMessage, 0)
		sn.RoundResponses[roundNbr] = responseList
	}

	// Check if we have all replies from the children
	if sm.Rm != nil {
		responseList = append(responseList, sm)
	}
	if len(responseList) != len(sn.Children(view)) {
		sn.RoundResponses[roundNbr] = responseList
		dbg.Lvl4(sn.Name(), "Received response but still waiting on other children responses (stored", len(responseList), " responses)")
		return nil
	}

	ri := sn.Rounds[roundNbr]
	if ri == nil {
		return fmt.Errorf("No Round Interface for this round nbr :(")
	}
	// Fillinwithdefaultmessage is used to fill the exception with missing
	// children and all
	out := &SigningMessage{
		Type:         Response,
		ViewNbr:         view,
		RoundNbr:     roundNbr,
		LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
		Rm: &ResponseMessage{},
	}
	err := ri.Response(responseList, out)
	delete(sn.RoundResponses, roundNbr)
	if err != nil {
		return err
	}
	isroot := sn.IsRoot(view)
	// if error put it up if parent exists
	if err != nil && !isroot {
		sn.PutUpError(view, err)
		return err
	}

	// if no error send up own response
	if err == nil && !isroot {
		/*if Round.Log.Getv() == nil && sn.ShouldIFail("response") {*/
		//dbg.Lvl4(Round.Name, "failing on response")
		//return nil
		/*}*/

		// create and putup own response message
		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		ctx := context.TODO()
		dbg.Lvl4(sn.Name(), "put up response to", sn.Parent(view))
		err = sn.PutUp(ctx, view, out)
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
		sn.SignatureBroadcast(&SigningMessage{
			Type:     SignatureBroadcast,
			ViewNbr:     view,
			RoundNbr: roundNbr,
			SBm: &SignatureBroadcastMessage{},
		})
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
			ViewNbr:         view,
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
func (sn *Node) SignatureBroadcast(sm *SigningMessage) error {
	view := sm.ViewNbr
	RoundNbr := sm.RoundNbr
	dbg.Lvl3(sn.Name(), "received SignatureBroadcast on", view)
	sn.PeerStatusRcvd = 0

	ri := sn.Rounds[RoundNbr]
	if ri == nil {
		return fmt.Errorf("No round created for this round number (signature broadcast)")
	}
	out := make([]*SigningMessage, sn.NChildren(view))
	for i := range out {
		out[i] = &SigningMessage{
			Type:     SignatureBroadcast,
			ViewNbr:     view,
			RoundNbr: RoundNbr,
			SBm: &SignatureBroadcastMessage{},
		}
	}

	err := ri.SignatureBroadcast(sm, out)
	if err != nil {
		return err
	}

	if len(sn.Children(view)) > 0 {
		dbg.Lvl3(sn.Name(), "in SignatureBroadcast is calling", len(sn.Children(view)), "children")
		ctx := context.TODO()
		msgs := make([]coconet.BinaryMarshaler, len(out))
		for i := range msgs {
			msgs[i] = out[i]
		}
		if err := sn.PutDown(ctx, view, msgs); err != nil {
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
			ViewNbr:         view,
			Type:         StatusReturn,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			SRm:          &sn.PeerStatus})
	}
	return err
}
