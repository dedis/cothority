package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/assert"
)

func TestNewPGP(t *testing.T) {
	pgp := NewPGP()
	assert.NotNil(t, pgp)
	pgp2 := NewPGP()
	assert.NotEqual(t, pgp.ArmorPrivate(), pgp2.ArmorPrivate())
	assert.NotEqual(t, pgp.ArmorPublic(), pgp2.ArmorPublic())
}

func TestNewPGPPublic(t *testing.T) {
	pgp := NewPGP()
	pgp2 := NewPGPPublic(pgp.ArmorPublic())
	assert.Equal(t, pgp.ArmorPublic(), pgp2.ArmorPublic())
}

func TestPGP_Sign(t *testing.T) {
	msg1 := []byte("msg1")
	msg2 := []byte("msg2")
	pgp := NewPGP()
	pgp2 := NewPGPPublic(pgp.ArmorPublic())
	_, err := pgp2.Sign(msg1)
	assert.NotNil(t, "Cannot sign with missing private key!")
	sig1, err := pgp.Sign(msg1)
	log.ErrFatal(err)
	sig2, err := pgp.Sign(msg2)
	log.ErrFatal(err)
	assert.NotEqual(t, sig1, sig2, "Found signature-collision! (NOT)")
}

func TestPGP_Verify(t *testing.T) {
	msg1 := []byte("msg1")
	msg2 := []byte("msg2")
	pgp := NewPGP()
	pgp2 := NewPGPPublic(pgp.ArmorPublic())
	_, err := pgp2.Sign(msg1)
	assert.NotNil(t, "Cannot sign with missing private key!")
	sig1, err := pgp.Sign(msg1)
	log.ErrFatal(err)
	sig2, err := pgp.Sign(msg2)

	log.ErrFatal(pgp2.Verify(msg1, sig1))
	err = pgp2.Verify(msg1, sig2)
	assert.NotNil(t, err, "Should not verify with wrong signature")
	log.ErrFatal(pgp2.Verify(msg2, sig2))
}
