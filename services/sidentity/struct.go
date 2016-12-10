package sidentity

import (
	"bytes"
	"encoding/binary"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000 * 3

// How many ms at most should be the time difference between a device/cothority node and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

// It specifies the minimum amount of remaining ms (before the expiration of the current valid cert
// for a website) before asking for a fresh cert
//const refresh_bound = 172800000 // 2 days * 24 hours/day * 3600 sec/hour * 1000 ms/sec (REALISTIC)
const refresh_bound = 3000

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

// CreateIdentity starts a new identity-skipchain with the initial
// Config and asking all nodes in Roster to participate.
type CreateIdentity struct {
	Config *common_structs.Config
	Roster *sda.Roster
}

// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the Root and Data-skipchain.
type CreateIdentityReply struct {
	Root *skipchain.SkipBlock
	Data *skipchain.SkipBlock
}

// ConfigUpdate verifies if a new update is available.
type ConfigUpdate struct {
	ID skipchain.SkipBlockID // the Hash of the genesis skipblock
}

// ConfigUpdateReply returns the updated configuration.
type ConfigUpdateReply struct {
	Config *common_structs.Config
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID skipchain.SkipBlockID
	*common_structs.Config
}

// ProposeUpdate verifies if a new config is available.
type ProposeUpdate struct {
	ID skipchain.SkipBlockID
}

// ProposeUpdateReply returns the updated propose-configuration.
type ProposeUpdateReply struct {
	Propose *common_structs.Config
}

// ProposeVote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type ProposeVote struct {
	ID        skipchain.SkipBlockID
	Signer    string
	Signature *crypto.SchnorrSig
}

// ProposeVoteReply returns the signed new skipblock if the threshold of
// votes have arrived.
type ProposeVoteReply struct {
	Data *skipchain.SkipBlock
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*Storage
}

type PropagateCert struct {
	*Storage
}

type PropagatePoF struct {
	Storages []*Storage
}

type UpdateSkipBlock struct {
	ID      skipchain.SkipBlockID
	Storage *Storage
}

type GetValidSbPath struct {
	ID    skipchain.SkipBlockID
	Hash1 skipchain.SkipBlockID
	Hash2 skipchain.SkipBlockID
}
type GetValidSbPathReply struct {
	Skipblocks []*skipchain.SkipBlock
	Cert       *common_structs.Cert
}

type PushPublicKey struct {
	//ID       skipchain.SkipBlockID
	Roster   *sda.Roster
	Public   abstract.Point
	ServerID *network.ServerIdentity
}

type PushPublicKeyReply struct {
}

type PullPublicKey struct {
	//ID       skipchain.SkipBlockID
	ServerID *network.ServerIdentity
}

type PullPublicKeyReply struct {
	Public abstract.Point
}

type GetCert struct {
	ID skipchain.SkipBlockID
}

type GetCertReply struct {
	SbHash skipchain.SkipBlockID
	Cert   *common_structs.Cert
}

type GetPoF struct {
	ID skipchain.SkipBlockID
}

type GetPoFReply struct {
	SbHash skipchain.SkipBlockID
	PoF    *common_structs.SignatureResponse
}

type LockIdentities struct {
}
