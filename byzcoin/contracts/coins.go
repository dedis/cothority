package contracts

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// ContractCoinID denotes a contract that can store and transfer coins.
var ContractCoinID = "coin"

// CoinName is a well-known InstanceID that identifies coins as belonging
// to this contract.
var CoinName = iid("byzCoin")

// ContractCoin is a coin implementation that holds one instance per coin.
// If you spawn a new ContractCoin, it will create an account with a value
// of 0 coins. For spawn, an optional argument is 'public' which, if set,
// is used to calculate the instanceID as follows:
//  coinID := sha256(ContractCoinID | public)
//
// The following invoke-methods are available:
//  - mint will add the number of coins in the argument "coins" to the
//    current coin instance. The argument must be a 64-bit uint in LittleEndian
//  - transfer will send the coins given in the argument "coins" to the
//    instance given in the argument "destination" or to the public key in the
//    argument "public". The "coins"-argument must be a 64-bit uint in
//    LittleEndian. The "destination" must be a 64-bit instanceID. The
//    "public" must be a 32-bit marshalled ed25519 point.
//  - fetch takes "coins" out of the account and returns it as an output
//    parameter for the next instruction to interpret.
//  - store puts the coins given to the instance back into the account.
// You can only delete a contractCoin instance if the account is empty.

func contractCoinFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &contractCoin{}
	err := protobuf.Decode(in, &c.Coin)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

type contractCoin struct {
	byzcoin.BasicContract
	byzcoin.Coin
}

func (c *contractCoin) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// Spawn creates a new coin account as a separate instance.
	ca := inst.DeriveID("")
	// If 'public' is given as argument, it is used to calculate the instanceID.
	if t := inst.Spawn.Args.Search("public"); t != nil {
		h := sha256.New()
		h.Write([]byte(ContractCoinID))
		h.Write(t)
		ca = byzcoin.NewInstanceID(h.Sum(nil))
	}

	log.Lvlf3("Spawning coin to %x", ca.Slice())
	if t := inst.Spawn.Args.Search("type"); t != nil {
		if len(t) != len(byzcoin.InstanceID{}) {
			return nil, nil, errors.New("type needs to be an InstanceID")
		}
		c.Name = byzcoin.NewInstanceID(t)
	} else {
		c.Name = CoinName
	}
	var ciBuf []byte
	ciBuf, err = protobuf.Encode(&c.Coin)
	if err != nil {
		return nil, nil, errors.New("couldn't encode CoinInstance: " + err.Error())
	}
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, ca, ContractCoinID, ciBuf, darcID),
	}
	return
}

func (c *contractCoin) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

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
		err = c.SafeAdd(coinsArg)
		if err != nil {
			return
		}
	case "transfer":
		// transfer sends a given amount of coins to another account.
		target := inst.Invoke.Args.Search("destination")
		publicBuf := inst.Invoke.Args.Search("public")
		if target == nil && publicBuf != nil {
			h := sha256.New()
			h.Write([]byte(ContractCoinID))
			h.Write(publicBuf)
			target = h.Sum(nil)
		}
		if inst.InstanceID.Equal(byzcoin.NewInstanceID(target)) {
			return nil, nil, errors.New("cannot send to ourselves")
		}

		var (
			v   []byte
			cid string
			did darc.ID
		)
		v, _, cid, did, err = rst.GetValues(target)
		if err == nil && cid != ContractCoinID {
			err = errors.New("destination is not a coin contract")
		}
		var targetCI byzcoin.Coin
		if err != nil && err.Error() == "key not set" {
			if publicBuf == nil {
				return nil, nil, errors.New("destination not available and 'public' not given")
			}

			// Create darc and coin
			log.Lvlf2("Creating new darc and coin instance for public key %x", publicBuf)
			targetPub := cothority.Suite.Point()
			err = targetPub.UnmarshalBinary(publicBuf)
			if err != nil {
				return
			}
			targetID := darc.NewIdentityEd25519(targetPub)
			exp := expression.Expr(targetID.String())
			r := darc.InitRules(nil, []darc.Identity{targetID})
			r.AddRule("invoke:transfer", exp)
			r.AddRule("invoke:fetch", exp)
			r.AddRule("invoke:store", exp)
			r.AddRule("delete", exp)
			d := darc.NewDarc(r, []byte("transaction created darc"))
			var darcBuf []byte
			darcBuf, err = d.ToProto()
			if err != nil {
				return
			}
			sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, byzcoin.NewInstanceID(d.GetBaseID()),
				byzcoin.ContractDarcID, darcBuf, nil))
			targetCI.Name = c.Name
		} else if err != nil {
			return
		} else {
			err = protobuf.Decode(v, &targetCI)
			if err != nil {
				return nil, nil, errors.New("couldn't unmarshal target account: " + err.Error())
			}
		}

		err = c.SafeSub(coinsArg)
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
		err = c.SafeSub(coinsArg)
		if err != nil {
			return
		}
		cout = append(cout, byzcoin.Coin{Name: c.Name, Value: coinsArg})
	case "store":
		// store moves all coins from this instruction into the account.
		cout = []byzcoin.Coin{}
		for _, co := range coins {
			if c.Name.Equal(co.Name) {
				err = c.SafeAdd(co.Value)
				if err != nil {
					return
				}
			} else {
				cout = append(cout, co)
			}
		}
	default:
		err = errors.New("Coin contract can only mine and transfer")
		return
	}

	// Finally update the coin value.
	var ciBuf []byte
	ciBuf, err = protobuf.Encode(&c.Coin)
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
		ContractCoinID, ciBuf, darcID))
	return
}

func (c *contractCoin) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	if c.Value > 0 {
		err = errors.New("cannot destroy a coinInstance that still has coins in it")
		return
	}
	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractCoinID, nil, darcID),
	}
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
