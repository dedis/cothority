package service

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
)

// Hash computes the digest of the hash function
func (tx Transaction) Hash() []byte {
	h := sha256.New()
	if tx.Action != 0 {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(tx.Action))
		h.Write(b)
	}
	h.Write(tx.Key)
	h.Write(tx.Kind)
	h.Write(tx.Value)
	return h.Sum(nil)
}

// ToDarcRequest converts the Transaction content into a darc.Request.
func (tx Transaction) ToDarcRequest() (*darc.Request, error) {
	if len(tx.Key) < darcIDLen {
		return nil, errors.New("incorrect transaction length")
	}
	baseID := tx.Key[0:32]
	action := string(tx.Kind)
	if tx.Action != 0 {
		action += "_" + tx.Action.String()
	}
	ids := make([]*darc.Identity, len(tx.Signatures))
	sigs := make([][]byte, len(tx.Signatures))
	for i, sig := range tx.Signatures {
		ids[i] = &sig.Signer
		sigs[i] = sig.Signature // TODO shallow copy is ok?
	}
	req := darc.NewRequest2(baseID, darc.Action(action), tx.Hash(), ids, sigs)
	return &req, nil
}
