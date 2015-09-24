package main

import (
	"github.com/dedis/crypto/abstract"
	"time"
	//	"github.com/dedis/crypto/poly/promise"
)

type HostID struct {
	PubKeyID string // Fingerprint of host's public key
	Location string // Network location: hostname[:port]
}

type Config struct {
}

// Session ID uniquely identifying a RandHound protocol run.
type SessionID struct {
	Client  []byte    // Initiating client
	Purpose string    // Purpose of randomness
	Time    time.Time // Scheduled initiation time
}

type GroupConfig struct {
	Servers []HostID // List of servers used
	F       int      // Faulty (Byzantine) hosts tolerated
	L       int      // Hosts that must be live
	K       int      // Trustee set size
	T       int      // Trustee set threshold
}

type I1 struct {
	//XXX	SID		SessionID	// Unique session identifier tuple
	//XXX	Config		GroupConfig	// XXX allow indirect reference?
	HRc []byte // Client's trustee-randomness commit
}

type R1 struct {
	//	HI1		[]byte		// Hash of I1 message responding to
	HRs []byte // Server's trustee-randomness commit
}

type I2 struct {
	//	SID		SessionID	// Unique session identifier tuple
	Rc []byte // Client's trustee-selection randomness
}

type R2 struct {
	//	HI2		[]byte		// Hash of I2 message responding to
	Rs   []byte // Server's trustee-selection randomness
	Deal []byte // Server's secret-sharing to trustees
}

type I3 struct {
	//	SID		SessionID	// Unique session identifier tuple
	// Client's list of signed R2 messages;
	// empty slices represent missing R2 messages.
	R2s [][]byte
}

type R3 struct {
	//	HI3		[]byte		// Hash of I3 message responding to
	Resp []R3Resp // Responses to dealt secret-shares
}

type R3Resp struct {
	Dealer int    // Server number of dealer
	Index  int    // Share number in deal we are validating
	Resp   []byte // Encoded response to dealer's Deal
}

type I4 struct {
	//	SID		SessionID	// Unique session identifier tuple
	// Client's list of signed R2 messages;
	// empty slices for missing or invalid R2s.
	R2s [][]byte
}

type R4 struct {
	//	HI4		[]byte		// Hash of I4 message responding to
	Shares []R4Share // Revealed secret-shares
}

type R4Share struct {
	Dealer int             // Server number of dealer
	Index  int             // Share number in dealer's Deal
	Share  abstract.Secret // Decrypted share dealt to this server
}

type Transcript struct {
	I1 []byte   // I1 message signed by client
	R1 [][]byte // R1 messages signed by resp servers
	I2 []byte
	R2 [][]byte
	I3 []byte
	R3 [][]byte
	I4 []byte
	R4 [][]byte
}

/*
func (t *Transcript) Verify(suite abstract.Suite) error {

	...

	var i1 I1
}
*/
