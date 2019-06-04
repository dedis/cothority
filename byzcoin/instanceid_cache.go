package byzcoin

import (
	"errors"
	"sync"

	"go.dedis.ch/cothority/v3/darc"
)

type instanceIDCache struct {
	sync.Mutex
	// Inner is a map from contract ID to a map of instance ID to darc ID.
	Inner map[string]map[InstanceID]darc.ID
}

func newInstanceIDCache() instanceIDCache {
	return instanceIDCache{
		Inner: make(map[string]map[InstanceID]darc.ID),
	}
}

func (c *instanceIDCache) rebuild(rst ReadOnlyStateTrie) error {
	tmpInner := make(map[string]map[InstanceID]darc.ID)
	err := rst.ForEach(func(k, v []byte) error {
		vals, err := decodeStateChangeBody(v)
		if err != nil {
			return err
		}

		// The convention is that if ContractID is empty then the entry
		// in the trie did not come from a contract, e.g., it came from
		// counters. So we return early here.
		if vals.ContractID == "" {
			return nil
		}

		var iid InstanceID
		copy(iid[:], k)

		if _, ok := tmpInner[vals.ContractID]; !ok {
			tmpInner[vals.ContractID] = make(map[InstanceID]darc.ID)
		}

		iidMap := tmpInner[vals.ContractID]
		iidMap[iid] = vals.DarcID
		tmpInner[vals.ContractID] = iidMap
		return nil
	})
	if err != nil {
		return err
	}
	c.Lock()
	defer c.Unlock()
	c.Inner = tmpInner
	return nil
}

func (c *instanceIDCache) update(scs StateChanges) error {
	c.Lock()
	defer c.Unlock()
	for _, sc := range scs {
		// Empty contract ID means the state change did not come from a
		// contract.
		if sc.ContractID == "" {
			continue
		}

		var iid InstanceID
		copy(iid[:], sc.InstanceID)

		// Handle create and delete separately.
		if sc.StateAction == Create {
			if _, ok := c.Inner[sc.ContractID]; !ok {
				c.Inner[sc.ContractID] = make(map[InstanceID]darc.ID)
			}
			iidMap := c.Inner[sc.ContractID]
			iidMap[iid] = sc.DarcID
			c.Inner[sc.ContractID] = iidMap
		} else if sc.StateAction == Remove {
			if _, ok := c.Inner[sc.ContractID]; !ok {
				return errors.New("cannot delete what does not exist")
			}
			iidMap := c.Inner[sc.ContractID]
			delete(iidMap, iid)
			c.Inner[sc.ContractID] = iidMap
		}
	}
	return nil
}

func (c *instanceIDCache) get(cid string) map[InstanceID]darc.ID {
	c.Lock()
	defer c.Unlock()
	out := make(map[InstanceID]darc.ID)
	for k, v := range c.Inner[cid] {
		out[k] = v
	}
	return out
}
