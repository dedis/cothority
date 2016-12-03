package sidentity

import (
	"encoding/binary"
	//"fmt"
	//"sort"
	//"strings"
	"bytes"

	"github.com/dedis/cothority/crypto"
	//"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 10000

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

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	LatestID skipchain.SkipBlockID
	ID       skipchain.SkipBlockID
}

// GetUpdateChainReply - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	Update []*skipchain.SkipBlock
	Cert   *common_structs.Cert
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

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID       skipchain.SkipBlockID
	Latest   *skipchain.SkipBlock
	Previous *skipchain.SkipBlock
}

type GetSkipblocks struct {
	ID     skipchain.SkipBlockID
	Latest *skipchain.SkipBlock
}

type GetSkipblocksReply struct {
	Skipblocks []*skipchain.SkipBlock
}

type GetValidSbPath struct {
	ID    skipchain.SkipBlockID
	Hash1 skipchain.SkipBlockID
	Hash2 skipchain.SkipBlockID
}
type GetValidSbPathReply struct {
	Skipblocks []*skipchain.SkipBlock
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
