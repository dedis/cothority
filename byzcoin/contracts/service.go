package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
)

func init() {
	if byzcoin.ContractsFn == nil {
		byzcoin.ContractsFn = make(map[string]byzcoin.ContractFn)
	}
	byzcoin.ContractsFn[ContractValueID] = contractValueFromBytes
	byzcoin.ContractsFn[ContractCoinID] = contractCoinFromBytes
	byzcoin.ContractsFn[ContractInsecureDarcID] = contractInsecureDarcFromBytes
}
