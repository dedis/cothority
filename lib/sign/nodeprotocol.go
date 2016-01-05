package sign

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
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
	msgchan := sn.MsgChans
	// heartbeat for intiating viewChanges, allows intial 500s setup time
	/* sn.hbLock.Lock()
	sn.heartbeat = time.NewTimer(500 * time.Second)
	sn.hbLock.Unlock() */

	// gossip to make sure we are up to date
	//	sn.StartGossip()
	//errReset := syscall.ECONNRESET.Error()
	for {
		select {
		case <-sn.closed:
			dbg.Lvl3("Received closed-message through channel")
			sn.StopHeartbeat()
			return nil
		default:
			dbg.Lvl4(sn.Name(), "waiting for message")
			nm, ok := <-msgchan
			dbg.Lvlf4("Message on %s is type %s", sn.Name(), nm.MsgType)

			// Do we have an errror ?
			if err := nm.Error(); err != nil {
				// One of the errors doesn't have an error-number applied, so we need
				// to check for the string - will probably be fixed in go 1.6
				if !ok || err == network.ErrClosed || err == network.ErrEOF ||
					err == io.ErrClosedPipe {
					dbg.Lvl3(sn.Name(), "getting from closed host")
					sn.Close()
					return network.ErrClosed
				}

				// if it is a unknown error, abort ?
				if err == network.ErrUnknown {
					dbg.Lvl1(sn.Name(), "Unknown error => ABORT")
					return err
				}
				if err == network.ErrTemp {
					/*if strings.Contains(errStr, errReset) {*/
					//dbg.Lvl2(sn.Name(), "connection reset error")
					//return coconet.ErrClosed
					/*}*/
					dbg.Lvl1(sn.Name(), "temporary error getting message (still continuing)")
					continue
				}
			}
			switch nm.MsgType {
			// if it is a bad message just ignore it
			default:
				continue
			case Announcement:
				dbg.Lvl3(sn.Name(), "got announcement")
				am := nm.Msg.(AnnouncementMessage)
				processSigningMsg(nm, am.SigningMessage)
				sn.ReceivedHeartbeat(am.ViewNbr)

				var err error
				if am.Vote != nil {
					err = sn.Propose(am.ViewNbr, am.RoundNbr, &am, nm.From)
					dbg.Lvl4(sn.Name(), "done proposing")
				} else {
					if !sn.ChildOf(am.ViewNbr, nm.From) {
						dbg.Fatal(sn.Name(), "received announcement from non-parent on view", am.ViewNbr)
						continue
					}
					err = sn.Announce(&am)
				}
				if err != nil {
					dbg.Error(sn.Name(), "announce error:", err)
				}

			// if it is a commitment or response it is from the child
			case Commitment:
				dbg.Lvl3(sn.Name(), "got commitment")
				cm := nm.Msg.(CommitmentMessage)
				processSigningMsg(nm, cm.SigningMessage)
				if !sn.ParentOf(cm.ViewNbr, nm.From) {
					dbg.Fatal(sn.Name(), "received commitment from non-child on view", cm.ViewNbr)
					continue
				}

				var err error
				if cm.Vote != nil {
					err = sn.Promise(cm.ViewNbr, cm.RoundNbr, &cm)
				} else {
					err = sn.Commit(&cm)
				}
				if err != nil {
					dbg.Error(sn.Name(), "commit error:", err)
				}
			case Challenge:
				dbg.Lvl3(sn.Name(), "got challenge")
				chm := nm.Msg.(ChallengeMessage)
				processSigningMsg(nm, chm.SigningMessage)
				if !sn.ChildOf(chm.ViewNbr, nm.From) {
					dbg.Fatal(sn.Name(), "received challenge from non-parent on view", chm.ViewNbr)
					continue
				}
				sn.ReceivedHeartbeat(chm.ViewNbr)

				var err error
				if chm.Vote != nil {
					err = sn.Accept(chm.ViewNbr, chm.RoundNbr, &chm)
				} else {
					err = sn.Challenge(&chm)
				}
				if err != nil {
					dbg.Error(sn.Name(), "challenge error:", err)
				}
			case Response:
				dbg.Lvl3(sn.Name(), "received response from", nm.From)
				rm := nm.Msg.(ResponseMessage)
				processSigningMsg(nm, rm.SigningMessage)
				if !sn.ParentOf(rm.ViewNbr, nm.From) {
					dbg.Fatal(sn.Name(), "received response from non-child on view", rm.ViewNbr)
					continue
				}

				var err error
				if rm.Vote != nil {
					err = sn.Accepted(rm.ViewNbr, rm.RoundNbr, &rm)
				} else {
					err = sn.Respond(&rm)
				}
				if err != nil {
					dbg.Error(sn.Name(), "response error:", err)
				}
			case SignatureBroadcast:
				dbg.Lvl3(sn.Name(), "received SignatureBroadcast", nm.From)
				sbm := nm.Msg.(SignatureBroadcastMessage)
				processSigningMsg(nm, sbm.SigningMessage)
				sn.ReceivedHeartbeat(sbm.ViewNbr)
				err := sn.SignatureBroadcast(&sbm)
				if err != nil {
					dbg.Lvl2(sn.Name(), "Error with signature broadcast:", err)
				}
			case StatusReturn:
				srm := nm.Msg.(StatusReturnMessage)
				processSigningMsg(nm, srm.SigningMessage)
				sn.StatusReturn(srm.ViewNbr, &srm)
			case CatchUpReq:
				cur := nm.Msg.(CatchUpRequest)
				processSigningMsg(nm, cur.SigningMessage)
				v := sn.VoteLog.Get(cur.Index)
				ctx := context.TODO()
				sn.PutTo(ctx, nm.From, &CatchUpResponse{
					SigningMessage: &SigningMessage{
						// ugly hack with atomic. do we really need ??
						// Anyway, voting mechanisms has been broken since many
						// months now, we need to start over.
						LastSeenVote: cur.LastSeenVote},
					Vote: v})
			case CatchUpResp:
				cur := nm.Msg.(CatchUpResponse)
				processSigningMsg(nm, cur.SigningMessage)
				if cur.Vote == nil || sn.VoteLog.Get(cur.Vote.Index) != nil {
					continue
				}
				vi := cur.Vote.Index
				// put in votelog to be streamed and applied
				sn.VoteLog.Put(vi, cur.Vote)
				// continue catching up
				sn.CatchUp(vi+1, nm.From)
			case VoteRequest:
				vrm := nm.Msg.(VoteRequestMessage)
				processSigningMsg(nm, vrm.SigningMessage)
				if vrm.ViewNbr == -1 {
					vrm.ViewNbr = sn.ViewNo
					if vrm.Vote.Type == AddVT {
						// XXX Voting process broken anyway.TODO
						//sn.AddPeerToPending(nm.From)
					}
				}
				// TODO sanity checks: check if view is == sn.ViewNo
				if sn.Root(vrm.ViewNbr) {
					dbg.Fatal("Group change not implementekd. BTH")
					//go sn.StartVotingRound(sm.Vrm.Vote)
					continue
				}
				sn.PutUp(context.TODO(), vrm.ViewNbr, &vrm)
			case GroupChanged:
				gcm := nm.Msg.(GroupChangedMessage)
				processSigningMsg(nm, gcm.SigningMessage)
				if !gcm.V.Confirmed {
					dbg.Lvl4(sn.Name(), " received attempt to group change not confirmed")
					continue
				}
				if gcm.V.Type == RemoveVT {
					dbg.Lvl4(sn.Name(), " received removal notice")
				} else if gcm.V.Type == AddVT {
					dbg.Lvl4(sn.Name(), " received addition notice")
					// XXX TODO Broken voting system anyway.
					// sn.NewView(gcm.ViewNbr, nm.From, nil, gcm.HostList)
				} else {
					dbg.Error(sn.Name(), "received GroupChanged for unacceptable action")
				}
				//	case StatusConnections:
				// Not used anymore it seems
			case CloseAll:
				ca := nm.Msg.(CloseAllMessage)
				sn.ReceivedHeartbeat(ca.ViewNbr)
				err := sn.CloseAll(ca.ViewNbr)
				return err
			}
		}
	}
}

// If we ever need to put more metadata in the messages itself, we can do it
// here.
func processSigningMsg(nm network.ApplicationMessage, sm *SigningMessage) {
	sm.From = nm.From
	if nm.MsgType == network.DefaultType {
		sm.Empty = true
	}
}

func (sn *Node) Announce(am *AnnouncementMessage) error {
	view := am.ViewNbr
	RoundNbr := am.RoundNbr
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
		dbg.Lvl3(sn.Name(), "Creating new round-type", rtype)
		r, err := NewRoundFromType(rtype, sn)
		if err != nil {
			dbg.Lvl3(sn.Name(), "Error getting new round in announcement")
			return err
		}
		sn.Rounds[RoundNbr] = r
		round = r
	}

	nChildren := sn.NChildren(view)
	out := make([]*AnnouncementMessage, nChildren)
	for i := range out {
		out[i] = &AnnouncementMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:      sn.ViewNo,
				LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
				RoundNbr:     RoundNbr},
			Message:   make([]byte, 0),
			RoundType: am.RoundType,
		}
	}
	err := round.Announcement(view, RoundNbr, am, out)
	if err != nil {
		dbg.Lvl3(sn.Name(), "Error on announcement", err)
		return err
	}

	if sn.NChildren(view) == 0 {
		// If we are a leaf, start the commit phase process
		sn.Commit(&CommitmentMessage{
			SigningMessage: &SigningMessage{
				RoundNbr: RoundNbr,
				ViewNbr:  view,
			},
		})
	} else {
		// And sending to all our children-nodes
		dbg.Lvlf4("%s sending to all children", sn.Name())
		ctx := context.TODO()
		for i, ch := range sn.Children(view) {
			if err := sn.PutDown(ctx, view, ch.Name(), out[i]); err != nil {
				dbg.Lvl2(sn.Name(), "Error putting down announcement to", ch.Name(), ":", err)
				return err
			}
		}
	}

	return nil
}

func (sn *Node) Commit(com *CommitmentMessage) error {
	view := com.ViewNbr
	roundNbr := com.RoundNbr
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()

	commitList, ok := sn.RoundCommits[roundNbr]
	if !ok {
		// first time we see a commit message for this round
		commitList = make([]*CommitmentMessage, 0)
		sn.RoundCommits[roundNbr] = commitList
	}
	// signingmessage nil <=> we are a leaf
	if !sn.Leaf(view) {
		commitList = append(commitList, com)
		sn.RoundCommits[roundNbr] = commitList
	}

	dbg.Lvl3("Got", len(sn.RoundCommits[roundNbr]), "of", sn.NChildren(view), "commits")
	// if we are not a leaf and we did not get enough commits yet (not all children replied)
	if len(sn.RoundCommits[roundNbr]) != sn.NChildren(view) {
		dbg.Lvl3(sn.Name(), "Not enough commits received to call the Commit of the round")
		return nil
	}

	ri := sn.Rounds[roundNbr]
	if ri == nil {
		dbg.Lvl3(sn.Name(), "No round interface for commit round number", roundNbr)
		return fmt.Errorf("No Round Interface defined for this round number (commitment)")
	}
	out := &CommitmentMessage{
		SigningMessage: &SigningMessage{
			ViewNbr:      view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			RoundNbr:     roundNbr,
		},
		Message: make([]byte, 0),
	}
	err := ri.Commitment(sn.RoundCommits[roundNbr], out)
	// now we can delete the commits for this round
	delete(sn.RoundCommits, roundNbr)

	if err != nil {
		return nil
	}

	if sn.Root(view) {
		sn.commitsDone <- roundNbr
		err = sn.Challenge(&ChallengeMessage{
			SigningMessage: &SigningMessage{
				RoundNbr: roundNbr,
				ViewNbr:  view,
			},
		})
	} else {
		// create and putup own commit message
		// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		ctx := context.TODO()
		dbg.Lvlf3("Out is %+v", out)
		if err = sn.PutUp(ctx, view, out); err != nil {
			dbg.Lvl2(sn.Name(), "Error putting up commit:", err)
		} else {
			dbg.Lvl3(sn.Name(), "puts up commit")
		}
	}
	return err
}

// initiated by root, propagated by all others
func (sn *Node) Challenge(chm *ChallengeMessage) error {
	view := chm.ViewNbr
	RoundNbr := chm.RoundNbr
	dbg.Lvl3("Challenge for round", RoundNbr)
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, RoundNbr)
	sn.roundmu.Unlock()

	children := sn.Children(view)

	challs := make([]*ChallengeMessage, len(children))
	i := 0
	for child := range children {
		challs[i] = &ChallengeMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:  view,
				RoundNbr: RoundNbr,
				To:       children[child].Name()},
			Message: make([]byte, 0),
		}
		i++
	}

	round := sn.Rounds[RoundNbr]
	if round == nil {
		dbg.Lvl3("No Round Interface created for this round. Children:",
			len(children))
	} else {
		err := round.Challenge(chm, challs)
		if err != nil {
			return err
		}
	}

	// if we are a leaf, send the respond up
	if sn.Leaf(view) {
		sn.Respond(&ResponseMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:  view,
				RoundNbr: RoundNbr,
			}})
	} else {
		// otherwise continue to pass down challenge
		for i, ch := range sn.Children(view) {
			ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
			if err := sn.PutDown(ctx, view, ch.Name(), challs[i]); err != nil {
				dbg.Error(sn.Name(), "PutDown on Challenge failed with children", ch.Name(), err)
				return err
			}
		}
	}
	// dbg.Lvl4(sn.Name(), "Done handling challenge message")
	return nil
}

// Respond send the response UP from leaf to parent
// called initially by the all the bottom leaves
func (sn *Node) Respond(rm *ResponseMessage) error {
	view := rm.ViewNbr
	roundNbr := rm.RoundNbr
	dbg.Lvl4(sn.Name(), "couting response on view, round", view, roundNbr, "Nchildren", sn.NChildren(view))
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, roundNbr)
	sn.roundmu.Unlock()
	sn.PeerStatus = StatusReturnMessage{Responders: 1, Peers: len(sn.Children(view))}

	responseList, ok := sn.RoundResponses[roundNbr]
	if !ok {
		responseList = make([]*ResponseMessage, 0)
		sn.RoundResponses[roundNbr] = responseList
	}

	// Check if we have all replies from the children
	if !sn.Leaf(view) {
		responseList = append(responseList, rm)
	}
	if len(responseList) != sn.NChildren(view) {
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
	out := &ResponseMessage{
		SigningMessage: &SigningMessage{
			ViewNbr:      view,
			RoundNbr:     roundNbr,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote))},
		Message:        make([]byte, 0),
		ExceptionV_hat: sn.suite.Point().Null(),
		ExceptionX_hat: sn.suite.Point().Null(),
	}
	err := ri.Response(responseList, out)
	delete(sn.RoundResponses, roundNbr)
	if err != nil {
		return err
	}
	isroot := sn.Root(view)
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
		dbg.Lvl4("Root: response done")
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
		sn.SignatureBroadcast(&SignatureBroadcastMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:  view,
				RoundNbr: roundNbr},
		})
		sn.done <- roundNbr
	}

	return err
}

// XXX Removed because unused + already too many ways of doing the same
// (roundsetup, monitor, NFS files ...)
//func (sn *Node) StatusConnections(view int, am *AnnouncementMessage) error {

// This will broadcast the final signature to give to client
// it contins the global Response adn global challenge
func (sn *Node) SignatureBroadcast(sm *SignatureBroadcastMessage) error {
	view := sm.ViewNbr
	RoundNbr := sm.RoundNbr
	dbg.Lvl3(sn.Name(), "received SignatureBroadcast on", view)
	sn.PeerStatusRcvd = 0

	ri := sn.Rounds[RoundNbr]
	if ri == nil {
		return fmt.Errorf("No round created for this round number (signature broadcast)")
	}
	out := make([]*SignatureBroadcastMessage, sn.NChildren(view))
	for i := range out {
		out[i] = &SignatureBroadcastMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:  view,
				RoundNbr: RoundNbr,
			},
			R0_hat:              sn.suite.Secret().One(),
			C:                   sn.suite.Secret().One(),
			X0_hat:              sn.suite.Point().Null(),
			V0_hat:              sn.suite.Point().Null(),
			RejectionPublicList: make([]abstract.Point, 0),
			RejectionCommitList: make([]abstract.Point, 0),
		}
	}

	err := ri.SignatureBroadcast(sm, out)
	if err != nil {
		return err
	}

	if !sn.Leaf(view) {
		dbg.Lvl3(sn.Name(), "in SignatureBroadcast is calling", sn.NChildren(view), "children")
		ctx := context.TODO()
		for i, ch := range sn.Children(view) {
			// Why oh why do we have to do this?
			out[i].X0_hat = sn.suite.Point().Add(out[i].X0_hat, sn.suite.Point().Null())
			if err := sn.PutDown(ctx, view, ch.Name(), out[i]); err != nil {
				return err
			}
		}
	} else {
		dbg.Lvl3(sn.Name(), "sending StatusReturn")
		return sn.StatusReturn(view, &StatusReturnMessage{
			SigningMessage: &SigningMessage{
				ViewNbr:  view,
				RoundNbr: RoundNbr},
		})
	}
	return nil
}

// StatusReturn just adds up all children and sends the result to
// the parent
func (sn *Node) StatusReturn(view int, sm *StatusReturnMessage) error {
	sn.PeerStatusRcvd += 1
	sn.PeerStatus.Responders += sm.Responders
	sn.PeerStatus.Peers += sm.Peers

	// Wait for other children before propagating the message
	if sn.PeerStatusRcvd < len(sn.Children(view)) {
		dbg.Lvl3(sn.Name(), "Waiting for other children")
		return nil
	}

	var err error = nil
	if sn.Root(view) {
		// Add the root-node
		sn.PeerStatus.Peers += 1
		dbg.Lvl3("We got", sn.PeerStatus.Responders, "responses from", sn.PeerStatus.Peers, "peers.")
	} else {
		dbg.Lvl4(sn.Name(), "puts up statusReturn for", sn.PeerStatus)
		ctx := context.TODO()
		sm.Responders = sn.PeerStatus.Responders
		sm.Peers = sn.PeerStatus.Peers
		err = sn.PutUp(ctx, view, sm)
	}
	dbg.Lvl3("Deleting round", sm.RoundNbr, sn.Rounds)
	delete(sn.Rounds, sm.RoundNbr)
	return err
}
