package libsc

/**
 * SkipChain methods.
 * Scenario: the conodes will run a service "SkipChainHolder" that will manage several SkipChains.
 * One conode might be leader. Each conode will store several SkipChains. It can receive update on new blocks for a
 * given SkipChains (identified by its SkipChain.ID).
 */

// NewSkipChain returns a new SkipChain with only a genesis block (that has no transactions)
func NewSkipChain(description string, config *SkipChainConfiguration) *SkipChain {
	//after creating the SkipChain struct, should use NewSkipBlock() to generate the genesis block
	return nil
}

// GenerateBlock is called only by the current leader. It creates and return a new block. Fails if the skipchain's
// verification function rejects those transactions
func (sc *SkipChain) GenerateBlock(transactions []Transaction) *SkipBlock {
	// should use NewSkipBlock()
	return nil
}

// BlockReceived adds a new block to an existing SkipChain (happens when we're not leader, and the leader creates a
// block). Fails if the new block does not passes the skipchain's validation function.
// OR, it might be an update to an old block (e.g. new forward links)
func (sc *SkipChain) BlockReceived(block *SkipBlock) {

}

// GetKeyValueStore returns the state (as of the last validated block) of the key=>value store of the skipchain.
// In particular, it might crawl the whole skipchain to aggregate the changes in the key=>value store.
func (sc *SkipChain) GetKeyValueStore() KeyValueStore {
	return make(KeyValueStore)
}

// Prune removes all unneeded skipblock between the most recent skipblock and the genesis block.
// A efficient way to prune is to add a new transaction with the current key=>value store dictionnary
// (obtained by calling sc.GetKeyValueStore()), then call sc.Prune(); then, since the key=>value store will
// be fully represented by the transaction in the last skipblock, Prune() will only keep the genesis block
// and this last block.
func (sc *SkipChain) Prune(){

}

/**
 * SkipBlock methods
 */

// NewSkipBlock returns a new SkipBlock instantiated with the correct values
func NewSkipBlock(sc *SkipChain, index SkipBlockIndex, backLinkIDs []*SkipBlockID, rosterID RosterID, transactions []Transaction) *SkipBlock {
	return nil
}

// Verify takes the verification function of the SkipChain to validate this block's transaction
func (sb *SkipBlock) Verify() bool {
	fn := sb.SkipChain.Configuration.VerificationFunction
	return fn(sb.SkipChain.KeyValueStore, sb.Data.Transactions)
}

// AddForwardLinks add a collection of forward links to an existing block, and returns this new block
func (sb *SkipBlock) AddForwardLinks(forwardLinks []*BlockLink) *SkipBlock {
	return nil
}



