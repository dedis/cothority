package conode

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/sign"
	"strconv"
	"time"
)

/*
Implements a merkle-tree hasher for incoming messages that
are passed to roundcosi.
*/

const RoundStamperType = "stamper"

type RoundStamper struct {
	*sign.RoundCosi
	Timestamp int64

	Proof       []hashid.HashId // the inclusion-proof of the data
	MTRoot      hashid.HashId   // mt root for subtree, passed upwards
	StampLeaves []hashid.HashId
	StampRoot   hashid.HashId
	StampProofs []proof.Proof
	StampQueue  [][]byte
	CombProofs  []proof.Proof
}

func init() {
	sign.RegisterRoundFactory(RoundStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundStamper(s)
		})
}

func NewRoundStamper(node *sign.Node) *RoundStamper {
	dbg.Lvl3("Making new RoundStamper", node.Name())
	round := &RoundStamper{}
	round.RoundCosi = sign.NewRoundCosi(node)
	round.Type = RoundStamperType
	return round
}

func (round *RoundStamper) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	dbg.Lvl3("New roundstamper announcement in round-nbr", roundNbr)
	if round.IsRoot {
		// We are root !
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		round.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		in.Am.Message = b.Bytes()
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(in.Am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		dbg.Lvl3("Received timestamp:", t)
		round.Timestamp = t
	}
	round.RoundCosi.Announcement(viewNbr, roundNbr, in, out)
	return nil
}

func (round *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	// compute the local Merkle root

	// give up if nothing to process
	if len(round.StampQueue) == 0 {
		round.StampRoot = make([]byte, hashid.Size)
		round.StampProofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		round.StampLeaves = make([]hashid.HashId, 0)
		for _, msg := range round.StampQueue {
			round.StampLeaves = append(round.StampLeaves, hashid.HashId(msg))
		}

		// create Merkle tree for this round's messages and check corectness
		round.StampRoot, round.StampProofs = proof.ProofTree(round.Suite.Hash, round.StampLeaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(round.Suite.Hash, round.StampRoot, round.StampLeaves, round.StampProofs) == true {
				dbg.Lvl4("Local Proofs of", round.Name, "successful for round "+
					strconv.Itoa(round.RoundNbr))
			} else {
				panic("Local Proofs" + round.Name + " unsuccessful for round " +
					strconv.Itoa(round.RoundNbr))
			}
		}
	}
	out.Com.MTRoot = round.StampRoot
	round.RoundCosi.Commitment(in, out)
	return nil
}

func (round *RoundStamper) QueueSet(queue [][]byte) {
	round.StampQueue = make([][]byte, len(queue))
	copy(round.StampQueue, queue)
}

// Challenge is already defined in RoundCosi

// Response is already defined in RoundCosi

func (round *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	round.RoundCosi.SignatureBroadcast(in, out)
	round.Proof = round.RoundCosi.Cosi.Proof
	round.MTRoot = round.RoundCosi.Cosi.MTRoot

	round.CombProofs = make([]proof.Proof, len(round.StampQueue))
	// Send back signature to clients
	for i, msg := range round.StampQueue {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(round.Proof))
		copy(combProof, round.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, round.StampProofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(round.Suite.Hash, round.MTRoot,
			round.StampLeaves[i], combProof) {
			dbg.Lvl2("Proof is OK for msg", msg)
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		round.CombProofs[i] = combProof
	}
	return nil
}
