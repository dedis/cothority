package sign

import (
	"errors"
	"io"
	"strconv"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

// Collective Signing via ElGamal
// 1. Announcement
// 2. Commitment
// 3. Challenge
// 4. Response

// Get multiplexes all messages from TCPHost using application logic
func (sn *Node) get() error {
	dbg.Lvl4(sn.Name(), "getting")
	defer dbg.Lvl4(sn.Name(), "done getting")

	sn.UpdateTimeout()
	dbg.Lvl4("Going to get", sn.Name())
	msgchan := sn.Host.Get()
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
					err = sn.Promise(sm.View, sm.Com.Round, sm)
				} else {
					err = sn.Commit(sm.View, sm.Com.Round, sm)
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
					err = sn.Accepted(sm.View, sm.Rm.Round, sm)
				} else {
					err = sn.Respond(sm.View, sm.Rm.Round, sm)
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
				sn.CatchUp(vi+1, sm.From)
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

	if sn.AnnounceFunc != nil {
		sn.AnnounceFunc(am)
	}
	if err := sn.TryFailure(view, am.Round); err != nil {
		return err
	}

	if err := sn.setUpRound(view, am); err != nil {
		return err
	}

	// Inform all children of announcement
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
	for i := range messgs {
		sm := SigningMessage{
			Type:         Announcement,
			View:         view,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Am:           am}
		messgs[i] = &sm
	}
	dbg.Lvl4(sn.Name(), "sending to all children")
	ctx := context.TODO()
	//ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	if err := sn.PutDown(ctx, view, messgs); err != nil {
		return err
	}

	// return sn.Commit(view, am)
	// If we are a leaf, start the commit phase process
	if len(sn.Children(view)) == 0 {
		sn.Commit(view, am.Round, nil)
	}
	return nil
}

func (sn *Node) Commit(view, Round int, sm *SigningMessage) error {
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, Round)
	sn.roundmu.Unlock()

	round := sn.Rounds[Round]
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

	// prepare to handle exceptions
	round.ExceptionList = make([]abstract.Point, 0)

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	round.ChildV_hat = make(map[string]abstract.Point, len(sn.Children(view)))
	// X for public key
	round.ChildX_hat = make(map[string]abstract.Point, len(sn.Children(view)))
	children := sn.Children(view)

	// Commits from children are the first Merkle Tree leaves for the round
	round.Leaves = make([]hashid.HashId, 0)
	round.LeavesFrom = make([]string, 0)

	for key := range children {
		round.ChildX_hat[key] = sn.suite.Point().Null()
		round.ChildV_hat[key] = sn.suite.Point().Null()
	}

	// TODO: fill in missing commit messages, and add back exception code
	for _, sm := range round.Commits {
		from := sm.From
		// MTR ==> root of sub-merkle tree
		round.Leaves = append(round.Leaves, sm.Com.MTRoot)
		round.LeavesFrom = append(round.LeavesFrom, from)
		round.ChildV_hat[from] = sm.Com.V_hat
		round.ChildX_hat[from] = sm.Com.X_hat
		round.ExceptionList = append(round.ExceptionList, sm.Com.ExceptionList...)

		// Aggregation
		// add good child server to combined public key, and point commit
		sn.add(round.X_hat, sm.Com.X_hat)
		sn.add(round.Log.V_hat, sm.Com.V_hat)
		//dbg.Print("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	if sn.Type == PubKey {
		dbg.Lvl4("sign.Node.Commit using PubKey")
		return sn.actOnCommits(view, Round)
	} else {
		dbg.Lvl4("sign.Node.Commit using Merkle")
		sn.AddChildrenMerkleRoots(Round)
		// compute the localmerkle root
		if sn.CommitFunc != nil {
			sn.AddLocalMerkleRoot(view, Round, sn.CommitFunc(view))
		} else {
			sn.AddLocalMerkleRoot(view, Round, make([]byte, hashid.Size))
		}
		sn.HashLog(Round)
		sn.ComputeCombinedMerkleRoot(view, Round)
		return sn.actOnCommits(view, Round)
	}
}

// Finalize commits by initiating the challenge pahse if root
// Send own commitment message up to parent if non-root
func (sn *Node) actOnCommits(view, Round int) error {
	round := sn.Rounds[Round]
	var err error

	if sn.IsRoot(view) {
		dbg.Lvl3("Commit root : Aggregate Public Key :", round.X_hat)
		sn.commitsDone <- Round
		err = sn.FinalizeCommits(view, Round)
	} else {
		// create and putup own commit message
		com := &CommitmentMessage{
			V:             round.Log.V,
			V_hat:         round.Log.V_hat,
			X_hat:         round.X_hat,
			MTRoot:        round.MTRoot,
			ExceptionList: round.ExceptionList,
			Vote:          round.Vote,
			Round:         Round}

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
	sn.LastSeenRound = max(sn.LastSeenRound, chm.Round)
	sn.roundmu.Unlock()

	round := sn.Rounds[chm.Round]
	if round == nil {
		return nil
	}

	// register challenge
	round.c = chm.C

	if sn.Type == PubKey {
		dbg.Lvl4(sn.Name(), "challenge: using pubkey", sn.Type, chm.Vote)
		if err := sn.SendChildrenChallenges(view, chm); err != nil {
			return err
		}
	} else {
		dbg.Lvl4(sn.Name(), "challenge: using merkle proofs")
		// messages from clients, proofs computed
		if sn.CommitedFor(round) {
			if err := sn.StoreLocalMerkleProof(view, chm); err != nil {
				return err
			}

		}
		if err := sn.SendChildrenChallengesProofs(view, chm); err != nil {
			return err
		}
	}

	// dbg.Lvl4(sn.Name(), "In challenge before response")
	sn.initResponseCrypto(chm.Round)
	// if we are a leaf, send the respond up
	if len(sn.Children(view)) == 0 {
		sn.Respond(view, chm.Round, nil)
	}
	// dbg.Lvl4(sn.Name(), "Done handling challenge message")
	return nil
}

func (sn *Node) initResponseCrypto(Round int) {
	round := sn.Rounds[Round]
	// generate response   r = v - xc
	round.r = sn.suite.Secret()
	round.r.Mul(sn.PrivKey, round.c).Sub(round.Log.v, round.r)
	// initialize sum of children's responses
	round.r_hat = round.r
}

func (sn *Node) Respond(view, Round int, sm *SigningMessage) error {
	dbg.Lvl4(sn.Name(), "couting response on view, round", view, Round, "Nchildren", len(sn.Children(view)))
	// update max seen round
	sn.roundmu.Lock()
	sn.LastSeenRound = max(sn.LastSeenRound, Round)
	sn.roundmu.Unlock()

	round := sn.Rounds[Round]
	if round == nil || round.Log.v == nil {
		// If I was not announced of this round, or I failed to commit
		return nil
	}

	if sm != nil {
		round.Responses = append(round.Responses, sm)
	}
	if len(round.Responses) != len(sn.Children(view)) {
		return nil
	}

	// initialize exception handling
	exceptionV_hat := sn.suite.Point().Null()
	exceptionX_hat := sn.suite.Point().Null()
	round.ExceptionList = make([]abstract.Point, 0)
	nullPoint := sn.suite.Point().Null()
	allmessgs := sn.FillInWithDefaultMessages(view, round.Responses)

	children := sn.Children(view)
	for _, sm := range allmessgs {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			// dbg.Lvl4(sn.Name(), "default in respose for child", from, sm)
			if children[from] != nil {
				round.ExceptionList = append(round.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				sn.add(exceptionX_hat, round.ChildX_hat[from])
				sn.add(exceptionV_hat, round.ChildV_hat[from])
			}
			continue
		case Response:
			// disregard response from children who did not commit
			_, ok := round.ChildV_hat[from]
			if ok == true && round.ChildV_hat[from].Equal(nullPoint) {
				continue
			}

			// dbg.Lvl4(sn.Name(), "accepts response from", from, sm.Type)
			round.r_hat.Add(round.r_hat, sm.Rm.R_hat)

			sn.add(exceptionV_hat, sm.Rm.ExceptionV_hat)

			sn.add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			round.ExceptionList = append(round.ExceptionList, sm.Rm.ExceptionList...)

		case Error:
			if sm.Err == nil {
				log.Errorln("Error message with no error")
				continue
			}

			// Report up non-networking error, probably signature failure
			log.Errorln(sn.Name(), "Error in respose for child", from, sm)
			err := errors.New(sm.Err.Err)
			sn.PutUpError(view, err)
			return err
		}
	}

	// remove exceptions from subtree that failed
	sn.sub(round.X_hat, exceptionX_hat)
	round.exceptionV_hat = exceptionV_hat

	return sn.actOnResponses(view, Round, exceptionV_hat, exceptionX_hat)
}

func (sn *Node) actOnResponses(view, Round int, exceptionV_hat abstract.Point, exceptionX_hat abstract.Point) error {
	dbg.Lvl4(sn.Name(), "got all responses for view, round", view, Round)
	round := sn.Rounds[Round]
	err := sn.VerifyResponses(view, Round)

	isroot := sn.IsRoot(view)
	// if error put it up if parent exists
	if err != nil && !isroot {
		sn.PutUpError(view, err)
		return err
	}

	// if no error send up own response
	if err == nil && !isroot {
		if round.Log.v == nil && sn.ShouldIFail("response") {
			dbg.Lvl4(sn.Name(), "failing on response")
			return nil
		}

		// create and putup own response message
		rm := &ResponseMessage{
			R_hat:          round.r_hat,
			ExceptionList:  round.ExceptionList,
			ExceptionV_hat: exceptionV_hat,
			ExceptionX_hat: exceptionX_hat,
			Round:          Round}

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
	if isroot {
		sn.SignatureBroadcast(view, nil, Round)
		sn.done <- Round
	}

	return err
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

// Called *only* by root node after receiving all commits
func (sn *Node) FinalizeCommits(view int, Round int) error {
	round := sn.Rounds[Round]

	// challenge = Hash(Merkle Tree Root/ Announcement Message, sn.Log.V_hat)
	if sn.Type == PubKey {
		round.c = hashElGamal(sn.suite, sn.Message, round.Log.V_hat)
	} else {
		round.c = hashElGamal(sn.suite, round.MTRoot, round.Log.V_hat)
	}

	proof := make([]hashid.HashId, 0)
	err := sn.Challenge(view, &ChallengeMessage{
		C:      round.c,
		MTRoot: round.MTRoot,
		Proof:  proof,
		Round:  Round,
		Vote:   round.Vote})
	return err
}

// Called by every node after receiving aggregate responses from descendants
func (sn *Node) VerifyResponses(view, Round int) error {
	round := sn.Rounds[Round]

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := sn.suite.Point()
	V_clean := sn.suite.Point()
	V_clean.Add(V_clean.Mul(nil, round.r_hat), Aux.Mul(round.X_hat, round.c))
	// T is the recreated V_hat
	T := sn.suite.Point().Null()
	T.Add(T, V_clean)
	T.Add(T, round.exceptionV_hat)

	var c2 abstract.Secret
	isroot := sn.IsRoot(view)
	if isroot {
		// round challenge must be recomputed given potential
		// exception list
		if sn.Type == PubKey {
			round.c = hashElGamal(sn.suite, sn.Message, round.Log.V_hat)
			c2 = hashElGamal(sn.suite, sn.Message, T)
		} else {
			round.c = hashElGamal(sn.suite, round.MTRoot, round.Log.V_hat)
			c2 = hashElGamal(sn.suite, round.MTRoot, T)
		}
	}

	// intermediary nodes check partial responses aginst their partial keys
	// the root node is also able to check against the challenge it emitted
	if !T.Equal(round.Log.V_hat) || (isroot && !round.c.Equal(c2)) {
		if DEBUG == true {
			panic(sn.Name() + "reports ElGamal Collective Signature failed for Round" + strconv.Itoa(Round))
		}
		return errors.New("Verifying ElGamal Collective Signature failed in " + sn.Name() + "for round " + strconv.Itoa(Round))
	} else if isroot {
		dbg.Lvl4(sn.Name(), "reports ElGamal Collective Signature succeeded for round", Round, "view", view)
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
	if sn.RoundsPerView == 0{
		// No view change asked
		return false
	}
	sn.roundmu.Lock()
	defer sn.roundmu.Unlock()

	// if this round is last one for this view
	if sn.LastSeenRound%sn.RoundsPerView == 0 {
		// dbg.Lvl4(sn.Name(), "TIME FOR VIEWCHANGE:", lsr, rpv)
		return true
	}
	return false
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
		sn.Commit(view, am.Round, nil)
	}
	return nil
}

// This will broadcast the final signature to give to client
// it contins the global Response adn global challenge
func (sn *Node) SignatureBroadcast(view int, sb *SignatureBroadcastMessage, round int) error {
	dbg.Lvl2(sn.Name(), "received SignatureBroadcast on", view)
	// Root is creating the sig broadcast
	if sb == nil {
		r := sn.Rounds[round]
		if sn.IsRoot(view) {
			sb = &SignatureBroadcastMessage{
				R0_hat: r.r_hat,
				C:      r.c,
				X0_hat: r.X_hat,
				V0_hat: r.Log.V_hat,
			}
		}
	}
	// messages from clients, proofs computed
	//if sn.CommitedFor(sn.Round) {
	sn.SendLocalMerkleProof(view, sb)
	//}

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

func (sn *Node) SendLocalMerkleProof(view int, sb *SignatureBroadcastMessage) {
	if sn.DoneFunc != nil {
		sn.DoneFunc(view, sn.MTRoot, nil, sn.Proof, sb, sn.suite)
	}
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

// Returns a secret that depends on on a message and a point
func hashElGamal(suite abstract.Suite, message []byte, p abstract.Point) abstract.Secret {
	pb, _ := p.MarshalBinary()
	c := suite.Cipher(pb)
	c.Message(nil, nil, message)
	return suite.Secret().Pick(c)
}

// Called when log for round if full and ready to be hashed
func (sn *Node) HashLog(Round int) error {
	round := sn.Rounds[Round]
	var err error
	round.HashedLog, err = sn.hashLog(Round)
	return err
}

// Auxilary function to perform the actual hashing of the log
func (sn *Node) hashLog(Round int) ([]byte, error) {
	round := sn.Rounds[Round]

	h := sn.suite.Hash()
	logBytes, err := round.Log.MarshalBinary()
	if err != nil {
		return nil, err
	}
	h.Write(logBytes)
	return h.Sum(nil), nil
}

// Getting actual View
func (sn *Node) GetView() int {
	return sn.ViewNo
}
