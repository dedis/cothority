package byzcoin

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin/viewchange"
	"github.com/dedis/cothority/darc"
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

// loadConfigFromColl loads the configuration data from the collections.
func loadConfigFromColl(coll CollectionView) (*ChainConfig, error) {
	// Find the genesis-darc ID.
	val, contract, _, err := getValueContract(coll, NewInstanceID(nil).Slice())
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
	if string(contractBuf) != ContractDarcID {
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

// ContractConfig can only be instantiated once per skipchain, and only for
// the genesis block.
func (s *Service) ContractConfig(cdb CollectionView, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	// Verify the darc signature if the config instance does not exist yet.
	pr, err := cdb.Get(ConfigInstanceID.Slice()).Proof()
	if err != nil {
		return
	}
	if pr.Match() {
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

func invokeContractConfig(cdb CollectionView, inst Instruction, coins []Coin) (sc []StateChange, cOut []Coin, err error) {
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
		oldConfig, err = loadConfigFromColl(cdb)
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

func updateRosterScs(cdb CollectionView, darcID darc.ID, newRoster onet.Roster) (StateChanges, error) {
	config, err := loadConfigFromColl(cdb)
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

func validRotation(oldRoster, newRoster onet.Roster) error {
	if !oldRoster.IsRotation(&newRoster) {
		return errors.New("the new roster is not a valid rotation of the old roster")
	}
	newRoster2 := onet.NewRoster(newRoster.List)
	if !newRoster2.ID.Equal(newRoster.ID) {
		return errors.New("re-created roster does not have the same ID")
	}
	if !newRoster2.Aggregate.Equal(newRoster.Aggregate) {
		return errors.New("re-created roster does not have the same aggregate public key")
	}
	return nil
}

func spawnContractConfig(cdb CollectionView, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
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
func (s *Service) ContractDarc(cdb CollectionView, inst Instruction, coins []Coin) (sc []StateChange, cOut []Coin, err error) {
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
			oldD, err := LoadDarcFromColl(cdb, darcID)
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
