package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
)

func init() {
	byzcoin.ContractsFn[ContractValueID] = contractValueFromBytes
	byzcoin.ContractsFn[ContractCoinID] = contractCoinFromBytes
	byzcoin.ContractsFn[ContractInsecureDarcID] = contractInsecureDarcFromBytes
}
