package ocs

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"fmt"

	"strings"

	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		Credential{},
		DataOCS{}, DataOCSWrite{}, DataOCSRead{}, DataOCSReaders{},
		CreateSkipchainsRequest{}, CreateSkipchainsReply{},
		WriteRequest{}, WriteReply{},
		ReadRequest{}, ReadReply{},
		SharedPublicRequest{}, SharedPublicReply{},
		DecryptKeyRequest{}, DecryptKeyReply{},
		GetReadRequests{}, GetReadRequestsReply{})
}

const (
	// ErrorParameter is used when one of the parameters is faulty or leads
	// to a fault.
	ErrorParameter = iota + 4000
	// ErrorProtocol is used when one of the protocols (propagation) returns
	// an error.
	ErrorProtocol
)

// Credential is a certificate or a private key. The 'Private' part is always
// nil in the skipchain.
type Credential struct {
	Public  abstract.Point
	Private abstract.Scalar
}

// NewCredential creates a nes public/private key pair and returns the
// filled out credential-structure.
func NewCredential() *Credential {
	kp := config.NewKeyPair(network.Suite)
	return &Credential{
		Public:  kp.Public,
		Private: kp.Secret,
	}
}

// HidePrivate returns the Credential without the private key.
func (c *Credential) HidePrivate() *Credential {
	return &Credential{
		Public: c.Public,
	}
}

// Credentials holds the list of credentials.
type Credentials struct {
	List []*Credential
}

// NewCredentials initializes a Credentials-structure.
func NewCredentials(c *Credential) *Credentials {
	return &Credentials{
		List: []*Credential{c},
	}
}

// SearchPublic returns the corresponding credential or nil if not found.
func (cs *Credentials) SearchPublic(p abstract.Point) *Credential {
	if cs == nil {
		return nil
	}
	for _, c := range cs.List {
		if c.Public.Equal(p) {
			return c
		}
	}
	return nil
}

// AddPseudo creates a new credential with the given pseudo
func (cs *Credentials) AddPseudo() *Credential {
	c := NewCredential()
	cs.List = append(cs.List, c)
	return c
}

// String prints the credentials.
func (cs *Credentials) String() string {
	var ret []string
	for _, c := range cs.List {
		var name string
		if c.Private != nil {
			name = "(*)"
		} else {
			name = "( )"
		}
		ret = append(ret, fmt.Sprintf("%s: %s", name, c.Public.String()))
	}
	return strings.Join(ret, "\n")
}

// HidePrivate removes private keys.
func (cs *Credentials) HidePrivate() *Credentials {
	ret := &Credentials{}
	if cs != nil && cs.List != nil {
		for _, c := range cs.List {
			ret.List = append(ret.List, c.HidePrivate())
		}
	}
	return ret
}

// Hash returns the hash of the credentials.
func (cs *Credentials) Hash() []byte {
	hash := network.Suite.Hash()
	if cs != nil {
		for _, l := range cs.List {
			l.Public.MarshalTo(hash)
			if l.Private != nil {
				l.Private.MarshalTo(hash)
			}
		}
	}
	return hash.Sum(nil)
}

// DataOCS holds eihter:
// - a read request
// - a write
// - a key-update
// - a write and a key-update
type DataOCS struct {
	Write   *DataOCSWrite
	Read    *DataOCSRead
	Readers *DataOCSReaders
}

// NewDataOCS returns a pointer to a DataOCS structure created from
// the given data-slice. If the slice is not a valid DataOCS-structure,
// nil is returned.
func NewDataOCS(b []byte) *DataOCS {
	_, dwi, err := network.Unmarshal(b)
	if err != nil {
		log.Error(err)
		return nil
	}
	if dwi == nil {
		log.Error("dwi is nil")
		return nil
	}
	dw, ok := dwi.(*DataOCS)
	if !ok {
		log.Error(err)
		return nil
	}
	return dw
}

// String returns a nice string.
func (dw *DataOCS) String() string {
	if dw == nil {
		return "nil-pointer"
	}
	if dw.Write != nil {
		return fmt.Sprintf("Write: file-length of %d", len(dw.Write.File))
	}
	if dw.Read != nil {
		return fmt.Sprintf("Read: %s read file %x", dw.Read.Public, dw.Read.File)
	}
	return "all nil DataOCS"
}

// DataOCSWrite stores the file and the encrypted secret
type DataOCSWrite struct {
	File []byte
	U    abstract.Point
	Cs   []abstract.Point
	// Readers is the ID of the DataOCSReaders block. If it is nil, then the
	// DataOCSReaders must be present in the same DataOCS as this DataOCSWrite.
	Readers []byte
}

// DataOCSReaders stores a new configuration for keys. If the same ID is already
// on the blockchain, it needs to be signed by a threshold of admins in the
// last block. If Admins is nil, no other block with the same ID can be stored.
// If ID is nil, this is a unique block for a single DataOCSWrite.
type DataOCSReaders struct {
	ID        []byte
	Readers   []abstract.Point
	Admins    []abstract.Point
	Threshold int
	Signature *crypto.SchnorrSig
}

// DataOCSRead stores a read-request which is the secret encrypted under the
// pseudonym's public key. The File is the skipblock-id of the skipblock
// holding the file.
type DataOCSRead struct {
	Public    abstract.Point
	File      skipchain.SkipBlockID
	Signature *crypto.SchnorrSig
}

// ReadDoc represents one read-request by a reader.
type ReadDoc struct {
	Reader abstract.Point
	ReadID skipchain.SkipBlockID
	FileID skipchain.SkipBlockID
}

// Requests and replies to/from the service

// CreateSkipchainsRequest asks for setting up a new OCS-skipchain.
type CreateSkipchainsRequest struct {
	Roster *onet.Roster
}

// CreateSkipchainsReply returns the skipchain-id of the OCS-skipchain
type CreateSkipchainsReply struct {
	OCS *skipchain.SkipBlock
	X   abstract.Point
}

// WriteRequest asks the OCS-skipchain to store a new file on the skipchain.
// Readers can be empty if Write points to a valid reader.
type WriteRequest struct {
	Write   *DataOCSWrite
	Readers *DataOCSReaders
	OCS     skipchain.SkipBlockID
}

// WriteReply returns the created skipblock which is the write-id
type WriteReply struct {
	SB *skipchain.SkipBlock
}

// ReadRequest asks the OCS-skipchain to allow a reader to access a document.
type ReadRequest struct {
	Read *DataOCSRead
	OCS  skipchain.SkipBlockID
}

// ReadReply is the added skipblock, if successful.
type ReadReply struct {
	SB *skipchain.SkipBlock
}

// SharedPublicRequest asks for the shared public key of the corresponding
// skipchain-ID.
type SharedPublicRequest struct {
	Genesis skipchain.SkipBlockID
}

// SharedPublicReply sends back the shared public key.
type SharedPublicReply struct {
	X abstract.Point
}

// DecryptKeyRequest is sent to the service with the read-request
type DecryptKeyRequest struct {
	Read skipchain.SkipBlockID
}

// DecryptKeyReply is sent back to the api with the key encrypted under the
// reader's public key.
type DecryptKeyReply struct {
	Cs      []abstract.Point
	XhatEnc abstract.Point
	X       abstract.Point
}

// GetReadRequests asks for a list of requests
type GetReadRequests struct {
	Start skipchain.SkipBlockID
	Count int
}

// GetReadRequestsReply returns the requests
type GetReadRequestsReply struct {
	Documents []*ReadDoc
}

// GetBunchRequest asks for a list of bunches
type GetBunchRequest struct {
}

// GetBunchReply returns the genesis blocks of all registered OCS.
type GetBunchReply struct {
	Bunches []*skipchain.SkipBlock
}
