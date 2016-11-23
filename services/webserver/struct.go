package webserver

import (
	"encoding/binary"
	//"fmt"
	//"sort"
	//"strings"
	"bytes"

	"github.com/dedis/cothority/crypto"
	//"github.com/dedis/cothority/log"
	//"github.com/dedis/cothority/network"
	//"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/ca"
	//"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	//"github.com/dedis/crypto/abstract"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

// How many msec since a skipblock is thought to be stale
const maxdiff = 2592000000 // (2592000000ms = 30 days * 24 hours/day * 3600 sec/hour * 1000 ms/sec)

// ID represents one skipblock and corresponds to its Hash.
type ID skipchain.SkipBlockID

func timestampToBytes(t int64) []byte {
	timeBuf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(timeBuf, t)
	return timeBuf
}

func bytesToTimestamp(b []byte) (int64, error) {
	t, err := binary.ReadVarint(bytes.NewReader(b))
	if err != nil {
		return t, err
	}
	return t, nil
}

// Messages between the Client-API and the Service

type Connect struct {
	ID skipchain.SkipBlockID
}

type ConnectReply struct {
	Latest *skipchain.SkipBlock
	Certs  []*ca.Cert
}

type Update struct {
	ID skipchain.SkipBlockID
}

type UpdateReply struct {
	Latest *skipchain.SkipBlock
	Certs  []*ca.Cert
}

type GetSkipblocks struct {
	ID     skipchain.SkipBlockID
	Latest *skipchain.SkipBlock
}

type GetSkipblocksReply struct {
	Skipblocks []*skipchain.SkipBlock
}

type GetValidSbPath struct {
	ID  skipchain.SkipBlockID
	Sb1 *skipchain.SkipBlock
	Sb2 *skipchain.SkipBlock
}

type GetValidSbPathReply struct {
	Skipblocks []*skipchain.SkipBlock
}

type ChallengeReq struct {
	ID skipchain.SkipBlockID
	// The latest known tls key for the web server we try to visit
	Challenge crypto.HashID
}

type ChallengeReply struct {
	// The signature (using the private tls key) of the chosen webserver upon the challenge
	Signature *crypto.SchnorrSig
}
