package contracts

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/onet/log"
)

// ContractCoinID denotes a contract that can store and transfer coins.
var ContractCoinID = "coin"

// CoinName is a well-known InstanceID that identifies coins as belonging
// to this contract.
var CoinName = iid("olCoin")

// safeUint64 is a uin64 that guards against overflow/underflow.
type safeUint64 uint64

var errOverflow = errors.New("integer overflow")
var errUnderflow = errors.New("integer underflow")

func newSafeUint64(value []byte) safeUint64 {
	return safeUint64(binary.LittleEndian.Uint64(value))
}

func (s safeUint64) add(a uint64) (safeUint64, error) {
	s1 := uint64(s) + a
	if s1 < uint64(s) {
		return s, errOverflow
	}
	return safeUint64(s1), nil
}

func (s safeUint64) sub(a uint64) (safeUint64, error) {
	if a <= uint64(s) {
		return safeUint64(uint64(s) - a), nil
	}
	return s, errUnderflow
}

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
func ContractCoin(cdb omniledger.CollectionView, inst omniledger.Instruction, c []omniledger.Coin) (sc []omniledger.StateChange, cOut []omniledger.Coin, err error) {
	cOut = c

	var value []byte
	var darcID darc.ID
	value, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.GetType() {
	case omniledger.SpawnType:
		// Spawn creates a new coin account as a separate instance.
		ca := omniledger.InstanceIDFromSlice(inst.Hash())
		log.Lvlf3("Spawning coin to %x", ca.Slice())
		sc = []omniledger.StateChange{
			omniledger.NewStateChange(omniledger.Create, ca, ContractCoinID, make([]byte, 8), darcID),
		}
		return
	case omniledger.InvokeType:
		// Invoke is one of "mint", "transfer", "fetch", or "store".
		coinsCurrent := newSafeUint64(value)
		var coinsArg uint64

		if inst.Invoke.Command != "store" {
			coinsBuf := inst.Invoke.Args.Search("coins")
			if coinsBuf == nil {
				err = errors.New("argument \"coins\" is missing")
				return
			}
			coinsArg = binary.LittleEndian.Uint64(coinsBuf)
		}
		switch inst.Invoke.Command {
		case "mint":
			// mint simply adds this amount of coins to the account.
			log.Lvl2("minting", coinsArg)
			coinsCurrent, err = coinsCurrent.add(coinsArg)
			if err != nil {
				return
			}
		case "transfer":
			// transfer sends a given amount of coins to another account.
			coinsCurrent, err = coinsCurrent.sub(coinsArg)
			if err != nil {
				return
			}

			target := inst.Invoke.Args.Search("destination")
			var (
				v   []byte
				cid string
				did darc.ID
			)
			v, cid, did, err = cdb.GetValues(target)
			if err == nil && cid != ContractCoinID {
				err = errors.New("destination is not a coin contract")
			}
			if err != nil {
				return
			}

			targetCoin := newSafeUint64(v)
			targetCoin, err = targetCoin.add(coinsArg)
			if err != nil {
				return
			}
			var w bytes.Buffer
			binary.Write(&w, binary.LittleEndian, targetCoin)

			log.Lvlf3("transferring %d to %x", coinsArg, target)
			sc = append(sc, omniledger.NewStateChange(omniledger.Update, omniledger.InstanceIDFromSlice(target),
				ContractCoinID, w.Bytes(), did))
		case "fetch":
			// fetch removes coins from the account and passes it on to the next
			// instruction.
			if coinsArg > uint64(coinsCurrent) {
				err = errors.New("not enough coins in instance")
				return
			}
			coinsCurrent, err = coinsCurrent.sub(coinsArg)
			if err != nil {
				return
			}
			cOut = append(cOut, omniledger.Coin{Name: CoinName, Value: coinsArg})
		case "store":
			// store moves all coins from this instruction into the account.
			cOut = []omniledger.Coin{}
			for _, co := range c {
				if bytes.Equal(co.Name.Slice(), CoinName.Slice()) {
					coinsCurrent, err = coinsCurrent.add(co.Value)
					if err != nil {
						return
					}
				} else {
					cOut = append(cOut, co)
				}
			}
		default:
			err = errors.New("Coin contract can only mine and transfer")
			return
		}
		// Finally update the coin value.
		var w bytes.Buffer
		binary.Write(&w, binary.LittleEndian, coinsCurrent)
		sc = append(sc, omniledger.NewStateChange(omniledger.Update, inst.InstanceID,
			ContractCoinID, w.Bytes(), darcID))
		return
	case omniledger.DeleteType:
		// Delete our coin address, but only if the current coin is empty.
		var value []byte
		value, _, _, err = cdb.GetValues(inst.InstanceID.Slice())
		if err != nil {
			return
		}
		coinsCurrent := binary.LittleEndian.Uint64(value)
		if coinsCurrent > 0 {
			err = errors.New("cannot destroy a coinInstance that still has coins in it")
			return
		}
		sc = omniledger.StateChanges{
			omniledger.NewStateChange(omniledger.Remove, inst.InstanceID, ContractCoinID, nil, darcID),
		}
		return
	}
	err = errors.New("instruction type not allowed")
	return
}

// iid uses sha256(in) in order to manufacture an InstanceID from in
// thereby handling the case where len(in) != 32.
//
// TODO: Find a nicer way to make well-known instance IDs.
func iid(in string) omniledger.InstanceID {
	h := sha256.New()
	h.Write([]byte(in))
	return omniledger.InstanceIDFromSlice(h.Sum(nil))
}
