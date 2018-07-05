package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// oneSubID is the subid for storing the OmniLedger config.
var oneSubID = SubID(func() [32]byte {
	var one [32]byte
	one[31] = 1
	return one
}())

// zeroDarc is a DarcID with all zeroes.
var zeroDarc = darc.ID(make([]byte, 32))

// GenesisReferenceID is 64 bytes of zeroes. Its value is a reference to the
// genesis-darc.
var GenesisReferenceID = InstanceID{zeroDarc, SubID{}}

// ContractConfigID denotes a config-contract
var ContractConfigID = "config"

// ContractDarcID denotes a darc-contract
var ContractDarcID = "darc"

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
		SubID:  oneSubID,
	}
	val, contract, err = getValueContract(coll, configID.Slice())
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

// ContractConfig can only be instantiated once per skipchain, and only for
// the genesis block.
func (s *Service) ContractConfig(cdb CollectionView, scID skipchain.SkipBlockID, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	if inst.GetType() == SpawnType {
		return spawnContractConfig(cdb, inst, coins)
	} else if inst.GetType() == InvokeType {
		return s.invokeContractConfig(cdb, scID, inst, coins)
	} else {
		return nil, coins, errors.New("unsupported instruction type")
	}
}

func (s *Service) invokeContractConfig(cdb CollectionView, scID skipchain.SkipBlockID, inst Instruction, coins []Coin) (sc []StateChange, c []Coin, err error) {
	c = coins
	// There are two situations where we need to change the roster, first
	// is when it is initiated by the client(s) that holds the genesis
	// signing key, in thise case we trust the client to do the right
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

		if newConfig.BlockInterval <= 0 {
			err = errors.New("block interval is less or equal to zero")
			return
		}

		return []StateChange{
			NewStateChange(Update, InstanceID{
				DarcID: inst.InstanceID.DarcID,
				SubID:  oneSubID,
			}, ContractConfigID, configBuf),
		}, c, nil
	} else if inst.Invoke.Command == "view_change" {
		config := &ChainConfig{}
		config, err = LoadConfigFromColl(cdb)
		if err != nil {
			return
		}
		newRosterBuf := inst.Invoke.Args.Search("roster")
		newRoster := onet.Roster{}
		err = protobuf.DecodeWithConstructors(newRosterBuf, &newRoster, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return
		}
		if err = validRotation(config.Roster, newRoster); err != nil {
			return
		}
		if err = s.withinInterval(scID, inst.Signatures[0].Signer.Ed25519.Point); err != nil {
			return
		}
		sc, err = updateRosterScs(cdb, inst.InstanceID.DarcID, newRoster)
		return
	}
	err = errors.New("invalid invoke command: " + inst.Invoke.Command)
	return
}

func updateRosterScs(cdb CollectionView, darcID darc.ID, newRoster onet.Roster) (StateChanges, error) {
	config, err := LoadConfigFromColl(cdb)
	if err != nil {
		return nil, err
	}
	config.Roster = newRoster
	configBuf, err := protobuf.Encode(config)
	if err != nil {
		return nil, err
	}
	return []StateChange{
		NewStateChange(Update, InstanceID{
			DarcID: darcID,
			SubID:  oneSubID,
		}, ContractConfigID, configBuf),
	}, nil
}

func validRotation(oldRoster, newRoster onet.Roster) error {
	n := len(oldRoster.List)
	if n != len(newRoster.List) {
		return fmt.Errorf("rosters lengths are not equal, need %d but got %d", n, len(newRoster.List))
	}

	var offset int
	for _, sid := range newRoster.List {
		if sid.Equal(oldRoster.List[0]) {
			break
		}
		offset++
	}
	for i := 0; i < n; i++ {
		if !oldRoster.List[i].Equal(newRoster.List[(i+offset)%n]) {
			return errors.New("invalid rotation")
		}
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
	if len(d.Rules) == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
		log.Error("couldn't verify darc")
		return
	}

	// sanity check the block interval
	intervalBuf := inst.Spawn.Args.Search("block_interval")
	interval, _ := binary.Varint(intervalBuf)
	if interval <= 0 {
		err = errors.New("block interval is less or equal to zero")
		return
	}

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
	}
	configBuf, err := protobuf.Encode(&config)
	if err != nil {
		return
	}

	return []StateChange{
		NewStateChange(Create, GenesisReferenceID, ContractConfigID, inst.InstanceID.DarcID),
		NewStateChange(Create, inst.InstanceID, ContractDarcID, darcBuf),
		NewStateChange(Create,
			InstanceID{
				DarcID: inst.InstanceID.DarcID,
				SubID:  oneSubID,
			}, ContractConfigID, configBuf),
	}, c, nil

}

// ContractDarc accepts the following instructions:
//   - Spawn - creates a new darc
//   - Invoke.Evolve - evolves an existing darc
func (s *Service) ContractDarc(coll CollectionView, scID skipchain.SkipBlockID, inst Instruction,
	coins []Coin) ([]StateChange, []Coin, error) {
	switch {
	case inst.Spawn != nil:
		if inst.Spawn.ContractID == ContractDarcID {
			darcBuf := inst.Spawn.Args.Search("darc")
			d, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, errors.New("given darc could not be decoded: " + err.Error())
			}
			return []StateChange{
				NewStateChange(Create, InstanceID{d.GetBaseID(), SubID{}}, ContractDarcID, darcBuf),
			}, coins, nil
		}
		// TODO The code below will never get called because this
		// contract is used only when tx.Spawn.ContractID is "darc", so
		// the if statement above gets executed and this contract
		// returns. Why do we need this part, if we do, how should we
		// fix it?
		c, found := s.contracts[inst.Spawn.ContractID]
		if !found {
			return nil, nil, errors.New("couldn't find this contract type")
		}
		return c(coll, scID, inst, coins)
	case inst.Invoke != nil:
		switch inst.Invoke.Command {
		case "evolve":
			darcBuf := inst.Invoke.Args.Search("darc")
			newD, err := darc.NewFromProtobuf(darcBuf)
			if err != nil {
				return nil, nil, err
			}
			oldD, err := LoadDarcFromColl(coll, InstanceID{newD.BaseID, SubID{}}.Slice())
			if err != nil {
				return nil, nil, err
			}
			if err := newD.SanityCheck(oldD); err != nil {
				return nil, nil, err
			}
			return []StateChange{
				NewStateChange(Update, inst.InstanceID, ContractDarcID, darcBuf),
			}, coins, nil
		default:
			return nil, nil, errors.New("invalid command: " + inst.Invoke.Command)
		}
	default:
		return nil, nil, errors.New("Only invoke and spawn are defined yet")
	}
}
