package ca

import (
	//"encoding/binary"
	//"fmt"
	//"sort"
	//"strings"

	//"github.com/dedis/cothority/crypto"
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

// How many ms at most should be the validity period of a config/cert
const bound = 31104000000 // == 1 year * 12 months/year * 30 days/month * 24 hours/day * 3600 sec/hour * 1000 ms/sec

// Messages between the Client-API and the Service

type CSR struct {
	// The ID of the site skipchain
	ID skipchain.SkipBlockID
	// The skipblock to which the cert will be pointing
	Config *common_structs.Config
	// Config included in the previous skipblock (will be nil in case that the cert is going to be acuired
	// for the config of the genesis block)
	PrevConfig *common_structs.Config
}

type CSRReply struct {
	Cert *common_structs.Cert
}

type GetPublicKey struct {
}

type GetPublicKeyReply struct {
	Public abstract.Point
}
