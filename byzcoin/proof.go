package byzcoin

import (
	"bytes"
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// NewProof creates a proof for key in the skipchain with the given id. It uses
// a StateTrie to look up the key and the skipblockdb to create the correct
// proof for the forward links.
func NewProof(c ReadOnlyStateTrie, s *skipchain.SkipBlockDB, id skipchain.SkipBlockID,
	key []byte) (p *Proof, err error) {
	p = &Proof{}
	pr, err := c.GetProof(key)
	if err != nil {
		return
	}
	p.InclusionProof = *pr
	sb := s.GetByID(id)
	if sb == nil {
		return nil, errors.New("didn't find skipchain")
	}
	p.Links = []skipchain.ForwardLink{{
		From:      []byte{},
		To:        id,
		NewRoster: sb.Roster,
	}}
	for len(sb.ForwardLink) > 0 && sb.Index < c.GetIndex() {
		var link *skipchain.ForwardLink
		// Corner-case when the database is downloading blocks and a proof is
		// requested before all blocks are stored - then we need to make sure that
		// we don't get the latest block, but the block corresponding to the
		// StateTrie.Index
		for height := len(sb.ForwardLink) - 1; height >= 0; height-- {
			link = sb.ForwardLink[height]
			sbTemp := s.GetByID(link.To)
			if sbTemp == nil {
				return nil, errors.New("missing block in chain")
			}
			if sbTemp.Index <= c.GetIndex() {
				sb = sbTemp
				break
			}
		}
		p.Links = append(p.Links, *link)
	}
	p.Latest = *sb
	return
}

// ErrorVerifyTrie is returned if the proof itself is not properly set up.
var ErrorVerifyTrie = errors.New("trie inclusion proof is wrong")

// ErrorVerifyTrieRoot is returned if the root of the trie is different than
// the stored value in the skipblock.
var ErrorVerifyTrieRoot = errors.New("root of trie is not in skipblock")

// ErrorVerifySkipchain is returned if the stored skipblock doesn't
// have a proper proof that it comes from the genesis block.
var ErrorVerifySkipchain = errors.New("stored skipblock is not properly evolved from genesis block")

// Verify takes a skipchain id and verifies that the proof is valid for this
// skipchain. It verifies the proof, that the merkle-root is stored in the
// skipblock of the proof and the fact that the skipblock is indeed part of the
// skipchain. If all verifications are correct, the error will be nil. It does
// not verify whether a certain key/value pair exists in the proof.
func (p Proof) Verify(scID skipchain.SkipBlockID) error {
	var header DataHeader
	err := protobuf.DecodeWithConstructors(p.Latest.Data, &header, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return err
	}
	if !bytes.Equal(p.InclusionProof.GetRoot(), header.TrieRoot) {
		return ErrorVerifyTrieRoot
	}
	var sbID skipchain.SkipBlockID
	var publics []kyber.Point
	for i, l := range p.Links {
		if i == 0 {
			// The first forward link is a pointer from []byte{} to the genesis
			// block and holds the roster of the genesis block.
			sbID = scID
			publics = l.NewRoster.ServicePublics(skipchain.ServiceName)
			continue
		}
		if err = l.Verify(pairing.NewSuiteBn256(), publics); err != nil {
			return ErrorVerifySkipchain
		}
		if !l.From.Equal(sbID) {
			return ErrorVerifySkipchain
		}
		sbID = l.To
		if l.NewRoster != nil {
			publics = l.NewRoster.ServicePublics(skipchain.ServiceName)
		}
	}
	return nil
}

// KeyValue returns the key and the values stored in the proof. The caller
// should check both the key and the value because it should not trust the
// service to always return a key/value pair (via the proof) that corresponds
// to the request.
func (p Proof) KeyValue() (key []byte, value []byte, contractID string, darcID darc.ID, err error) {
	k, vals := p.InclusionProof.KeyValue()
	if len(k) == 0 {
		err = errors.New("empty key")
		return
	}
	if len(vals) == 0 {
		err = errors.New("no value")
		return
	}
	var s StateChangeBody
	s, err = decodeStateChangeBody(vals)
	if err != nil {
		return
	}
	key = k
	value = s.Value
	contractID = string(s.ContractID)
	darcID = s.DarcID
	return
}

// Get returns the values associated with the given key. If the key is not in
// the proof, then an error is returned.
func (p Proof) Get(k []byte) (value []byte, contractID string, darcID darc.ID, err error) {
	vals := p.InclusionProof.Get(k)
	if len(vals) == 0 {
		err = errors.New("no value")
		return
	}
	var s StateChangeBody
	s, err = decodeStateChangeBody(vals)
	if err != nil {
		return
	}
	value = s.Value
	contractID = string(s.ContractID)
	darcID = s.DarcID
	return
}

// VerifyAndDecode verifies the contractID of the proof and tries to
// protobuf-decode the value to the given interface. It takes as an input the
// ContractID the instance should be a part of and a pre-allocated structure
// where the data of the instance is decoded into. It returns an error if the
// instance is not of type cid or if the decoding failed.
func (p Proof) VerifyAndDecode(suite network.Suite, cid string, value interface{}) error {
	_, buf, contractID, _, err := p.KeyValue()
	if err != nil {
		return err
	}
	if contractID != cid {
		return errors.New("not an instance of this contract")
	}
	return protobuf.DecodeWithConstructors(buf, value, network.DefaultConstructors(suite))
}
