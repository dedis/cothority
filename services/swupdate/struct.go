package swupdate

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/satori/go.uuid"
)

func init() {
	for _, msg := range []interface{}{
		Signature{},
		Policy{},
		storageMap{},
	} {
		network.RegisterPacketType(msg)
	}
}

type Signature struct {
	Sig string
}

func NewSignatures(sigs string) []*Signature {
	return []*Signature{}
}

type Policy struct {
	Name    string
	Version string
	// Represents how to fetch the source of that version -
	// only implementation so far will be deb-src://, but github://
	// and others are possible.
	Source     string
	Keys       []string
	Threshold  int
	BinaryHash string
}

func NewPolicy(str string) (*Policy, error) {
	p := &Policy{}
	_, err := toml.Decode(str, p)
	return p, err
}

type SwupChain struct {
	Root       *skipchain.SkipBlock
	Data       *skipchain.SkipBlock
	Policy     *Policy
	Signatures []*Signature
	Timestamp  *Timestamp
}

type Timestamp struct {
	Signature      string
	Hash           []byte
	InclusionProof string
}

func NewTimestamp(hash []byte) *Timestamp {
	log.Warn("Not yet implemented")
	return &Timestamp{Hash: hash}
}

type ProjectID uuid.UUID

type CreatePackage struct {
	Roster     *sda.Roster
	Policy     *Policy
	Signatures []*Signature
	Base       int
	Height     int
}

type CreatePackageRet struct {
	SwupChain *SwupChain
}

type UpdatePackage struct {
	SwupChain  *SwupChain
	Policy     *Policy
	Signatures []*Signature
}

type UpdatePackageRet struct {
	SwupChain *SwupChain
}
