package contracts

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// ContractCoinID denotes a contract that can store and transfer coins.
var ContractCoinID = "coin"

// CoinName is a well-known InstanceID that identifies coins as belonging
// to this contract.
var CoinName = iid("olCoin")

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
func ContractCoin(cdb byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte, c []byzcoin.Coin) (sc []byzcoin.StateChange, cOut []byzcoin.Coin, err error) {
	cOut = c

	err = inst.Verify(cdb, ctxHash)
	if err != nil {
		return
	}

	var value []byte
	var darcID darc.ID
	value, _, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}
	var ci byzcoin.Coin
	if inst.Spawn == nil {
		// Only if its NOT a spawn instruction is there data in the instance
		if value != nil {
			err = protobuf.Decode(value, &ci)
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal instance data: " + err.Error())
			}
		}
	}

	switch inst.GetType() {
	case byzcoin.SpawnType:
		// Spawn creates a new coin account as a separate instance.
		ca := inst.DeriveID("")
		log.Lvlf3("Spawning coin to %x", ca.Slice())
		if t := inst.Spawn.Args.Search("type"); t != nil {
			if len(t) != len(byzcoin.InstanceID{}) {
				return nil, nil, errors.New("type needs to be an InstanceID")
			}
			ci.Name = byzcoin.NewInstanceID(t)
		} else {
			ci.Name = CoinName
		}
		var ciBuf []byte
		ciBuf, err = protobuf.Encode(&ci)
		if err != nil {
			return nil, nil, errors.New("couldn't encode CoinInstance: " + err.Error())
		}
		sc = []byzcoin.StateChange{
			byzcoin.NewStateChange(byzcoin.Create, ca, ContractCoinID, ciBuf, darcID),
		}
		return
	case byzcoin.InvokeType:
		// Invoke is one of "mint", "transfer", "fetch", or "store".
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
			err = ci.SafeAdd(coinsArg)
			if err != nil {
				return
			}
		case "transfer":
			// transfer sends a given amount of coins to another account.
			target := inst.Invoke.Args.Search("destination")
			var (
				v   []byte
				cid string
				did darc.ID
			)
			v, _, cid, did, err = cdb.GetValues(target)
			if err == nil && cid != ContractCoinID {
				err = errors.New("destination is not a coin contract")
			}
			if err != nil {
				return
			}

			var targetCI byzcoin.Coin
			err = protobuf.Decode(v, &targetCI)
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal target account: " + err.Error())
			}
			err = ci.SafeSub(coinsArg)
			if err != nil {
				return
			}
			err = targetCI.SafeAdd(coinsArg)
			if err != nil {
				return
			}
			targetBuf, err := protobuf.Encode(&targetCI)
			if err != nil {
				return nil, nil, errors.New("couldn't marshal target account: " + err.Error())
			}

			log.Lvlf1("transferring %d to %x", coinsArg, target)
			sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, byzcoin.NewInstanceID(target),
				ContractCoinID, targetBuf, did))
		case "fetch":
			// fetch removes coins from the account and passes it on to the next
			// instruction.
			err = ci.SafeSub(coinsArg)
			if err != nil {
				return
			}
			cOut = append(cOut, byzcoin.Coin{Name: ci.Name, Value: coinsArg})
		case "store":
			// store moves all coins from this instruction into the account.
			cOut = []byzcoin.Coin{}
			for _, co := range c {
				if ci.Name.Equal(co.Name) {
					err = ci.SafeAdd(co.Value)
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
		var ciBuf []byte
		ciBuf, err = protobuf.Encode(&ci)
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
			ContractCoinID, ciBuf, darcID))
		return
	case byzcoin.DeleteType:
		// Delete our coin address, but only if the current coin is empty.
		if ci.Value > 0 {
			err = errors.New("cannot destroy a coinInstance that still has coins in it")
			return
		}
		sc = byzcoin.StateChanges{
			byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractCoinID, nil, darcID),
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
func iid(in string) byzcoin.InstanceID {
	h := sha256.New()
	h.Write([]byte(in))
	return byzcoin.NewInstanceID(h.Sum(nil))
}
