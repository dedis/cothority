package trie

import (
	"bytes"

	"golang.org/x/xerrors"
)

const entryKey = "dedis_trie"
const nonceKey = "dedis_trie_nonce"
const metaMaxLen = 31

func isIllegalKey(buf []byte) bool {
	if bytes.Equal(buf, []byte(entryKey)) || bytes.Equal(buf, []byte(nonceKey)) {
		return true
	}
	return false
}

// SetMetadata sets a key-value pair in the metadata namespace, the key must be
// no more than 31 bytes.
func (t *Trie) SetMetadata(key []byte, val []byte) error {
	return t.db.Update(func(b Bucket) error {
		return t.SetMetadataWithBucket(key, val, b)
	})
}

// SetMetadataWithBucket sets a key-value pair in the metadata namespace, the
// key must be no more than 31 bytes. It must be called in a DB.Update
// transaction.
func (t *Trie) SetMetadataWithBucket(key []byte, val []byte, b Bucket) error {
	if len(key) > metaMaxLen {
		return xerrors.Errorf("key must be %v bytes or shorter", metaMaxLen)
	}
	if isIllegalKey(key) {
		return xerrors.New("the key is illegal, it cannot be \"" + entryKey + "\" or \"" + nonceKey + "\"")
	}
	return b.Put(key, val)
}

// GetMetadata gets a value associated with the key in the metadata namespace,
// the key must be no more than 31 bytes. If the key does not exist, nil is
// returned.
func (t *Trie) GetMetadata(key []byte) []byte {
	var out []byte
	err := t.db.View(func(b Bucket) error {
		out = t.GetMetadataWithBucket(key, b)
		return nil
	})
	if err != nil {
		return nil
	}
	return out
}

// GetMetadataWithBucket gets a value associated with the key in the metadata
// namespace, the key must be no more than 31 bytes. If the key does not exist,
// nil is returned. It must be called in a transaction.
func (t *Trie) GetMetadataWithBucket(key []byte, b Bucket) []byte {
	if len(key) > metaMaxLen {
		return nil
	}
	if isIllegalKey(key) {
		return nil
	}
	return clone(b.Get(key))
}

// DeleteMetadata deletes the key-value pair from the metadata namespace, the
// key must be no more than 31 bytes. If the key does not exist then nothing is
// done and nil is returned.
func (t *Trie) DeleteMetadata(key []byte) error {
	return t.db.Update(func(b Bucket) error {
		return t.DeleteMetadataWithBucket(key, b)
	})
}

// DeleteMetadataWithBucket deletes the key-value pair from the metadata
// namespace, the key must be no more than 31 bytes. If the key does not exist
// then nothing is done and nil is returned. It must be execute in a DB.Update
// transaction.
func (t *Trie) DeleteMetadataWithBucket(key []byte, b Bucket) error {
	if len(key) > metaMaxLen {
		return xerrors.Errorf("key must be %v bytes or shorter", metaMaxLen)
	}
	if isIllegalKey(key) {
		return xerrors.New("the key is illegal, it cannot be \"" + entryKey + "\" or \"" + nonceKey + "\"")
	}
	return b.Delete(key)
}
