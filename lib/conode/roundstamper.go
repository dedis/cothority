package conode

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"errors"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/abstract"
)

// The name type of this round implementation
const RoundStamperType = "stamper"

type RoundStamper struct {
							   // Leaves, Root and Proof for a round
	CosiLeaves []hashid.HashId // can be removed after we verify protocol
	CosiRoot   hashid.HashId
	CosiProofs []proof.Proof
							   // Timestamp message for this Round
	Timestamp  int64

	peer       *Peer
	Merkle     *sign.MerkleStruct
	RoundNbr   int
	Node       *sign.Node

	Queue      []ReplyMessage
}

type ReplyMessage struct {
	Val   []byte
	To    string
	ReqNo byte
}

func RegisterRoundStamper(p *Peer) {
	sign.RegisterRoundFactory(RoundStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundStamper(p)
		})
}

func NewRoundStamper(peer *Peer) *RoundStamper {
	cbs := &RoundStamper{}
	cbs.peer = peer
	cbs.Node = peer.Node
	return cbs
}

// AnnounceFunc will keep the timestamp generated for this round
func (round *RoundStamper) Announcement(RoundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	am := in.Am
	// We are root !
	if am == nil {
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		round.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		am = &sign.AnnouncementMessage{Message: b.Bytes(), RoundType: RoundStamperType}
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		round.Timestamp = t
	}
	round.RoundNbr = RoundNbr
	if err := round.Node.TryFailure(round.Node.ViewNo, RoundNbr); err != nil {
		return err
	}

	if err := sign.MerkleSetup(round.Node, round.Node.ViewNo, RoundNbr, am); err != nil {
		return err
	}
	// Store the message for the round
	round.Merkle = round.Node.MerkleStructs[RoundNbr]
	round.Merkle.Msg = am.Message

	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		out[i].Am = am
	}
	return nil
}

func (round *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	// prepare to handle exceptions
	merkle := round.Merkle
	merkle.Commits = in
	merkle.ExceptionList = make([]abstract.Point, 0)

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	children := merkle.Children
	merkle.ChildV_hat = make(map[string]abstract.Point, len(children))
	// X for public key
	merkle.ChildX_hat = make(map[string]abstract.Point, len(children))

	for key := range children {
		merkle.ChildX_hat[key] = merkle.Suite.Point().Null()
		merkle.ChildV_hat[key] = merkle.Suite.Point().Null()
	}

	// Commits from children are the first Merkle Tree leaves for the round
	merkle.Leaves = make([]hashid.HashId, 0)
	merkle.LeavesFrom = make([]string, 0)
	for _, sm := range merkle.Commits {
		from := sm.From
		// MTR ==> root of sub-merkle tree
		merkle.Leaves = append(merkle.Leaves, sm.Com.MTRoot)
		merkle.LeavesFrom = append(merkle.LeavesFrom, from)
		merkle.ChildV_hat[from] = sm.Com.V_hat
		merkle.ChildX_hat[from] = sm.Com.X_hat
		merkle.ExceptionList = append(merkle.ExceptionList, sm.Com.ExceptionList...)

		// Aggregation
		// add good child server to combined public key, and point commit
		merkle.Add(merkle.X_hat, sm.Com.X_hat)
		merkle.Add(merkle.Log.V_hat, sm.Com.V_hat)
		//dbg.Lvl4("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	dbg.Lvl4("sign.Node.Commit using Merkle")
	merkle.MerkleAddChildren()
	// compute the local Merkle root

	round.peer.Mux.Lock()
	// get data from s once to avoid refetching from structure
	round.QueueSet(round.peer.Queue)
	round.peer.Mux.Unlock()

	// give up if nothing to process
	if len(round.Queue) == 0 {
		round.CosiRoot = make([]byte, hashid.Size)
		round.CosiProofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		round.CosiLeaves = make([]hashid.HashId, 0)
		for _, msg := range round.Queue {
			round.CosiLeaves = append(round.CosiLeaves, hashid.HashId(msg.Val))
		}

		// create Merkle tree for this round's messages and check corectness
		round.CosiRoot, round.CosiProofs = proof.ProofTree(round.Merkle.Suite.Hash, round.CosiLeaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(round.Merkle.Suite.Hash, round.CosiRoot, round.CosiLeaves, round.CosiProofs) == true {
				dbg.Lvl4("Local Proofs of", round.Node.Name(), "successful for round " + strconv.Itoa(int(round.Node.LastRound())))
			} else {
				panic("Local Proofs" + round.Node.Name() + " unsuccessful for round " + strconv.Itoa(int(round.Node.LastRound())))
			}
		}
	}

	round.Merkle.MerkleAddLocal(round.CosiRoot)
	round.Merkle.MerkleHashLog()
	round.Merkle.ComputeCombinedMerkleRoot()

	com := &sign.CommitmentMessage{
		V:             round.Merkle.Log.V,
		V_hat:         round.Merkle.Log.V_hat,
		X_hat:         round.Merkle.X_hat,
		MTRoot:        round.Merkle.MTRoot,
		ExceptionList: round.Merkle.ExceptionList,
		Vote:          round.Merkle.Vote,
		Messages:      round.Node.Messages}
	round.Node.Messages = 0 // TODO : why ?
	out.Com = com
	return nil

}

func (round *RoundStamper) QueueSet(Queue [][]MustReplyMessage) {
	// messages read will now be processed
	Queue[READING], Queue[PROCESSING] = Queue[PROCESSING], Queue[READING]
	Queue[READING] = Queue[READING][:0]
	round.Queue = make([]ReplyMessage, len(Queue[PROCESSING]))
	for i, q := range (Queue[PROCESSING]) {
		round.Queue[i] = ReplyMessage{
			Val: q.Tsm.Sreq.Val,
			To: q.To,
			ReqNo: byte(q.Tsm.ReqNo),
		}
	}
}

func (round *RoundStamper) Challenge(chm *sign.SigningMessage, out []*sign.SigningMessage) error {

	merkle := round.Merkle
	// we are root
	if chm.Chm == nil {
		msg := round.Merkle.Msg
		msg = append(msg, []byte(merkle.MTRoot)...)
		round.Merkle.C = sign.HashElGamal(merkle.Suite, msg, round.Merkle.Log.V_hat)
		//proof := make([]hashid.HashId, 0)

		chm.Chm = &sign.ChallengeMessage{
			C:      round.Merkle.C,
			MTRoot: round.Merkle.MTRoot,
			Proof:  round.Merkle.Proof,
			Vote:   round.Merkle.Vote}
	} else { // we are a leaf
		// register challenge
		round.Merkle.C = chm.Chm.C
	}
	// compute response share already + localmerkle proof
	round.Merkle.InitResponseCrypto()
	// messages from clients, proofs computed
	if round.Merkle.Log.Getv() != nil {
		if err := round.Merkle.StoreLocalMerkleProof(chm.Chm); err != nil {
			return err
		}
	}
	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		out[i].Chm = chm.Chm
	}

	return nil
}

// TODO make that sms == nil in case we are a leaf to stay consistent with
// others calls
func (round *RoundStamper) Response(sms []*sign.SigningMessage, out *sign.SigningMessage) error {
	// initialize exception handling
	exceptionV_hat := round.Merkle.Suite.Point().Null()
	exceptionX_hat := round.Merkle.Suite.Point().Null()
	round.Merkle.ExceptionList = make([]abstract.Point, 0)
	nullPoint := round.Merkle.Suite.Point().Null()

	children := round.Merkle.Children
	for _, sm := range sms {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			// dbg.Lvl4(sn.Name(), "default in respose for child", from, sm)
			if children[from] != nil {
				round.Merkle.ExceptionList = append(round.Merkle.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				round.Merkle.Add(exceptionX_hat, round.Merkle.ChildX_hat[from])
				round.Merkle.Add(exceptionV_hat, round.Merkle.ChildV_hat[from])
			}
			continue
		case sign.Response:
			// disregard response from children who did not commit
			_, ok := round.Merkle.ChildV_hat[from]
			if ok == true && round.Merkle.ChildV_hat[from].Equal(nullPoint) {
				continue
			}

			// dbg.Lvl4(sn.Name(), "accepts response from", from, sm.Type)
			round.Merkle.R_hat.Add(round.Merkle.R_hat, sm.Rm.R_hat)

			round.Merkle.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)

			round.Merkle.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			round.Merkle.ExceptionList = append(round.Merkle.ExceptionList, sm.Rm.ExceptionList...)

		case sign.Error:
			if sm.Err == nil {
				dbg.Lvl2("Error message with no error")
				continue
			}

			// Report up non-networking error, probably signature failure
			dbg.Lvl2(round.Merkle.Name, "Error in respose for child", from, sm)
			err := errors.New(sm.Err.Err)
			return err
		}
	}

	// remove exceptions from subtree that failed
	round.Merkle.Sub(round.Merkle.X_hat, exceptionX_hat)
	round.Merkle.ExceptionV_hat = exceptionV_hat
	round.Merkle.ExceptionX_hat = exceptionX_hat

	dbg.Lvl4(round.Merkle.Name, "got all responses")
	err := round.Merkle.VerifyResponses()
	if err != nil {
		dbg.Lvl3(round.Node.Name(), "Could not verify responses..")
		return err
	}
	rm := &sign.ResponseMessage{
		R_hat:          round.Merkle.R_hat,
		ExceptionList:  round.Merkle.ExceptionList,
		ExceptionV_hat: round.Merkle.ExceptionV_hat,
		ExceptionX_hat: round.Merkle.ExceptionX_hat,
	}
	out.Rm = rm
	return nil
}

func (round *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	// Root is creating the sig broadcast
	sb := in.SBm
	if sb == nil && round.Node.IsRoot(round.Merkle.View) {
		dbg.Lvl2(round.Node.Name(), ": sending number of messages:", round.Node.Messages)
		sb = &sign.SignatureBroadcastMessage{
			R0_hat:   round.Merkle.R_hat,
			C:        round.Merkle.C,
			X0_hat:   round.Merkle.X_hat,
			V0_hat:   round.Merkle.Log.V_hat,
			Messages: round.Node.Messages,
		}
	} else {
		round.Node.Messages = sb.Messages
	}
	// Inform all children of broadcast  - just copy the one that came in
	for i := range out {
		out[i].SBm = sb
	}
	// Send back signature to clients
	for i, msg := range round.Queue {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(round.Merkle.Proof))
		copy(combProof, round.Merkle.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, round.CosiProofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(round.Merkle.Suite.Hash, round.Merkle.MTRoot,
			round.CosiLeaves[i], combProof) {
			dbg.Lvl2("Proof is OK for msg", msg)
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: SeqNo(msg.ReqNo),
			Srep: &StampSignature{
				SuiteStr:   round.Merkle.Suite.String(),
				Timestamp:  round.Timestamp,
				MerkleRoot: round.Merkle.MTRoot,
				Prf:        combProof,
				Response:   sb.R0_hat,
				Challenge:  sb.C,
				AggCommit:  sb.V0_hat,
				AggPublic:  sb.X0_hat,
			}}
		round.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	round.Timestamp = 0
	return nil
}

// Send message to client given by name
func (round *RoundStamper) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := round.peer.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		round.peer.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		log.Warnf("%p error putting to client: %v", round, err)
	}
}
