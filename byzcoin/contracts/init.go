package contracts

import (
	"go.dedis.ch/cothority/v4/byzcoin"
	"go.dedis.ch/onet/v4/log"
)

func init() {
	err := byzcoin.RegisterGlobalContract(ContractValueID, contractValueFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractCoinID, contractCoinFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractInsecureDarcID, contractInsecureDarcFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
}
