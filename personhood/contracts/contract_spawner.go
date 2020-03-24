package contracts

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"regexp"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/onet/v3/network"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// ContractSpawnerID denotes a contract that can spawn new instances.
var ContractSpawnerID = "spawner"

// SpawnerCoin defines which coin type is allowed to spawn new instances.
var SpawnerCoin = byzcoin.NewInstanceID([]byte("SpawnerCoin"))

// ContractSpawnerFromBytes returns a Spawner instance from a slice of bytes.
func ContractSpawnerFromBytes(in []byte) (byzcoin.Contract, error) {
	c := &ContractSpawner{}
	err := protobuf.Decode(in, &c.SpawnerStruct)
	if err != nil {
		return nil, errors.New("couldn't unmarshal instance data: " + err.Error())
	}
	return c, nil
}

// ContractSpawner embeds the BasicContract.
type ContractSpawner struct {
	byzcoin.BasicContract
	SpawnerStruct
}

// VerifyInstruction allows non-darc-verified calls for instructions that send coins.
func (c ContractSpawner) VerifyInstruction(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, ctxHash []byte) error {
	if inst.GetType() != byzcoin.SpawnType {
		if err := inst.Verify(rst, ctxHash); err != nil {
			return err
		}
	}
	return nil
}

// Spawn creates a new spawner. Depending on what the client wants to spawn, this method will check the
// price for that instance, and spawn a new instance if the price is correct.
// Currently the coins are simply burned and will never be seen again. In future versions, the coins will
// be part of the mining reward for the nodes participating in the consensus.
// The following instances and their arguments can be spawned:
//   - ContractSpawnerID only with a valid darc. The arguments will be parsed for the costs:
//     cost(Darc|Coin|Credential|Party|RoPaSci)
//   - ContractDarcID takes the 'darc' argument and puts the calling darc as the parent darc
//   - ContractCoinID takes 'coinName' and 'darcID' as arguments creates a coin using all inputs from
//     'coinName' to the new coin, protected by the darcID. The IID of the coin is defined by
//     sha256( "coin" | darcID )
//   - ContractCredentialID takes 'credential' and 'darcID' as arguments and creates a new credential instance
//     with the content of 'credential', protected by 'darcID' and at IID of sha256( "credential" | darcID )
//   - ContractPopPartyID directly calls ContractPopParty.Spawn
//   - ContractRoPaSciID directly calls ContractRoPaSci.Spawn
//   - ContractValueID directly calls ContractValue.Spawn
func (c *ContractSpawner) Spawn(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// Spawn creates a new coin account as a separate instance.
	ca := inst.DeriveID("")
	var instBuf []byte
	cID := inst.Spawn.ContractID
	switch cID {
	case ContractSpawnerID:
		c.CostCWrite = &byzcoin.Coin{}
		c.CostCRead = &byzcoin.Coin{}
		c.CostValue = &byzcoin.Coin{}
		err = c.parseArgs(inst.Spawn.Args, rst.GetVersion())
		if err != nil {
			return nil, nil, errors.New("couldn't parse args: " + err.Error())
		}
		instBuf, err = protobuf.Encode(&c.SpawnerStruct)
		if err != nil {
			return nil, nil, errors.New("couldn't encode SpawnerInstance: " + err.Error())
		}

	case byzcoin.ContractDarcID:
		if err = c.getCoins(cout, c.CostDarc); err != nil {
			return
		}
		instBuf = inst.Spawn.Args.Search("darc")
		d, err := darc.NewFromProtobuf(instBuf)
		if err != nil {
			return nil, nil, errors.New("couldn't decode darc: " + err.Error())
		}

		if rst.GetVersion() > 1 {
			// Allowing unlimited rules while spawning darcs could give attackers
			// a way break the system.
			//
			// So, we whitelist allowed darc rules - there is no spawn allowed, and all
			// invokes need to be in this list. Just to be sure that there is
			// no command in the future that will allow to spawn things as an
			// invoke.
			//
			// When using the spawner-instance, darcs with spawn:calypsoRead
			// can be allowed, as they can only be used as instructions to a
			// calypsoWrite instance that will check if coins are needed or not.
			allowed := regexp.MustCompile("^(_sign|invoke:(" +
				"darc\\.evolve|" +
				"spawner\\.update|" +
				"coin\\.(fetch|store|transfer)|" +
				"credential\\.(update|recover)|" +
				"popParty\\.(barrier|finalize|mine|addParty)|" +
				"ropasci\\.(second|confirm)|" +
				"value\\.update)|" +
				"spawn:(calypsoRead))$")
			for _, rule := range d.Rules.List {
				if !allowed.MatchString(string(rule.Action)) {
					return nil, nil, errors.New("cannot spawn darc with rule: " +
						string(rule.Action))
				}
			}
		}
		ca = byzcoin.NewInstanceID(d.GetBaseID())
		darcID = d.GetBaseID()

	case contracts.ContractCoinID:
		if err = c.getCoins(cout, c.CostCoin); err != nil {
			return
		}
		coin := &byzcoin.Coin{
			Name: contracts.CoinName,
		}
		if name := inst.Spawn.Args.Search("coinName"); name != nil {
			coin.Name = byzcoin.NewInstanceID(name)
		}

		// Start with the maximum value for addCoin,
		// then eventually adjust with the argument.
		// The for-loop below will get as much as possible from cout.
		addCoin := uint64(math.MaxUint64)
		for _, arg := range inst.Spawn.Args {
			if arg.Name == "coinValue" {
				if len(arg.Value) < 8 {
					return nil, nil,
						errors.New("getCoin needs to have a value of 8 bytes")
				}
				addCoin = binary.LittleEndian.Uint64(arg.Value)
			}
		}

		for i := range cout {
			if cout[i].Name.Equal(coin.Name) && addCoin > 0 {
				if addCoin <= cout[i].Value {
					err = cout[i].SafeTransfer(coin, addCoin)
					addCoin = 0
				} else {
					err = cout[i].SafeTransfer(coin, cout[i].Value)
					addCoin -= cout[i].Value
				}
				if err != nil {
					return nil, nil, err
				}
				log.Lvl2("Initial balance is:", coin.Value)
			}
		}
		darcID = inst.Spawn.Args.Search("darcID")
		coinID := inst.Spawn.Args.Search("coinID")
		if coinID == nil {
			coinID = darcID
		}
		h := sha256.New()
		h.Write([]byte(contracts.ContractCoinID))
		h.Write(coinID)
		ca = byzcoin.NewInstanceID(h.Sum(nil))
		instBuf, err = protobuf.Encode(coin)
		if err != nil {
			return nil, nil, err
		}

	case ContractCredentialID:
		if err = c.getCoins(cout, c.CostCredential); err != nil {
			return
		}
		instBuf = inst.Spawn.Args.Search("credential")
		var cred CredentialStruct
		err = protobuf.Decode(instBuf, &cred)
		if err != nil {
			return nil, nil, err
		}
		darcID = inst.Spawn.Args.Search("darcID")
		var credID []byte
		if credID = inst.Spawn.Args.Search("credID"); credID == nil {
			credID = darcID
		}
		h := sha256.New()
		h.Write([]byte("credential"))
		h.Write(credID)
		ca = byzcoin.NewInstanceID(h.Sum(nil))

	case calypso.ContractWriteID:
		w := inst.Spawn.Args.Search("write")
		if w == nil || len(w) == 0 {
			err = errors.New("need a write request in 'write' argument")
			return
		}
		var write calypso.Write
		err = protobuf.DecodeWithConstructors(w, &write, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			err = errors.New("couldn't unmarshal write: " + err.Error())
			return
		}
		if write.Cost.Value != c.CostCRead.Value ||
			write.Cost.Name != c.CostCRead.Name {
			err = fmt.Errorf("spawned calypso write needs to have cost at %d", c.CostCRead.Value)
		}
		if err = c.getCoins(cout, *c.CostCWrite); err != nil {
			return
		}
		return calypso.ContractWrite{}.Spawn(rst, inst, cout)

	case ContractPopPartyID:
		if err = c.getCoins(cout, c.CostParty); err != nil {
			return
		}
		return ContractPopParty{}.Spawn(rst, inst, cout)

	case ContractRoPaSciID:
		if err = c.getCoins(cout, c.CostRoPaSci); err != nil {
			return
		}
		return ContractRoPaSci{}.Spawn(rst, inst, cout)

	case contracts.ContractValueID:
		if err = c.getCoins(cout, *c.CostValue); err != nil {
			return
		}
		return contracts.ContractValue{}.Spawn(rst, inst, cout)

	default:
		return nil, nil, errors.New("don't know how to spawn this type of contract")
	}
	log.Lvlf3("Spawning %s instance to %x", cID, ca.Slice())
	sc = []byzcoin.StateChange{
		byzcoin.NewStateChange(byzcoin.Create, ca, cID, instBuf, darcID),
	}
	return
}

func (c ContractSpawner) getCoins(coins []byzcoin.Coin, cost byzcoin.Coin) error {
	if cost.Value == 0 {
		return nil
	}
	for i := range coins {
		if coins[i].Name.Equal(cost.Name) {
			if coins[i].Value >= cost.Value {
				return coins[i].SafeSub(cost.Value)
			}
		}
	}
	return fmt.Errorf("don't have enough coins for spawning: needed %d", cost.Value)
}

// Invoke can be used to update the prices of the coins. The following command is supported:
//  - update to update the coin values.
func (c *ContractSpawner) Invoke(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	switch inst.Invoke.Command {
	case "update":
		// updates the values of the contract
		err = c.SpawnerStruct.parseArgs(inst.Invoke.Args, rst.GetVersion())
		if err != nil {
			return
		}
	default:
		err = errors.New("personhood contract can only update")
		return
	}

	// Finally update the coin value.
	var ciBuf []byte
	ciBuf, err = protobuf.Encode(&c.SpawnerStruct)
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, inst.InstanceID,
		ContractSpawnerID, ciBuf, darcID))
	return
}

// Delete removes the SpawnerInstance
func (c *ContractSpawner) Delete(rst byzcoin.ReadOnlyStateTrie, inst byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	sc = byzcoin.StateChanges{
		byzcoin.NewStateChange(byzcoin.Remove, inst.InstanceID, ContractSpawnerID, nil, darcID),
	}
	return
}

func (ss *SpawnerStruct) parseArgs(args byzcoin.Arguments, v byzcoin.Version) error {
	for _, cost := range []struct {
		name string
		coin *byzcoin.Coin
	}{
		{"costDarc", &ss.CostDarc},
		{"costCoin", &ss.CostCoin},
		{"costCredential", &ss.CostCredential},
		{"costParty", &ss.CostParty},
		{"costRoPaSci", &ss.CostRoPaSci},
		{"costCWrite", ss.CostCWrite},
		{"costCRead", ss.CostCRead},
		{"costValue", ss.CostValue},
	} {
		if arg := args.Search(cost.name); arg != nil {
			err := protobuf.Decode(arg, cost.coin)
			if err != nil {
				return fmt.Errorf("couldn't decode coin %s: %s", cost.name, err)
			}
		} else {
			if v > 1 {
				cost.coin.Name = contracts.CoinName
				cost.coin.Value = 100
			}
		}
		log.Lvl2("Setting cost of", cost.name, "to", cost.coin.Value)
	}
	// This is a check to make sure that older spawn-instructions don't get
	// a wrong data.
	if args.Search("costRoPaSci") == nil {
		ss.CostCWrite = nil
		ss.CostCRead = nil
	}
	if args.Search("costValue") == nil {
		ss.CostValue = nil
	}
	return nil
}
