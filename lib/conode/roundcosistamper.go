package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/dbg"
"github.com/dedis/cothority/lib/coconet"
)

/*
Implements a Stamper and a Cosi-round
 */

const RoundCosiStamperType = "cosistamper"

type RoundCosiStamper struct {
	*StampListener
	*RoundStamper
	ClientQueue []ReplyMessage
}

type ReplyMessage struct {
	Val   []byte
	To    string
	ReqNo byte
}

func init() {
	sign.RegisterRoundFactory(RoundCosiStamperType,
		func(node *sign.Node) sign.Round {
			return NewRoundCosiStamper(node)
		})
}

func NewRoundCosiStamper(node *sign.Node) *RoundCosiStamper {
	dbg.Lvlf3("Making new roundcosistamper %+v", node)
	round := &RoundCosiStamper{}
	round.StampListener = NewStampListener(node.Name())
	round.RoundStamper = NewRoundStamper(node)
	round.Type = RoundCosiStamperType
	return round
}

func (round *RoundCosiStamper) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage,
out []*sign.SigningMessage) error {
	dbg.Lvl3("Starting new announcement")
	round.RoundStamper.Announcement(viewNbr, roundNbr, in, out)
	return nil
}

func (round *RoundCosiStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.Mux.Lock()
	// messages read will now be processed
	round.Queue[READING], round.Queue[PROCESSING] = round.Queue[PROCESSING], round.Queue[READING]
	round.Queue[READING] = round.Queue[READING][:0]
	round.ClientQueue = make([]ReplyMessage, len(round.Queue[PROCESSING]))

	queue := make([][]byte, len(round.Queue[PROCESSING]))
	for i, q := range (round.Queue[PROCESSING]) {
		queue[i] = q.Tsm.Sreq.Val
		round.ClientQueue[i] = ReplyMessage{
			Val: q.Tsm.Sreq.Val,
			To: q.To,
			ReqNo: byte(q.Tsm.ReqNo),
		}
	}
	// get data from s once to avoid refetching from structure
	round.RoundStamper.QueueSet(queue)
	round.Mux.Unlock()

	round.RoundStamper.Commitment(in, out)
	return nil
}

func (round *RoundCosiStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundStamper.Challenge(in, out)
	return nil
}

func (round *RoundCosiStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	round.RoundStamper.Response(in, out)
	return nil
}

func (round *RoundCosiStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundStamper.SignatureBroadcast(in, out)
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
func (round *RoundCosiStamper) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := round.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		round.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		dbg.Lvl1("%p error putting to client: %v", round, err)
	}
}
