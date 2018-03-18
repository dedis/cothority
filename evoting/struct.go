package evoting

import (
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
	uuid "github.com/satori/go.uuid"

	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func init() {
	network.RegisterMessages(
		Link{}, LinkReply{},
		LookupSciper{}, LookupSciperReply{},
		Open{}, OpenReply{},
		Cast{}, CastReply{},
		Shuffle{}, ShuffleReply{},
		Decrypt{}, DecryptReply{},
		GetBox{}, GetBoxReply{},
		GetMixes{}, GetMixesReply{},
		GetPartials{}, GetPartialsReply{},
		Reconstruct{}, ReconstructReply{},
		Ping{},
	)
}

var VerificationID = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, ServiceName))
var VerificationFunction = []skipchain.VerifierID{VerificationID}

// LookupSciper takes a sciper number and returns elements of the user.
type LookupSciper struct {
	Sciper string
	// If LookupURL is set, use it instead of the default (for testing).
	LookupURL string
}

// LookupSciperReply returns the elements of the vcard from
// https://people.epfl.ch/cgi-bin/people/vCard?id=sciper
type LookupSciperReply struct {
	FullName string
	Email    string
	URL      string
	Title    string
}

// Link message.
type Link struct {
	Pin    string       // Pin of the running service.
	Roster *onet.Roster // Roster that handles elections.
	Key    kyber.Point  // Key is a front-end public key.
	Admins []uint32     // Admins is a list of election administrators.
}

// LinkReply message.
type LinkReply struct {
	ID skipchain.SkipBlockID // ID of the master skipchain.
}

// Open message.
type Open struct {
	User      uint32                // Token for authentication.
	ID        skipchain.SkipBlockID // ID of the master skipchain.
	Election  *lib.Election         // Election object.
	Signature []byte
}

// OpenReply message.
type OpenReply struct {
	ID  skipchain.SkipBlockID // ID of the election skipchain.
	Key kyber.Point           // Key assigned by the DKG.
}

// Cast message.
type Cast struct {
	ID     skipchain.SkipBlockID // ID of the election skipchain.
	Ballot *lib.Ballot           // Ballot to be casted.

	User      uint32 // Token for authentication.
	Signature []byte
}

// CastReply message.
type CastReply struct{}

// Shuffle message.
type Shuffle struct {
	ID        skipchain.SkipBlockID // ID of the election skipchain.
	User      uint32
	Signature []byte
}

// ShuffleReply message.
type ShuffleReply struct{}

// Decrypt message.
type Decrypt struct {
	ID        skipchain.SkipBlockID // ID of the election skipchain.
	User      uint32
	Signature []byte
}

// DecryptReply message.
type DecryptReply struct{}

// GetBox message.
type GetBox struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.
}

// GetBoxReply message.
type GetBoxReply struct {
	Box *lib.Box // Box of encrypted ballots.
}

// GetMixes message.
type GetMixes struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.
}

// GetMixesReply message.
type GetMixesReply struct {
	Mixes []*lib.Mix // Mixes from all conodes.
}

// GetPartials message.
type GetPartials struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.
}

// GetPartialsReply message.
type GetPartialsReply struct {
	Partials []*lib.Partial // Partials from all conodes.
}

// Reconstruct message.
type Reconstruct struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.
}

// ReconstructReply message.
type ReconstructReply struct {
	Points []kyber.Point // Points are the decrypted plaintexts.
}

// Ping message.
type Ping struct {
	Nonce uint32 // Nonce can be any integer.
}
