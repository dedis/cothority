package sign

// Functions used in collective signing
// That are direclty related to the generation/ verification/ sending
// of the Merkle Tree Signature

import (
	"bytes"
	"sort"
	"strconv"

	//log "github.com/Sirupsen/logrus"
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"

	"github.com/ineiti/cothorities/helpers/coconet"
	"github.com/ineiti/cothorities/helpers/hashid"
	"github.com/ineiti/cothorities/helpers/proof"
)

func (sn *Node) AddChildrenMerkleRoots(Round int) {
	sn.roundLock.RLock()
	round := sn.Rounds[Round]
	sn.roundLock.RUnlock()
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

func (sn *Node) AddLocalMerkleRoot(view, Round int) {
	sn.roundLock.RLock()
	round := sn.Rounds[Round]
	sn.roundLock.RUnlock()
	// add own local mtroot to leaves
	if sn.CommitFunc != nil {
		round.LocalMTRoot = sn.CommitFunc(view)
	} else {
		round.LocalMTRoot = make([]byte, hashid.Size)
	}
	round.Leaves = append(round.Leaves, round.LocalMTRoot)
}

func (sn *Node) ComputeCombinedMerkleRoot(view, Round int) {
	sn.roundLock.RLock()
	round := sn.Rounds[Round]
	sn.roundLock.RUnlock()
	// add hash of whole log to leaves
	round.Leaves = append(round.Leaves, round.HashedLog)

	// compute MT root based on Log as right child and
	// MT of leaves as left child and send it up to parent
	sort.Sort(hashid.ByHashId(round.Leaves))
	left, proofs := proof.ProofTree(sn.Suite().Hash, round.Leaves)
	right := round.HashedLog
	moreLeaves := make([]hashid.HashId, 0)
	moreLeaves = append(moreLeaves, left, right)
	round.MTRoot, _ = proof.ProofTree(sn.Suite().Hash, moreLeaves)

	// Hashed Log has to come first in the proof; len(sn.CMTRoots)+1 proofs
	round.Proofs = make(map[string]proof.Proof, 0)
	children := sn.Children(view)
	for name := range children {
		round.Proofs[name] = append(round.Proofs[name], right)
	}
	round.Proofs["local"] = append(round.Proofs["local"], right)

	// separate proofs by children (need to send personalized proofs to children)
	// also separate local proof (need to send it to timestamp server)
	sn.SeparateProofs(proofs, round.Leaves, Round)
}

// Create Merkle Proof for local client (timestamp server)
// Send Merkle Proof to local client (timestamp server)
func (sn *Node) SendLocalMerkleProof(view int, chm *ChallengeMessage) error {
	if sn.DoneFunc != nil {
		sn.roundLock.RLock()
		round := sn.Rounds[chm.Round]
		sn.roundLock.RUnlock()
		proofForClient := make(proof.Proof, len(chm.Proof))
		copy(proofForClient, chm.Proof)

		// To the proof from our root to big root we must add the separated proof
		// from the localMKT of the client (timestamp server) to our root
		proofForClient = append(proofForClient, round.Proofs["local"]...)

		// if want to verify partial and full proofs
		// dbg.Lvl3("*****")
		// dbg.Lvl3(sn.Name(), chm.Round, proofForClient)
		if DEBUG == true {
			sn.VerifyAllProofs(view, chm, proofForClient)
		}

		// 'reply' to client
		// TODO: add error to done function
		sn.DoneFunc(view, chm.MTRoot, round.MTRoot, proofForClient)
	}

	return nil
}

// Create Personalized Merkle Proofs for children servers
// Send Personalized Merkle Proofs to children servers
func (sn *Node) SendChildrenChallengesProofs(view int, chm *ChallengeMessage) error {
	round := sn.Rounds[chm.Round]
	// proof from big root to our root will be sent to all children
	baseProof := make(proof.Proof, len(chm.Proof))
	copy(baseProof, chm.Proof)

	// for each child, create personalized part of proof
	// embed it in SigningMessage, and send it
	for name, conn := range sn.Children(view) {
		newChm := *chm
		newChm.Proof = append(baseProof, round.Proofs[name]...)

		var messg coconet.BinaryMarshaler
		messg = &SigningMessage{View: view, Type: Challenge, Chm: &newChm}

		// send challenge message to child
		// dbg.Lvl3("connection: sending children challenge proofs:", name, conn)
		if err := conn.Put(messg); err != nil {
			return err
		}
	}

	return nil
}

// Identify which proof corresponds to which leaf
// Needed given that the leaves are sorted before passed to the function that create
// the Merkle Tree and its Proofs
func (sn *Node) SeparateProofs(proofs []proof.Proof, leaves []hashid.HashId, Round int) {
	sn.roundLock.RLock()
	round := sn.Rounds[Round]
	sn.roundLock.RUnlock()
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

// Check that starting from its own committed message each child can reach our subtrees' mtroot
// Also checks that starting from local mt root we can get to  our subtrees' mtroot <-- could be in diff fct
func (sn *Node) checkChildrenProofs(Round int) {
	sn.roundLock.RLock()
	round := sn.Rounds[Round]
	sn.roundLock.RUnlock()
	cmtAndLocal := make([]hashid.HashId, len(round.CMTRoots))
	copy(cmtAndLocal, round.CMTRoots)
	cmtAndLocal = append(cmtAndLocal, round.LocalMTRoot)

	proofs := make([]proof.Proof, 0)
	for _, name := range round.CMTRootNames {
		proofs = append(proofs, round.Proofs[name])
	}

	if proof.CheckLocalProofs(sn.Suite().Hash, round.MTRoot, cmtAndLocal, proofs) == true {
		dbg.Lvl3("Chidlren Proofs of", sn.Name(), "successful for round "+strconv.Itoa(sn.nRounds))
	} else {
		panic("Children Proofs" + sn.Name() + " unsuccessful for round " + strconv.Itoa(sn.nRounds))
	}
}

func (sn *Node) VerifyAllProofs(view int, chm *ChallengeMessage, proofForClient proof.Proof) {
	sn.roundLock.RLock()
	round := sn.Rounds[chm.Round]
	sn.roundLock.RUnlock()
	// proof from client to my root
	proof.CheckProof(sn.Suite().Hash, round.MTRoot, round.LocalMTRoot, round.Proofs["local"])
	// proof from my root to big root
	dbg.Lvl3(sn.Name(), "verifying for view", view)
	proof.CheckProof(sn.Suite().Hash, chm.MTRoot, round.MTRoot, chm.Proof)
	// proof from client to big root
	proof.CheckProof(sn.Suite().Hash, chm.MTRoot, round.LocalMTRoot, proofForClient)
}
