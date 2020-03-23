package darc

import (
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3/network"

	"github.com/ethereum/go-ethereum/common"
)

func init() {
	network.RegisterMessages(
		Darc{}, Identity{}, Signature{},
	)
}

// PROTOSTART
// type :expression.Expr:bytes
// type :ID:bytes
// type :Action:string
// type :common.Address:bytes
// package darc;
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "DarcProto";

// ***
// These are the messages used in the API-calls
// ***

// Darc is the basic structure representing an access control. A Darc can
// evolve in the way that a new Darc points to the previous one and is signed
// by the owner(s) of the previous Darc.
type Darc struct {
	// Version should be monotonically increasing over the evolution of a
	// Darc.
	Version uint64
	// Description is a free-form field that can hold any data as required
	// by the user. Darc itself will never depend on any of the data in
	// here.
	Description []byte
	// BaseID is the ID of the first darc in the chain of evolution. It is
	// not set if the darc is on version 0.
	// optional
	BaseID ID
	// PrevID is the previous darc ID in the chain of evolution.
	PrevID ID
	// Rules map an action to an expression.
	Rules Rules
	// Signature is calculated on the Request-representation of the darc.
	// It needs to be created by identities that have the "_evolve" action
	// from the previous valid Darc.
	Signatures []Signature
	// VerificationDarcs are a list of darcs that the verifier needs to
	// verify this darc. It is not needed in online verification where the
	// verifier stores all darcs.
	VerificationDarcs []*Darc
}

// Identity is a generic structure can be either an Ed25519 public key, a Darc
// or a X509 Identity.
type Identity struct {
	// Darc identity
	Darc *IdentityDarc
	// Public-key identity
	Ed25519 *IdentityEd25519
	// Public-key identity from an X.509 certificate.
	X509EC *IdentityX509EC
	// A claim which has been signed by a proxy or proxies.
	Proxy *IdentityProxy
	// Address of an EVM contract
	EvmContract *IdentityEvmContract
}

// IdentityEd25519 holds a Ed25519 public key (Point)
type IdentityEd25519 struct {
	Point kyber.Point
}

// IdentityX509EC holds a public key from a X509EC
type IdentityX509EC struct {
	Public []byte
}

// IdentityProxy holds the info necessary to verify a claim
// from an external authentication system via an Authentication Proxy.
type IdentityProxy struct {
	Data   string
	Public kyber.Point
}

// IdentityDarc is a structure that points to a Darc with a given ID on a
// skipchain. The signer should belong to the Darc.
type IdentityDarc struct {
	// Signer SignerEd25519
	ID ID
}

// IdentityEvmContract holds the address of an EVM contract.
type IdentityEvmContract struct {
	BEvmID  []byte // BEvm InstanceID
	Address common.Address
}

// Signature is a signature on a Darc to accept a given decision.
// can be verified using the appropriate identity.
type Signature struct {
	// The signature itself
	Signature []byte
	// Signer is the Idenity (public key or another Darc) of the signer
	Signer Identity
}

// Signer is a generic structure that can hold different types of signers
type Signer struct {
	Ed25519     *SignerEd25519
	X509EC      *SignerX509EC
	Proxy       *SignerProxy
	EvmContract *SignerEvmContract
}

// SignerEd25519 holds a public and private keys necessary to sign Darcs
type SignerEd25519 struct {
	Point  kyber.Point
	Secret kyber.Scalar
}

// SignerX509EC holds a public and private keys necessary to sign Darcs,
// but the private key will not be given out.
type SignerX509EC struct {
	Point  []byte
	secret []byte
}

// SignerProxy holds the information necessary to verify claims
// coming from external authentication systems via Authentication Proxies.
type SignerProxy struct {
	Data         string
	Public       kyber.Point
	getSignature func([]byte) ([]byte, error)
}

// SignerEvmContract holds the address of an EVM contract.
type SignerEvmContract struct {
	BEvmID  []byte // BEvm InstanceID
	Address common.Address
}

// Request is the structure that the client must provide to be verified
type Request struct {
	BaseID     ID
	Action     Action
	Msg        []byte
	Identities []Identity
	Signatures [][]byte
}

// Rules is a list of action-expression associations.
type Rules struct {
	List []Rule
}

// Rule is a pair of action and expression.
type Rule struct {
	Action Action
	Expr   expression.Expr
}
