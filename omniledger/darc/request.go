package darc

import (
	"crypto/sha256"
	"errors"
)

// Hash computes the digest of the request, the identities and signatures are
// not included.
func (r *Request) Hash() []byte {
	return r.innerRequest.Hash()
}

func (r innerRequest) Hash() []byte {
	h := sha256.New()
	h.Write(r.BaseID)
	h.Write([]byte(r.Action))
	h.Write(r.Msg)
	for _, i := range r.Identities {
		h.Write([]byte(i.String()))
	}
	return h.Sum(nil)
}

// GetIdentityStrings returns a slice of identity strings, this is useful for
// creating a parser.
func (r *Request) GetIdentityStrings() []string {
	res := make([]string, len(r.Identities))
	for i, id := range r.Identities {
		res[i] = id.String()
	}
	return res
}

// MsgToDarc attempts to return a darc given the matching darcBuf. This
// function should *not* be used as a way to verify the darc, it only checks
// that darcBuf can be decoded and matches with the Msg part of the request.
func (r *Request) MsgToDarc(darcBuf []byte) (*Darc, error) {
	d, err := NewDarcFromProto(darcBuf)
	if err != nil {
		return nil, err
	}

	if !d.GetID().Equal(r.Msg) {
		return nil, errors.New("darc IDs are not equal")
	}

	if len(r.Signatures) != len(r.Identities) {
		return nil, errors.New("signature and identitity length mismatch")
	}
	darcSigs := make([]*Signature, len(r.Signatures))
	for i := range r.Signatures {
		darcSigs[i] = &Signature{
			Signature: r.Signatures[i],
			Signer:    *r.Identities[i],
		}
	}
	d.Signatures = darcSigs

	return d, nil
}

// InitRequest initialises a request, the caller must provide all the fields of
// the request. There is no guarantee that this request is valid, please see
// InitAndSignRequest is a valid request needs to be created.
func InitRequest(baseID ID, action Action, msg []byte, ids []*Identity, sigs [][]byte) Request {
	inner := innerRequest{
		BaseID:     baseID,
		Action:     action,
		Msg:        msg,
		Identities: ids,
	}
	return Request{
		inner,
		sigs,
	}
}

// InitAndSignRequest creates a new request which can be verified by a Darc.
func InitAndSignRequest(baseID ID, action Action, msg []byte, signers ...*Signer) (*Request, error) {
	if len(signers) == 0 {
		return nil, errors.New("there are no signers")
	}
	signerIDs := make([]*Identity, len(signers))
	for i, s := range signers {
		signerIDs[i] = s.Identity()
	}
	inner := innerRequest{
		BaseID:     baseID,
		Action:     action,
		Msg:        msg,
		Identities: signerIDs,
	}
	digest := inner.Hash()
	sigs := make([][]byte, len(signers))
	for i, s := range signers {
		var err error
		sigs[i], err = s.Sign(digest)
		if err != nil {
			return nil, err
		}
	}
	return &Request{
		inner,
		sigs,
	}, nil
}
