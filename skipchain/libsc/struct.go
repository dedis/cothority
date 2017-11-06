package libsc

import (
	"github.com/satori/go.uuid"
)

// SkipBlockID represents the Hash of the SkipBlock
type SkipBlockID []byte

// SkipChainID is an unique SkipChain identifier
type SkipChainID uuid.UUID

// SkipBlockIndex is a way to numerically order the blocks, e.g. with an int
type SkipBlockIndex int

// RosterID points a specific onet.Roster
type RosterID []byte

// TransactionKey defines he type of the key in the key=>value store
type TransactionKey string

// TransactionValue defines he type of the value in the key=>value store
type TransactionValue []byte

// KeyValueStore is used to represent both the KeyValue map in one transaction, and the global key
// value map of a skipchain at one given point in time
type KeyValueStore map[TransactionKey]TransactionValue

// SkipBlockVerificationFunction is a verification function that takes the current state of the skipchain
// (the global key->value store) and a new transaction, and returns true/false if acceptable
type SkipBlockVerificationFunction *func(KeyValueStore,Transaction)bool

// SkipChain holds a collection of Skipblocks in a map (the genesis block is always at 0), and a configuration
type SkipChain struct {

	// A unique way to identify this skipchain
	ID SkipChainID

	// A human-readable description
	Description string

	// The latest version of the key=>value store as described in the blocks.
	// The blocks provide the ground truth ! this one might be null or not up-to-date
	KeyValueStore

	// GenesisData is non-null only in the genesis block, and contains information constant for all blocks
	Configuration *SkipChainConfiguration

	// Contains all the blocks of this SkipChain
	Blocks map[SkipBlockIndex]SkipBlock
}

// SkipBlock is the basic data-structure holding one block in the chain.
type SkipBlock struct {

	// Convenience pointer to the mother skipchain. Is not hashed, nor has semantic meaning in this block
	*SkipChain

	// Holds all block information that does not change after being instantiated
	Data *SkipBlockData

	// ForwardLinks will be calculated once future SkipBlocks are available
	ForwardLinks []*BlockLink
}

// SkipBlockData is the part of the skipblock that does not change after its creation
type SkipBlockData struct {

	// Useful when we receive a SkipBlock "out of the blue", to determine to which SkipChain it belongs
	SkipChainID

	// Index of the block in the chain. Index == 0 -> genesis-block.
	Index SkipBlockIndex

	// Height of that SkipBlock, starts at 1.
	Height int

	// BackLink is a slice of hashes to previous SkipBlocks
	BackLinkIDs []SkipBlockID

	// Possibly only present in the genesis block, to fix whatever configuration we will use
	ConfigurationHash *[]byte

	// Roster holds the roster-definition of that SkipBlock
	Roster RosterID

	// Transactions is an ordered set of transaction stored in this SkipBlock
	Transactions []Transaction
}

// SkipChainConfiguration contains general information about the SkipChain, e.g. heights and verification function
type SkipChainConfiguration struct {

	// The max height determines the height of the next block
	MaximumHeight int

	// For deterministic SkipChains, chose a value >= 1 - higher bases mean more 'height = 1' SkipBlocks
	// For random SkipChains, chose a value of 0
	BaseHeight int

	// Node will apply this verification function on new transactions to determine if they are acceptable or not
	VerificationFunction SkipBlockVerificationFunction
}

// Transaction contains an unordered collection of key=>value changes to the global key=>value store of this SkipChain.
type Transaction struct {

	//Contains key=>value pairs that will be included in the global key=>value map of the skipchain
	KeyValues KeyValueStore
}

// BlockLink links to a (future) block, and this link is signed by the cothority at the time of creation of the old
// skipblock
type BlockLink struct {
	Hash      SkipBlockID
	Signature []byte
}
