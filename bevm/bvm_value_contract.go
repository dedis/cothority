package bevm

import (
	"crypto/sha256"

	"go.dedis.ch/cothority/v3/byzcoin"
)

var ContractBvmValueID = "bvm_value"

type BvmValue struct {
	byzcoin.BasicContract

	contents BvmValueContents
}

type BvmValueContents struct {
	Value []byte
}

func ComputeInstanceID(bvmContractID byzcoin.InstanceID, key []byte) byzcoin.InstanceID {
	h := sha256.New()
	h.Write(bvmContractID[:])
	h.Write(key)

	return byzcoin.NewInstanceID(h.Sum(nil))
}
