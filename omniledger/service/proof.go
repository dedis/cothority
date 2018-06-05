package service

import (
	"bytes"
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// Proof represents everything necessary to verify a given
// key/value pair is stored in a skipchain. The proof is in three parts:
//   1. InclusionProof proofs the presence or absence of the key. In case of
//   the key being present, the value is included in the proof
//   2. Latest is used to verify the merkle tree root used in the collection-proof
//   is stored in the latest skipblock
//   3. Links proves that the latest skipblock is part of the skipchain
//
// This Structure could later be moved to cothority/skipchain.
type Proof struct {
	// InclusionProof is the deserialized InclusionProof
	InclusionProof collection.Proof
	// Providing the latest skipblock to retrieve the Merkle tree root.
	Latest skipchain.SkipBlock
	// Proving the path to the latest skipblock. The first ForwardLink has an
	// empty-sliced `From` and the genesis-block in `To`, together with the
	// roster of the genesis-block in the `NewRoster`.
	Links []skipchain.ForwardLink
}

// NewProof creates a proof for key in the skipchain with the given id. It uses
// the collectionDB to look up the key and the skipblockdb to create the correct
// proof for the forward links.
func NewProof(c *collectionDB, s *skipchain.SkipBlockDB, id skipchain.SkipBlockID,
	key []byte) (p *Proof, err error) {
	p = &Proof{}
	p.InclusionProof, err = c.coll.Get(key).Proof()
	if err != nil {
		return
	}
	sb := s.GetByID(id)
	if sb == nil {
		return nil, errors.New("didn't find skipchain")
	}
	p.Links = []skipchain.ForwardLink{{
		From:      []byte{},
		To:        id,
		NewRoster: sb.Roster,
	}}
	for len(sb.ForwardLink) > 0 {
		link := sb.ForwardLink[len(sb.ForwardLink)-1]
		p.Links = append(p.Links, *link)
		sb = s.GetByID(link.To)
		if sb == nil {
			return nil, errors.New("missing block in chain")
		}
	}
	p.Latest = *sb
	// p.ProofBytes = p.proof.Consistent()
	return
}

// ErrorVerifyCollection is returned if the collection-proof itself
// is not properly set up.
var ErrorVerifyCollection = errors.New("collection inclusion proof is wrong")

// ErrorVerifyCollectionRoot is returned if the root of the collection
// is different than the stored value in the skipblock.
var ErrorVerifyCollectionRoot = errors.New("root of collection is not in skipblock")

// ErrorVerifySkipchain is returned if the stored skipblock doesn't
// have a proper proof that it comes from the genesis block.
var ErrorVerifySkipchain = errors.New("stored skipblock is not properly evolved from genesis block")

// Verify takes a skipchain id and verifies that the proof is valid for this skipchain.
// It verifies the collection-proof, that the merkle-root is stored in the skipblock
// of the proof and the fact that the skipblock is indeed part of the skipchain.
// If all verifications are correct, the error will be nil.
func (p Proof) Verify(scID skipchain.SkipBlockID) error {
	if !p.InclusionProof.Consistent() {
		return ErrorVerifyCollection
	}
	_, d, err := network.Unmarshal(p.Latest.Data, cothority.Suite)
	if err != nil {
		return err
	}
	if !bytes.Equal(p.InclusionProof.TreeRootHash(), d.(*DataHeader).CollectionRoot) {
		return ErrorVerifyCollectionRoot
	}
	var sbID skipchain.SkipBlockID
	var publics []kyber.Point
	for i, l := range p.Links {
		if i == 0 {
			// The first forward link is a pointer from []byte{} to the genesis
			// block and holds the roster of the genesis block.
			sbID = scID
			publics = l.NewRoster.Publics()
			continue
		}
		if err = l.Verify(cothority.Suite, publics); err != nil {
			return ErrorVerifySkipchain
		}
		if !l.From.Equal(sbID) {
			return ErrorVerifySkipchain
		}
		sbID = l.To
		if l.NewRoster != nil {
			publics = l.NewRoster.Publics()
		}
	}
	return nil
}

// KeyValue returns the key and the values stored in the proof.
func (p Proof) KeyValue() (key []byte, values [][]byte, err error) {
	key = p.InclusionProof.Key
	values, err = p.InclusionProof.RawValues()
	return
}
