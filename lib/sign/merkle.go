package sign

// Functions used in collective signing
// That are direclty related to the generation/ verification/ sending
// of the Merkle Tree Signature

import (
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/coconet"
	"errors"
	"sync/atomic"
	"golang.org/x/net/context"
)

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
