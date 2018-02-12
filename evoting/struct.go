package evoting

import (
	"strconv"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

func init() {
	network.RegisterMessages(
		Link{}, LinkReply{},
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

// Login message.
type Login struct {
	ID        skipchain.SkipBlockID // ID of the master skipchain.
	User      uint32                // User identifier.
	Signature []byte                // Signature from the front-end.
}

// Digest appends the digits of the user identifier to the skipblock ID.
func (l *Login) Digest() []byte {
	message := l.ID
	for _, c := range strconv.Itoa(int(l.User)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	return message
}

// Sign creates a Schnorr signature of the login digest.
func (l *Login) Sign(secret kyber.Scalar) error {
	sig, err := schnorr.Sign(lib.Suite, secret, l.Digest())
	l.Signature = sig
	return err
}

// Verify checks the Schnorr signature.
func (l *Login) Verify(public kyber.Point) error {
	return schnorr.Verify(lib.Suite, public, l.Digest(), l.Signature)
}

// LoginReply message.
type LoginReply struct {
	Token     string          // Token (time-limited) for further calls.
	Admin     bool            // Admin indicates if user has admin rights.
	Elections []*lib.Election // Elections the user participates in.
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
	Token    string                // Token for authentication.
	ID       skipchain.SkipBlockID // ID of the master skipchain.
	Election *lib.Election         // Election object.
}

// OpenReply message.
type OpenReply struct {
	ID  skipchain.SkipBlockID // ID of the election skipchain.
	Key kyber.Point           // Key assigned by the DKG.
}

// Cast message.
type Cast struct {
	Token  string                // Token for authentication.
	ID     skipchain.SkipBlockID // ID of the election skipchain.
	Ballot *lib.Ballot           // Ballot to be casted.
}

// CastReply message.
type CastReply struct{}

// Shuffle message.
type Shuffle struct {
	Token string                // Token for authentication.
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// ShuffleReply message.
type ShuffleReply struct{}

// Decrypt message.
type Decrypt struct {
	Token string                // Token for authentication.
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// DecryptReply message.
type DecryptReply struct{}

// GetBox message.
type GetBox struct {
	Token string                // Token for authentication.
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// GetBoxReply message.
type GetBoxReply struct {
	Box *lib.Box // Box of encrypted ballots.
}

// GetMixes message.
type GetMixes struct {
	Token string                // Token for authentication.
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// GetMixesReply message.
type GetMixesReply struct {
	Mixes []*lib.Mix // Mixes from all conodes.
}

// GetPartials message.
type GetPartials struct {
	Token string                // Token for authentication.
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// GetPartialsReply message.
type GetPartialsReply struct {
	Partials []*lib.Partial // Partials from all conodes.
}

// Reconstruct message.
type Reconstruct struct {
	Token string                // Token for authentication
	ID    skipchain.SkipBlockID // ID of the election skipchain.
}

// ReconstructReply message.
type ReconstructReply struct {
	Points []kyber.Point // Points are the decrypted plaintexts.
}

// Ping message.
type Ping struct {
	Nonce uint32 // Nonce can be any integer.
}
