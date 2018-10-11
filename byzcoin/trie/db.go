package trie

type database interface {
	Update(func(transaction) error) error
	View(func(transaction) error) error
	Close() error
}

type transaction interface {
	Bucket([]byte) bucket
	CreateBucketIfNotExists([]byte) (bucket, error)
}

type bucket interface {
	Delete([]byte) error
	Put([]byte, []byte) error
	Get([]byte) []byte
	ForEach(func(k, v []byte) error) error
}
