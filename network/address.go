package network

import (
	"net"
	"strings"
)

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
	// Local represents a channel based connection type
	Local = "local"
	// UnvalidConnType represents a non valid connection type
	UnvalidConnType = "wrong"
)

// typeAddressSep is the separator between the type of connection and the actual
// ip address.
const typeAddressSep = "://"

func connType(t string) ConnType {
	ct := ConnType(t)
	types := []ConnType{PlainTCP, TLS, PURB, Local}
	for _, t := range types {
		if t == ct {
			return ct
		}
	}
	return UnvalidConnType
}

// ConnType return the connection type from this address
// It returns an unvalid connection type if the address is not valid or if the
// connection type is not known.
func (a Address) ConnType() ConnType {
	if !a.Valid() {
		return UnvalidConnType
	}
	vals := strings.Split(string(a), typeAddressSep)
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
	vals := strings.Split(string(a), typeAddressSep)
	if len(vals) != 2 {
		return ""
	}
	return vals[1]
}

// Valid returns true if the address is well formed or false otherwise.
// An address is well formed if it is of the form: ConnType:NetworkAddress.
// ConnType must be one of the constants defined in this file,
// NetworkAddress must contain the IP address + Port number.
// Ex. tls:192.168.1.10:5678
func (a Address) Valid() bool {
	vals := strings.Split(string(a), typeAddressSep)
	if len(vals) != 2 {
		return false
	}
	if connType(vals[0]) == UnvalidConnType {
		return false
	}

	if ip, _, e := net.SplitHostPort(vals[1]); e != nil {
		return false
	} else if ip == "localhost" {
		// localhost is not recognized by net.ParseIP ?
		return true
	} else if net.ParseIP(ip) == nil {
		return false
	}
	return true
}

func (a Address) String() string {
	return string(a)
}

// Host returns the host part of the address.
// ex: "tcp://127.0.0.1:2000" => "127.0.0.1"
func (a Address) Host() string {
	na := a.NetworkAddress()
	if na == "" {
		return ""
	}
	h, _, e := net.SplitHostPort(a.NetworkAddress())
	if e != nil {
		return ""
	}
	return h
}

// Port will return the port part of the Address
func (a Address) Port() string {
	na := a.NetworkAddress()
	if na == "" {
		return ""
	}
	_, p, e := net.SplitHostPort(na)
	if e != nil {
		return ""
	}
	return p

}

// NewAddress takes a connection type and the raw address. It returns a
// correctly formatted address, which will be of type t.
func NewAddress(t ConnType, network string) Address {
	return Address(string(t) + typeAddressSep + network)
}
