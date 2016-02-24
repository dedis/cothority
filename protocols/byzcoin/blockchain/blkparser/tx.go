// Basically adapation from the file at https://github.com/tsileo/blkparser
package blkparser

import (
	"encoding/binary"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

type Tx struct {
	Hash     string
	Size     uint32
	LockTime uint32
	Version  uint32
	TxInCnt  uint32
	TxOutCnt uint32
	TxIns    []*TxIn
	TxOuts   []*TxOut
}

type TxIn struct {
	InputHash string
	InputVout uint32
	ScriptSig []byte
	Sequence  uint32
}

type TxOut struct {
	Addr     string
	Value    uint64
	Pkscript []byte
}

// ParseTxs parses bitcoin transactions (to form a Block use `NewBlock`)
func ParseTxs(txsraw []byte) (txs []*Tx, err error) {
	offset := int(0)
	txcnt, txcnt_size := DecodeVariableLengthInteger(txsraw[offset:])
	offset += txcnt_size

	txs = make([]*Tx, txcnt)

	txoffset := int(0)
	for i := range txs {
		txs[i], txoffset = NewTx(txsraw[offset:])
		txs[i].Hash = GetShaString(txsraw[offset : offset+txoffset])
		txs[i].Size = uint32(txoffset)
		offset += txoffset
	}

	return
}

func NewTx(rawtx []byte) (tx *Tx, offset int) {
	tx = new(Tx)
	tx.Version = binary.LittleEndian.Uint32(rawtx[0:4])
	offset = 4

	txincnt, txincntsize := DecodeVariableLengthInteger(rawtx[offset:])
	offset += txincntsize

	tx.TxInCnt = uint32(txincnt)
	tx.TxIns = make([]*TxIn, txincnt)

	txoffset := int(0)
	for i := range tx.TxIns {
		tx.TxIns[i], txoffset = NewTxIn(rawtx[offset:])
		offset += txoffset

	}

	txoutcnt, txoutcntsize := DecodeVariableLengthInteger(rawtx[offset:])
	offset += txoutcntsize

	tx.TxOutCnt = uint32(txoutcnt)
	tx.TxOuts = make([]*TxOut, txoutcnt)

	for i := range tx.TxOuts {
		tx.TxOuts[i], txoffset = NewTxOut(rawtx[offset:])
		offset += txoffset
	}

	tx.LockTime = binary.LittleEndian.Uint32(rawtx[offset : offset+4])
	offset += 4

	return
}

func NewTxIn(txinraw []byte) (txin *TxIn, offset int) {
	txin = new(TxIn)
	txin.InputHash = HashString(txinraw[0:32])
	txin.InputVout = binary.LittleEndian.Uint32(txinraw[32:36])
	offset = 36

	scriptsig, scriptsigsize := DecodeVariableLengthInteger(txinraw[offset:])
	offset += scriptsigsize

	txin.ScriptSig = txinraw[offset : offset+scriptsig]
	offset += scriptsig

	txin.Sequence = binary.LittleEndian.Uint32(txinraw[offset : offset+4])
	offset += 4
	return
}

func NewTxOut(txoutraw []byte) (txout *TxOut, offset int) {
	txout = new(TxOut)
	txout.Value = binary.LittleEndian.Uint64(txoutraw[0:8])
	offset = 8

	pkscript, pkscriptsize := DecodeVariableLengthInteger(txoutraw[offset:])
	offset += pkscriptsize

	txout.Pkscript = txoutraw[offset : offset+pkscript]
	offset += pkscript

	_, addrhash, _, err := txscript.ExtractPkScriptAddrs(txout.Pkscript, &chaincfg.MainNetParams)
	if err != nil {
		return
	}
	if len(addrhash) != 0 {
		txout.Addr = addrhash[0].EncodeAddress()
	} else {
		txout.Addr = ""
	}

	return
}
