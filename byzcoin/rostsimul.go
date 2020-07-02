package byzcoin

import (
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/protobuf"
)

// ROSTSimul makes it easy to test contracts without having to use the whole
// of byzcoin. It is a simulation of a ReadOnlyStateTrie without the consensus
// algorithm of ByzCoin.
// Several convenience methods make it easy to do basic tasks.
type ROSTSimul struct {
	Values  map[string]StateChangeBody
	Version Version
}

// NewROSTSimul returns a new ReadOnlyStateTrie that can be used to simulate
// contracts.
func NewROSTSimul() *ROSTSimul {
	return &ROSTSimul{
		Values:  make(map[string]StateChangeBody),
		Version: CurrentVersion,
	}
}

const coinID = "coin"

// GetValues implements the interface of a ReadOnlyStateTrie
func (s *ROSTSimul) GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error) {
	scb, ok := s.Values[string(key)]
	if !ok {
		err = errors.New("this key doesn't exist")
		return
	}
	value = scb.Value
	version = scb.Version
	contractID = scb.ContractID
	darcID = scb.DarcID
	return
}

// GetProof is not implemented yet
func (s *ROSTSimul) GetProof(key []byte) (*trie.Proof, error) {
	return nil, errors.New("not implemented")
}

// GetIndex always returns -1
func (s *ROSTSimul) GetIndex() int {
	return -1
}

// GetVersion returns the version stored,
// which can be updated to test contracts.
func (s *ROSTSimul) GetVersion() Version {
	return s.Version
}

// GetNonce is not implemented.
func (s *ROSTSimul) GetNonce() ([]byte, error) {
	return nil, errors.New("not implemented")
}

// ForEach is not implemented.
func (s *ROSTSimul) ForEach(func(k, v []byte) error) error {
	return errors.New("not implemented")
}

// LoadConfig is not implemented
func (s *ROSTSimul) LoadConfig() (*ChainConfig, error) {
	return nil, errors.New("not implemented")
}

// LoadDarc is not implemented
func (s *ROSTSimul) LoadDarc(id darc.ID) (*darc.Darc, error) {
	return nil, errors.New("not implemented")
}

// StoreAllToReplica stores all stateChanges, without checking for validity!
func (s *ROSTSimul) StoreAllToReplica(scs StateChanges) (ReadOnlyStateTrie, error) {
	for _, sc := range scs {
		s.Values[string(sc.InstanceID)] = StateChangeBody{
			StateAction: Update,
			ContractID:  sc.ContractID,
			Value:       sc.Value,
			Version:     sc.Version,
			DarcID:      sc.DarcID,
		}
	}
	return s, nil
}

// GetSignerCounter is not implemented
func (s *ROSTSimul) GetSignerCounter(id darc.Identity) (uint64, error) {
	return 0, fmt.Errorf("not yet implemented")
}

//
// Convenience methods for easier usage in tests
//

// CreateBasicDarc stores a darc with the _sign and "invoke:darc.
// evolve" rules set to the signer.
func (s *ROSTSimul) CreateBasicDarc(signer *darc.Identity, desc string) (*darc.Darc,
	error) {
	if signer == nil {
		id := darc.NewIdentityEd25519(cothority.Suite.Point())
		signer = &id
	}
	ids := []darc.Identity{*signer}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte(desc))
	err := s.CreateSCB(Create, ContractDarcID,
		NewInstanceID(d.GetBaseID()), d, nil)
	if err != nil {
		return nil, fmt.Errorf("store darc: %v", err)
	}
	return d, nil
}

// CreateCoin stores a new coin under a new random ID.
func (s *ROSTSimul) CreateCoin(name string, value uint64) (id InstanceID,
	err error) {
	coin := Coin{Name: NewInstanceID([]byte(name)),
		Value: value}
	return s.CreateRandomInstance(coinID, &coin, nil)
}

// CreateRandomInstance adds a new instance given by the contractID and the
// contract-structure. The ID will be chosen randomly,
// the darcID can be nil, in which case it will be initialized to all-zeros.
func (s *ROSTSimul) CreateRandomInstance(cID string,
	value interface{}, darcID darc.ID) (id InstanceID, err error) {
	id = NewInstanceID(random.Bits(256, true, random.New()))
	err = s.CreateSCB(Create, cID, id, value, darcID)
	return
}

// CreateSCB adds a new instance given by the contractID and the
// contract-structure. The ID will be chosen randomly,
// the darcID can be nil, in which case it will be initialized to all-zeros.
func (s *ROSTSimul) CreateSCB(sa StateAction, cID string, id InstanceID,
	value interface{}, darcID darc.ID) (err error) {
	valueBuf, err := protobuf.Encode(value)
	if err != nil {
		err = fmt.Errorf("couldn't encode coin: %v", err)
	}
	if darcID == nil {
		darcID = make([]byte, 32)
	}
	version := uint64(0)
	if sa == Update {
		version = s.Values[string(id[:])].Version + 1
	}
	s.Values[string(id[:])] = StateChangeBody{
		StateAction: sa,
		ContractID:  cID,
		Value:       valueBuf,
		Version:     version,
		DarcID:      darcID,
	}
	return nil
}

// SetCoin 'mints' coins by setting its value directly.
func (s *ROSTSimul) SetCoin(id InstanceID, value uint64) error {
	coin, err := s.GetCoin(id)
	if err != nil {
		return fmt.Errorf("couldn't get coin: %v", err)
	}
	coin.Value = value
	return s.CreateSCB(Update, coinID, id, &coin, nil)
}

// GetCoin returns a coin given its id.
func (s *ROSTSimul) GetCoin(id InstanceID) (coin Coin, err error) {
	sc, found := s.Values[string(id[:])]
	if !found {
		err = errors.New("didn't find this coin")
		return
	}
	if sc.ContractID != coinID {
		err = fmt.Errorf("id doesn't point to a coin, but to '%s'",
			sc.ContractID)
		return
	}
	if err = protobuf.Decode(sc.Value, &coin); err != nil {
		err = fmt.Errorf("couldn't decode coin: %v", err)
	}
	return
}

// WithdrawCoin looks up given coin if it has enough value. If yes,
// withdraws that value,
// updates the coin, and returns the updated coin as well as the withdrawn
// coin.
func (s *ROSTSimul) WithdrawCoin(id InstanceID,
	value uint64) (updated, withdrawn Coin, err error) {
	updated, err = s.GetCoin(id)
	if err != nil {
		err = fmt.Errorf("couldn't get coin: %v", err)
		return
	}
	if updated.Value < value {
		err = errors.New("coin doesn't have enough value")
		return
	}
	updated.Value -= value
	err = s.SetCoin(id, updated.Value)
	if err != nil {
		err = fmt.Errorf("couldn't set coin to new value: %v", err)
		return
	}
	withdrawn.Name = updated.Name
	withdrawn.Value = value
	return
}
