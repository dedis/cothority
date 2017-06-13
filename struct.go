package logread

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"fmt"

	"strings"

	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	for _, msg := range []interface{}{
		Credential{},
		DataACL{}, DataACLEvolve{},
		DataWlr{}, DataWlrWrite{}, DataWlrRead{}, DataWlrConfig{},
		CreateSkipchainsRequest{}, CreateSkipchainsReply{},
		EvolveACLRequest{}, EvolveACLReply{},
		WriteRequest{}, WriteReply{},
		ReadRequest{}, ReadReply{},
		EncryptKeyRequest{}, EncryptKeyReply{},
		DecryptKeyRequest{}, DecryptKeyReply{},
	} {
		network.RegisterMessage(msg)
	}
}

const (
	// ErrorParameter is used when one of the parameters is faulty or leads
	// to a fault.
	ErrorParameter = iota + 4000
	// ErrorProtocol is used when one of the protocols (propagation) returns
	// an error.
	ErrorProtocol
)

// Credential as stored in the acl-skipchain. The 'Private' part is always
// nil in the skipchain.
type Credential struct {
	Pseudonym string
	Public    abstract.Point
	Private   abstract.Scalar
}

// NewCredential creates a nes public/private key pair and returns the
// filled out credential-structure.
func NewCredential(n string) *Credential {
	kp := config.NewKeyPair(network.Suite)
	return &Credential{
		Pseudonym: n,
		Public:    kp.Public,
		Private:   kp.Secret,
	}
}

// HidePrivate returns the Credential without the private key.
func (c *Credential) HidePrivate() *Credential {
	return &Credential{
		Pseudonym: c.Pseudonym,
		Public:    c.Public,
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

// SearchPseudo returns the corresponding credential or nil if not found.
func (cs *Credentials) SearchPseudo(n string) *Credential {
	if cs == nil {
		return nil
	}
	for _, c := range cs.List {
		if c.Pseudonym == n {
			return c
		}
	}
	return nil
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
func (cs *Credentials) AddPseudo(n string) *Credential {
	if cs.SearchPseudo(n) != nil {
		return nil
	}
	c := NewCredential(n)
	cs.List = append(cs.List, c)
	return c
}

// FindPseudo searches for the name and returns the credential and the index.
// If it is not found, an index of -1 is returned.
func (cs *Credentials) FindPseudo(n string) (cred *Credential, index int) {
	for index, cred = range cs.List {
		if cred.Pseudonym == n {
			return
		}
	}
	return nil, -1
}

// DelPseudo removes credentials with a given pseudo
func (cs *Credentials) DelPseudo(n string) {
	_, remove := cs.FindPseudo(n)
	if remove >= 0 {
		cs.List = append(cs.List[0:remove], cs.List[remove:]...)
	}
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
		name = name + " " + c.Pseudonym
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
			hash.Write([]byte(l.Pseudonym))
			l.Public.MarshalTo(hash)
			if l.Private != nil {
				l.Private.MarshalTo(hash)
			}
		}
	}
	return hash.Sum(nil)
}

// DataACL represents the current state of access control.
type DataACL struct {
	Admins  *Credentials
	Writers *Credentials
	Readers *Credentials
}

func NewDataACL(b []byte) *DataACL {
	_, dacli, err := network.Unmarshal(b)
	if err != nil {
		return nil
	}
	dacl, ok := dacli.(*DataACLEvolve)
	if !ok {
		return nil
	}
	return dacl.ACL
}

// Hash returns the hash of all credential-hashes.
func (da *DataACL) Hash() []byte {
	hash := network.Suite.Hash()
	hash.Write(da.Admins.Hash())
	hash.Write(da.Writers.Hash())
	hash.Write(da.Readers.Hash())
	return hash.Sum(nil)
}

// HidePrivate removes the private keys from all credentials.
func (da *DataACL) HidePrivate() *DataACL {
	return &DataACL{
		Admins:  da.Admins.HidePrivate(),
		Writers: da.Writers.HidePrivate(),
		Readers: da.Readers.HidePrivate(),
	}
}

// String returns a nice view of this ACL.
func (da *DataACL) String() string {
	return fmt.Sprintf("-- Admins: %s\n-- Writers: %s\n-- Readers: %s",
		da.Admins, da.Writers, da.Readers)
}

// DataACLEvolve represents a new set of Access Control Lists, signed by one
// of the previous admins.
type DataACLEvolve struct {
	ACL       *DataACL
	Signature *crypto.SchnorrSig
}

// NewDataACLEvolve returns an initialized structure, including the signature
// over the hash of the new ACL.
func NewDataACLEvolve(a *DataACL, last *skipchain.SkipBlock, priv abstract.Scalar) *DataACLEvolve {
	var data []byte
	acl := a.HidePrivate()
	if last != nil {
		data = append(last.SkipChainID(), last.Hash...)
	}
	data = append(data, acl.Hash()...)
	sig, err := crypto.SignSchnorr(network.Suite, priv, data)
	if err != nil {
		return nil
	}

	return &DataACLEvolve{
		ACL:       acl.HidePrivate(),
		Signature: &sig,
	}
}

// VerifySig verifies if the signature is valid.
func (eda *DataACLEvolve) VerifySig(prev *skipchain.SkipBlock, pub abstract.Point) error {
	var data []byte
	if prev != nil {
		data = append(prev.SkipChainID(), prev.Hash...)
	}
	data = append(data, eda.ACL.Hash()...)
	return crypto.VerifySchnorr(network.Suite, pub, data, *eda.Signature)
}

// DataWlr has either the configuration (only valid in genesis-block),
// write or grant field non-nil.
type DataWlr struct {
	Config *DataWlrConfig
	Write  *DataWlrWrite
	Read   *DataWlrRead
}

// NewDataWlr returns a pointer to a DataWlr structure created from
// the given data-slice. If the slice is not a valid DataWlr-structure,
// nil is returned.
func NewDataWlr(b []byte) *DataWlr {
	_, dwi, err := network.Unmarshal(b)
	if err != nil {
		return nil
	}
	dw, ok := dwi.(*DataWlr)
	if !ok {
		return nil
	}
	return dw
}

// String returns a nice string.
func (dw *DataWlr) String() string {
	if dw == nil {
		return "nil-pointer"
	}
	if dw.Config != nil {
		return fmt.Sprintf("Config: ACL at %x", dw.Config.ACL)
	}
	if dw.Write != nil {
		return fmt.Sprintf("Write: file-length of %d", len(dw.Write.File))
	}
	if dw.Read != nil {
		return fmt.Sprintf("Read: %s read file %x", dw.Read.Pseudonym, dw.Read.File)
	}
	return "all nil DataWlr"
}

// DataWlrConfig points to the responsible acl-skipchain
type DataWlrConfig struct {
	ACL skipchain.SkipBlockID
}

// DataWlrWrite stores the file and the encrypted secret
type DataWlrWrite struct {
	File []byte
	// TODO: this has to be the key encrypted under the shared secret
	Key       []byte
	Signature *crypto.SchnorrSig
}

// DataWlrRead stores a read-request which is the secret encrypted under the
// pseudonym's public key. The File is the skipblock-id of the skipblock
// holding the file.
type DataWlrRead struct {
	Pseudonym string
	Public    abstract.Point
	File      skipchain.SkipBlockID
	EncKey    []byte
	Signature *crypto.SchnorrSig
}

// Requests and replies to/from the service

// CreateSkipchainsRequest asks for setting up a new wlr/acl skipchain pair.
type CreateSkipchainsRequest struct {
	Roster *onet.Roster
	ACL    *DataACLEvolve
}

// CreateSkipchainsReply holds the two genesis-skipblocks and the admin-credentials.
type CreateSkipchainsReply struct {
	ACL   *skipchain.SkipBlock
	Wlr   *skipchain.SkipBlock
	Admin *Credential
}

// EvolveACLRequest asks the skipchain to add NewAcls to the ACL-skipchain.
type EvolveACLRequest struct {
	ACL     skipchain.SkipBlockID
	NewAcls *DataACLEvolve
}

// EvolveACLReply returns the newly created skipblock, if successful.
type EvolveACLReply struct {
	SB *skipchain.SkipBlock
}

// WriteRequest asks the wlr-skipchain to store a new file on the skipchain.
type WriteRequest struct {
	Write *DataWlrWrite
	Wlr   skipchain.SkipBlockID
}

// WriteReply returns the created skipblock
type WriteReply struct {
	SB *skipchain.SkipBlock
}

// ReadRequest asks the wlr-skipchain to allow a reader to access a document.
type ReadRequest struct {
	Read *DataWlrRead
	Wlr  skipchain.SkipBlockID
}

// ReadReply is the added skipblock, if successful.
type ReadReply struct {
	SB *skipchain.SkipBlock
}

// EncryptKeyRequest is sent to the service which should know what to do...
type EncryptKeyRequest struct {
	Roster *onet.Roster
	// Probably something like the hash?
}

// EncryptKeyReply is sent back to the api with something useful
type EncryptKeyReply struct {
	Aggregate abstract.Point
}

// DecryptKeyRequest is sent to the service with the read-request
type DecryptKeyRequest struct {
	Read skipchain.SkipBlockID
}

// DecryptKeyReply is sent back to the api with the key encrypted under the
// reader's public key.
type DecryptKeyReply struct {
	KeyParts []*ElGamal
}
