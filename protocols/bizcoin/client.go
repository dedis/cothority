package bizcoin

type BizCoinClient struct {
	bz *BizCoin
}

func NewClient(bz *BizCoin) *BizCoinClient {
	return &BizCoinClient{bz: bz}
}

func (bcc *BizCoinClient) triggerTransactions(/* TODO Input data: where to look for persisted blocks & number of transactions*/) {
	// TODOs:
	// - parse the blocks from a directory
	// - in BitCosi the node holds a transaction_pool (what will be the equivalent in BizCoin?)
	// - for a number (param): call bcc.bz.handleNewTransaction((tr blockchain.Tx)
}