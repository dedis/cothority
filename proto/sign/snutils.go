package sign

import (
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/logutils"
)

func (sn *Node) multiplexOnChildren(view int, sm *SigningMessage) {
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(view))
	for i := range messgs {
		messgs[i] = sm
	}

	// ctx, _ := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	ctx := context.TODO()
	if err := sn.PutDown(ctx, view, messgs); err != nil {
		log.Errorln("failed to putdown messg to children")
	}
}

// Returns the list of children for new view (peers - parent)
func (sn *Node) childrenForNewView(parent string) []string {
	peers := sn.Peers()
	children := make([]string, 0, len(peers)-1)
	for p := range peers {
		if p == parent {
			continue
		}
		children = append(children, p)
	}

	return children
}

func (sn *Node) StopHeartbeat() {
	sn.hbLock.Lock()
	if sn.heartbeat != nil {
		sn.heartbeat.Stop()
	}
	sn.hbLock.Unlock()
}

func (sn *Node) ReceivedHeartbeat(view int) {
	// XXX heartbeat should be associated with a specific view
	// if we get a heartbeat for an old view then nothing should change
	// there is a problem here where we could, if we receive a heartbeat
	// from an old view, try viewchanging into a view that we have already been to
	sn.hbLock.Lock()
	// hearbeat is nil if we have sust close the signing node
	if sn.heartbeat != nil {
		sn.heartbeat.Stop()
		sn.heartbeat = time.AfterFunc(HEARTBEAT, func() {
			dbg.Lvl4(sn.Name(), "NO HEARTBEAT - try view change:", view)
			sn.TryViewChange(view + 1)
		})
	}
	sn.hbLock.Unlock()

}

func (sn *Node) TryRootFailure(view, roundNbr int) bool {
	if sn.IsRoot(view) && sn.FailAsRootEvery != 0 {
		if sn.RoundsAsRoot != 0 && sn.RoundsAsRoot%sn.FailAsRootEvery == 0 {
			log.Errorln(sn.Name() + "was imposed root failure on round" + strconv.Itoa(roundNbr))
			log.WithFields(log.Fields{
				"file":  logutils.File(),
				"type":  "root_failure",
				"round": roundNbr,
			}).Info(sn.Name() + "Root imposed failure")
			// It doesn't make sense to try view change twice
			// what we essentially end up doing is double setting sn.ViewChanged
			// it is up to our followers to time us out and go to the next leader
			// sn.TryViewChange(view + 1)
			return true
		}
	}

	return false
}

// Simulate failure in system
func (sn *Node) TryFailure(view, roundNbr int) error {
	if sn.TryRootFailure(view, roundNbr) {
		return ErrImposedFailure
	}

	if !sn.IsRoot(view) && sn.FailAsFollowerEvery != 0 && roundNbr %sn.FailAsFollowerEvery == 0 {
		// when failure rate given fail with that probability
		if (sn.FailureRate > 0 && sn.ShouldIFail("")) || (sn.FailureRate == 0) {
			log.WithFields(log.Fields{
				"file":  logutils.File(),
				"type":  "follower_failure",
				"round": roundNbr,
			}).Info(sn.Name() + "Follower imposed failure")
			return errors.New(sn.Name() + "was imposed follower failure on round" + strconv.Itoa(roundNbr))
		}
	}

	// doing this before annoucing children to avoid major drama
	if !sn.IsRoot(view) && sn.ShouldIFail("commit") {
		log.Warn(sn.Name(), "not announcing or commiting for round", roundNbr)
		return ErrImposedFailure
	}
	return nil
}

// Create round lasting secret and commit point v and V
// Initialize log structure for the round
func (sn *Node) initCommitCrypto(roundNbr int) {
	round := sn.Rounds[roundNbr]
	// generate secret and point commitment for this round
	rand := sn.suite.Cipher([]byte(sn.Name()))
	round.Log = SNLog{}
	round.Log.v = sn.suite.Secret().Pick(rand)
	round.Log.V = sn.suite.Point().Mul(nil, round.Log.v)
	// initialize product of point commitments
	round.Log.V_hat = sn.suite.Point().Null()
	round.Log.Suite = sn.suite
	sn.add(round.Log.V_hat, round.Log.V)

	round.X_hat = sn.suite.Point().Null()
	sn.add(round.X_hat, sn.PubKey)
}

// Figure out which kids did not submit messages
// Add default messages to messgs, one per missing child
// as to make it easier to identify and add them to exception lists in one place
func (sn *Node) FillInWithDefaultMessages(view int, messgs []*SigningMessage) []*SigningMessage {
	children := sn.Children(view)

	allmessgs := make([]*SigningMessage, len(messgs))
	copy(allmessgs, messgs)

	for c := range children {
		found := false
		for _, m := range messgs {
			if m.From == c {
				found = true
				break
			}
		}

		if !found {
			allmessgs = append(allmessgs, &SigningMessage{View: view, Type: Default, From: c})
		}
	}

	return allmessgs
}

// accommodate nils
func (sn *Node) add(a abstract.Point, b abstract.Point) {
	if a == nil {
		a = sn.suite.Point().Null()
	}
	if b != nil {
		a.Add(a, b)
	}

}

// accommodate nils
func (sn *Node) sub(a abstract.Point, b abstract.Point) {
	if a == nil {
		a = sn.suite.Point().Null()
	}
	if b != nil {
		a.Sub(a, b)
	}

}

func (sn *Node) subExceptions(a abstract.Point, keys []abstract.Point) {
	for _, k := range keys {
		sn.sub(a, k)
	}
}

func (sn *Node) updateLastSeenVote(hv int, from string) {
	if int(atomic.LoadInt64(&sn.LastSeenVote)) < hv {
		atomic.StoreInt64(&sn.LastSeenVote, int64(hv))
	}
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxint64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
