package trie

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
)

type OpType int

const (
	OpSet OpType = iota + 1
	OpDel
)

type instr struct {
	ty OpType
	k  []byte
	v  []byte
}

func (r *instr) toBytes() []byte {
	tyBuf := []byte{byte(r.ty)}
	lenK := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenK, uint32(len(r.k)))
	lenV := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenV, uint32(len(r.v)))

	res := append(tyBuf, lenK...)
	res = append(res, r.k...)
	res = append(res, lenV...)
	res = append(res, r.v...)
	return res
}

// EphemeralTrie represents an ephemeral lazy copy of a Trie. The keys and
// values stored in this object will not go into the Trie from which it is
// created until the Commit function is called. The EphemeralTrie becomes
// invalid if the source Trie is modified directly.
type EphemeralTrie struct {
	source     *Trie
	overlay    map[string][]byte
	deleteList map[string][]byte
	instrList  []instr

	sync.Mutex
}

func (t *EphemeralTrie) Clone() *EphemeralTrie {
	clone := EphemeralTrie{
		source: t.source,
	}
	for k, v := range t.overlay {
		val := make([]byte, len(v))
		copy(val, v)
		clone.overlay[k] = val
	}
	for k, v := range t.deleteList {
		val := make([]byte, len(v))
		copy(val, v)
		clone.deleteList[k] = val
	}
	clone.instrList = make([]instr, len(t.instrList))
	copy(clone.instrList, t.instrList)
	return &clone
}

// Get gets the value for the given key.
func (t *EphemeralTrie) Get(k []byte) ([]byte, error) {
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
func (t *EphemeralTrie) Set(k, v []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.set(k, v)
}

func (t *EphemeralTrie) set(k, v []byte) error {
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
func (t *EphemeralTrie) Delete(k []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.del(k)
}

func (t *EphemeralTrie) del(k []byte) error {
	t.deleteList[string(k)] = nil

	t.instrList = append(t.instrList, instr{
		ty: OpDel,
		k:  k,
		v:  nil,
	})
	return nil
}

func (t *EphemeralTrie) Batch(pairs []KVPair) error {
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
			return errors.New("no such operation")
		}
	}
	return nil
}

// Commit commits all operations performed on the EphemeralTrie since creation
// or the previous commit to the source Trie.
func (t *EphemeralTrie) Commit() error {
	err := t.source.db.Update(func(b bucket) error {
		for _, instr := range t.instrList {
			switch instr.ty {
			case OpSet:
				if err := t.source.startSet(instr.k, instr.v, b); err != nil {
					return err
				}
			case OpDel:
				if err := t.source.startDel(instr.k, b); err != nil {
					return err
				}
			default:
				return errors.New("invalid instruction during commit")
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

func (t *EphemeralTrie) GetRoot() []byte {
	var root []byte
	err := t.source.db.UpdateDryRun(func(b bucket) error {
		for _, instr := range t.instrList {
			switch instr.ty {
			case OpSet:
				if err := t.source.startSet(instr.k, instr.v, b); err != nil {
					return err
				}
			case OpDel:
				if err := t.source.startDel(instr.k, b); err != nil {
					return err
				}
			default:
				return errors.New("invalid instruction during commit")
			}
		}
		root = append([]byte{}, t.source.getRoot(b)...)
		return nil
	})
	if err != nil {
		return nil
	}
	return root
}

func (t *EphemeralTrie) GetProof(key []byte) (*Proof, error) {
	return nil, errors.New("not implemented")
}

// PendingOpsHash returns the hash of the pending operations.
func (t *EphemeralTrie) PendingOpsHash() []byte {
	h := sha256.New()
	// TODO hash length of t.instrList
	for _, instr := range t.instrList {
		h.Write(instr.toBytes())
	}
	return h.Sum(nil)
}

func (t *EphemeralTrie) isDeleted(k []byte) bool {
	if _, ok := t.deleteList[string(k)]; ok {
		return true
	}
	return false
}
