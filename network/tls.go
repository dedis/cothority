package network

import (
	"crypto/tls"
	"errors"

	"fmt"

	"github.com/dedis/cothority/log"
)

/*
Implementation of a TLS-host to allow for tls-connections. Only the server
will authenticate to the client.
*/

// NewTLSRouter returns a new Router using TLSHost as the underlying Host.
func NewTLSRouter(sid *ServerIdentity, key TLSKeyPEM) (*Router, error) {
	h, err := NewTLSHost(sid, key)
	if err != nil {
		return nil, err
	}
	r := NewRouter(sid, h)
	return r, nil
}

// TLSHost implements the Host interface using TLS connections.
type TLSHost struct {
	*TLSListener
	addr Address
	cert TLSCertPEM
	key  TLSKeyPEM
}

// NewTLSHost returns a new Host using TLS connection based type.
func NewTLSHost(si *ServerIdentity, key TLSKeyPEM) (*TLSHost, error) {
	h := &TLSHost{
		addr: si.Address,
		cert: si.Cert,
		key:  key,
	}
	var err error
	h.TLSListener, err = NewTLSListener(si, key)
	return h, err
}

// Connect establishes a connection to the remote host given in addr.
func (h *TLSHost) Connect(si *ServerIdentity) (Conn, error) {
	log.Lvl3(h.addr, "connecting to", si.Address)
	addr := si.Address
	switch addr.ConnType() {
	case TLS:
		c, err := NewTLSConn(si)
		return c, err
	}
	return nil, fmt.Errorf("Don't support connection: %s", addr.ConnType())
}

// TLSListener implements the Host-interface using TLS as a communication channel.
type TLSListener struct {
	*TCPListener
	TLSConfig *tls.Config
}

// NewTLSListener returns a TLSListener. This function binds to the given
// address.
// It returns the listener and an error if one occurred during
// the binding.
// A subsequent call to Address() gives the actual listening
// address which is different if you gave it a ":0"-address.
func NewTLSListener(si *ServerIdentity, key TLSKeyPEM) (*TLSListener, error) {
	log.Lvl3("Starting to listen on", si)
	addr := si.Address
	if addr.ConnType() != TLS {
		return nil, errors.New("TLSListener can't listen on non-TLS address")
	}
	config, err := key.ConfigServer(si.Cert)
	if err != nil {
		return nil, err
	}
	tcpL, err := NewTCPListener(NewAddress(PlainTCP, si.Address.NetworkAddress()))
	tlsL := &TLSListener{
		TCPListener: tcpL,
		TLSConfig:   config,
	}
	return tlsL, err
}

// Listen starts to listen for incoming connections and calls fn for every
// connection-request it receives.
// If the connection is closed, an error will be returned.
func (t *TLSListener) Listen(fn func(Conn)) error {
	receiver := func(tc Conn) {
		tcpc := tc.(*TCPConn)
		tlsC := TLSConn{tcpc}
		log.Lvl3("Starting handshake")
		if err := tls.Server(tcpc.conn, t.TLSConfig).Handshake(); err != nil {
			log.Error("Couldn't complete handshake:", err)
			return
		}
		log.Lvl3("Handshake done")
		tlsC.endpoint = NewTLSAddress(tc.Remote().String())
		go fn(tlsC)
	}
	return t.listen(receiver)
}

// TLSConn implements the Conn interface using TLS.
type TLSConn struct {
	*TCPConn
}

// NewTLSConn will open a TLSConn to the given address.
// In case of an error it returns a nil TLSConn and the error.
func NewTLSConn(si *ServerIdentity) (*TLSConn, error) {
	//addr := si.Address
	//netAddr := addr.NetworkAddress()
	config, err := si.Cert.ConfigClient()
	if err != nil {
		return nil, err
	}
	if si.Address.ConnType() != TLS {
		return nil, errors.New("This is not a TLS-address.")
	}
	c, err := NewTCPConn(si.Address)
	if err != nil {
		return nil, err
	}
	config.ServerName = si.Address.Host()
	if err = tls.Client(c.conn, config).Handshake(); err != nil {
		return nil, err
	}
	return &TLSConn{c}, nil
}

// NewTLSClient returns a new client using the TLS network communication
// layer.
func NewTLSClient() *Client {
	fn := func(own, remote *ServerIdentity) (Conn, error) {
		return NewTLSConn(remote)
	}
	return newClient(fn)
}

// NewTLSAddress returns a new Address that has type PlainTLS with the given
// address addr.
func NewTLSAddress(addr string) Address {
	return NewAddress(TLS, addr)
}
