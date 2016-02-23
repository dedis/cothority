package blockchain

import "github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"

type Parser struct {
	Path      string
	Magic     [4]byte
	CurrentId uint32
}

func NewParser(path string, magic [4]byte) (parser *Parser, err error) {
	parser = new(Parser)
	parser.Path = path
	parser.Magic = magic
	parser.CurrentId = 0
	return
}

func (p *Parser) Parse(first_block, last_block int) ([]blkparser.Tx, error) {

	Chain, _ := blkparser.NewBlockchain(p.Path, p.Magic)

	var transactions []blkparser.Tx

	for i := 0; i < last_block; i++ {
		raw, err := Chain.FetchNextBlock()

		if raw == nil || err != nil {
			if err != nil {
				return transactions, err
			}
		}

		bl, err := blkparser.NewBlock(raw[:])
		if err != nil {
			return transactions, err
		}

		// Read block till we reach start_block
		if i < first_block {
			continue
		}

		for _, tx := range bl.Txs {
			transactions = append(transactions, *tx)
		}

	}
	return transactions, nil
}
