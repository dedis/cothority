package service

import (
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/onet.v2/log"
)

// Here we give a definition of pre-defined contracts.

// ZeroNonce is 32 bytes of zeroes and can have a special meaning.
var ZeroNonce = Nonce([32]byte{})

// ZeroDarc is a DarcID with all zeroes.
var ZeroDarc = darc.ID(make([]byte, 32))

// ConfigID is 64 bytes of zeroes.
var ConfigID = ObjectID{ZeroDarc, ZeroNonce}

// ContractConfigID denotes a config-contract
var ContractConfigID = "config"

// ContractConfig can only be instantiated once per skipchain, and that only for
// the genesis block.
func (s *Service) ContractConfig(cdb collection.Collection, tx Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	if tx.Spawn == nil {
		return nil, nil, errors.New("Config can only be spawned")
	}
	darcBuf := tx.Spawn.Args.Search("darc")
	d, err := darc.NewDarcFromProto(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return
	}
	if len(d.Rules) == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(); err != nil {
		log.Error("couldn't verify darc")
		return
	}
	return []StateChange{
		NewStateChange(Create, ConfigID, ContractConfigID, tx.ObjectID.DarcID),
		NewStateChange(Create, tx.ObjectID, ContractDarcID, darcBuf),
	}, nil, nil
}

// ContractDarcID denotes a darc-contract
var ContractDarcID = "darc"

// CmdDarcEvolve is needed to evolve a darc.
var CmdDarcEvolve = "Evolve"

// ContractDarc accepts the following instructions:
//   - Spawn - creates a new darc
//   - Invoke.Evolve - evolves an existing darc
func (s *Service) ContractDarc(cdb collection.Collection, tx Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	return nil, nil, errors.New("Not yet implemented")
}
