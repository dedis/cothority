package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"

	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
)

type Block struct {
	Magic      [4]byte
	BlockSize  uint32
	HeaderHash string
	*Header
	TransactionList
}

type TrBlock struct {
	Block
}

type Header struct {
	MerkleRoot string
	Parent     string
	ParentKey  string
	PublicKey  string
	LeaderId   net.IP
}

func (trb *TrBlock) NewTrBlock(transactions TransactionList, header *Header) *TrBlock {
	return NewTrBlock(transactions, header)
}

func (t *TrBlock) NewHeader(transactions TransactionList, parent string, parentKey string) *Header {
	return NewHeader(transactions, parent, parentKey)
}

func (trb *Block) Calculate_root(transactions TransactionList) (res string) {
	return HashRootTransactions(transactions)
}

// Porting to public method non related to Header / TrBlock whatsoever
func NewTrBlock(transactions TransactionList, header *Header) *TrBlock {
	trb := new(TrBlock)
	trb.Magic = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}
	trb.HeaderHash = trb.Hash(header)
	trb.TransactionList = transactions
	trb.BlockSize = 0
	trb.Header = header
	return trb
}

func NewHeader(transactions TransactionList, parent, parentKey string) *Header {
	hdr := new(Header)
	hdr.Parent = parent
	hdr.ParentKey = parentKey
	hdr.MerkleRoot = HashRootTransactions(transactions)
	return hdr
}
func HashRootTransactions(transactions TransactionList) string {
	var hashes []hashid.HashId

	for _, t := range transactions.Txs {
		temp, _ := hex.DecodeString(t.Hash)
		hashes = append(hashes, temp)
	}
	out, _ := proof.ProofTree(sha256.New, hashes)
	return hex.EncodeToString(out)
}

func (trb *Block) Hash(h *Header) (res string) {
	//change it to be more portable
	return HashHeader(h)

}

func HashHeader(h *Header) string {
	data := fmt.Sprintf("%v", h)
	sha := sha256.New()
	sha.Write([]byte(data))
	hash := sha.Sum(nil)
	return hex.EncodeToString(hash)
}

func (trb *TrBlock) Print() {
	log.Println("Header:")
	log.Printf("Leader %v", trb.LeaderId)
	//log.Printf("Pkey %v", trb.PublicKey)
	log.Printf("Parent %v", trb.Parent)
	log.Printf("ParentKey %v", trb.ParentKey)
	log.Printf("Merkle %v", trb.MerkleRoot)
	//trb.TransactionList.Print()
	//log.Println("Rest:")
	log.Printf("Hash %v", trb.HeaderHash)

	return
}
