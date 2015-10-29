package main

// This file regroups different packets used throughout the process
const maxSize = 256

// A packet containing some info about our system
type SystemPacket struct {
	Soft     int           // the soft limits of number of connection the user can have
	Hostname [maxSize]byte // the hostname of the machine so we can try pinging it out
}

// A packet used to ACK a verification a protocol or whatever
// It contains a first  Int to know of what kind of ACK are we talking about
// Then the second int represent the ACK status itself for this specific ACK
type Ack struct {
	Type int
	Code int
}

// Theses consts represent the type of ACK we are reading
const (
	TYPE_SYS = iota // ACK for a SystemPacket
)

// These consts are there for meaningful interpretation of the reponse ACK after
// an SystemPacket sent ;)
const (
	SYS_OK         = iota // everything is fine
	SYS_WRONG_HOST        //hostname is not valid
	SYS_WRONG_SOFT        // soft limits is not enough or wrong. See development team.
	SYS_WRONG_SIG         // THe signature sent after systempacket is not valid
)
