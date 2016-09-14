package swupdate

import (
	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/cothority/services/timestamp"
	"github.com/satori/go.uuid"
)

func init() {
	for _, msg := range []interface{}{
		Policy{},
		Release{},
		storage{},
	} {
		network.RegisterPacketType(msg)
	}
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
	SourceHash string
}

func NewPolicy(str string) (*Policy, error) {
	p := &Policy{}
	_, err := toml.Decode(str, p)
	return p, err
}

// Niktin calls this 'Snapshot'
type Release struct {
	Policy      *Policy
	Signatures  []string
	VerifyBuild bool
}

type SwupChain struct {
	Root    *skipchain.SkipBlock
	Data    *skipchain.SkipBlock
	Release *Release
}

// XXX maybe don't alias the timestamp package's type?
type Timestamp timestamp.SignatureResponse

type ProjectID uuid.UUID

type CreatePackage struct {
	Roster  *sda.Roster
	Release *Release
	Base    int
	Height  int
}

type CreatePackageRet struct {
	SwupChain *SwupChain
}

type UpdatePackage struct {
	SwupChain *SwupChain
	Release   *Release
}

type UpdatePackageRet struct {
	SwupChain *SwupChain
}

// PackageSC searches for the skipchain responsible for the PackageName.
type PackageSC struct {
	PackageName string
}

// If no skipchain for PackageName is found, both first and last are nil.
// If the skipchain has been found, both the genesis-block and the latest
// block will be returned.
type PackageSCRet struct {
	First *skipchain.SkipBlock
	Last  *skipchain.SkipBlock
}

// Request skipblocks needed to get to the latest version of the package.
// LastKnownSB is the latest skipblock known to the client.
type LatestBlock struct {
	LastKnownSB skipchain.SkipBlockID
}

// Returns the timestamp of the latest skipblock, together with an eventual
// shortes-link of skipblocks needed to go from the LastKnownSB to the
// current skipblock.
type LatestBlockRet struct {
	Timestamp *Timestamp
	Update    []*skipchain.SkipBlock
}
