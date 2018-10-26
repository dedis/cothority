package byzcoin

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sort"
	"sync"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

const bucketStateChangeStorage = "statechangestorage"

// StateChangeEntry is the object stored to keep track of instance history. It
// contains the state change and the block index
type StateChangeEntry struct {
	StateChange StateChange
	BlockIndex  int
}

// StateChangeEntries is a list of StateChangeEntry and it can be marshaled
// and unmarshaled
type StateChangeEntries []StateChangeEntry

// Marshal will encode the list of entries into a byte array that
// can be store inside the database
func (sce StateChangeEntries) Marshal() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(sce)

	return b.Bytes(), err
}

// Unmarshal will decode a byte array and try populate it as
// a list of StateChangeEntry
func (sce *StateChangeEntries) Unmarshal(buf []byte) error {
	b := bytes.Buffer{}
	_, err := b.Write(buf)
	if err != nil {
		return err
	}

	d := gob.NewDecoder(&b)

	return d.Decode(sce)
}

// Len returns the length of the array
func (sce StateChangeEntries) Len() int {
	return len(sce)
}

// Swap swaps the position of two elements in the array
func (sce StateChangeEntries) Swap(i, j int) {
	sce[i], sce[j] = sce[j], sce[i]
}

// Less returns true when i's version is less than j's
func (sce StateChangeEntries) Less(i, j int) bool {
	return sce[i].StateChange.Version < sce[j].StateChange.Version
}

type stateChangeStorage struct {
	db *bolt.DB
}

func newStateChangeStorage(c *onet.Context) *stateChangeStorage {
	db, _ := c.GetAdditionalBucket([]byte(bucketStateChangeStorage))
	return &stateChangeStorage{db: db}
}

// this will append the state changes at the end of the list
// of the right instance for each state and then sort them by version
func (s *stateChangeStorage) append(scs StateChanges) error {
	// prepare the state changes per InstanceID
	sorted := make(map[string]StateChangeEntries)
	for _, sc := range scs {
		_, ok := sorted[string(sc.InstanceID)]
		if !ok {
			sorted[string(sc.InstanceID)] = make(StateChangeEntries, 0)
		}

		sorted[string(sc.InstanceID)] = append(sorted[string(sc.InstanceID)], StateChangeEntry{StateChange: sc})
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketStateChangeStorage))
		if err != nil {
			return err
		}

		// append each list of state changes (or create the entry)
		for _, scs := range sorted {
			key := scs[0].StateChange.InstanceID

			sort.Sort(scs)

			// Get the previous versions if any
			v := b.Get(key)
			var prev StateChangeEntries
			if v != nil {
				if err := prev.Unmarshal(v); err != nil {
					return err
				}
			}

			if len(prev) == 0 {
				prev = scs
			} else {
				for _, sc := range scs {
					prevVersion := prev[len(prev)-1].StateChange.Version

					if sc.StateChange.Version == prevVersion+1 {
						prev = append(prev, sc)
					} else if sc.StateChange.Version > prevVersion {
						return errors.New("State change version mismatch")
					}

					// The last case only means the state change has been already
					// stored so we ignore it
				}
			}

			buf, err := prev.Marshal()
			if err != nil {
				return err
			}

			err = b.Put(key, buf)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// This will return the list of state changes for the given instance
func (s *stateChangeStorage) getAll(iid []byte) (entries StateChangeEntries, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketStateChangeStorage))
		v := b.Get(iid[:])

		if v != nil {
			return entries.Unmarshal(v)
		}

		return nil
	})

	return
}

// This will return the state change entry for the given instance and version.
// Use the bool return value to check if the version exists
func (s *stateChangeStorage) getByVersion(iid []byte, v int) (StateChangeEntry, bool, error) {
	entries, err := s.getAll(iid)
	if err != nil {
		return StateChangeEntry{}, false, err
	}

	for _, e := range entries {
		if e.StateChange.Version == v {
			return e, true, nil
		}
	}

	return StateChangeEntry{}, false, nil
}

// SafeAdd will add a to the value of the coin if there will be no
// overflow.
func (c *Coin) SafeAdd(a uint64) error {
	s1 := c.Value + a
	if s1 < c.Value || s1 < a {
		return errors.New("uint64 overflow")
	}
	c.Value = s1
	return nil
}

// SafeSub subtracts a from the value of the coin if there
// will be no underflow.
func (c *Coin) SafeSub(a uint64) error {
	if a <= c.Value {
		c.Value -= a
		return nil
	}
	return errors.New("uint64 underflow")
}

type bcNotifications struct {
	sync.Mutex
	// waitChannels will be informed by Service.updateTrieCallback that a
	// given ClientTransaction has been included. updateTrieCallback will
	// send true for a valid ClientTransaction and false for an invalid
	// ClientTransaction.
	waitChannels map[string]chan bool
	// blockListeners will be notified every time a block is created.
	// It is up to them to filter out block creations on chains they are not
	// interested in.
	blockListeners []chan skipchain.SkipBlockID
}

func (bc *bcNotifications) createWaitChannel(ctxHash []byte) chan bool {
	bc.Lock()
	defer bc.Unlock()
	ch := make(chan bool, 1)
	bc.waitChannels[string(ctxHash)] = ch
	return ch
}

func (bc *bcNotifications) informWaitChannel(ctxHash []byte, valid bool) {
	bc.Lock()
	defer bc.Unlock()
	ch := bc.waitChannels[string(ctxHash)]
	if ch != nil {
		ch <- valid
	}
}

func (bc *bcNotifications) deleteWaitChannel(ctxHash []byte) {
	bc.Lock()
	defer bc.Unlock()
	delete(bc.waitChannels, string(ctxHash))
}

func (bc *bcNotifications) informBlock(id skipchain.SkipBlockID) {
	bc.Lock()
	defer bc.Unlock()
	for _, x := range bc.blockListeners {
		if x != nil {
			x <- id
		}
	}
}

func (bc *bcNotifications) registerForBlocks(ch chan skipchain.SkipBlockID) int {
	bc.Lock()
	defer bc.Unlock()

	for i := 0; i < len(bc.blockListeners); i++ {
		if bc.blockListeners[i] == nil {
			bc.blockListeners[i] = ch
			return i
		}
	}

	// If we got here, no empty spots left, append and return the position of the
	// new element (on startup: after append(nil, ch), len == 1, so len-1 = index 0.
	bc.blockListeners = append(bc.blockListeners, ch)
	return len(bc.blockListeners) - 1
}

func (bc *bcNotifications) unregisterForBlocks(i int) {
	bc.Lock()
	defer bc.Unlock()
	bc.blockListeners[i] = nil
}

func (c ChainConfig) sanityCheck(old *ChainConfig) error {
	if c.BlockInterval <= 0 {
		return errors.New("block interval is less or equal to zero")
	}
	// too small would make it impossible to even send through a config update tx to fix it,
	// so don't allow that.
	if c.MaxBlockSize < 16000 {
		return errors.New("max block size is less than 16000")
	}
	// onet/network.MaxPacketSize is 10 megs, leave some headroom anyway.
	if c.MaxBlockSize > 8*1e6 {
		return errors.New("max block size is greater than 8 megs")
	}
	if len(c.Roster.List) < 3 {
		return errors.New("need at least 3 nodes to have a majority")
	}
	if old != nil {
		return old.checkNewRoster(c.Roster)
	}
	return nil
}

// checkNewRoster makes sure that the new roster follows the rules we need
// in byzcoin:
//   - no new node can join as leader
//   - only one node joining or leaving
func (c ChainConfig) checkNewRoster(newRoster onet.Roster) error {
	// Check new leader was in old roster
	if index, _ := c.Roster.Search(newRoster.List[0].ID); index < 0 {
		return errors.New("new leader must be in previous roster")
	}

	// Check we don't change more than one one
	added := 0
	oldList := onet.NewRoster(c.Roster.List)
	for _, si := range newRoster.List {
		if i, _ := oldList.Search(si.ID); i >= 0 {
			oldList.List = append(oldList.List[:i], oldList.List[i+1:]...)
		} else {
			added++
		}
	}
	if len(oldList.List)+added > 1 {
		return errors.New("can only change one node at a time - adding or removing")
	}
	return nil
}
