package blockchain

import (
	"log"
	"net"
)

type KeyBlock struct {
	Block
}

func (*KeyBlock) NewKeyBlock(transactions TransactionList, header *Header) (tr KeyBlock) {
	trb := new(KeyBlock)
	trb.Magic = [4]byte{0xD9, 0xB4, 0xBE, 0xF9}
	trb.HeaderHash = trb.Hash(header)
	trb.TransactionList = transactions
	trb.BlockSize = 0
	trb.Header = header
	return *trb
}

func (t *KeyBlock) NewHeader(transactions TransactionList, parent string, IP net.IP, key string) (hd Header) {
	hdr := new(Header)
	hdr.LeaderId = IP
	hdr.PublicKey = key
	hdr.ParentKey = parent
	hdr.MerkleRoot = t.Calculate_root(transactions)
	return *hdr
}

func (trb *KeyBlock) Print() {
	log.Println("Header:")
	log.Printf("Leader %v", trb.LeaderId)
	//log.Printf("Pkey %v", trb.PublicKey)
	log.Printf("ParentKey %v", trb.ParentKey)
	log.Printf("Merkle %v", trb.MerkleRoot)
	//log.Println("Rest:")
	log.Printf("Hash %v", trb.HeaderHash)
	return
}
