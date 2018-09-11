package service

import (
	"bytes"
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// NewProof creates a proof for key in the skipchain with the given id. It uses
// the collectionDB to look up the key and the skipblockdb to create the correct
// proof for the forward links.
func NewProof(c CollectionView, s *skipchain.SkipBlockDB, id skipchain.SkipBlockID,
	key []byte) (p *Proof, err error) {
	p = &Proof{}
	p.InclusionProof, err = c.Get(key).Proof()
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
	var header DataHeader
	err := protobuf.DecodeWithConstructors(p.Latest.Data, &header, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return err
	}
	if !bytes.Equal(p.InclusionProof.TreeRootHash(), header.CollectionRoot) {
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

// ContractValue verifies the contractID of the proof and tries to
// protobuf-decode the value to the given interface. It takes as an input
// the ContractID the instance should be a part of and a pre-allocated
// structure where the data of the instance is decoded into.
// It returns an error if the instance is not of type cid or if the
// decoding failed.
func (p *Proof) ContractValue(suite network.Suite, cid string, value interface{}) error {
	values, err := p.InclusionProof.RawValues()
	if err != nil {
		return err
	}
	if bytes.Compare(values[1], []byte(cid)) != 0 {
		return errors.New("not an instance of this contract")
	}
	return protobuf.DecodeWithConstructors(values[0], value, network.DefaultConstructors(suite))
}
