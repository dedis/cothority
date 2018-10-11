package trie

type database interface {
	Update(func(bucket) error) error
	View(func(bucket) error) error
	Close() error
}

/*
type transaction interface {
	Bucket([]byte) bucket
	CreateBucketIfNotExists([]byte) (bucket, error)
}
*/

type bucket interface {
	Delete([]byte) error
	Put([]byte, []byte) error
	Get([]byte) []byte
	ForEach(func(k, v []byte) error) error
}
