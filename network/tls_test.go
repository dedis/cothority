package network

import (
	"math/big"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/require"
)

func TestNewTLSCert(t *testing.T) {
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{})
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{})
	require.Equal(t, []string{"ch"}, c2.Subject.Country)
	require.Equal(t, []string{"ch"}, c1.Subject.Country)
	require.Equal(t, []string{"dedis"}, c1.Subject.OrganizationalUnit)
	require.Equal(t, []string{"lca1"}, c2.Subject.OrganizationalUnit)
}

func TestNewTLSKeyCert(t *testing.T) {
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{})
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{})
	_, err := NewTLSKC(c1, 2048)
	log.ErrFatal(err)
	_, err = NewTLSKC(c2, 2048)
	log.ErrFatal(err)
}

func TestNewTLSRouter(t *testing.T) {
	si1 := NewTestServerIdentity("tls://localhost:2000")
	si2 := NewTestServerIdentity("tls://localhost:2001")
	c1 := NewTLSCert(big.NewInt(0), "ch", "epfl", "dedis", 10, []byte{})
	c2 := NewTLSCert(big.NewInt(1), "ch", "epfl", "lca1", 10, []byte{})
	si1.TLSKC, _ = NewTLSKC(c1, 2048)
	si2.TLSKC, _ = NewTLSKC(c2, 2048)
	r1, err := NewTLSRouter(si1)
	log.ErrFatal(err)
	r2, err := NewTLSRouter(si2)
	log.ErrFatal(err)

	go r1.Start()
	go r2.Start()
	defer r1.Stop()
	defer r2.Stop()

	_, err = r2.connect(si1)
	log.ErrFatal(err)
}
