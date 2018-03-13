package skipchain

import (
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

func init() {
	network.RegisterMessages(
		// - API calls
		// Store new skipblock
		&StoreSkipBlock{},
		&StoreSkipBlockReply{},
		// Requests for data
		&GetUpdateChain{},
		&GetUpdateChainReply{},
		// Request updated block
		&GetSingleBlock{},
		// Fetch all skipchains
		&GetAllSkipchains{},
		&GetAllSkipchainsReply{},
		// Create link with client
		&CreateLinkPrivate{},
		// Unlink a client
		&Unlink{},
		// List all client keys
		&Listlink{},
		// Returns a list of public keys
		&ListlinkReply{},
		// Setting authentication
		&SettingAuthentication{},
		// Adding a skipchain to follow
		&AddFollow{},
		// Removing a skipchain from following
		&DelFollow{},
		// EmptyReply for calls that only return errors
		&EmptyReply{},
		// Lists all skipchains we follow
		&ListFollow{},
		// Returns the genesis-blocks of all skipchains we follow
		&ListFollowReply{},
		// - Internal calls
		// Propagation
		&PropagateSkipBlocks{},
		// Request forward-signature
		&ForwardSignature{},
		// - Data structures
		&SkipBlockFix{},
		&SkipBlock{},
		// Own service
		&Service{},
		// - Protocol messages
		&ProtoExtendSignature{},
		&ProtoExtendRoster{},
		&ProtoExtendRosterReply{},
		&ProtoGetBlocks{},
		&ProtoGetBlocksReply{},
	)
}

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls

// StoreSkipBlock - Requests a new skipblock to be appended to the given
// SkipBlock. If the given TargetSkipChainID is an empty slice, then a genesis
// block is created.  Otherwise, the new block is added to the skipchain
// specified by TargetSkipChainID.
type StoreSkipBlock struct {
	TargetSkipChainID SkipBlockID
	NewBlock          *SkipBlock
	Signature         *[]byte
}

// StoreSkipBlockReply - returns the signed SkipBlock with updated backlinks
type StoreSkipBlockReply struct {
	Previous *SkipBlock
	Latest   *SkipBlock
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	LatestID SkipBlockID
}

// GetUpdateChainReply - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	Update []*SkipBlock
}

// GetAllSkipchains - returns all known last blocks of skipchains.
type GetAllSkipchains struct {
}

// GetAllSkipchainsReply - returns all known last blocks of skipchains.
type GetAllSkipchainsReply struct {
	SkipChains []*SkipBlock
}

// Internal calls

// PropagateSkipBlocks sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlocks struct {
	SkipBlocks []*SkipBlock
}

// ForwardSignature is called once a new skipblock has been accepted by
// signing the forward-link, and then the older skipblocks need to
// update their forward-links. Each cothority needs to get the necessary
// blocks and propagate the skipblocks itself.
type ForwardSignature struct {
	// TargetHeight is the index in the backlink-slice of the skipblock
	// to update
	TargetHeight int
	// Previous is the second-newest skipblock
	Previous SkipBlockID
	// Newest is the newest skipblock, signed by previous
	Newest *SkipBlock
}

// GetSingleBlock asks for a single block.
type GetSingleBlock struct {
	ID SkipBlockID
}

// GetSingleBlockByIndex asks for a single block.
type GetSingleBlockByIndex struct {
	Genesis SkipBlockID
	Index   int
}

// Internal calls

// GetBlock asks for an updated block, in case for a conode that is not
// in the roster-list of that block.
type GetBlock struct {
	ID SkipBlockID
}

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	SkipBlock *SkipBlock
}

// GetBlockReply returns the requested block.
type GetBlockReply struct {
	SkipBlock *SkipBlock
}

// Protocol messages

// ProtoExtendSignature can be used as proof that a node accepted to be included
// in a new roster.
type ProtoExtendSignature struct {
	SI        network.ServerIdentityID
	Signature []byte
}

// ProtoExtendRoster asks a conode whether it would be OK to accept a new block
// with himself as part of the roster.
type ProtoExtendRoster struct {
	Block SkipBlock
}

// ProtoStructExtendRoster embeds the treenode.
type ProtoStructExtendRoster struct {
	*onet.TreeNode
	ProtoExtendRoster
}

// ProtoExtendRosterReply is a signature on the Genesis-id.
type ProtoExtendRosterReply struct {
	Signature *[]byte
}

// ProtoStructExtendRosterReply embeds the treenode
type ProtoStructExtendRosterReply struct {
	*onet.TreeNode
	ProtoExtendRosterReply
}

// ProtoGetBlocks requests from another conode up to Count blocks,
// traversing the skiplist forward from SBID.
type ProtoGetBlocks struct {
	SBID  SkipBlockID
	Count int
	// Do the returned blocks skip forward in the chain, or
	// are direct neighbors (not Skipping).
	Skipping bool
}

// ProtoStructGetBlocks embeds the treenode
type ProtoStructGetBlocks struct {
	*onet.TreeNode
	ProtoGetBlocks
}

// ProtoGetBlocksReply returns a slice of blocks - either from update or from getblock
type ProtoGetBlocksReply struct {
	SkipBlocks []*SkipBlock
}

// ProtoStructGetBlocksReply embeds the treenode
type ProtoStructGetBlocksReply struct {
	*onet.TreeNode
	ProtoGetBlocksReply
}

// CreateLinkPrivate asks to store the given public key in the list of administrative
// clients.
type CreateLinkPrivate struct {
	Public    kyber.Point
	Signature []byte
}

// Unlink requests the conode to remove the link from its Internal
// table of links. The signature has to be on the message
// "unlink:" + the byte-representation of the public key to remove.
type Unlink struct {
	Public    kyber.Point
	Signature []byte
}

// Listlink requests a list of all public keys stored in this
// conode and allowed to request administrative tasks.
type Listlink struct{}

// ListlinkReply returns the list of public keys allowed to
// do administrative tasks on this conode. If the list is empty,
// then this node is not secured.
type ListlinkReply struct {
	Publics []kyber.Point
}

// EmptyReply is an empty reply. If there was an error in the
// request, it will be returned
type EmptyReply struct{}

// SettingAuthentication sets the authentication bit that enables restriction
// of the skipchains that are accepted. It needs to be signed by one of the
// clients. The signature is on []byte{0} if Authentication is false and on
// []byte{1} if the Authentication is true.
// TODO: perhaps we need to protect this against replay-attacks by adding a
// monotonically increasing nonce that is also stored on the conode.
type SettingAuthentication struct {
	Authentication int
	Signature      []byte
}

// AddFollow adds a skipchain to follow. The Signature is on the SkipchainID concatenated
// with the Follow as a byte and the Conode.
// The Follow is one of the following:
//   * FollowID will store this skipchain-id and only allow evolution of
//   this skipchain. This implies NewChainNone.
//   * FollowType asks all stored skipchains if it knows that skipchain. All
//   PolicyNewChain are allowed.
//   * FollowLookup takes a ip:port where the skipchain can be found. All
//   PolicyNewChain are allowed.
// The NewChain-policy is ignored for FollowID, but for the other policies
// it is defined as follows:
//   * NewChainNone doesn't allow any new chains from any node from this skipchain.
//   * NewChainAnyNode allows new chains if any node from this skipchain is present.
//   * NewChainStrictNodes allows new chains only one or more nodes from this skipchain
//   are present in the new chain.
type AddFollow struct {
	SkipchainID SkipBlockID
	Follow      FollowType
	NewChain    PolicyNewChain
	Conode      *network.ServerIdentity
	Signature   []byte
}

// DelFollow removes a skipchain from following. The Signature is on the SkipchainID.
type DelFollow struct {
	SkipchainID SkipBlockID
	Signature   []byte
}

// ListFollow returns all followed lists all skipchains we follow.
// The signature has to be on the following message:
// "listfollow:" + the public key of the conode
type ListFollow struct {
	Signature []byte
}

// ListFollowReply returns the genesis-blocks of all skipchains we follow
type ListFollowReply struct {
	Follow    *[]FollowChainType
	FollowIDs *[]SkipBlockID
}
