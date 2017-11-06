package libsc

// SkipBlockID represents the Hash of the SkipBlock
type SkipBlockID []byte

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
type SkipBlockVerificationFunction func(KeyValueStore,Transaction)bool

// SkipBlock is the basic data-structure holding one block in the chain.
type SkipBlock struct {

	// Holds all block information that does not change after being instantiated
	Data *SkipBlockData

	// ForwardLink will be calculated once future SkipBlocks are available
	ForwardLink []*BlockLink
}

// SkipBlockData is the part of the skipblock that does not change after its creation
type SkipBlockData struct {

	// Index of the block in the chain. Index == 0 -> genesis-block.
	Index int

	// Height of that SkipBlock, starts at 1.
	Height int

	// GenesisID is the ID of the genesis-block.
	GenesisID SkipBlockID

	// BackLink is a slice of hashes to previous SkipBlocks
	BackLinkIDs []SkipBlockID

	// Roster holds the roster-definition of that SkipBlock
	Roster RosterID

	// Transactions is an ordered set of transaction stored in this SkipBlock
	Transactions []Transaction

	// GenesisData is non-null only in the genesis block, and contains information constant for all blocks
	Configuration *SkipChainConfiguration
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
