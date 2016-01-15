package sign

import (
	"github.com/dedis/cothority/lib/dbg"

	"fmt"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"runtime/debug"
)

/*
RoundCosi implements the collective signature protocol using
Schnorr signatures to collectively sign on a message. By default
the message is only the collection of all Commits, but another
round can add any message it wants in the Commitment-phase.
*/

// The name type of this round implementation
const RoundCosiType = "cosi"

type RoundCosi struct {
	*RoundStruct
	Cosi       *CosiStruct
	SaveViewNo int
}

func init() {
	RegisterRoundFactory(RoundCosiType,
		func(node *Node) Round {
			return NewRoundCosi(node)
		})
}

func NewRoundCosi(node *Node) *RoundCosi {
	dbg.Lvlf3("Making new RoundCosi", node.Name())
	round := &RoundCosi{}
	round.RoundStruct = NewRoundStruct(node, RoundCosiType)
	return round
}

func (round *RoundCosi) CheckChildren() {
	c := round.Node.Children(round.Node.ViewNo)
	if len(c) != len(round.Cosi.Children) {
		dbg.Print("Children in cosi and node are different")
		dbg.Printf("round.Cosi: %+v", round.Cosi)
		dbg.Printf("Node.Children: %+v", round.Node.Children(round.Node.ViewNo))
		dbg.Print("viewNbr:", round.SaveViewNo, "Node.ViewNo:", round.Node.ViewNo)
		debug.PrintStack()
	}
}

// AnnounceFunc will keep the timestamp generated for this round
func (round *RoundCosi) Announcement(viewNbr, roundNbr int, in *AnnouncementMessage, out []*AnnouncementMessage) error {
	if err := round.Node.TryFailure(round.Node.ViewNo, roundNbr); err != nil {
		return err
	}

	// Store the message for the round
	//round.Merkle = round.Node.MerkleStructs[roundNbr]
	round.Cosi = NewCosi(round.Node, viewNbr, roundNbr, in)
	round.SaveViewNo = round.Node.ViewNo
	round.CheckChildren()

	round.Cosi.Msg = in.Message
	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		out[i] = in
	}
	return nil
}

func (round *RoundCosi) Commitment(in []*CommitmentMessage, out *CommitmentMessage) error {
	cosi := round.Cosi
	cosi.Commits = in

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	children := cosi.Children
	cosi.ChildV_hat = make(map[string]abstract.Point, len(children))
	// X for public key
	cosi.ChildX_hat = make(map[string]abstract.Point, len(children))

	for _, n := range children {
		cosi.ChildX_hat[n.Name()] = cosi.Suite.Point().Null()
		cosi.ChildV_hat[n.Name()] = cosi.Suite.Point().Null()
	}

	// Commits from children are the first Merkle Tree leaves for the round
	cosi.Leaves = make([]hashid.HashId, 0)
	cosi.LeavesFrom = make([]string, 0)
	for _, sm := range cosi.Commits {
		from := sm.From
		// MTR ==> root of sub-merkle tree
		cosi.Leaves = append(cosi.Leaves, sm.MTRoot)
		cosi.LeavesFrom = append(cosi.LeavesFrom, from)
		cosi.ChildV_hat[from] = sm.V_hat
		cosi.ChildX_hat[from] = sm.X_hat

		// Aggregation
		// add good child server to combined public key, and point commit
		cosi.X_hat.Add(cosi.X_hat, sm.X_hat)
		cosi.Log.V_hat.Add(cosi.Log.V_hat, sm.V_hat)
		//dbg.Lvl4("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	dbg.Lvl4("Node.Commit using Merkle")
	cosi.MerkleAddChildren()

	round.Cosi.MerkleAddLocal(out.MTRoot)
	round.Cosi.MerkleHashLog()
	round.Cosi.ComputeCombinedMerkleRoot()

	out.V = round.Cosi.Log.V
	out.V_hat = round.Cosi.Log.V_hat
	out.X_hat = round.Cosi.X_hat
	out.MTRoot = round.Cosi.MTRoot
	return nil

}

func (round *RoundCosi) Challenge(in *ChallengeMessage, out []*ChallengeMessage) error {

	cosi := round.Cosi
	// we are root
	if round.IsRoot {
		msg := cosi.Msg
		msg = append(msg, []byte(cosi.MTRoot)...)
		cosi.C = cosi.HashElGamal(msg, cosi.Log.V_hat)
		//proof := make([]hashid.HashId, 0)

		in.C = cosi.C
		in.MTRoot = cosi.MTRoot
		in.Proof = cosi.Proof
	} else { // we are a leaf
		// register challenge
		cosi.C = in.C
	}
	// compute response share already + localmerkle proof
	cosi.InitResponseCrypto()
	// messages from clients, proofs computed
	if cosi.Log.Getv() != nil {
		if err := cosi.StoreLocalMerkleProof(in); err != nil {
			return err
		}
	}

	// proof from big root to our root will be sent to all children
	baseProof := make(proof.Proof, len(in.Proof))
	copy(baseProof, in.Proof)

	round.CheckChildren()
	if len(cosi.Children) != len(out) {
		return fmt.Errorf("Children (%d) and output (%d) are of different length. Should be %d / %d",
			len(cosi.Children), len(out), len(round.Node.Children(round.Node.ViewNo)),
			round.Node.ViewNo)
	}
	// for each child, create personalized part of proof
	// embed it in SigningMessage, and send it
	var i = 0
	for _, n := range cosi.Children {
		out[i].C = in.C
		out[i].MTRoot = in.MTRoot
		out[i].Proof = append(baseProof, cosi.Proofs[n.Name()]...)
		out[i].To = n.Name()
		i++
	}
	return nil
}

// TODO make that in == nil in case we are a leaf to stay consistent with
// others calls
func (round *RoundCosi) Response(in []*ResponseMessage, out *ResponseMessage) error {
	dbg.Lvl4(round.Cosi.Name, "got all responses")
	for _, sm := range in {
		round.Cosi.R_hat.Add(round.Cosi.R_hat, sm.R_hat)
	}
	err := round.Cosi.VerifyResponses()
	if err != nil {
		dbg.Lvl3(round.Node.Name(), "Could not verify responses..")
		return err
	}
	out.R_hat = round.Cosi.R_hat
	return nil
}

func (round *RoundCosi) SignatureBroadcast(in *SignatureBroadcastMessage, out []*SignatureBroadcastMessage) error {
	// Root is creating the sig broadcast
	if round.IsRoot {
		in.R0_hat = round.Cosi.R_hat
		in.C = round.Cosi.C
		in.X0_hat = round.Cosi.X_hat
		in.V0_hat = round.Cosi.Log.V_hat
	}
	// Inform all children of broadcast  - just copy the one that came in
	for i := range out {
		out[i] = in
	}
	return nil
}
