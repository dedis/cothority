package service

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/collection"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"gopkg.in/dedis/onet.v2/log"
)

// Here we give a definition of pre-defined contracts.

// ZeroNonce is 32 bytes of zeroes and can have a special meaning.
var ZeroNonce = Nonce([32]byte{})

// OneNonce has 32 bytes of zeros except the LSB is set to one.
var OneNonce = Nonce(func() [32]byte {
	var nonce [32]byte
	nonce[31] = 1
	return nonce
}())

// ZeroDarc is a DarcID with all zeroes.
var ZeroDarc = darc.ID(make([]byte, 32))

// GenesisReferenceID is 64 bytes of zeroes. Its value is a reference to the
// genesis-darc.
var GenesisReferenceID = ObjectID{ZeroDarc, ZeroNonce}

// ContractConfigID denotes a config-contract
var ContractConfigID = "config"

// Config stores all the configuration information for one skipchain. It will
// be stored under the key "GenesisDarcID || OneNonce", in the collections. The
// GenesisDarcID is the value of GenesisReferenceID.
type Config struct {
	BlockInterval time.Duration
}

// ContractConfig can only be instantiated once per skipchain, and only for
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

	// sanity check the block interval
	intervalBuf := tx.Spawn.Args.Search("block_interval")
	interval, _ := binary.Varint(intervalBuf)
	if interval == 0 {
		err = errors.New("block interval is zero")
		return
	}

	// create the config to be stored by state changes
	config := Config{
		BlockInterval: time.Duration(interval),
	}
	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	return []StateChange{
		NewStateChange(Create, GenesisReferenceID, ContractConfigID, tx.ObjectID.DarcID),
		NewStateChange(Create, tx.ObjectID, ContractDarcID, darcBuf),
		NewStateChange(Create,
			ObjectID{
				DarcID:     tx.ObjectID.DarcID,
				InstanceID: OneNonce,
			}, ContractDarcID, configBuf),
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
