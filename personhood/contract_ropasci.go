package personhood

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"go.dedis.ch/cothority/v3/byzcoin/contracts"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// ContractRoPaSciID denotes a contract that allows two players to play rock-paper-scissors.
var ContractRoPaSciID = "ropasci"

// ContractRoPaSciFromBytes returns a ContractRoPaSci structure given a slice of bytes.
func ContractRoPaSciFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractRoPaSci{}
	err := protobuf.Decode(in, &c.RoPaSciStruct)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

// ContractRoPaSci embeds the BasicContract.
type ContractRoPaSci struct {
	byzcoin.BasicContract
	RoPaSciStruct
}

// VerifyInstruction overrides the definition in BasicContract and is used to allow the second player to
// add a move without appearing in the darc.
func (c *ContractRoPaSci) VerifyInstruction(rst byzcoin.GlobalState, inst byzcoin.Instruction, ctxHash []byte) error {
	if c.FirstPlayer >= 0 {
		return errors.New("this instance has already finished")
	}
	if inst.Invoke != nil {
		if inst.Invoke.Command == "second" && c.SecondPlayer >= 0 {
			return errors.New("second player already set his bet")
		}
	}
	return nil
}

// Spawn creates a new RoPaSci contract. The following arguments must be set:
//  - struct that holds a protobuf-encoded byte slice of RoPaSciStruct
func (c ContractRoPaSci) Spawn(rst byzcoin.GlobalState, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// Spawn creates a new ropasci as a separate instance.
	ca := inst.DeriveID("")
	log.Lvlf3("Spawning RoPaSci to %x", ca.Slice())
	var rpsBuf []byte
	if rpsBuf = inst.Spawn.Args.Search("struct"); rpsBuf == nil {
		err = errors.New("rock paper scissors needs struct argument")
		return
	}
	err = protobuf.Decode(rpsBuf, &c.RoPaSciStruct)
	if err != nil {
		return nil, nil, errors.New("couldn't decode RoPaScoInstance: " + err.Error())
	}
	if len(c.FirstPlayerHash) != 32 {
		return nil, nil, errors.New("ropasci needs a hash from player 1")
	}
	if len(coins) == 0 || coins[0].Value == 0 {
		return nil, nil, errors.New("ropasci needs some coins as input")
	}
	c.Stake = coins[0]
	c.SecondPlayer = -1
	c.FirstPlayer = -1
	cout[0].Value = 0
	rpsBuf, err = protobuf.Encode(&c.RoPaSciStruct)
	if err != nil {
		return
	}
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, ca, ContractRoPaSciID, rpsBuf, darcID),
	}
	return
}

// Invoke allows to play the RoPaSci game. It takes one of the following commands:
//  - second to add a second move to the instance. The 'account' argument must point to an
//    account that will be used to pay out the reward
//  - confirm is sent by the first player and uses the 'prehash' argument to prove what the move
//    was. If the first player wins, the coins go to the coin instance in 'account'
//
//  TODO:
//   - add a 'recover' for the second player, in case the first player doesn't confirm
func (c *ContractRoPaSci) Invoke(rst byzcoin.GlobalState, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "second":
		account := inst.Invoke.Args.Search("account")
		if len(account) != 32 {
			return nil, nil, errors.New("need a valid account")
		}
		val, _, cid, _, err := rst.GetValues(account)
		if err != nil {
			return nil, nil, err
		}
		if cid != contracts.ContractCoinID {
			return nil, nil, errors.New("account is not of coin type")
		}
		var coin2 byzcoin.Coin
		err = protobuf.Decode(val, &coin2)
		if err != nil {
			return nil, nil, errors.New("couldn't decode coin: " + err.Error())
		}
		if !coin2.Name.Equal(c.Stake.Name) {
			return nil, nil, errors.New("not same type of coin")
		}
		if len(coins) == 0 {
			return nil, nil, errors.New("didn't get any coins as input")
		}
		if !coins[0].Name.Equal(c.Stake.Name) {
			return nil, nil, errors.New("input is not of same type as player 1's coins")
		}
		if coins[0].Value != c.Stake.Value {
			return nil, nil, errors.New("input coin-value doesn't match player 1")
		}
		choice := inst.Invoke.Args.Search("choice")
		if len(choice) != 1 {
			return nil, nil, errors.New("need a 1-byte choice")
		}
		c.SecondPlayerAccount = byzcoin.NewInstanceID(account)
		c.SecondPlayer = int(choice[0]) % 3

	case "confirm":
		preHash := inst.Invoke.Args.Search("prehash")
		if len(preHash) != 32 {
			return nil, nil, errors.New("prehash needs to be of length 32")
		}
		fph := sha256.Sum256(preHash)
		if bytes.Compare(c.FirstPlayerHash, fph[:]) != 0 {
			return nil, nil, errors.New("wrong prehash for first player")
		}
		firstAccountBuf := inst.Invoke.Args.Search("account")
		if len(firstAccountBuf) != 32 {
			return nil, nil, errors.New("wrong account for player 1")
		}
		var cid string
		_, _, cid, _, err = rst.GetValues(firstAccountBuf)
		if err != nil {
			return
		}
		if cid != contracts.ContractCoinID {
			return nil, nil, errors.New("account is not of coin type")
		}
		var winner []byte
		c.FirstPlayer = int(preHash[0]) % 3
		switch (3 + c.FirstPlayer - c.SecondPlayer) % 3 {
		case 0:
			log.Lvl2("draw - no winner")
		case 1:
			log.Lvl2("player 1 wins")
			winner = firstAccountBuf
		case 2:
			log.Lvl2("player 2 wins")
			winner = c.SecondPlayerAccount.Slice()
		}
		if winner != nil {
			var val []byte
			var winnerDarc darc.ID
			val, _, _, winnerDarc, err = rst.GetValues(winner)
			if err != nil {
				return
			}
			var coin byzcoin.Coin
			err = protobuf.Decode(val, &coin)
			if err != nil {
				return
			}
			coin.Value += c.Stake.Value * 2
			if coin.Value < c.Stake.Value {
				return nil, nil, errors.New("coin overflow")
			}
			var coinBuf []byte
			coinBuf, err = protobuf.Encode(&coin)
			if err != nil {
				return
			}
			sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, byzcoin.NewInstanceID(winner),
				contracts.ContractCoinID, coinBuf, winnerDarc))
		}
	default:
		err = errors.New("rps contract can only 'second' or 'confirm'")
		return
	}

	buf, err := protobuf.Encode(&c.RoPaSciStruct)
	if err != nil {
		return
	}
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
		ContractRoPaSciID, buf, darcID))
	return
}

// Delete removes an existing RoPaSci instance
func (c *ContractRoPaSci) Delete(rst byzcoin.GlobalState, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractRoPaSciID, nil, darcID),
	}
	return
}
