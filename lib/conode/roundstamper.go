package conode

import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"strconv"

	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"time"
	"bytes"
	"encoding/binary"
	"sort"
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/coconet"
	"log"
)

/*
Implements the basic Collective Signature using Schnorr signatures.
 */

const RoundStamperType = "cosi"

type RoundStamper struct {
	peer         *Peer
								 // Leaves, Root and Proof for a round
	Root         hashid.HashId
	Suite        abstract.Suite
								 // Timestamp message for this Round
	Timestamp    int64
	TimestampMsg []byte

								 // merkle tree roots of children in strict order
	CMTRoots     []hashid.HashId
	CMTRootNames []string
	Log          sign.SNLog      // round lasting log structure
	HashedLog    []byte
								 // own big merkle subtree
	MTRoot       hashid.HashId   // mt root for subtree, passed upwards
	Leaves       []hashid.HashId // leaves used to build the merkle subtre
	LeavesFrom   []string        // child names for leaves

								 // mtRoot before adding HashedLog
	LocalMTRoot  hashid.HashId

	Proofs       map[string]proof.Proof
	Proof        []hashid.HashId
}


func init() {
	sign.RegisterRoundFactory(RoundStamperType,
		func(s *sign.Node) sign.Round {
			return NewRoundStamper(s)
		})
}

func NewRoundStamper(node *sign.Node) *RoundStamper {
	rt := &RoundStamper{}
	rt.Suite = node.Suite()
	rt.Log.Suite = rt.Suite
	return rt
}

func (rt *RoundStamper) Announcement(round int, in *sign.SigningMessage,
out []*sign.SigningMessage) error {
	am := in.Am
	if am == nil {
		// Adding timestamp
		ts := time.Now().UTC()
		var b bytes.Buffer
		rt.Timestamp = ts.Unix()
		binary.Write(&b, binary.LittleEndian, ts.Unix())
		rt.TimestampMsg = b.Bytes()
		am = &sign.AnnouncementMessage{Message: rt.TimestampMsg,
			RoundType:RoundStamperType}
	} else {
		// otherwise decode it
		var t int64
		if err := binary.Read(bytes.NewBuffer(am.Message), binary.LittleEndian, &t); err != nil {
			dbg.Lvl1("Unmashaling timestamp has failed")
		}
		rt.TimestampMsg = am.Message
		rt.Timestamp = t
	}

	// Inform all children of announcement - just copy the one that came in
	for i := range out {
		out[i].Am = am
	}
	return nil
}

func (rt *RoundStamper) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	// Commits from children are the first Merkle Tree leaves for the round
	rt.Leaves = make([]hashid.HashId, 0)
	rt.LeavesFrom = make([]string, 0)
	for _, sm := range in {
		from := sm.From
		rt.Leaves = append(rt.Leaves, sm.Com.MTRoot)
		rt.LeavesFrom = append(rt.LeavesFrom, from)
	}

	rt.peer.Mux.Lock()
	// get data from s once to avoid refetching from structure
	Queue := rt.peer.Queue
	// messages read will now be processed
	rt.peer.Queue[READING], rt.peer.Queue[PROCESSING] = rt.peer.Queue[PROCESSING], rt.peer.Queue[READING]
	rt.peer.Queue[READING] = rt.peer.Queue[READING][:0]

	// give up if nothing to process
	if len(Queue[PROCESSING]) == 0 {
		rt.peer.Mux.Unlock()
		rt.Root = make([]byte, hashid.Size)
		rt.Proofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		rt.Leaves = make([]hashid.HashId, 0)
		for _, msg := range Queue[PROCESSING] {
			rt.Leaves = append(rt.Leaves, hashid.HashId(msg.Tsm.Sreq.Val))
		}
		rt.peer.Mux.Unlock()

		// create Merkle tree for this round's messages and check corectness
		rt.Root, rt.Proofs = proof.ProofTree(rt.peer.Suite.Hash, rt.Leaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(rt.peer.Suite.Hash, rt.Root, rt.Leaves, rt.Proofs) == true {
				dbg.Lvl4("Local Proofs of", rt.peer.Name(), "successful for round " + strconv.Itoa(int(rt.peer.LastRound())))
			} else {
				panic("Local Proofs" + rt.peer.Name() + " unsuccessful for round " + strconv.Itoa(int(rt.peer.LastRound())))
			}
		}
	}

	dbg.Lvl4("sign.Node.Commit using Merkle")
	rt.MerkleAddChildren()
	rt.MerkleAddLocal(rt.Root)
	rt.MerkleHashLog()
	rt.ComputeCombinedMerkleRoot(out.View)

	out.Com.MTRoot = rt.MTRoot

	return nil
}

func (rt *RoundStamper) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	msg := rt.TimestampMsg
	msg = append(msg, []byte(rt.MTRoot)...)
	in.Chm.MTRoot = msg
	in.Chm.Proof = rt.Proof
	// messages from clients, proofs computed
	if err := rt.StoreLocalMerkleProof(in.Chm); err != nil {
		return err
	}

	// Create Personalized Merkle Proofs for children servers
	// Send Personalized Merkle Proofs to children servers
	// proof from big root to our root will be sent to all children
	chm := in.Chm
	ViewNbr := in.View
	RoundNbr := in.RoundNbr
	baseProof := make(proof.Proof, len(chm.Proof))
	copy(baseProof, chm.Proof)

	// for each child, create personalized part of proof
	// embed it in SigningMessage, and send it
	for name, conn := range rt.peer.Node.Children(ViewNbr) {
		newChm := *chm
		newChm.Proof = append(baseProof, rt.Proofs[name]...)

		out[]
		messg = &sign.SigningMessage{View: ViewNbr, RoundNbr: RoundNbr,
			Type: sign.Challenge, Chm: &newChm}

		// send challenge message to child
		// dbg.Lvl4("connection: sending children challenge proofs:", name, conn)
		if err := conn.PutData(messg); err != nil {
			return err
		}
	}

	return nil
}

func (rt *RoundStamper) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (rt *RoundStamper) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	// Send back signature to clients
	rt.peer.Mux.Lock()
	sb := in.SBm
	for i, msg := range rt.peer.Queue[PROCESSING] {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(rt.Proof))
		copy(combProof, rt.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, rt.Proofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(rt.Suite.Hash, rt.MTRoot,
			rt.Leaves[i], combProof) {
			dbg.Lvl2("Proof is OK for msg", msg)
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: msg.Tsm.ReqNo,
			Srep: &StampSignature{
				SuiteStr:   rt.Suite.String(),
				Timestamp:  rt.Timestamp,
				MerkleRoot: rt.MTRoot,
				Prf:        combProof,
				Response:   sb.R0_hat,
				Challenge:  sb.C,
				AggCommit:  sb.V0_hat,
				AggPublic:  sb.X0_hat,
			}}
		rt.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client", msg.To)
	}
	rt.peer.Mux.Unlock()
	rt.Timestamp = 0
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
		dbg.Lvl1("%p error putting to client: %v", cs, err)
	}
}

// Adds a child-node to the Merkle-tree and updates the root-hashes
func (round *RoundStamper) MerkleAddChildren() {
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
func (round *RoundStamper) MerkleAddLocal(localMTroot hashid.HashId) {
	// add own local mtroot to leaves
	round.LocalMTRoot = localMTroot
	round.Leaves = append(round.Leaves, round.LocalMTRoot)
}

// Hashes the log of the round-structure
func (round *RoundStamper) MerkleHashLog() error {
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

func (round *RoundStamper) ComputeCombinedMerkleRoot(view int) {
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
	for name := range round.peer.Node.Children(view) {
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
func (round *RoundStamper) SeparateProofs(proofs []proof.Proof, leaves []hashid.HashId) {
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

// Create Merkle Proof for local client (timestamp server) and
// store it in Node so that we can send it to the clients during
// the SignatureBroadcast
func (round *RoundStamper) StoreLocalMerkleProof(chm *sign.ChallengeMessage) error {
	proofForClient := make(proof.Proof, len(chm.Proof))
	copy(proofForClient, chm.Proof)

	// To the proof from our root to big root we must add the separated proof
	// from the localMKT of the client (timestamp server) to our root
	proofForClient = append(proofForClient, round.Proofs["local"]...)

	// if want to verify partial and full proofs
	if dbg.DebugVisible > 2 {
		//round.sn.VerifyAllProofs(view, chm, proofForClient)
	}
	round.Proof = proofForClient
	round.MTRoot = chm.MTRoot
	return nil
}

