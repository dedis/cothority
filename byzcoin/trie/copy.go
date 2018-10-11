package trie

type TrieCopy struct {
	source  Trie
	overlay Trie
}

func (t *TrieCopy) Get(k []byte) ([]byte, error) {
	return t.source.Get(k)
}

func (t *TrieCopy) Set(k, v []byte) error {
	return nil
}

func (t *TrieCopy) Delete(k []byte) error {
	return nil
}
