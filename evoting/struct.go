package evoting

import (
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/network"

	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/skipchain"
)

func init() {
	network.RegisterMessage(Ping{})
	network.RegisterMessages(Link{}, LinkReply{})
	network.RegisterMessages(LookupSciper{}, LookupSciperReply{})
	network.RegisterMessages(Open{}, OpenReply{})
	network.RegisterMessages(Cast{}, CastReply{})
	network.RegisterMessages(Shuffle{}, ShuffleReply{})
	network.RegisterMessages(Decrypt{}, DecryptReply{})
	network.RegisterMessages(GetElections{}, GetElectionsReply{})
	network.RegisterMessages(GetBox{}, GetBoxReply{})
	network.RegisterMessages(GetMixes{}, GetMixesReply{})
	network.RegisterMessages(GetPartials{}, GetPartialsReply{})
	network.RegisterMessages(Reconstruct{}, ReconstructReply{})
}

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
	Pin       string                 // Pin of the running service.
	Roster    *onet.Roster           // Roster that handles elections.
	Key       kyber.Point            // Key is a front-end public key.
	Admins    []uint32               // Admins is a list of election administrators.
	ID        *skipchain.SkipBlockID // ID of the master skipchain to update; optional.
	User      *uint32                // User identifier; optional (required with ID).
	Signature *[]byte                // Signature authenticating the message; optional (required with ID).
}

// LinkReply message.
type LinkReply struct {
	ID skipchain.SkipBlockID // ID of the master skipchain.
}

// Open message.
type Open struct {
	ID       skipchain.SkipBlockID // ID of the master skipchain.
	Election *lib.Election         // Election object.

	User      uint32 // User identifier.
	Signature []byte // Signature authenticating the message.
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

	User      uint32 // User identifier.
	Signature []byte // Signature authenticating the message.
}

// CastReply message.
type CastReply struct {
	ID skipchain.SkipBlockID // Hash of the block storing the transaction
}

// Shuffle message.
type Shuffle struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.

	User      uint32 // User identifier.
	Signature []byte // Signature authenticating the message.
}

// ShuffleReply message.
type ShuffleReply struct{}

// Decrypt message.
type Decrypt struct {
	ID skipchain.SkipBlockID // ID of the election skipchain.

	User      uint32 // User identifier.
	Signature []byte // Signature authenticating the message.
}

// DecryptReply message.
type DecryptReply struct{}

// GetElections message.
type GetElections struct {
	User       uint32                // User identifier.
	Master     skipchain.SkipBlockID // Master skipchain ID.
	Stage      lib.ElectionState     // Election Stage filter. 0 for all elections.
	Signature  []byte                // Signature authenticating the message.
	CheckVoted bool                  // Check if user has voted in the elections.
}

// GetElectionsReply message.
type GetElectionsReply struct {
	Elections []*lib.Election // Elections is the retrieved list of elections.
	IsAdmin   bool            // Is the user in the list of admins in the master?
	Master    lib.Master
}

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
