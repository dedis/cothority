package conode

import (
	"github.com/dedis/cothority/proto/sign"
)

// Callbacks holds the functions that are used to define the
// behaviour of a peer. All different peer-types use the
// cothority-tree, but they can interact differently with
// each other
type Callbacks interface {
	// RoundMessageFunc is called from the root-node whenever an
	// announcement is made. It returns an AnnounceFunc which
	// has to write the "Message"-field of its AnnouncementMessage
	// argument.
	RoundMessageFunc() sign.RoundMessageFunc
	// OnAnnounceFunc returns an OnAnnouncFunc which takes a AnnouncementMessage as
	// a parameters. It is used so clients of this api can save the message for this round
	// and give to any application layer they need.
	OnAnnounceFunc() sign.OnAnnounceFunc
	// CommitFunc is called whenever a commitement is ready to
	// be signed. It's sign.CommitFunc has to return a slice
	// of bytes that will go into the merkle-tree.
	CommitFunc(*Peer) sign.CommitFunc

	// ValidateFunc is called in validation mode. In validation mode, each
	// signer's contribution is broadcasted and is run through this function.
	// It must returns false if this peer does not validate the message (for
	// example a certificate for a domain owned by someone else!)
	//	ValidateFunc() sign.ValidateFunc
	// OnDone is called whenever the signature is completed and
	// the results are propagated through the tree.
	OnDone(*Peer) sign.OnDoneFunc
	// Setup will be called before the peer joins the tree
	// You can do any stuff inside like listening for clients connections etc
	Setup(*Peer) error
}
