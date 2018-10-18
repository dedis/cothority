package byzcoin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin/viewchange"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// ContractConfigID denotes a config-contract
var ContractConfigID = "config"

// ContractDarcID denotes a darc-contract
var ContractDarcID = "darc"

// ConfigInstanceID represents the 0-id of the configuration instance.
var ConfigInstanceID = InstanceID{}

// CmdDarcEvolve is needed to evolve a darc.
var CmdDarcEvolve = "evolve"

// ContractFn is the type signature of the class functions which can be
// registered with the ByzCoin service.
type ContractFn func(st ReadOnlyStateTrie, inst Instruction, inCoins []Coin) (sc []StateChange, outCoins []Coin, err error)

// RegisterContract stores the contract in a map and will call it whenever a
// contract needs to be done. GetService makes it possible to give either an
// `onet.Context` or `onet.Server` to `RegisterContract`.
func RegisterContract(s skipchain.GetService, kind string, f ContractFn) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerContract(kind, f)
}

// LoadDarcFromTrie loads a darc which should be stored in key.
func LoadDarcFromTrie(st ReadOnlyStateTrie, key []byte) (*darc.Darc, error) {
	darcBuf, contract, _, err := st.GetValues(key)
	if err != nil {
		return nil, err
	}
	if contract != ContractDarcID {
		return nil, errors.New("expected contract to be darc but got: " + contract)
	}
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// ContractConfig can only be instantiated once per skipchain, and only for
// the genesis block.
func (s *Service) ContractConfig(cdb ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	// Verify the darc signature if the config instance does not exist yet.
	pr, err := cdb.GetProof(ConfigInstanceID.Slice())
	if err != nil {
		return
	}
	ok, err := pr.Exists(ConfigInstanceID.Slice())
	if err != nil {
		return
	}
	if ok {
		err = inst.VerifyDarcSignature(cdb)
		if err != nil {
			return
		}
	}
	switch inst.GetType() {
	case SpawnType:
		return spawnContractConfig(cdb, inst, coins)
	case InvokeType:
		return invokeContractConfig(cdb, inst, coins)
	default:
		return nil, coins, errors.New("unsupported instruction type")
	}
}

func invokeContractConfig(cdb ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cOut []Coin, err error) {
	cOut = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// There are two situations where we need to change the roster, first
	// is when it is initiated by the client(s) that holds the genesis
	// signing key, in this case we trust the client to do the right
	// thing. The second is during a view-change, so we need to do
	// additional validation to make sure a malicious node doesn't freely
	// change the roster.
	if inst.Invoke.Command == "update_config" {
		configBuf := inst.Invoke.Args.Search("config")
		newConfig := ChainConfig{}
		err = protobuf.DecodeWithConstructors(configBuf, &newConfig, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return
		}
		var oldConfig *ChainConfig
		oldConfig, err = loadConfigFromTrie(cdb)
		if err != nil {
			return
		}
		if err = newConfig.sanityCheck(oldConfig); err != nil {
			return
		}
		sc = []StateChange{
			NewStateChange(Update, NewInstanceID(nil), ContractConfigID, configBuf, darcID),
		}
		return
	} else if inst.Invoke.Command == "view_change" {
		var req viewchange.NewViewReq
		err = protobuf.DecodeWithConstructors(inst.Invoke.Args.Search("newview"), &req, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return
		}
		// If everything is correctly signed, then we trust it, no need
		// to do additional verification.
		sigBuf := inst.Invoke.Args.Search("multisig")
		err = cosi.Verify(cothority.Suite, req.Roster.Publics(),
			req.Hash(), sigBuf, cosi.NewThresholdPolicy(len(req.Roster.List)-len(req.Roster.List)/3))
		if err != nil {
			return
		}

		sc, err = updateRosterScs(cdb, darcID, req.Roster)
		return
	}
	err = errors.New("invalid invoke command: " + inst.Invoke.Command)
	return
}

func updateRosterScs(cdb ReadOnlyStateTrie, darcID darc.ID, newRoster onet.Roster) (StateChanges, error) {
	config, err := loadConfigFromTrie(cdb)
	if err != nil {
		return nil, err
	}
	config.Roster = newRoster
	configBuf, err := protobuf.Encode(config)
	if err != nil {
		return nil, err
	}

	return []StateChange{
		NewStateChange(Update, NewInstanceID(nil), ContractConfigID, configBuf, darcID),
	}, nil
}

func spawnContractConfig(cdb ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	c = coins
	darcBuf := inst.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Error("couldn't decode darc")
		return
	}
	if d.Rules.Count() == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
		log.Error("couldn't verify darc")
		return
	}

	intervalBuf := inst.Spawn.Args.Search("block_interval")
	interval, _ := binary.Varint(intervalBuf)
	bsBuf := inst.Spawn.Args.Search("max_block_size")
	maxsz, _ := binary.Varint(bsBuf)

	rosterBuf := inst.Spawn.Args.Search("roster")
	roster := onet.Roster{}
	err = protobuf.DecodeWithConstructors(rosterBuf, &roster, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return
	}

	// create the config to be stored by state changes
	config := ChainConfig{
		BlockInterval: time.Duration(interval),
		Roster:        roster,
		MaxBlockSize:  int(maxsz),
	}
	if err = config.sanityCheck(nil); err != nil {
		return
	}

	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	id := d.GetBaseID()
	return []StateChange{
		NewStateChange(Create, ConfigInstanceID, ContractConfigID, configBuf, id),
		NewStateChange(Create, NewInstanceID(id), ContractDarcID, darcBuf, id),
	}, c, nil

}

// ContractDarc accepts the following instructions:
//   - Spawn - creates a new darc
//   - Invoke.Evolve - evolves an existing darc
func (s *Service) ContractDarc(cdb ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cOut []Coin, err error) {
	cOut = coins
	err = inst.VerifyDarcSignature(cdb)
	if err != nil {
		return
	}
	switch inst.GetType() {
	case SpawnType:
		if inst.Spawn.ContractID == ContractDarcID {
			darcBuf := inst.Spawn.Args.Search("darc")
			d, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, errors.New("given darc could not be decoded: " + err.Error())
			}
			id := d.GetBaseID()
			return []StateChange{
				NewStateChange(Create, NewInstanceID(id), ContractDarcID, darcBuf, id),
			}, coins, nil
		}

		c, found := s.contracts[inst.Spawn.ContractID]
		if !found {
			return nil, nil, errors.New("couldn't find this contract type: " + inst.Spawn.ContractID)
		}
		return c(cdb, inst, coins)

	case InvokeType:
		switch inst.Invoke.Command {
		case "evolve":
			var darcID darc.ID
			_, _, darcID, err = cdb.GetValues(inst.InstanceID.Slice())
			if err != nil {
				return
			}

			darcBuf := inst.Invoke.Args.Search("darc")
			newD, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, err
			}
			oldD, err := LoadDarcFromTrie(cdb, darcID)
			if err != nil {
				return nil, nil, err
			}
			if err := newD.SanityCheck(oldD); err != nil {
				return nil, nil, err
			}
			return []StateChange{
				NewStateChange(Update, inst.InstanceID, ContractDarcID, darcBuf, darcID),
			}, coins, nil
		default:
			return nil, nil, errors.New("invalid command: " + inst.Invoke.Command)
		}
	case DeleteType:
		return nil, nil, errors.New("delete on a Darc instance is not supported")
	default:
		return nil, nil, errors.New("unknown instruction type")
	}
}

// loadConfigFromTrie loads the configuration data from the trie.
func loadConfigFromTrie(st ReadOnlyStateTrie) (*ChainConfig, error) {
	// Find the genesis-darc ID.
	val, contract, _, err := getValueContract(st, NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	if string(contract) != ContractConfigID {
		return nil, errors.New("did not get " + ContractConfigID)
	}

	config := ChainConfig{}
	err = protobuf.DecodeWithConstructors(val, &config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func getValueContract(st ReadOnlyStateTrie, key []byte) (value []byte, contract string, darcID darc.ID, err error) {
	value, contract, darcID, err = st.GetValues(key)
	if err != nil {
		return
	}
	if value == nil {
		err = errKeyNotSet
		return
	}
	return
}

func getInstanceDarc(c ReadOnlyStateTrie, iid InstanceID) (*darc.Darc, error) {
	// From instance ID, find the darcID that controls access to it.
	_, _, dID, err := c.GetValues(iid.Slice())
	if err != nil {
		return nil, err
	}

	// Fetch the darc itself.
	value, contract, _, err := c.GetValues(dID)
	if err != nil {
		return nil, err
	}

	if string(contract) != ContractDarcID {
		return nil, fmt.Errorf("for instance %v, expected Kind to be 'darc' but got '%v'", iid, string(contract))
	}
	return darc.NewFromProtobuf(value)
}
