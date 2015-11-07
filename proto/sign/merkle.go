package sign

// Functions used in collective signing
// That are direclty related to the generation/ verification/ sending
// of the Merkle Tree Signature

import (
	"strconv"

	//log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
)


// Create Merkle Proof for local client (timestamp server) and
// store it in Node so that we can send it to the clients during
// the SignatureBroadcast
func (sn *Node) StoreLocalMerkleProof(view int, chm *ChallengeMessage) error {
	if sn.Callbacks != nil {
		sn.roundLock.RLock()
		round := sn.Rounds[chm.RoundNbr]
		sn.roundLock.RUnlock()
		proofForClient := make(proof.Proof, len(chm.Proof))
		copy(proofForClient, chm.Proof)

		// To the proof from our root to big root we must add the separated proof
		// from the localMKT of the client (timestamp server) to our root
		proofForClient = append(proofForClient, round.Proofs["local"]...)

		// if want to verify partial and full proofs
		if dbg.DebugVisible > 2{
			sn.VerifyAllProofs(view, chm, proofForClient)
		}
		sn.Proof = proofForClient
		sn.MTRoot = chm.MTRoot
	}

	return nil
}

// Create Personalized Merkle Proofs for children servers
// Send Personalized Merkle Proofs to children servers
func (sn *Node) SendChildrenChallengesProofs(view int, chm *ChallengeMessage) error {
	round := sn.Rounds[chm.RoundNbr]
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
		// dbg.Lvl4("connection: sending children challenge proofs:", name, conn)
		if err := conn.PutData(messg); err != nil {
			return err
		}
	}

	return nil
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
		dbg.Lvl4("Chidlren Proofs of", sn.Name(), "successful for round "+strconv.Itoa(sn.nRounds))
	} else {
		panic("Children Proofs" + sn.Name() + " unsuccessful for round " + strconv.Itoa(sn.nRounds))
	}
}

func (sn *Node) VerifyAllProofs(view int, chm *ChallengeMessage, proofForClient proof.Proof) {
	sn.roundLock.RLock()
	round := sn.Rounds[chm.RoundNbr]
	sn.roundLock.RUnlock()
	// proof from client to my root
	proof.CheckProof(sn.Suite().Hash, round.MTRoot, round.LocalMTRoot, round.Proofs["local"])
	// proof from my root to big root
	dbg.Lvl4(sn.Name(), "verifying for view", view)
	proof.CheckProof(sn.Suite().Hash, chm.MTRoot, round.MTRoot, chm.Proof)
	// proof from client to big root
	proof.CheckProof(sn.Suite().Hash, chm.MTRoot, round.LocalMTRoot, proofForClient)
}
