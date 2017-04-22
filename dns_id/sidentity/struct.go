package sidentity

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"github.com/satori/go.uuid"
)

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

// How many msec to wait before a timeout is generated in the propagation
const propagateTimeout = 20000

// How many ms at most should be the time difference between a device/cothority node and the
// the time reflected on the proposed config for the former to sign off
const maxdiff_sign = 300000

// It specifies the minimum amount of remaining ms (before the expiration of the current valid cert
// for a website) before asking for a fresh cert
//const refresh_bound = 172800000 // 2 days * 24 hours/day * 3600 sec/hour * 1000 ms/sec (REALISTIC)
const refresh_bound = 3000

// ID represents one skipblock and corresponds to its Hash.
type ID []byte

// VerifierID represents one of the verifications used to accept or
// deny a msg.
type VerifierID uuid.UUID

type MsgVerifier func(msg []byte, s *skipchain.SkipBlock) bool


// RegisterVerification stores the verification in a map and will
// call it whenever a verification needs to be done.
func RegisterVerification(c *onet.Context, v VerifierID, f MsgVerifier) error {
	scs := c.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).RegisterVerification(v, f)
}

var (
	// VerifyNone does nothing and returns true always.
	VerifyNone = VerifierID(uuid.Nil)
)

// Copy returns a deep copy of the Storage
func (sid *Storage) Copy() *Storage {
	sid.Lock()
	defer sid.Unlock()
	b, err := network.Marshal(sid)
	if err != nil {
		log.Error("Couldn't marshal Storage:", err)
		return nil
	}
	_, msg, err := network.Unmarshal(b)
	if err != nil {
		log.Error("Couldn't unmarshal Storage:", err)
	}
	sidNew := msg.(*Storage)
	if len(sidNew.Votes) == 0 {
		sidNew.Votes = make(map[string]*crypto.SchnorrSig)
	}
	if len(sidNew.ConfigBlocks) == 0 {
		sidNew.ConfigBlocks = make(map[string]*common_structs.ConfigPlusNextHash)
	}
	return sidNew
}

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
	Roster *onet.Roster
}


// CreateIdentityReply is the reply when a new Identity has been added. It
// returns the Root and Data-skipchain.
type CreateIdentityReply struct {
	Root *skipchain.SkipBlock
	Data *skipchain.SkipBlock
}

type CreateIdentityLight struct {
	Config *common_structs.Config
	Roster *onet.Roster
}

type CreateIdentityLightReply struct {
}

// ConfigUpdate verifies if a new update is available.
type ConfigUpdate struct {
	ID []byte // the Hash of the genesis skipblock
}

// ConfigUpdateReply returns the updated configuration.
type ConfigUpdateReply struct {
	Config *common_structs.Config
}

// ProposeSend sends a new proposition to be stored in all identities. It
// either replies a nil-message for success or an error.
type ProposeSend struct {
	ID []byte
	*common_structs.Config
}

type ProposeSendChain struct {
	ID []byte
	Blocks []*common_structs.Config
}


// ProposeUpdate verifies if a new config is available.
type ProposeUpdate struct {
	ID []byte
}

// ProposeUpdateReply returns the updated propose-configuration.
type ProposeUpdateReply struct {
	Propose *common_structs.Config
}


// ProposeVote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type ProposeVote struct {
	ID        []byte
	Signer    string
	Signature *crypto.SchnorrSig
}



// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*Storage
}

type PropagateIdentityLight struct {
	*Storage
}

type PropagateCert struct {
	*Storage
}

type PropagatePoF struct {
	Storages []*Storage
}

type UpdateSkipBlock struct {
	ID         []byte
	Storage    *Storage
	SbPrevious *skipchain.SkipBlock
}

type GetValidSbPath struct {
	ID    []byte
	Hash1 []byte
	Hash2 []byte
}
type GetValidSbPathReply struct {
	Skipblocks []*skipchain.SkipBlock
	Cert       *common_structs.Cert
	// Hash of the skiblock the config of which has been certified by the (latest) 'Cert'
	Hash []byte
	PoF  *common_structs.SignatureResponse
}


type GetValidSbPathLight struct {
	ID    []byte
	Hash1 []byte
	Hash2 []byte
}
type GetValidSbPathLightReply struct {
	Configblocks []*common_structs.Config
	Cert       *common_structs.Cert
	// Hash of the skiblock the config of which has been certified by the (latest) 'Cert'
	Hash []byte
	PoF  *common_structs.SignatureResponse
}

type PushPublicKey struct {
	//ID       []byte
	Roster   *onet.Roster
	Public   abstract.Point
	ServerID *network.ServerIdentity
}

type PushPublicKeyReply struct {
}

type PullPublicKey struct {
	//ID       []byte
	ServerID *network.ServerIdentity
}

type PullPublicKeyReply struct {
	Public abstract.Point
}

type GetCert struct {
	ID []byte
}

type GetCertReply struct {
	SbHash []byte
	Cert   *common_structs.Cert
}

type GetPoF struct {
	ID []byte
}

type GetPoFReply struct {
	SbHash []byte
	PoF    *common_structs.SignatureResponse
}

type LockIdentities struct {
}
