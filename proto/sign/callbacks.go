package sign

// Callbacks holds the functions that are used to define the
// behaviour of a peer. All different peer-types use the
// cothority-tree, but they can interact differently with
// each other
type Callbacks interface {
	// AnnounceFunc is called from the root-node whenever an
	// announcement is made. It returns an AnnounceFunc which
	// has to write the "Message"-field of its AnnouncementMessage
	// argument.
	AnnounceFunc(*Peer) AnnounceFunc
	// CommitFunc is called whenever a commitement is ready to
	// be d. It's CommitFunc has to return a slice
	// of bytes that will go into the merkle-tree.
	CommitFunc(*Peer) CommitFunc
	// OnDone is called whenever the ture is completed and
	// the results are propagated through the tree.
	OnDone(*Peer) DoneFunc
	// Listen starts the port to let timestamps enter the system
	Listen(*Peer) error
}

