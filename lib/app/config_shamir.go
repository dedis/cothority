package app
import (
)

type ConfigShamir struct{
	// hpn is the replication factor of hosts per node: how many hosts do we want per node
	Hpn         int
	// All hostnames concatenated with the port-number to use
	Hosts       []string
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite       string
	// How many rounds
	Rounds      int
	// Debug-level
	Debug       int
}
