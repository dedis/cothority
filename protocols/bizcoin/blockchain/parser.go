package blockchain

import (
	"log"

	"github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"
)

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

func (p *Parser) Parse(first_block, last_block int) []blkparser.Tx {

	Chain, _ := blkparser.NewBlockchain(p.Path, p.Magic)

	var transactions []blkparser.Tx

	for i := 0; i < last_block; i++ {
		raw, err := Chain.FetchNextBlock()

		if raw == nil || err != nil {
			log.Println("End of Chain")
		}

		bl, err := blkparser.NewBlock(raw[:])

		if err != nil {
			println("Block inconsistent:", err.Error())
			break
		}

		// Read block till we reach start_block
		if i < first_block {
			continue
		}

		for _, tx := range bl.Txs {
			transactions = append(transactions, *tx)
		}

	}
	return transactions
}
