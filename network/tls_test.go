package network

import (
	"math/big"
	"testing"

	"net"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/require"
)

func TestNewTLSCert(t *testing.T) {
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{},
		[]net.IP{})
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{},
		[]net.IP{})
	require.Equal(t, []string{"ch"}, c2.Subject.Country)
	require.Equal(t, []string{"ch"}, c1.Subject.Country)
	require.Equal(t, []string{"dedis"}, c1.Subject.OrganizationalUnit)
	require.Equal(t, []string{"lca1"}, c2.Subject.OrganizationalUnit)
}

func TestNewTLSKeyCert(t *testing.T) {
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{},
		[]net.IP{})
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{},
		[]net.IP{})
	_, _, err := NewCertKey(c1, 256)
	log.ErrFatal(err)
	_, _, err = NewCertKey(c2, 256)
	log.ErrFatal(err)
}

func TestNewTLSRouter(t *testing.T) {
	si1 := NewTestServerIdentity("tls://localhost:2000")
	si2 := NewTestServerIdentity("tls://localhost:2001")
	ips, err := net.LookupIP("localhost")
	log.ErrFatal(err)
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{}, ips)
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{}, ips)
	var key1, key2 TLSKeyPEM
	si1.Cert, key1, _ = NewCertKey(c1, 256)
	si2.Cert, key2, _ = NewCertKey(c2, 256)
	r1, err := NewTLSRouter(si1, key1)
	log.ErrFatal(err)
	r2, err := NewTLSRouter(si2, key2)
	log.ErrFatal(err)

	go r1.Start()
	go r2.Start()
	defer r1.Stop()
	defer r2.Stop()

	c21, err := r2.connect(si1)
	log.ErrFatal(err)
	msg := &BigMsg{Array: []byte{1, 2, 3}}
	log.ErrFatal(c21.Send(msg))
	log.ErrFatal(c21.Close())

	si1_numerical := NewTestServerIdentityTLS("tls://127.0.0.1:2000", si1.Cert)
	c21, err = r2.connect(si1_numerical)
	log.ErrFatal(err)
	log.ErrFatal(c21.Send(msg))
	log.ErrFatal(c21.Close())
}
