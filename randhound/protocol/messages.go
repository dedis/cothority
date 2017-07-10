package protocol

import (
	"time"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	types := []interface{}{
		I1{}, R1{},
		I2{}, R2{},
		I3{}, R3{},
		WI1{}, WR1{},
		WI2{}, WR2{},
		WI3{}, WR3{},
	}
	for _, p := range types {
		network.RegisterMessage(p)
	}
}

// I1 is the message sent by the client to the servers in step 1.
type I1 struct {
	Sig     []byte    // Schnorr signature
	SID     []byte    // Session identifier
	Groups  int       // Number of groups
	Seed    []byte    // Sharding seed
	Purpose string    // Purpose of protocol run
	Time    time.Time // Timestamp of protocol initiation
}

// R1 is the reply sent by the servers to the client in step 2.
type R1 struct {
	Sig       []byte           // Schnorr signature
	SID       []byte           // Session identifier
	HI1       []byte           // Hash of I1
	EncShares []*Share         // Encrypted shares
	Coeffs    []abstract.Point // Commitments to polynomial coefficients
	V         abstract.Point   // Server commitment used to sign chosen secrets
}

// I2 is the message sent by the client to the servers in step 3.
type I2 struct {
	Sig           []byte           // Schnorr signature
	SID           []byte           // Session identifier
	ChosenSecrets []uint32         // Chosen secrets
	EncShares     []*Share         // Encrypted PVSS shares
	Evals         []abstract.Point // Commitments of polynomial evaluations
	C             abstract.Scalar  // Challenge used to sign chosen secrets
}

// R2 is the reply sent by the servers to the client in step 4.
type R2 struct {
	Sig []byte          // Schnorr signature
	SID []byte          // Session identifier
	HI2 []byte          // Hash of I2
	R   abstract.Scalar // Response used to sign chosen secrets
}

// I3 is the message sent by the client to the servers in step 5.
type I3 struct {
	Sig   []byte // Schnorr signature
	SID   []byte // Session identifier
	CoSig []byte // Collective signature on chosen secrets
}

// R3 is the reply sent by the servers to the client in step 6.
type R3 struct {
	Sig       []byte   // Schnorr signature
	SID       []byte   // Session identifier
	HI3       []byte   // Hash of I3
	DecShares []*Share // Decrypted PVSS shares
}

// WI1 is a onet-wrapper around I1.
type WI1 struct {
	*onet.TreeNode
	I1
}

// WR1 is a onet-wrapper around R1.
type WR1 struct {
	*onet.TreeNode
	R1
}

// WI2 is a onet-wrapper around I2.
type WI2 struct {
	*onet.TreeNode
	I2
}

// WR2 is a onet-wrapper around R2.
type WR2 struct {
	*onet.TreeNode
	R2
}

// WI3 is a onet-wrapper around I3.
type WI3 struct {
	*onet.TreeNode
	I3
}

// WR3 is a onet-wrapper around R3.
type WR3 struct {
	*onet.TreeNode
	R3
}
