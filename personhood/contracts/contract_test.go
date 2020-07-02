package contracts

import (
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/protobuf"

	"go.dedis.ch/kyber/v3/util/random"

	"go.dedis.ch/cothority/v3"

	"go.dedis.ch/cothority/v3/byzcoin/trie"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
)

type rstSimul struct {
	values  map[string]byzcoin.StateChangeBody
	version byzcoin.Version
}

func newRstSimul() *rstSimul {
	return &rstSimul{
		values:  make(map[string]byzcoin.StateChangeBody),
		version: byzcoin.CurrentVersion,
	}
}

func (s *rstSimul) GetValues(key []byte) (value []byte, version uint64, contractID string, darcID darc.ID, err error) {
	scb, ok := s.values[string(key)]
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
func (s *rstSimul) GetProof(key []byte) (*trie.Proof, error) {
	return nil, errors.New("not implemented")
}
func (s *rstSimul) GetIndex() int {
	return -1
}
func (s *rstSimul) GetVersion() byzcoin.Version {
	return s.version
}
func (s *rstSimul) GetNonce() ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (s *rstSimul) ForEach(func(k, v []byte) error) error {
	return errors.New("not implemented")
}
func (s *rstSimul) StoreAllToReplica(scs byzcoin.StateChanges) (byzcoin.ReadOnlyStateTrie, error) {
	return nil, errors.New("not implemented")
}
func (s *rstSimul) Process(scs byzcoin.StateChanges) {
	for _, sc := range scs {
		s.values[string(sc.InstanceID)] = byzcoin.StateChangeBody{
			StateAction: byzcoin.Update,
			ContractID:  sc.ContractID,
			Value:       sc.Value,
			Version:     sc.Version,
			DarcID:      sc.DarcID,
		}
	}
}

func (s *rstSimul) GetSignerCounter(id darc.Identity) (uint64, error) {
	return 0, fmt.Errorf("not yet implemented")
}

func (s *rstSimul) addDarc(signer *darc.Identity, desc string) (*darc.Darc,
	error) {
	if signer == nil {
		id := darc.NewIdentityEd25519(cothority.Suite.Point())
		signer = &id
	}
	ids := []darc.Identity{*signer}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte(desc))
	buf, err := d.ToProto()
	if err != nil {
		return nil, fmt.Errorf("couldn't convert darc to protobuf: %v",
			err)
	}
	s.values[string(d.GetBaseID())] = byzcoin.StateChangeBody{
		Value:      buf,
		Version:    0,
		ContractID: byzcoin.ContractDarcID,
		DarcID:     nil,
	}
	return d, nil
}

func (s *rstSimul) createCoin(name string, value uint64) (id byzcoin.InstanceID,
	err error) {
	coin := byzcoin.Coin{Name: byzcoin.NewInstanceID([]byte(name)),
		Value: value}
	id = byzcoin.NewInstanceID(random.Bits(256, true, random.New()))
	coinBuf, err := protobuf.Encode(&coin)
	if err != nil {
		err = fmt.Errorf("couldn't encode coin: %v", err)
	}
	s.values[string(id[:])] = byzcoin.StateChangeBody{
		StateAction: byzcoin.Create,
		ContractID:  contracts.ContractCoinID,
		Value:       coinBuf,
		Version:     0,
		DarcID:      emptyInstance[:],
	}
	return
}

func (s *rstSimul) setCoin(id byzcoin.InstanceID, value uint64) error {
	coin, err := s.getCoin(id)
	if err != nil {
		return fmt.Errorf("couldn't get coin: %v", err)
	}
	coin.Value = value
	coinBuf, err := protobuf.Encode(&coin)
	if err != nil {
		return fmt.Errorf("couldn't encode coin: %v", err)
	}
	s.values[string(id[:])] = byzcoin.StateChangeBody{
		StateAction: byzcoin.Update,
		ContractID:  contracts.ContractCoinID,
		Value:       coinBuf,
		Version:     0,
		DarcID:      emptyInstance[:],
	}
	return nil
}

func (s *rstSimul) getCoin(id byzcoin.InstanceID) (coin byzcoin.Coin, err error) {
	sc, found := s.values[string(id[:])]
	if !found {
		err = errors.New("didn't find this coin")
		return
	}
	if sc.ContractID != contracts.ContractCoinID {
		err = fmt.Errorf("id doesn't point to a coin, but to '%s'",
			sc.ContractID)
		return
	}
	if err = protobuf.Decode(sc.Value, &coin); err != nil {
		err = fmt.Errorf("couldn't decode coin: %v", err)
	}
	return
}

/**
 * Looks up given coin if it has enough value. If yes, withdraws that value,
 * updates the coin, and returns the updated coin as well as the withdrawn
 * coin.
 */
func (s *rstSimul) withdrawCoin(id byzcoin.InstanceID,
	value uint64) (updated, withdrawn byzcoin.Coin, err error) {
	updated, err = s.getCoin(id)
	if err != nil {
		err = fmt.Errorf("couldn't get coin: %v", err)
		return
	}
	if updated.Value < value {
		err = errors.New("coin doesn't have enough value")
		return
	}
	updated.Value -= value
	err = s.setCoin(id, updated.Value)
	if err != nil {
		err = fmt.Errorf("couldn't set coin to new value: %v", err)
		return
	}
	withdrawn.Name = updated.Name
	withdrawn.Value = value
	return
}
