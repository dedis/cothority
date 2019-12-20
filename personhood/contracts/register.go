package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3/log"
)

func init() {
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractPopPartyID,
		ContractPopPartyFromBytes))
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractSpawnerID,
		ContractSpawnerFromBytes))
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractCredentialID,
		ContractCredentialFromBytes))
	log.ErrFatal(byzcoin.RegisterGlobalContract(ContractRoPaSciID,
		ContractRoPaSciFromBytes))
}

func newArg(name string, val []byte) byzcoin.Argument {
	return byzcoin.Argument{Name: name, Value: val}
}
