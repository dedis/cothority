package trie

import (
	"sync"

	"golang.org/x/xerrors"
)

// OpType is the operation type that modifies state.
type OpType int

const (
	// OpSet is the set operation.
	OpSet OpType = iota + 1
	// OpDel is the delete operation.
	OpDel
)

type instr struct {
	ty OpType
	k  []byte
	v  []byte
}

// StagingTrie represents a lazy copy of a Trie for staging operations. The
// keys and values stored in this object will not go into the source Trie from
// which it is created until the Commit function is called. The StagingTrie
// becomes invalid if the source Trie is modified directly.
type StagingTrie struct {
	source     *Trie
	overlay    map[string][]byte
	deleteList map[string][]byte
	instrList  []instr

	sync.Mutex
}

// GetNonce returns the nonce from the source Trie.
func (t *StagingTrie) GetNonce() ([]byte, error) {
	return t.source.nonce, nil
}

// Clone makes a clone of the uncommitted data of the staging trie. The source
// trie used for creating the staging trie is not cloned.
func (t *StagingTrie) Clone() *StagingTrie {
	t.Lock()
	defer t.Unlock()
	out := StagingTrie{
		source:     t.source,
		overlay:    make(map[string][]byte),
		deleteList: make(map[string][]byte),
		instrList:  nil,
	}
	for k, v := range t.overlay {
		val := clone(v)
		out.overlay[k] = val
	}
	for k, v := range t.deleteList {
		val := clone(v)
		out.deleteList[k] = val
	}
	out.instrList = make([]instr, len(t.instrList))
	copy(out.instrList, t.instrList)
	return &out
}

// GetMetadata gets the source's metadata, which gets a value associated with
// the key in the metadata namespace. The key must be no more than 31 bytes. If
// the key does not exist, nil is returned.
func (t *StagingTrie) GetMetadata(key []byte) []byte {
	return t.source.GetMetadata(key)
}

// Get gets the value for the given key.
func (t *StagingTrie) Get(k []byte) ([]byte, error) {
	t.Lock()
	defer t.Unlock()

	if t.isDeleted(k) {
		return nil, nil
	}

	if v, ok := t.overlay[string(k)]; ok {
		return v, nil
	}
	return t.source.Get(k)
}

// Set sets a key/value pair, it will overwrite if necessary.
func (t *StagingTrie) Set(k, v []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.set(k, v)
}

func (t *StagingTrie) set(k, v []byte) error {
	delete(t.deleteList, string(k))
	t.overlay[string(k)] = v

	t.instrList = append(t.instrList, instr{
		ty: OpSet,
		k:  k,
		v:  v,
	})
	return nil
}

// Delete deletes a key/value pair.
func (t *StagingTrie) Delete(k []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.del(k)
}

func (t *StagingTrie) del(k []byte) error {
	t.deleteList[string(k)] = nil
	delete(t.overlay, string(k))

	t.instrList = append(t.instrList, instr{
		ty: OpDel,
		k:  k,
		v:  nil,
	})
	return nil
}

// Batch is similar to Set, but for multiple key-value pairs.
func (t *StagingTrie) Batch(pairs []KVPair) error {
	t.Lock()
	defer t.Unlock()
	for _, p := range pairs {
		switch p.Op() {
		case OpSet:
			if err := t.set(p.Key(), p.Val()); err != nil {
				return err
			}
		case OpDel:
			if err := t.del(p.Key()); err != nil {
				return err
			}
		default:
			return xerrors.New("no such operation")
		}
	}
	return nil
}

// Commit commits all operations performed on the StagingTrie since creation
// or the previous commit to the source Trie.
func (t *StagingTrie) Commit() error {
	t.Lock()
	defer t.Unlock()
	err := t.source.db.Update(func(b Bucket) error {
		for _, instr := range t.instrList {
			switch instr.ty {
			case OpSet:
				if err := t.source.SetWithBucket(instr.k, instr.v, b); err != nil {
					return err
				}
			case OpDel:
				if err := t.source.DeleteWithBucket(instr.k, b); err != nil {
					return err
				}
			default:
				return xerrors.New("invalid instruction during commit")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	t.overlay = make(map[string][]byte)
	t.deleteList = make(map[string][]byte)
	t.instrList = nil
	return nil
}

// GetRoot returns the root of the trie.
func (t *StagingTrie) GetRoot() []byte {
	t.Lock()
	defer t.Unlock()
	var root []byte
	err := t.source.db.UpdateDryRun(func(b Bucket) error {
		for _, instr := range t.instrList {
			switch instr.ty {
			case OpSet:
				if err := t.source.SetWithBucket(instr.k, instr.v, b); err != nil {
					return err
				}
			case OpDel:
				if err := t.source.DeleteWithBucket(instr.k, b); err != nil {
					return err
				}
			default:
				return xerrors.New("invalid instruction during get root")
			}
		}
		root = clone(t.source.GetRootWithBucket(b))
		return nil
	})
	if err != nil {
		return nil
	}
	return root
}

// GetProof gets the inclusion/absence proof for the given key.
func (t *StagingTrie) GetProof(key []byte) (*Proof, error) {
	t.Lock()
	defer t.Unlock()
	p := &Proof{}
	err := t.source.db.UpdateDryRun(func(b Bucket) error {
		// run the pending instructions
		for _, instr := range t.instrList {
			switch instr.ty {
			case OpSet:
				if err := t.source.SetWithBucket(instr.k, instr.v, b); err != nil {
					return err
				}
			case OpDel:
				if err := t.source.DeleteWithBucket(instr.k, b); err != nil {
					return err
				}
			default:
				return xerrors.New("invalid instruction during get proof")
			}
		}
		// create the proof
		rootKey := t.source.GetRootWithBucket(b)
		if rootKey == nil {
			return xerrors.New("no root key")
		}
		p.Nonce = clone(t.source.nonce)
		return t.source.getProof(0, rootKey, t.source.binSlice(key), p, b)
	})
	return p, err
}

func (t *StagingTrie) isDeleted(k []byte) bool {
	if _, ok := t.deleteList[string(k)]; ok {
		return true
	}
	return false
}

// ForEach runs the callback cb on every key/value pair of the trie. The
// iteration stops and the function returns an error when the callback returns
// an error.
func (t *StagingTrie) ForEach(cb func(k, v []byte) error) error {
	// iterate over the overlay
	// iterate over the items that are not deleted in the trie
	t.Lock()
	defer t.Unlock()

	for k, v := range t.overlay {
		if err := cb([]byte(k), v); err != nil {
			return err
		}
	}

	return t.source.ForEach(func(k, v []byte) error {
		if t.isDeleted(k) {
			return nil
		}
		if _, ok := t.overlay[string(k)]; ok {
			return nil
		}
		return cb(k, v)
	})
}

// sanityCheck checks the invariant: the deleted values does not appear in the
// overlay.
func (t *StagingTrie) sanityCheck() error {
	t.Lock()
	defer t.Unlock()
	for k := range t.deleteList {
		if _, ok := t.overlay[k]; ok {
			return xerrors.New("deleted key in overlay")
		}
	}
	return nil
}
