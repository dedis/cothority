package ca

import (
	//"encoding/binary"
	//"fmt"
	//"sort"
	//"strings"

	"github.com/dedis/cothority/crypto"
	//"github.com/dedis/cothority/log"
	//"github.com/dedis/cothority/network"
	//"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many ms at most should be the time difference between a CA and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

type Cert struct {
	// Site's ID
	ID skipchain.SkipBlockID
	// The pointed config's hash
	Hash crypto.HashID
	// The signature of the certification authority upon the 'Hash'
	Signature *crypto.SchnorrSig
	// The public key of the certification authority
	Public abstract.Point
}

// Messages between the Client-API and the Service

type CSR struct {
	// The ID of the site skipchain
	ID skipchain.SkipBlockID
	// The skipblock to which the cert will be pointing
	Config *common_structs.Config
}

type CSRReply struct {
	Cert *Cert
}

type GetPublicKey struct {
}

type GetPublicKeyReply struct {
	Public abstract.Point
}
