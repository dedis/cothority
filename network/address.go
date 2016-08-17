package network

import "net"
import "strings"

// ConnType is a type to represent what is the type of a Conn
// It can be of different types such as TCP, TLS etc.
type ConnType string

// Address contains the ConnType and the actual network address. It is used to connect
// to a remote host with a Conn and to listen by a Listener.
// A network address is comprised of the IP address and the port number joined
// by a colon.
// For the moment, there is no IPv6 support.
type Address string

const (
	// PlainTCP represents a vanilla TCP connection
	PlainTCP ConnType = "tcp"
	// TLS represents a TLS encrypted connection over TCP
	TLS = "tls"
	// PURB represents a PURB encryption connection over TCP
	PURB = "purb"
	// Unvalid represents a non valid connection type
	UnvalidConnType = "wrong"
)

func connType(t string) ConnType {
	ct := ConnType(t)
	if ct == PlainTCP || ct == TLS || ct == PURB {
		return ct
	}
	return UnvalidConnType
}

// ConnType return the connection type from this address
func (a Address) ConnType() ConnType {
	vals := strings.Split(string(a), ":")
	if len(vals) == 0 {
		return UnvalidConnType
	}
	return connType(vals[0])
}

// NetworkAddress returns the network address part of an Address. That includes
// the IP address and the port joined by a colon.
// It returns an empty string if the a.Valid() returns false.
func (a Address) NetworkAddress() string {
	if !a.Valid() {
		return ""
	}
	vals := strings.Split(string(a), ":")
	if len(vals) != 3 {
		return ""
	}
	return vals[1] + ":" + vals[2]
}

// Valid returns true if the address is well formed or false otherwise.
// An address is well formed if it is of the form: ConnType:NetworkAddress.
// ConnType must be one of the constants defined in this file,
// NetworkAddress must contain the IP address + Port number.
// Ex. tls:192.168.1.10:5678
func (a Address) Valid() bool {
	vals := strings.Split(string(a), ":")
	if len(vals) != 3 {
		return false
	}
	if connType(vals[0]) == UnvalidConnType {
		return false
	}

	if _, _, e := net.SplitHostPort(vals[1] + ":" + vals[2]); e != nil {
		return false
	}
	return true
}
