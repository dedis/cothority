package conode

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sign"
	"golang.org/x/net/context"
)

/*
Implements a Stamper and a Cosi-round
*/

const RoundStamperListenerType = "stamperlistener"

type RoundStamperListener struct {
	*StampListener
	*RoundStamper
	ClientQueue   []ReplyMessage
	RoundMessages int
}

type ReplyMessage struct {
	Val   []byte
	To    string
	ReqNo byte
}

func init() {
	sign.RegisterRoundFactory(RoundStamperListenerType,
		func(node *sign.Node) sign.Round {
			return NewRoundStamperListener(node)
		})
}

func NewRoundStamperListener(node *sign.Node) *RoundStamperListener {
	dbg.Lvl3("Making new RoundStamperListener", node.Name())
	round := &RoundStamperListener{}
	round.StampListener = NewStampListener(node.Name(), node.Suite())
	round.RoundStamper = NewRoundStamper(node)
	round.Type = RoundStamperListenerType
	return round
}

// Announcement is already defined in RoundStamper

func (round *RoundStamperListener) Commitment(in []*sign.CommitmentMessage, out *sign.CommitmentMessage) error {
	round.Mux.Lock()
	// messages read will now be processed
	round.Queue[READING], round.Queue[PROCESSING] = round.Queue[PROCESSING], round.Queue[READING]
	round.Queue[READING] = round.Queue[READING][:0]
	msgs := len(round.Queue[PROCESSING])
	out.Messages = msgs
	for _, m := range in {
		out.Messages += m.Messages
	}
	if round.IsRoot {
		round.RoundMessages = out.Messages
		round.Node.Messages += out.Messages
	}

	round.ClientQueue = make([]ReplyMessage, msgs)
	queue := make([][]byte, len(round.Queue[PROCESSING]))
	for i, q := range round.Queue[PROCESSING] {
		queue[i] = q.Tsm.Val
		round.ClientQueue[i] = ReplyMessage{
			Val:   q.Tsm.Val,
			To:    q.To,
			ReqNo: byte(q.Tsm.ReqNo),
		}
	}
	// get data from s once to avoid refetching from structure
	round.RoundStamper.QueueSet(queue)
	round.Mux.Unlock()

	round.RoundStamper.Commitment(in, out)
	return nil
}

// Challenge is already defined in RoundStamper

// Response is already defined in RoundStamper
func (round *RoundStamperListener) SignatureBroadcast(in *sign.SignatureBroadcastMessage, out []*sign.SignatureBroadcastMessage) error {
	round.RoundStamper.SignatureBroadcast(in, out)
	if round.IsRoot {
		in.Messages = round.RoundMessages
	} else {
		round.Node.Messages = in.Messages
	}
	for _, o := range out {
		o.Messages = in.Messages
	}
	for i, msg := range round.ClientQueue {
		respMessg := &StampSignature{
			ReqNo:               SeqNo(msg.ReqNo),
			Timestamp:           round.Timestamp,
			MerkleRoot:          round.MTRoot,
			Prf:                 round.RoundStamper.CombProofs[i],
			Response:            in.R0_hat,
			Challenge:           in.C,
			AggCommit:           in.V0_hat,
			AggPublic:           in.X0_hat,
			RejectionPublicList: in.RejectionPublicList,
			RejectionCommitList: in.RejectionCommitList,
		}
		round.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	return nil
}

// Send message to client given by name
func (round *RoundStamperListener) PutToClient(name string, data network.ProtocolMessage) {
	ctx := context.TODO()
	err := round.Clients[name].Send(ctx, data)
	if err == network.ErrClosed {
		round.Clients[name].Close()
		return
	}
	if err != nil {
		dbg.Lvl1("%p error putting to client: %v", round, err)
	}
}
