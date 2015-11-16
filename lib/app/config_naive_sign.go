package app

import ()

// Configuration-structure for the 'naive' signing-implementation
type NaiveConfig struct {
	// Hosts per node
	Ppm int

	// A list of all hosts that will participate. The first one in the list
	// is the master
	Hosts []string

	// What suite to use - standard is ed25519
	Suite string

	// How many rounds to measure
	Rounds int

	// The debug-level to use when running the application
	Debug int

	// Whether to skip the checks
	SkipChecks bool
}
