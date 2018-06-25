package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// zeroNonce is 32 bytes of zeroes.
var zeroNonce = Nonce([32]byte{})

// zeroSubID is 32 bytes of zeroes as a subid for storing Darcs.
var zeroSubID = SubID([32]byte{})

// one is the subid for storing the omniledger config.
var one = SubID(func() [32]byte {
	var one [32]byte
	one[31] = 1
	return one
}())

// ZeroDarc is a DarcID with all zeroes.
var ZeroDarc = darc.ID(make([]byte, 32))

// GenesisReferenceID is 64 bytes of zeroes. Its value is a reference to the
// genesis-darc.
var GenesisReferenceID = InstanceID{ZeroDarc, zeroSubID}

// ContractConfigID denotes a config-contract
var ContractConfigID = "config"

// ContractDarcID denotes a darc-contract
var ContractDarcID = "darc"

// ContractValueID denotes a contract that can store and update
// key values.
var ContractValueID = "value"

// ContractCoinID denotes a contract that can store and transfer coins.
var ContractCoinID = "coin"

// CmdDarcEvolve is needed to evolve a darc.
var CmdDarcEvolve = "evolve"

// LoadConfigFromColl loads the configuration data from the collections.
func LoadConfigFromColl(coll CollectionView) (*ChainConfig, error) {
	// Find the genesis-darc ID.
	val, contract, err := getValueContract(coll, GenesisReferenceID.Slice())
	if err != nil {
		return nil, err
	}
	if string(contract) != ContractConfigID {
		return nil, errors.New("did not get " + ContractConfigID)
	}
	if len(val) != 32 {
		return nil, errors.New("value has a invalid length")
	}
	// Use the genesis-darc ID to create the config key and read the config.
	configID := InstanceID{
		DarcID: darc.ID(val),
		SubID:  one,
	}
	val, contract, err = getValueContract(coll, configID.Slice())
	if err != nil {
		return nil, err
	}
	if string(contract) != ContractConfigID {
		return nil, errors.New("did not get " + ContractConfigID)
	}
	config := ChainConfig{}
	err = protobuf.Decode(val, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// LoadBlockIntervalFromColl loads the block interval from the collections.
func LoadBlockIntervalFromColl(coll CollectionView) (time.Duration, error) {
	config, err := LoadConfigFromColl(coll)
	if err != nil {
		return defaultInterval, err
	}
	return config.BlockInterval, nil
}

// LoadDarcFromColl loads a darc which should be stored in key.
func LoadDarcFromColl(coll CollectionView, key []byte) (*darc.Darc, error) {
	rec, err := coll.Get(key).Record()
	if err != nil {
		return nil, err
	}
	vs, err := rec.Values()
	if err != nil {
		return nil, err
	}
	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf, ok := vs[1].([]byte)
	if !ok {
		return nil, errors.New("can not cast value to byte slice")
	}
	if string(contractBuf) != "darc" {
		return nil, errors.New("expected contract to be darc but got: " + string(contractBuf))
	}
	darcBuf, ok := vs[0].([]byte)
	if !ok {
		return nil, errors.New("cannot cast value to byte slice")
	}
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// BytesToObjID converts a byte slice to an InstanceID, it expects the byte slice
// to be 64 bytes.
func BytesToObjID(x []byte) InstanceID {
	if len(x) < 64 {
		return InstanceID{}
	}
	var sub SubID
	copy(sub[:], x[32:64])
	return InstanceID{
		DarcID: x[0:32],
		SubID:  sub,
	}
}

// ContractConfig can only be instantiated once per skipchain, and only for
// the genesis block.
func (s *Service) ContractConfig(cdb CollectionView, tx Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	c = coins
	if tx.GetType() != SpawnType {
		return nil, nil, errors.New("Config can only be spawned")
	}
	darcBuf := tx.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return
	}
	if len(d.Rules) == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
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
	config := ChainConfig{
		BlockInterval: time.Duration(interval),
	}
	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	return []StateChange{
		NewStateChange(Create, GenesisReferenceID, ContractConfigID, tx.InstanceID.DarcID),
		NewStateChange(Create, tx.InstanceID, ContractDarcID, darcBuf),
		NewStateChange(Create,
			InstanceID{
				DarcID: tx.InstanceID.DarcID,
				SubID:  one,
			}, ContractConfigID, configBuf),
	}, c, nil
}

// ContractDarc accepts the following instructions:
//   - Spawn - creates a new darc
//   - Invoke.Evolve - evolves an existing darc
func (s *Service) ContractDarc(coll CollectionView, tx Instruction,
	coins []Coin) ([]StateChange, []Coin, error) {
	switch {
	case tx.Spawn != nil:
		if tx.Spawn.ContractID == ContractDarcID {
			darcBuf := tx.Spawn.Args.Search("darc")
			d, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, errors.New("given darc could not be decoded: " + err.Error())
			}
			return []StateChange{
				NewStateChange(Create, InstanceID{d.GetBaseID(), zeroSubID}, ContractDarcID, darcBuf),
			}, coins, nil
		}
		// TODO The code below will never get called because this
		// contract is used only when tx.Spawn.ContractID is "darc", so
		// the if statement above gets executed and this contract
		// returns. Why do we need this part, if we do, how should we
		// fix it?
		c, found := s.contracts[tx.Spawn.ContractID]
		if !found {
			return nil, nil, errors.New("couldn't find this contract type")
		}
		return c(coll, tx, coins)
	case tx.Invoke != nil:
		switch tx.Invoke.Command {
		case "evolve":
			darcBuf := tx.Invoke.Args.Search("darc")
			newD, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, err
			}
			oldD, err := LoadDarcFromColl(coll, InstanceID{newD.BaseID, zeroSubID}.Slice())
			if err != nil {
				return nil, nil, err
			}
			if err := newD.SanityCheck(oldD); err != nil {
				return nil, nil, err
			}
			return []StateChange{
				NewStateChange(Update, tx.InstanceID, ContractDarcID, darcBuf),
			}, coins, nil
		default:
			return nil, nil, errors.New("invalid command: " + tx.Invoke.Command)
		}
	default:
		return nil, nil, errors.New("Only invoke and spawn are defined yet")
	}
}

// ContractValue is a simple key/value storage where you
// can put any data inside as wished.
func (s *Service) ContractValue(cdb CollectionView, tx Instruction, c []Coin) ([]StateChange, []Coin, error) {
	switch {
	case tx.Spawn != nil:
		var subID SubID
		copy(subID[:], tx.Hash())
		return []StateChange{
			NewStateChange(Create, InstanceID{tx.InstanceID.DarcID, subID},
				ContractValueID, tx.Spawn.Args.Search("value")),
		}, c, nil
	case tx.Invoke != nil:
		if tx.Invoke.Command != "update" {
			return nil, nil, errors.New("Value contract can only update")
		}
		return []StateChange{
			NewStateChange(Update, tx.InstanceID,
				ContractValueID, tx.Invoke.Args.Search("value")),
		}, c, nil
	case tx.Delete != nil:
		return StateChanges{
			NewStateChange(Remove, tx.InstanceID, ContractValueID, nil),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}

var olCoin = NewObjectID([]byte("olCoin"))

// ContractCoin is a coin implementation that holds one instance per coin.
// If you spawn a new ContractCoin, it will create an account with a value
// of 0 coins.
// The following methods are available:
//  - mint will add the number of coins in the argument "coins" to the
//    current coin instance. The argument must be a 64-bit uint in LittleEndian
//  - transfer will send the coins given in the argument "coins" to the
//    instance given in the argument "destination". The "coins"-argument must
//    be a 64-bit uint in LittleEndian. The "destination" must be a 64-bit
//    instanceID
//  - fetch takes "coins" out of the account and returns it as an output
//    parameter for the next instruction to interpret.
//  - store puts the coins given to the instance back into the account.
// You can only delete a contractCoin instance if the account is empty.
func (s *Service) ContractCoin(cdb CollectionView, inst Instruction, c []Coin) (sc []StateChange, cOut []Coin, err error) {
	cOut = c
	switch {
	case inst.Spawn != nil:
		// Spawn creates a new coin account as a separate instance. The subID is
		// taken from the hash of the instruction.
		var subID Nonce
		copy(subID[:], inst.Hash())
		ca := ObjectID{inst.ObjectID.DarcID, subID}
		log.Lvlf2("Spawing coin to %x", ca)
		return []StateChange{
			NewStateChange(Create, ca, ContractCoinID, make([]byte, 8)),
		}, c, nil
	case inst.Invoke != nil:
		// Invoke is one of "mint", "transfer", "fetch", or "store".
		value, err := cdb.GetValue(inst.ObjectID.Slice())
		if err != nil {
			return nil, nil, err
		}
		coinsCurrent := binary.LittleEndian.Uint64(value)
		coinsBuf := inst.Invoke.Args.Search("coins")
		if coinsBuf == nil {
			return nil, nil, errors.New("please give coins")
		}
		coinsArg := binary.LittleEndian.Uint64(coinsBuf)
		var sc []StateChange
		switch inst.Invoke.Command {
		case "mint":
			// mint simply adds this amount of coins to the account.
			log.Lvl2("minting", coinsArg)
			coinsCurrent += coinsArg
		case "transfer":
			// transfer sends a given amount of coins to another account.
			if coinsArg > coinsCurrent {
				return nil, nil, errors.New("not enough coins in instance")
			}
			coinsCurrent -= coinsArg
			target := inst.Invoke.Args.Search("destination")
			v, cid, err := cdb.GetValues(target)
			if err == nil && cid != ContractCoinID {
				err = errors.New("destination is not a coin contract")
			}
			if err != nil {
				return nil, nil, err
			}
			targetCoin := binary.LittleEndian.Uint64(v)
			var w bytes.Buffer
			binary.Write(&w, binary.LittleEndian, targetCoin+coinsArg)
			log.Lvlf2("transferring %d to %x", coinsArg, target)
			sc = append(sc, NewStateChange(Update, NewObjectID(target),
				ContractCoinID, w.Bytes()))
		case "fetch":
			// fetch removes coins from the account and passes it on to the next
			// instruction.
			if coinsArg > coinsCurrent {
				return nil, nil, errors.New("not enough coins in instance")
			}
			coinsCurrent -= coinsArg
			cOut = append(cOut, Coin{Name: olCoin, Value: coinsArg})
		case "store":
			// store moves all coins from the last instruction into the account.
			cOut = []Coin{}
			for _, co := range c {
				if co.Name.Equal(olCoin) {
					coinsCurrent += co.Value
				} else {
					cOut = append(cOut, co)
				}
			}
		default:
			return nil, nil, errors.New("Coin contract can only mine and transfer")
		}
		// Finally update our own coin value and send one or two stateChanges to
		// the system.
		var w bytes.Buffer
		binary.Write(&w, binary.LittleEndian, coinsCurrent)
		return append(sc, NewStateChange(Update, inst.ObjectID,
			ContractCoinID, w.Bytes())), c, nil
	case inst.Delete != nil:
		// Delete our coin address, but only if the current coin is empty.
		value, err := cdb.GetValue(inst.ObjectID.Slice())
		if err != nil {
			return nil, nil, err
		}
		coinsCurrent := binary.LittleEndian.Uint64(value)
		if coinsCurrent > 0 {
			return nil, nil, errors.New("cannot destroy a coinInstance that still has coins in it")
		}
		return StateChanges{
			NewStateChange(Remove, inst.ObjectID, ContractCoinID, nil),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}
