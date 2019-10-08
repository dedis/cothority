package byzcoin

import (
	"bytes"
	"errors"

	"golang.org/x/xerrors"

	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// NewProof creates a proof for key in the skipchain with the given id. It uses
// a StateTrie to look up the key and the skipblockdb to create the correct
// proof for the forward links.
func NewProof(c ReadOnlyStateTrie, s *skipchain.SkipBlockDB, id skipchain.SkipBlockID,
	key []byte) (p *Proof, err error) {
	p = &Proof{}
	pr, err := c.GetProof(key)
	if err != nil {
		return nil, xerrors.Errorf("couldn't get proof: %+v", err)
	}
	p.InclusionProof = *pr
	sb := s.GetByID(id)
	if sb == nil {
		return nil, xerrors.New("didn't find skipchain")
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
				return nil, xerrors.New("missing block in chain")
			}
			if sbTemp.Index <= sb.Index {
				return nil, skipchain.ErrorInconsistentForwardLink
			}
			if sbTemp.Index <= c.GetIndex() {
				sb = sbTemp
				break
			}
		}
		p.Links = append(p.Links, *link)
	}
	if c.GetIndex() != sb.Index {
		return nil, xerrors.New("didn't find skipblock with same index as" +
			" state-trie")
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

// ErrorVerifyHash is returned if the latest block hash does not match
// the target of the last forward link.
var ErrorVerifyHash = errors.New("last forward link does not point to the latest block")

// ErrorMissingForwardLinks is returned if no forward-link is found
// in the proof.
var ErrorMissingForwardLinks = errors.New("missing forward-links")

// ErrorMalformedForwardLink is returned when the new roster is not defined in the link
// but is expected to be.
var ErrorMalformedForwardLink = errors.New("missing new roster from the forward-link")

// VerifyFromBlock takes a skipchain id and the first block of the proof. It
// verifies that the proof is valid for this skipchain. It verifies the proof,
// that the merkle-root is stored in the skipblock of the proof and the fact that
// the skipblock is indeed part of the skipchain. It also uses the provided block
// to insure the first roster is correct. If all verifications are correct, the error
// will be nil. It does not verify wether a certain key/value pair exists in the proof.
func (p Proof) VerifyFromBlock(verifiedBlock *skipchain.SkipBlock) error {
	if len(p.Links) > 0 {
		// Hash of the block has been verified previously so we can trust the roster
		// coming from it which should be the same. If not, the proof won't verified.
		p.Links[0].NewRoster = verifiedBlock.Roster
	}

	// The signature of the first link is not checked as we use it as
	// a synthetic link to provide the initial roster.
	return p.Verify(verifiedBlock.Hash)
}

// Verify takes a skipchain id and verifies that the proof is valid for this
// skipchain. It verifies the proof, that the merkle-root is stored in the
// skipblock of the proof and the fact that the skipblock is indeed part of the
// skipchain. If all verifications are correct, the error will be nil. It does
// not verify whether a certain key/value pair exists in the proof.
//
// Notice: this verification alone is not sufficient. The roster of the first link
// must be verified before. See Proof.VerifyFromBlock for example.
func (p Proof) Verify(sbID skipchain.SkipBlockID) error {
	err := p.VerifyInclusionProof(&p.Latest)
	if err != nil {
		return err
	}

	if len(p.Links) == 0 {
		return ErrorMissingForwardLinks
	}
	if p.Links[0].NewRoster == nil {
		return ErrorMalformedForwardLink
	}

	// Get the first from the synthetic link which is assumed to be verified
	// before against the block with ID stored in the To field by the caller.
	publics := p.Links[0].NewRoster.ServicePublics(skipchain.ServiceName)

	for _, l := range p.Links[1:] {
		if err = l.VerifyWithScheme(pairing.NewSuiteBn256(), publics, p.Latest.SignatureScheme); err != nil {
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

	// Check that the given latest block matches the last forward link target
	if !p.Latest.CalculateHash().Equal(sbID) {
		return ErrorVerifyHash
	}

	return nil
}

// VerifyInclusionProof verifies that the inclusion proof matches the skipblock
// given in parameter.
func (p Proof) VerifyInclusionProof(latest *skipchain.SkipBlock) error {
	var header DataHeader
	err := protobuf.Decode(latest.Data, &header)
	if err != nil {
		return err
	}
	if !bytes.Equal(p.InclusionProof.GetRoot(), header.TrieRoot) {
		return ErrorVerifyTrieRoot
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
