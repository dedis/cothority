package network

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

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
	t := &TLSListener{
		TCPListener: &TCPListener{
			quit:         make(chan bool),
			quitListener: make(chan bool),
		},
	}
	global, _ := GlobalBind(addr.NetworkAddress())
	config, err := key.ConfigServer(si.Cert)
	if err != nil {
		return nil, err
	}
	for i := 0; i < MaxRetryConnect; i++ {
		log.Lvl3("Calling listener on", global)

		ln, err := tls.Listen("tcp", global, config)
		if err == nil {
			t.listener = ln
			break
		}
		time.Sleep(WaitRetry)
	}
	if t.listener == nil {
		return nil, errors.New("Error opening listener: " + err.Error())
	}
	t.addr = t.listener.Addr()
	log.Lvl3("Got address", t.addr)
	return t, nil
}

// Listen starts to listen for incoming connections and calls fn for every
// connection-request it receives.
// If the connection is closed, an error will be returned.
func (t *TLSListener) Listen(fn func(Conn)) error {
	receiver := func(tc Conn) {
		tls := TLSConn{TCPConn: tc.(*TCPConn)}
		tls.endpoint = NewTLSAddress(tc.Remote().String())
		go fn(tls)
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
	addr := si.Address
	netAddr := addr.NetworkAddress()
	config, err := si.Cert.ConfigClient()
	if err != nil {
		return nil, err
	}
	var conn *tls.Conn
	for i := 0; i < MaxRetryConnect; i++ {
		log.Lvl3("Connecting to", netAddr)
		conn, err = tls.Dial("tcp", netAddr, config)
		if err == nil {
			log.Lvl3("client: connected to:", conn.RemoteAddr())

			state := conn.ConnectionState()
			for _, v := range state.PeerCertificates {
				b, _ := x509.MarshalPKIXPublicKey(v.PublicKey)
				log.Lvlf3("Connection by %s with key %x",
					v.Subject, b)
			}
			log.Lvl3("client: handshake:", state.HandshakeComplete)
			log.Lvl3("client: mutual:", state.NegotiatedProtocolIsMutual)
			return &TLSConn{&TCPConn{
				endpoint: addr,
				conn:     conn,
			}}, nil
		}
		time.Sleep(WaitRetry)
	}
	return nil, errors.New("Error opening listener: " + err.Error())
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
