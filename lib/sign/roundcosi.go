package sign

import (
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"errors"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/crypto/abstract"
	"fmt"
)

// The name type of this round implementation
const RoundCosiType = "cosi"

type RoundCosi struct {
	*RoundStruct

	Cosi *CosiStrut
	Node *Node
}

func init() {
	RegisterRoundFactory(RoundCosiType,
		func(s *Node) Round {
			return NewRoundCosi(s)
		})
}

func NewRoundCosi(node *Node) *RoundCosi {
	round := &RoundCosi{}
	round.Node = node
	round.RoundStruct = NewRoundStruct(round.Node)
	return round
}

// AnnounceFunc will keep the timestamp generated for this round
func (round *RoundCosi) Announcement(viewNbr, roundNbr int, in *SigningMessage, out []*SigningMessage) error {
	if err := round.Node.TryFailure(round.Node.ViewNo, roundNbr); err != nil {
		return err
	}

	// Store the message for the round
	//round.Merkle = round.Node.MerkleStructs[roundNbr]
	round.Cosi = NewCosi(round.Node, viewNbr, roundNbr, in.Am)
	round.Cosi.Msg = in.Am.Message

	round.SetRoundType(RoundCosiType, out)
	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		*out[i].Am = *in.Am
	}
	return nil
}

func (round *RoundCosi) Commitment(in []*SigningMessage, out *SigningMessage) error {
	// prepare to handle exceptions
	cosi := round.Cosi
	cosi.Commits = in
	cosi.ExceptionList = make([]abstract.Point, 0)

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	children := cosi.Children
	cosi.ChildV_hat = make(map[string]abstract.Point, len(children))
	// X for public key
	cosi.ChildX_hat = make(map[string]abstract.Point, len(children))

	for key := range children {
		cosi.ChildX_hat[key] = cosi.Suite.Point().Null()
		cosi.ChildV_hat[key] = cosi.Suite.Point().Null()
	}

	// Commits from children are the first Merkle Tree leaves for the round
	cosi.Leaves = make([]hashid.HashId, 0)
	cosi.LeavesFrom = make([]string, 0)
	for _, sm := range cosi.Commits {
		from := sm.From
		// MTR ==> root of sub-merkle tree
		cosi.Leaves = append(cosi.Leaves, sm.Com.MTRoot)
		cosi.LeavesFrom = append(cosi.LeavesFrom, from)
		cosi.ChildV_hat[from] = sm.Com.V_hat
		cosi.ChildX_hat[from] = sm.Com.X_hat
		cosi.ExceptionList = append(cosi.ExceptionList, sm.Com.ExceptionList...)

		// Aggregation
		// add good child server to combined public key, and point commit
		cosi.Add(cosi.X_hat, sm.Com.X_hat)
		cosi.Add(cosi.Log.V_hat, sm.Com.V_hat)
		//dbg.Lvl4("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	dbg.Lvl4("Node.Commit using Merkle")
	cosi.MerkleAddChildren()

	round.Cosi.MerkleAddLocal(out.Com.MTRoot)
	round.Cosi.MerkleHashLog()
	round.Cosi.ComputeCombinedMerkleRoot()

	out.Com.V = round.Cosi.Log.V
	out.Com.V_hat = round.Cosi.Log.V_hat
	out.Com.X_hat = round.Cosi.X_hat
	out.Com.MTRoot = round.Cosi.MTRoot
	out.Com.ExceptionList = round.Cosi.ExceptionList
	out.Com.Messages = round.Node.Messages

	// Reset message counter for statistics
	round.Node.Messages = 0
	return nil

}

func (round *RoundCosi) Challenge(in *SigningMessage, out []*SigningMessage) error {

	cosi := round.Cosi
	// we are root
	if round.IsRoot {
		msg := cosi.Msg
		msg = append(msg, []byte(cosi.MTRoot)...)
		cosi.C = HashElGamal(cosi.Suite, msg, cosi.Log.V_hat)
		//proof := make([]hashid.HashId, 0)

		in.Chm.C = cosi.C
		in.Chm.MTRoot = cosi.MTRoot
		in.Chm.Proof = cosi.Proof
	} else { // we are a leaf
		// register challenge
		cosi.C = in.Chm.C
	}
	// compute response share already + localmerkle proof
	cosi.InitResponseCrypto()
	// messages from clients, proofs computed
	if cosi.Log.Getv() != nil {
		if err := cosi.StoreLocalMerkleProof(in.Chm); err != nil {
			return err
		}
	}

	// proof from big root to our root will be sent to all children
	baseProof := make(proof.Proof, len(in.Chm.Proof))
	copy(baseProof, in.Chm.Proof)

	if len(cosi.Children) != len(out) {
		return fmt.Errorf("Children and output are of different length")
	}
	// for each child, create personalized part of proof
	// embed it in SigningMessage, and send it
	var i = 0
	for name, _ := range cosi.Children {
		out[i].Chm.C = in.Chm.C
		out[i].Chm.MTRoot = in.Chm.MTRoot
		out[i].Chm.Proof = append(baseProof, cosi.Proofs[name]...)
		out[i].To = name
		i++
	}
	return nil
}

// TODO make that sms == nil in case we are a leaf to stay consistent with
// others calls
func (round *RoundCosi) Response(sms []*SigningMessage, out *SigningMessage) error {
	// initialize exception handling
	exceptionV_hat := round.Cosi.Suite.Point().Null()
	exceptionX_hat := round.Cosi.Suite.Point().Null()
	round.Cosi.ExceptionList = make([]abstract.Point, 0)
	nullPoint := round.Cosi.Suite.Point().Null()

	children := round.Cosi.Children
	for _, sm := range sms {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			// dbg.Lvl4(sn.Name(), "default in respose for child", from, sm)
			if children[from] != nil {
				round.Cosi.ExceptionList = append(round.Cosi.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				round.Cosi.Add(exceptionX_hat, round.Cosi.ChildX_hat[from])
				round.Cosi.Add(exceptionV_hat, round.Cosi.ChildV_hat[from])
			}
			continue
		case Response:
			// disregard response from children who did not commit
			_, ok := round.Cosi.ChildV_hat[from]
			if ok == true && round.Cosi.ChildV_hat[from].Equal(nullPoint) {
				continue
			}

			// dbg.Lvl4(sn.Name(), "accepts response from", from, sm.Type)
			round.Cosi.R_hat.Add(round.Cosi.R_hat, sm.Rm.R_hat)

			round.Cosi.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)

			round.Cosi.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			round.Cosi.ExceptionList = append(round.Cosi.ExceptionList, sm.Rm.ExceptionList...)

		case Error:
			if sm.Err == nil {
				dbg.Lvl2("Error message with no error")
				continue
			}

			// Report up non-networking error, probably signature failure
			dbg.Lvl2(round.Cosi.Name, "Error in respose for child", from, sm)
			err := errors.New(sm.Err.Err)
			return err
		}
	}

	// remove exceptions from subtree that failed
	round.Cosi.Sub(round.Cosi.X_hat, exceptionX_hat)
	round.Cosi.ExceptionV_hat = exceptionV_hat
	round.Cosi.ExceptionX_hat = exceptionX_hat

	dbg.Lvl4(round.Cosi.Name, "got all responses")
	err := round.Cosi.VerifyResponses()
	if err != nil {
		dbg.Lvl3(round.Node.Name(), "Could not verify responses..")
		return err
	}

	out.Rm.R_hat = round.Cosi.R_hat
	out.Rm.ExceptionList = round.Cosi.ExceptionList
	out.Rm.ExceptionV_hat = round.Cosi.ExceptionV_hat
	out.Rm.ExceptionX_hat = round.Cosi.ExceptionX_hat
	return nil
}

func (round *RoundCosi) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	// Root is creating the sig broadcast
	if round.IsRoot {
		dbg.Lvl2(round.Node.Name(), ": sending number of messages:", round.Node.Messages)
		in.SBm.R0_hat = round.Cosi.R_hat
		in.SBm.C = round.Cosi.C
		in.SBm.X0_hat = round.Cosi.X_hat
		in.SBm.V0_hat = round.Cosi.Log.V_hat
		in.SBm.Messages = round.Node.Messages
	} else {
		round.Node.Messages = in.SBm.Messages
	}
	// Inform all children of broadcast  - just copy the one that came in
	for i := range out {
		*out[i].SBm = *in.SBm
	}
	return nil
}
