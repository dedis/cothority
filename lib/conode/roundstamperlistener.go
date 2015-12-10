package conode

import (
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
)

/*
Implements a Stamper and a Cosi-round
*/

const RoundStamperListenerType = "stamperlistener"

type RoundStamperListener struct {
	*StampListener
	*RoundStamper
	ClientQueue   []ReplyMessage
	roundMessages int
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
	round.StampListener = NewStampListener(node.Name())
	round.RoundStamper = NewRoundStamper(node)
	round.Type = RoundStamperListenerType
	return round
}

// Announcement is already defined in RoundStamper

func (round *RoundStamperListener) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.Mux.Lock()
	// messages read will now be processed
	round.Queue[READING], round.Queue[PROCESSING] = round.Queue[PROCESSING], round.Queue[READING]
	round.Queue[READING] = round.Queue[READING][:0]
	msgs := len(round.Queue[PROCESSING])
	out.Com.Messages = msgs
	for _, m := range in {
		out.Com.Messages += m.Com.Messages
	}
	if round.IsRoot {
		round.roundMessages = out.Com.Messages
		round.Node.Messages += out.Com.Messages
	}

	round.ClientQueue = make([]ReplyMessage, msgs)
	queue := make([][]byte, len(round.Queue[PROCESSING]))
	for i, q := range round.Queue[PROCESSING] {
		queue[i] = q.Tsm.Sreq.Val
		round.ClientQueue[i] = ReplyMessage{
			Val:   q.Tsm.Sreq.Val,
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

func (round *RoundStamperListener) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundStamper.SignatureBroadcast(in, out)
	if round.IsRoot {
		in.SBm.Messages = round.roundMessages
	}
	for _, o := range out {
		o.SBm.Messages = in.SBm.Messages
	}
	for i, msg := range round.ClientQueue {
		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: SeqNo(msg.ReqNo),
			Srep: &StampSignature{
				SuiteStr:   round.Suite.String(),
				Timestamp:  round.Timestamp,
				MerkleRoot: round.MTRoot,
				Prf:        round.RoundStamper.CombProofs[i],
				Response:   in.SBm.R0_hat,
				Challenge:  in.SBm.C,
				AggCommit:  in.SBm.V0_hat,
				AggPublic:  in.SBm.X0_hat,
			}}
		round.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	return nil
}

// Send message to client given by name
func (round *RoundStamperListener) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := round.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		round.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		dbg.Lvl1("%p error putting to client: %v", round, err)
	}
}
