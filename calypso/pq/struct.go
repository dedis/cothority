package pq

import (
	"crypto/sha256"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

func init() {
	network.RegisterMessages(VerifyWrite{}, VerifyWriteReply{})
}

type suite interface {
	kyber.Group
	kyber.XOFFactory
}

type WriteRequest struct {
	Threshold int
	Write     Write
	Sigs      map[int][]byte
}

// WriteReply is returned upon successfully spawning a Write instance.
type WriteReply struct {
	*byzcoin.AddTxResponse
	byzcoin.InstanceID
}

// Read is the data stored in a read instance. It has a pointer to the write
// instance and the public key used to re-encrypt the secret to.
type Read struct {
	PQWrite byzcoin.InstanceID
	//Xc    kyber.Point
}

// ReadReply is is returned upon successfully spawning a Read instance.
type ReadReply struct {
	*byzcoin.AddTxResponse
	byzcoin.InstanceID
}

func (wr *WriteRequest) CheckSignatures(suite suite) error {
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
		err := schnorr.Verify(cothority.Suite, pk, buf, wr.Sigs[i])
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
