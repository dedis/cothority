package sign

// Callbacks holds the functions that are used to define the
// behaviour of a peer. All different peer-types use the
// cothority-tree, but they can interact differently with
// each other
type Callbacks interface {
	// AnnounceFunc is called from the root-node whenever an
	// announcement is made.
	Announcement(*AnnouncementMessage)
	// CommitFunc is called whenever a commitment is ready to
	// be sent. It takes the messages of its children and returns
	// the new message to be sent
	///Commitment([]CommitmentMessage) *CommitmentMessage
	// Actual Commitment which only returns new Merkle-tree
	Commitment() []byte
	// OnDone is called whenever the ture is completed and
	// the results are propagated through the tree.
	OnDone(*Peer) DoneFunc
	// Setup can be used to start listening on a port for requests or
	// any other setup that needs to be done
	Setup(*Peer) error
}

