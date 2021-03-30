package storage

import (
	"go.dedis.ch/cothority/v3/skipchain"
)

// Storage defines the primitives needed to store proxy data.
type Storage interface {
	// GetBlock should return the block id from the storage, or -1 if not found.
	GetBlock(blockHash []byte) (int, error)

	// StoreBlock should store the block.
	StoreBlock(*skipchain.SkipBlock) (int, error)

	// Query executes the query and returns the result.
	Query(query string) ([]byte, error)
}
