package chainiac

import (
	"github.com/dedis/cothority/skipchain/service"
	"github.com/satori/go.uuid"
)

var (
	// VerifyRoot depends on a data-block being a slice of public keys
	// that are used to sign the next block. The private part of those
	// keys are supposed to be offline. It makes sure
	// that every new block is signed by the keys present in the previous block.
	VerifyRoot = service.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Root"))
	// VerifyControl makes sure this chain is a child of a Root-chain and
	// that there is now new block if a newer parent is present.
	// It also makes sure that no more than 1/3 of the members of the roster
	// change between two blocks.
	VerifyControl = service.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Control"))
	// VerifyData makes sure that:
	//   - it has a parent-chain with `VerificationControl`
	//   - its Roster doesn't change between blocks
	//   - if there is a newer parent, no new block will be appended to that chain.
	VerifyData = service.VerifierID(uuid.NewV5(uuid.NamespaceURL, "Data"))
)

// VerificationRoot is used to create a root-chain that has 'Control'-chains
// as its children.
var VerificationRoot = []service.VerifierID{service.VerifyBase, VerifyRoot}

// VerificationControl is used in chains that depend on a 'Root'-chain.
var VerificationControl = []service.VerifierID{service.VerifyBase, VerifyControl}

// VerificationData is used in chains that depend on a 'Control'-chain.
var VerificationData = []service.VerifierID{service.VerifyBase, VerifyData}
