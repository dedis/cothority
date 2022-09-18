package ots

import (
	"crypto/sha256"
	"errors"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessages()
}

func (w *Write) VerifyWrite(suite suites.Suite, darcID darc.ID) error {
	hash := sha256.New()
	hash.Write(darcID)
	// TODO: Check if this is safe
	h := suite.Point().Pick(suite.XOF(hash.Sum(nil)))
	_, validShares, err := pvss.VerifyEncShareBatch(suite, h, w.Publics,
		w.Proofs, w.Shares)
	if err != nil {
		return err
	}
	if len(validShares) < len(w.Shares) {
		return errors.New("cannot verify encrypted shares: invalid shares")
	}
	return nil
}
