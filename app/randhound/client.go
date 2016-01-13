package randhound

import "github.com/dedis/crypto/poly"

type Client struct {

	// TODO: figure out which variables from the old RandHound client (see
	// app/rand/cli.go) are necessary and which ones are covered by SDA

	t Transcript // Third-party verifiable message transcript

	r1 []R1 // Decoded R1 messages
	r2 []R2 // Decoded R2 messages
	r3 []R3 // Decoded R3 messages
	r4 []R4 // Decoded R4 messages

	Rc     []byte           // Client's trustee-selection random value
	Rs     [][]byte         // Server's trustee-selection random values
	deals  []poly.Promise   // Unmarshaled deals from servers
	shares []poly.PriShares // Revealed shares
}
