package bevm

import (
	"crypto/sha256"

	"go.dedis.ch/cothority/v3/byzcoin"
)

var ContractBEvmValueID = "bevm_value"

type BEvmValue struct {
	byzcoin.BasicContract

	contents BEvmValueContents
}

type BEvmValueContents struct {
	Value []byte
}

func ComputeInstanceID(bevmContractID byzcoin.InstanceID, key []byte) byzcoin.InstanceID {
	h := sha256.New()
	h.Write(bevmContractID[:])
	h.Write(key)

	return byzcoin.NewInstanceID(h.Sum(nil))
}
