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
	Leaves []hashid.HashId // can be removed after we verify protocol
	Root   hashid.HashId
	Proofs []proof.Proof
	// Timestamp message for this Round
	Timestamp int64

	peer     *Peer
	Round    *sign.RoundMerkle
	RoundNbr int
	sn       *sign.Node
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
	cbs.sn = peer.Node
	return cbs
}

// AnnounceFunc will keep the timestamp generated for this round
func (cs *RoundStamper) Announcement(RoundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	am := in.Am
	// We are root !
	if am == nil {
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		cs.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		am = &sign.AnnouncementMessage{Message: b.Bytes(), RoundType: RoundStamperType}
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		cs.Timestamp = t
	}
	cs.RoundNbr = RoundNbr
	if err := cs.sn.TryFailure(cs.sn.ViewNo, RoundNbr); err != nil {
		return err
	}

	if err := sign.RoundSetup(cs.sn, cs.sn.ViewNo, RoundNbr, am); err != nil {
		return err
	}
	// Store the message for the round
	cs.Round = cs.sn.Rounds[RoundNbr]
	cs.Round.Msg = am.Message

	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		out[i].Am = am
	}
	return nil
}

func (cs *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	// prepare to handle exceptions
	round := cs.Round
	round.Commits = in
	round.ExceptionList = make([]abstract.Point, 0)

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	children := round.Children
	round.ChildV_hat = make(map[string]abstract.Point, len(children))
	// X for public key
	round.ChildX_hat = make(map[string]abstract.Point, len(children))

	// Commits from children are the first Merkle Tree leaves for the round
	round.Leaves = make([]hashid.HashId, 0)
	round.LeavesFrom = make([]string, 0)

	for key := range children {
		round.ChildX_hat[key] = round.Suite.Point().Null()
		round.ChildV_hat[key] = round.Suite.Point().Null()
	}

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
		round.Add(round.X_hat, sm.Com.X_hat)
		round.Add(round.Log.V_hat, sm.Com.V_hat)
		//dbg.Lvl4("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	dbg.Lvl4("sign.Node.Commit using Merkle")
	round.MerkleAddChildren()
	// compute the local Merkle root

	cs.peer.Mux.Lock()
	// get data from s once to avoid refetching from structure
	Queue := cs.peer.Queue
	// messages read will now be processed
	cs.peer.Queue[READING], cs.peer.Queue[PROCESSING] = cs.peer.Queue[PROCESSING], cs.peer.Queue[READING]
	cs.peer.Queue[READING] = cs.peer.Queue[READING][:0]

	// give up if nothing to process
	if len(Queue[PROCESSING]) == 0 {
		cs.peer.Mux.Unlock()
		cs.Root = make([]byte, hashid.Size)
		cs.Proofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		cs.Leaves = make([]hashid.HashId, 0)
		for _, msg := range Queue[PROCESSING] {
			cs.Leaves = append(cs.Leaves, hashid.HashId(msg.Tsm.Sreq.Val))
		}
		cs.peer.Mux.Unlock()

		// create Merkle tree for this round's messages and check corectness
		cs.Root, cs.Proofs = proof.ProofTree(cs.Round.Suite.Hash, cs.Leaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(cs.Round.Suite.Hash, cs.Root, cs.Leaves, cs.Proofs) == true {
				dbg.Lvl4("Local Proofs of", cs.peer.Name(), "successful for round "+strconv.Itoa(int(cs.peer.LastRound())))
			} else {
				panic("Local Proofs" + cs.peer.Name() + " unsuccessful for round " + strconv.Itoa(int(cs.peer.LastRound())))
			}
		}
	}

	cs.Round.MerkleAddLocal(cs.Root)
	cs.Round.MerkleHashLog()
	cs.Round.ComputeCombinedMerkleRoot()

	com := &sign.CommitmentMessage{
		V:             cs.Round.Log.V,
		V_hat:         cs.Round.Log.V_hat,
		X_hat:         cs.Round.X_hat,
		MTRoot:        cs.Round.MTRoot,
		ExceptionList: cs.Round.ExceptionList,
		Vote:          cs.Round.Vote,
		Messages:      cs.sn.Messages}
	cs.sn.Messages = 0 // TODO : why ?
	out.Com = com
	return nil

}

func (cs *RoundStamper) Challenge(chm *sign.SigningMessage, out []*sign.SigningMessage) error {

	round := cs.Round
	// we are root
	if chm.Chm == nil {
		msg := cs.Round.Msg
		msg = append(msg, []byte(round.MTRoot)...)
		cs.Round.C = sign.HashElGamal(round.Suite, msg, cs.Round.Log.V_hat)
		//proof := make([]hashid.HashId, 0)

		chm.Chm = &sign.ChallengeMessage{
			C:      cs.Round.C,
			MTRoot: cs.Round.MTRoot,
			Proof:  cs.Round.Proof,
			Vote:   cs.Round.Vote}
	} else { // we are a leaf
		// register challenge
		cs.Round.C = chm.Chm.C
		//cs.Round.MTRoot = chm.MTRoot
	}
	// compute response share already + localmerkle proof
	cs.Round.InitResponseCrypto()
	// messages from clients, proofs computed
	if cs.Round.Log.Getv() != nil {
		if err := cs.Round.StoreLocalMerkleProof(chm.Chm); err != nil {
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
func (cs *RoundStamper) Response(sms []*sign.SigningMessage, out *sign.SigningMessage) error {
	// initialize exception handling
	exceptionV_hat := cs.Round.Suite.Point().Null()
	exceptionX_hat := cs.Round.Suite.Point().Null()
	cs.Round.ExceptionList = make([]abstract.Point, 0)
	nullPoint := cs.Round.Suite.Point().Null()

	children := cs.Round.Children
	for _, sm := range sms {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			// dbg.Lvl4(sn.Name(), "default in respose for child", from, sm)
			if children[from] != nil {
				cs.Round.ExceptionList = append(cs.Round.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				cs.Round.Add(exceptionX_hat, cs.Round.ChildX_hat[from])
				cs.Round.Add(exceptionV_hat, cs.Round.ChildV_hat[from])
			}
			continue
		case sign.Response:
			// disregard response from children who did not commit
			_, ok := cs.Round.ChildV_hat[from]
			if ok == true && cs.Round.ChildV_hat[from].Equal(nullPoint) {
				continue
			}

			// dbg.Lvl4(sn.Name(), "accepts response from", from, sm.Type)
			cs.Round.R_hat.Add(cs.Round.R_hat, sm.Rm.R_hat)

			cs.Round.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)

			cs.Round.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			cs.Round.ExceptionList = append(cs.Round.ExceptionList, sm.Rm.ExceptionList...)

		case sign.Error:
			if sm.Err == nil {
				dbg.Lvl2("Error message with no error")
				continue
			}

			// Report up non-networking error, probably signature failure
			dbg.Lvl2(cs.Round.Name, "Error in respose for child", from, sm)
			err := errors.New(sm.Err.Err)
			return err
		}
	}

	// remove exceptions from subtree that failed
	cs.Round.Sub(cs.Round.X_hat, exceptionX_hat)
	cs.Round.ExceptionV_hat = exceptionV_hat
	cs.Round.ExceptionX_hat = exceptionX_hat

	dbg.Lvl4(cs.Round.Name, "got all responses")
	err := cs.Round.VerifyResponses()
	if err != nil {
		dbg.Lvl3(cs.sn.Name(), "Could not verify responses..")
		return err
	}
	rm := &sign.ResponseMessage{
		R_hat:          cs.Round.R_hat,
		ExceptionList:  cs.Round.ExceptionList,
		ExceptionV_hat: cs.Round.ExceptionV_hat,
		ExceptionX_hat: cs.Round.ExceptionX_hat,
	}
	out.Rm = rm
	return nil
}

func (cs *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	// Root is creating the sig broadcast
	sb := in.SBm
	if sb == nil && cs.sn.IsRoot(cs.Round.View) {
		dbg.Lvl2(cs.sn.Name(), ": sending number of messages:", cs.sn.Messages)
		sb = &sign.SignatureBroadcastMessage{
			R0_hat:   cs.Round.R_hat,
			C:        cs.Round.C,
			X0_hat:   cs.Round.X_hat,
			V0_hat:   cs.Round.Log.V_hat,
			Messages: cs.sn.Messages,
		}
	} else {
		cs.sn.Messages = sb.Messages
	}
	// Inform all children of broadcast  - just copy the one that came in
	for i := range out {
		out[i].SBm = sb
	}
	// Send back signature to clients
	cs.peer.Mux.Lock()
	for i, msg := range cs.peer.Queue[PROCESSING] {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(cs.Round.Proof))
		copy(combProof, cs.Round.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, cs.Proofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(cs.Round.Suite.Hash, cs.Round.MTRoot,
			cs.Leaves[i], combProof) {
			dbg.Lvl2("Proof is OK for msg", msg)
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: msg.Tsm.ReqNo,
			Srep: &StampSignature{
				SuiteStr:   cs.Round.Suite.String(),
				Timestamp:  cs.Timestamp,
				MerkleRoot: cs.Round.MTRoot,
				Prf:        combProof,
				Response:   sb.R0_hat,
				Challenge:  sb.C,
				AggCommit:  sb.V0_hat,
				AggPublic:  sb.X0_hat,
			}}
		cs.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	cs.peer.Mux.Unlock()
	cs.Timestamp = 0
	return nil
}

// Send message to client given by name
func (cs *RoundStamper) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := cs.peer.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		cs.peer.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		log.Warnf("%p error putting to client: %v", cs, err)
	}
}
