package pqots

import (
	"crypto/sha256"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

func init() {
	network.RegisterMessages(VerifyWriteRequest{}, VerifyWriteReply{},
		DecryptKeyRequest{}, DecryptKeyReply{})
}

type suite interface {
	kyber.Group
	kyber.XOFFactory
}

func (wr *WriteTxn) CheckSignatures(suite suite) error {
	validSig := 0
	success := false
	wb, err := protobuf.Encode(&wr.Write)
	if err != nil {
		return xerrors.Errorf("Cannot verify signatures: %v", err)
	}
	h := sha256.New()
	h.Write(wb)
	buf := h.Sum(nil)
	for i, pk := range wr.Write.Publics {
		err := schnorr.Verify(suite, pk, buf, wr.Sigs[i])
		if err == nil {
			validSig++
			if validSig >= wr.Threshold {
				success = true
				break
			}
		} else {
			log.Errorf("Cannot verify signature for node %d: %v", i, err)
		}
	}
	if success {
		return nil
	}
	return xerrors.Errorf("Cannot verify enough signatures to accept the" +
		" write transaction")
}
