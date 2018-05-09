package service

import (
	"errors"

	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/onet.v2/log"
)

// Here we give a definition of pre-defined classe.

// CmdCreate is the only command that is pre-defined.
var CmdCreate = "Create"

// ConfigID is 32 bytes of zeroes.
var ConfigID = make([]byte, 32)

// ZeroNonce is 32 bytes of zeroes and can have a special meaning.
var ZeroNonce = make([]byte, 32)

// KindConfig denotes a config-class
var KindConfig = "config"

// ClassConfig can only be instantiated once per skipchain, and that only for
// the genesis block.
func (s *Service) ClassConfig(cdb collection.Collection, tx Instruction, kind string, state []byte) ([]StateChange, error) {
	switch tx.Command {
	case CmdCreate:
		darc := &darc.Darc{}
		err := protobuf.Decode(tx.Data, darc)
		if err != nil {
			log.Error("couldn't get darc")
			return nil, err
		}
		if err = darc.Verify(); err != nil {
			log.Error("couldn't verify darc")
			return nil, err
		}
		if len(darc.Rules) == 0 {
			return nil, errors.New("don't accept darc with empty rules")
		}
		return []StateChange{
			NewStateChange(Create, ConfigID, nil, KindConfig, tx.DarcID),
			NewStateChange(Create, tx.DarcID, nil, KindDarc, tx.Data),
		}, nil
	default:
		return nil, errors.New("Unknown command")
	}
}

// KindDarc denotes a darc-class
var KindDarc = "darc"

// CmdDarcEvolve is needed to evolve a darc.
var CmdDarcEvolve = "Evolve"

// ClassDarc has the following methods:
//   - Create - creates a new darc
//   - Evolve - evolves an existing darc
func (s *Service) ClassDarc(cdb collection.Collection, tx Instruction, kind string, state []byte) ([]StateChange, error) {
	switch tx.Command {
	case CmdCreate:
		return nil, errors.New("Not yet implemented")
	case CmdDarcEvolve:
		return nil, errors.New("Not yet implemented")
	default:
		return nil, errors.New("Unknown command")
	}
}
