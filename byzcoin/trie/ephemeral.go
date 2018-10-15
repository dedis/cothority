package trie

import (
	"errors"
	"sync"
)

type instrType int

const (
	setInstr instrType = iota
	delInstr
)

type instr struct {
	ty instrType
	k  []byte
	v  []byte
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

	delete(t.deleteList, string(k))
	t.overlay[string(k)] = v

	t.instrList = append(t.instrList, instr{
		ty: setInstr,
		k:  k,
		v:  v,
	})
	return nil
}

// Delete deletes a key/value pair.
func (t *EphemeralTrie) Delete(k []byte) error {
	t.Lock()
	defer t.Unlock()

	t.deleteList[string(k)] = nil

	t.instrList = append(t.instrList, instr{
		ty: delInstr,
		k:  k,
		v:  nil,
	})
	return nil
}

// Commit commits all operations performed on the EphemeralTrie since creation
// or the previous commit to the source Trie.
func (t *EphemeralTrie) Commit() error {
	err := t.source.db.Update(func(b bucket) error {
		for _, instr := range t.instrList {
			switch instr.ty {
			case setInstr:
				if err := t.source.startSet(instr.k, instr.v, b); err != nil {
					return err
				}
			case delInstr:
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

func (t *EphemeralTrie) isDeleted(k []byte) bool {
	if _, ok := t.deleteList[string(k)]; ok {
		return true
	}
	return false
}
