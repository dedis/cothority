package sign
import (
	"github.com/dedis/cothority/lib/hashid"
	"sort"
	"github.com/dedis/cothority/lib/proof"
	"bytes"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

/*
 * This is a module for the round-struct that does all the
 * calculation for a merkle-hash-tree.
 */

// Adds a child-node to the Merkle-tree and updates the root-hashes
func (round *Round)MerkleAddChildren() {
	// children commit roots
	round.CMTRoots = make([]hashid.HashId, len(round.Leaves))
	copy(round.CMTRoots, round.Leaves)
	round.CMTRootNames = make([]string, len(round.Leaves))
	copy(round.CMTRootNames, round.LeavesFrom)

	// concatenate children commit roots in one binary blob for easy marshalling
	round.Log.CMTRoots = make([]byte, 0)
	for _, leaf := range round.Leaves {
		round.Log.CMTRoots = append(round.Log.CMTRoots, leaf...)
	}
}

// Adds the local Merkle-tree root, usually from a stamper or
// such
func (round *Round)MerkleAddLocal(localMTroot hashid.HashId) {
	// add own local mtroot to leaves
	round.LocalMTRoot = localMTroot
	round.Leaves = append(round.Leaves, round.LocalMTRoot)
}

// Hashes the log of the round-structure
func (round *Round)MerkleHashLog() error {
	var err error

	h := round.Suite.Hash()
	logBytes, err := round.Log.MarshalBinary()
	if err != nil {
		return err
	}
	h.Write(logBytes)
	round.HashedLog = h.Sum(nil)
	return err
}


func (round *Round) ComputeCombinedMerkleRoot() {
	// add hash of whole log to leaves
	round.Leaves = append(round.Leaves, round.HashedLog)

	// compute MT root based on Log as right child and
	// MT of leaves as left child and send it up to parent
	sort.Sort(hashid.ByHashId(round.Leaves))
	left, proofs := proof.ProofTree(round.Suite.Hash, round.Leaves)
	right := round.HashedLog
	moreLeaves := make([]hashid.HashId, 0)
	moreLeaves = append(moreLeaves, left, right)
	round.MTRoot, _ = proof.ProofTree(round.Suite.Hash, moreLeaves)

	// Hashed Log has to come first in the proof; len(sn.CMTRoots)+1 proofs
	round.Proofs = make(map[string]proof.Proof, 0)
	for name := range round.Children {
		round.Proofs[name] = append(round.Proofs[name], right)
	}
	round.Proofs["local"] = append(round.Proofs["local"], right)

	// separate proofs by children (need to send personalized proofs to children)
	// also separate local proof (need to send it to timestamp server)
	round.SeparateProofs(proofs, round.Leaves)
}

// Identify which proof corresponds to which leaf
// Needed given that the leaves are sorted before passed to the function that create
// the Merkle Tree and its Proofs
func (round *Round) SeparateProofs(proofs []proof.Proof, leaves []hashid.HashId) {
	// separate proofs for children servers mt roots
	for i := 0; i < len(round.CMTRoots); i++ {
		name := round.CMTRootNames[i]
		for j := 0; j < len(leaves); j++ {
			if bytes.Compare(round.CMTRoots[i], leaves[j]) == 0 {
				// sn.Proofs[i] = append(sn.Proofs[i], proofs[j]...)
				round.Proofs[name] = append(round.Proofs[name], proofs[j]...)
				continue
			}
		}
	}

	// separate proof for local mt root
	for j := 0; j < len(leaves); j++ {
		if bytes.Compare(round.LocalMTRoot, leaves[j]) == 0 {
			round.Proofs["local"] = append(round.Proofs["local"], proofs[j]...)
		}
	}
}

func (round *Round) InitResponseCrypto() {
	round.R = round.Suite.Secret()
	round.R.Mul(round.PrivKey, round.C).Sub(round.Log.v, round.R)
	// initialize sum of children's responses
	round.r_hat = round.R
}

// Create Merkle Proof for local client (timestamp server) and
// store it in Node so that we can send it to the clients during
// the SignatureBroadcast
func (round *Round) StoreLocalMerkleProof(chm *ChallengeMessage) error {
	proofForClient := make(proof.Proof, len(chm.Proof))
	copy(proofForClient, chm.Proof)

	// To the proof from our root to big root we must add the separated proof
	// from the localMKT of the client (timestamp server) to our root
	proofForClient = append(proofForClient, round.Proofs["local"]...)

	// if want to verify partial and full proofs
	if dbg.DebugVisible > 2 {
		//sn.VerifyAllProofs(view, chm, proofForClient)
	}
	round.Proof = proofForClient
	round.MTRoot = chm.MTRoot
	return nil
}

