package sign

import (
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"golang.org/x/net/context"
)

func (sn *Node) ApplyVotes(ch chan *Vote) {
	go func() {
		for v := range ch {
			if sn.RoundTypes[v.Round] == EmptyRT {
				sn.RoundTypes[v.Round] = RoundType(v.Type)
			}
			sn.ApplyVote(v)
		}
	}()
}

// HERE: after we change to the new view, we could send our parent
// a notification that we are ready to use the new view

func (sn *Node) ApplyVote(v *Vote) {
	atomic.StoreInt64(&sn.LastAppliedVote, int64(v.Index))
	lav := atomic.LoadInt64(&sn.LastAppliedVote)
	lsv := atomic.LoadInt64(&sn.LastSeenVote)
	atomic.StoreInt64(&sn.LastSeenVote, maxint64(lav, lsv))

	switch v.Type {
	case ViewChangeVT:
		sn.ChangeView(v.Vcv)
	case AddVT:
		sn.AddAction(v.Av.View, v)
	case RemoveVT:
		sn.AddAction(v.Rv.View, v)
	case ShutdownVT:
		sn.Close()
	default:
		log.Errorln("applyvote: unkown vote type")
	}
}

func (sn *Node) AddAction(view int, v *Vote) {
	sn.Actions[view] = append(sn.Actions[view], v)
}

func (sn *Node) ApplyAction(view int, v *Vote) {
	dbg.Lvl4(sn.Name(), "APPLYING ACTION")
	switch v.Type {
	case AddVT:
		sn.AddPeerToHostlist(view, v.Av.Name)
		if sn.Name() == v.Av.Parent {
			sn.AddChildren(view, v.Av.Name)
		}
	case RemoveVT:
		// removes node from Hostlist, and from children list
		sn.RemovePeer(view, v.Rv.Name)
		// not closing TCP connection on remove because if view
		// does not go through, connection essential to old/ current view closed
	default:
		log.Errorln("applyvote: unkown action type")
	}
}

func (sn *Node) NotifyOfAction(view int, v *Vote) {
	dbg.Lvl4(sn.Name(), "Notifying node to be added/removed of action")
	gcm := &SigningMessage{
		Type:         GroupChanged,
		From:         sn.Name(),
		View:         view,
		LastSeenVote: int(sn.LastSeenVote),
		Gcm: &GroupChangedMessage{
			V:        v,
			HostList: sn.HostListOn(view)}}

	switch v.Type {
	case AddVT:
		if sn.Name() == v.Av.Parent {
			sn.PutTo(context.TODO(), v.Av.Name, gcm)
		}
	case RemoveVT:
		if sn.Name() == v.Rv.Parent {
			sn.PutTo(context.TODO(), v.Rv.Name, gcm)
		}
	default:
		log.Errorln("notifyofaction: unkown action type")
	}
}

func (sn *Node) AddSelf(parent string) error {
	dbg.Lvl4("AddSelf: connecting to:", parent)
	err := sn.ConnectTo(parent)
	if err != nil {
		return err
	}

	dbg.Lvl4("AddSelf: putting group change message to:", parent)
	return sn.PutTo(
		context.TODO(),
		parent,
		&SigningMessage{
			Type: GroupChange,
			View: -1,
			Vrm: &VoteRequestMessage{
				Vote: &Vote{
					Type: AddVT,
					Av: &AddVote{
						Name:   sn.Name(),
						Parent: parent}}}})
}

func (sn *Node) RemoveSelf() error {
	return sn.PutUp(
		context.TODO(),
		int(sn.ViewNo),
		&SigningMessage{
			Type: GroupChange,
			View: -1,
			Vrm: &VoteRequestMessage{
				Vote: &Vote{
					Type: RemoveVT,
					Rv: &RemoveVote{
						Name:   sn.Name(),
						Parent: sn.Parent(sn.ViewNo)}}}})
}

func (sn *Node) CatchUp(vi int, from string) {
	dbg.Lvl4(sn.Name(), "attempting to catch up vote", vi)

	ctx := context.TODO()
	sn.PutTo(ctx, from,
		&SigningMessage{
			From:  sn.Name(),
			Type:  CatchUpReq,
			Cureq: &CatchUpRequest{Index: vi}})
}

func (sn *Node) StartGossip() {
	go func() {
		t := time.Tick(GOSSIP_TIME)
		for {
			select {
			case <-t:
				sn.viewmu.Lock()
				c := sn.HostListOn(sn.ViewNo)
				sn.viewmu.Unlock()
				if len(c) == 0 {
					log.Errorln(sn.Name(), "StartGossip: none in hostlist for view:", sn.ViewNo, len(c))
					continue
				}
				sn.randmu.Lock()
				from := c[sn.Rand.Int()%len(c)]
				sn.randmu.Unlock()
				dbg.Lvl4("Gossiping with:", from)
				sn.CatchUp(int(atomic.LoadInt64(&sn.LastAppliedVote)+1), from)
			case <-sn.closed:
				dbg.Lvl3("stopping gossip: closed")
				return
			}
		}
	}()
}
