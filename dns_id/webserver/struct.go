package webserver

import (
	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"gopkg.in/dedis/onet.v1/crypto"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

// How many msec since a skipblock is thought to be stale (according to its PoF)
//const maxdiff = 300000 // 300000 ms = 5 minutes * 60 sec/min * 1000 ms/sec // (REALISTIC)
const maxdiff = 2000

// ID represents one skipblock and corresponds to its Hash.
type ID []byte

// Messages between the Client-API and the Service

type Connect struct {
	ID []byte
}

type ConnectReply struct {
	Latest *skipchain.SkipBlock
	Certs  []*common_structs.Cert
}

type GetValidSbPathLight struct {
	FQDN  string
	Hash1 []byte
	Hash2 []byte
	// To be signed with the web servers (current) tls private key
	Challenge []byte
}

type GetValidSbPathLightReply struct {
	Configblocks []*common_structs.Config
	Cert       *common_structs.Cert
	PoF        *common_structs.SignatureResponse
	// The signature (using the private tls key) of the chosen webserver upon the challenge
	Signature *crypto.SchnorrSig
}
