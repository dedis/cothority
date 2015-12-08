package main

import (
	"github.com/dedis/cothority/lib/sign"
)

func main() {
	// First, let's read our config
	configFile := "config.toml"
	// YOu should create your own config in lib/app.
	// TOML is a pretty simple and readable format
	conf := &app.ConfigSkeleton{}
	if err := app.ReadTomlConfig(conf, configFile); err != nil {
		dbg.Fatal("Could not read toml config:", err)
	}

	// Read the private / public keys + binded address
	privFile := "key.priv"
	pubFile := "key.pub"
	if sec, err := cliutils.ReadPrivKey(suite, privFile); err != nil {
		dbg.Fatal("Error reading private key file:", err)
	} else {
		conf.Secret = sec
	}
	if pub, addr, err := cliutils.ReadPubKey(suite, pubFile); err != nil {
		dbg.Fatal("Error reading public key file:", err)
	} else {
		conf.Public = pub
		address = addr
	}
	// It creates a new peer that handles many thing for you ..
	peer := conode.NewPeer(address, conf)
	// Let's put 10 rounds. If you want infinite run loop, put something < 0
	maxRounds = 10
	// Here it will create a new round each seconds automatically.
	// If you need more fined grained control, you must implement yourself the
	// conode.Peer struct (it's quite easy).
	peer.LoopRounds(RoundSkeletonType, maxRounds)

}

// The name type of this round implementation
const RoundSkeletonType = "empty"

// RoundSkeleton is the barebone struct that will be used for a round.
// You can inherit of some already implemented rounds such as roundcosi, or
// roundexception etc. You should read and understand the code of the round you are embedding
// in your structs.
type RoundSkeleton struct {
	// RoundStruct will store some important information that you may use later.
	// It's really only a STORE so you don't have to rewrite all the fields you
	// would need.
	*sign.RoundStruct
}

// You need to register the round you are making
func init() {
	// You register by giving the type, and a function that takes a sign.Node in
	// input (basically the underlying protocol) and returns a Round.
	RegisterRoundFactory(RoundSkeletonType,
		func(node *sign.Node) Round {
			return NewRoundSkeleton(node)
		})
}

// Your New Round function
func NewRoundSkeleton(node *Node) *RoundSkeleton {
	dbg.Lvl3("Making new RoundSkeleton", node.Name())
	round := &RoundSkeleton{}
	// Because we embed ROundStruct we need to initialize it
	round.RoundStruct = NewRoundStruct(node, RoundSkeleton)
	return round
}

// The first phase is the announcement phase.
// For all phases, the signature is the same, it takes sone Input message and
// Output messages and returns an error if something went wrong.
// For announcement we just give for now the viewNbr (view = what is in the tree
// at the instant) and the round number so we know where/when are we in the run.
func (round *RoundSkeleton) Announcement(viewNbr, roundNbr int, in *SigningMessage, out []*SigningMessage) error {
	return nil
}

// Commitment phase
func (round *RoundSkeleton) Commitment(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

// Challenge phase
func (round *RoundSkeleton) Challenge(in *SigningMessage, out []*SigningMessage) error {
	return nil
}

// Challenge phase
func (round *RoundSkeleton) Response(in []*SigningMessage, out *SigningMessage) error {
	return nil
}

// SignatureBroadcast phase
func (round *RoundSkeleton) SignatureBroadcast(in *SigningMessage, out []*SigningMessage) error {
	return nil
}
