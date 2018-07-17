package contracts

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/log"
)

// ContractCoinID denotes a contract that can store and transfer coins.
var ContractCoinID = "coin"

var olCoin = service.NewInstanceID([]byte("olCoin"))

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
func ContractCoin(cdb service.CollectionView, inst service.Instruction, c []service.Coin) (sc []service.StateChange, cOut []service.Coin, err error) {
	cOut = c
	switch {
	case inst.Spawn != nil:
		// Spawn creates a new coin account as a separate instance. The subID is
		// taken from the hash of the instruction.
		ca := service.InstanceID{
			DarcID: inst.InstanceID.DarcID,
			SubID:  service.NewSubID(inst.Hash()),
		}
		log.Lvlf3("Spawing coin to %x", ca.Slice())
		return []service.StateChange{
			service.NewStateChange(service.Create, ca, ContractCoinID, make([]byte, 8)),
		}, c, nil
	case inst.Invoke != nil:
		// Invoke is one of "mint", "transfer", "fetch", or "store".
		value, _, err := cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, nil, err
		}
		coinsCurrent := binary.LittleEndian.Uint64(value)
		coinsBuf := inst.Invoke.Args.Search("coins")
		if coinsBuf == nil {
			return nil, nil, errors.New("please give coins")
		}
		coinsArg := binary.LittleEndian.Uint64(coinsBuf)
		var sc []service.StateChange
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
			log.Lvlf3("transferring %d to %x", coinsArg, target)
			sc = append(sc, service.NewStateChange(service.Update, service.NewInstanceID(target),
				ContractCoinID, w.Bytes()))
		case "fetch":
			// fetch removes coins from the account and passes it on to the next
			// instruction.
			if coinsArg > coinsCurrent {
				return nil, nil, errors.New("not enough coins in instance")
			}
			coinsCurrent -= coinsArg
			cOut = append(cOut, service.Coin{Name: olCoin, Value: coinsArg})
		case "store":
			// store moves all coins from the last instruction into the account.
			cOut = []service.Coin{}
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
		return append(sc, service.NewStateChange(service.Update, inst.InstanceID,
			ContractCoinID, w.Bytes())), c, nil
	case inst.Delete != nil:
		// Delete our coin address, but only if the current coin is empty.
		value, _, err := cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return nil, nil, err
		}
		coinsCurrent := binary.LittleEndian.Uint64(value)
		if coinsCurrent > 0 {
			return nil, nil, errors.New("cannot destroy a coinInstance that still has coins in it")
		}
		return service.StateChanges{
			service.NewStateChange(service.Remove, inst.InstanceID, ContractCoinID, nil),
		}, c, nil
	}
	return nil, nil, errors.New("didn't find any instruction")
}
