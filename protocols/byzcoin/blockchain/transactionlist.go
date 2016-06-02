package blockchain

import (
	"crypto/sha256"
	"encoding/binary"
	"log"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
)

type TransactionList struct {
	Txs   []blkparser.Tx `json:"tx,omitempty"`
	TxCnt uint32         `json:"n_tx"`
	Fees  float64        `json:"-"`
}

func (tl *TransactionList) HashSum() []byte {
	h := sha256.New()
	for _, tx := range tl.Txs {
		if _, err := h.Write([]byte(tx.Hash)); err != nil {
			dbg.Error("Couldn't hash TX list", err)
		}
	}
	if err := binary.Write(h, binary.LittleEndian, tl.TxCnt); err != nil {
		dbg.Error("Couldn't hash TX list", err)
	}
	if err := binary.Write(h, binary.LittleEndian, tl.Fees); err != nil {
		dbg.Error("Couldn't hash TX list", err)
	}
	return h.Sum(nil)
}

func NewTransactionList(transactions []blkparser.Tx, n int) (tr TransactionList) {
	tran := new(TransactionList)
	tran.TxCnt = 0
	tran.Fees = 0

	if n > len(transactions) {
		n = len(transactions)
	}
	txs := make([]blkparser.Tx, 0)
	for i := 0; i < n; i++ {
		txs = append(txs, transactions[i])
		tran.TxCnt += 1
		tran.Fees += 0.01
	}
	tran.Txs = txs

	return *tran
}

func (tran *TransactionList) Print() {

	for _, tx := range tran.Txs {

		log.Printf("TxId: %v", tx.Hash)

		log.Println("TxIns:")
		if tx.TxInCnt == 1 && tx.TxIns[0].InputVout == 4294967295 {
			log.Printf("TxIn coinbase, newly generated coins")
		} else {
			for txin_index, txin := range tx.TxIns {
				log.Printf("TxIn index: %v", txin_index)
				log.Printf("TxIn Input_Hash: %v", txin.InputHash)
				log.Printf("TxIn Input_Index: %v", txin.InputVout)
			}
		}

		log.Println("TxOuts:")

		for txo_index, txout := range tx.TxOuts {
			log.Printf("TxOut index: %v", txo_index)
			log.Printf("TxOut value: %v", txout.Value)
			txout_addr := txout.Addr
			if txout_addr != "" {
				log.Printf("TxOut address: %v", txout_addr)
			} else {
				log.Printf("TxOut address: can't decode address")
			}
		}
	}

}
