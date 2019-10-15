package byzcoin

import (
	"bytes"
	"sync"

	"go.dedis.ch/cothority/v4/skipchain"
	"golang.org/x/xerrors"
)

// stateChangeCache is a simple struct that maintains a cache of state changes
// keyed on the skipchain ID. It only keeps one value because state changes
// should only happen at block interval boundaries. So we do not expect
// interleaving state changes for the same skipchain. The advantage of this
// approach is that we do not need to worry about deleting used cache because
// the memory usage stays constant at one entry per Skipchain.
type stateChangeCache struct {
	sync.Mutex
	cache map[string]*stateChangeValue
}

type stateChangeValue struct {
	digest     []byte
	merkleRoot []byte
	txOut      []TxResult
	states     StateChanges
}

func newStateChangeCache() stateChangeCache {
	return stateChangeCache{
		cache: make(map[string]*stateChangeValue),
	}
}

func (c *stateChangeCache) get(scID skipchain.SkipBlockID, digest []byte) (merkleRoot []byte, txOut TxResults, states StateChanges, err error) {
	c.Lock()
	defer c.Unlock()
	key := string(scID)
	out, ok := c.cache[key]
	if !ok {
		err = xerrors.New("key does not exist")
		return
	}
	if !bytes.Equal(out.digest, digest) {
		err = xerrors.New("digest is not the same")
		return
	}

	merkleRoot = out.merkleRoot
	txOut = out.txOut
	states = out.states
	return
}

func (c *stateChangeCache) update(scID skipchain.SkipBlockID, digest []byte, merkleRoot []byte, txOut TxResults, states StateChanges) {
	c.Lock()
	defer c.Unlock()
	key := string(scID)
	c.cache[key] = &stateChangeValue{
		digest:     digest,
		merkleRoot: merkleRoot,
		txOut:      txOut,
		states:     states,
	}
}
