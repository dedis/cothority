package lib

import (
	"testing"

	"go.dedis.ch/kyber/v3"

	"github.com/stretchr/testify/assert"
)

// genBox generates a box of encrypted ballots.
func genBox(key kyber.Point, n int) *Box {
	ballots := make([]*Ballot, n)
	for i := range ballots {
		a, b := Encrypt(key, []byte{byte(i)})
		ballots[i] = &Ballot{User: uint32(i), Alpha: a, Beta: b}
	}
	return &Box{Ballots: ballots}
}

func TestSplit(t *testing.T) {
	_, X := RandomKeyPair()
	ballots := genBox(X, 2).Ballots

	a, b := Split(ballots)
	assert.Equal(t, ballots[0].Alpha, a[0][0])
	assert.Equal(t, ballots[0].Beta, b[0][0])
	assert.Equal(t, ballots[1].Alpha, a[0][1])
	assert.Equal(t, ballots[1].Beta, b[0][1])
}

func TestCombine(t *testing.T) {
	_, X1 := RandomKeyPair()
	_, X2 := RandomKeyPair()

	a, b := [][]kyber.Point{{X1, X2}, {X2, X1}}, [][]kyber.Point{{X2, X1}, {X1, X2}}
	ballots := Combine(a, b)

	assert.Equal(t, X1, ballots[0].Alpha)
	assert.Equal(t, X2, ballots[1].Alpha)
	assert.Equal(t, X2, ballots[0].Beta)
	assert.Equal(t, X1, ballots[1].Beta)
	assert.Equal(t, X2, ballots[0].AdditionalAlphas[0])
	assert.Equal(t, X1, ballots[1].AdditionalAlphas[0])
	assert.Equal(t, X1, ballots[0].AdditionalBetas[0])
	assert.Equal(t, X2, ballots[1].AdditionalBetas[0])
}

func TestScipersToBuf(t *testing.T) {
	scipers := []uint32{0x100000, 0x100001, 0x100002}

	buf := scipersToBuf(scipers[0:1])
	assert.Equal(t, 3, len(buf))
	assert.Equal(t, uint8(0x0), buf[0])
	assert.Equal(t, uint8(0x0), buf[1])
	assert.Equal(t, uint8(0x10), buf[2])

	buf = scipersToBuf(scipers[0:2])
	assert.Equal(t, 6, len(buf))

	buf = scipersToBuf(scipers)
	assert.Equal(t, 9, len(buf))
	assert.Equal(t, uint8(0x0), buf[0])
	assert.Equal(t, uint8(0x0), buf[1])
	assert.Equal(t, uint8(0x10), buf[2])
	assert.Equal(t, uint8(0x1), buf[3])
	assert.Equal(t, uint8(0x0), buf[4])
	assert.Equal(t, uint8(0x10), buf[5])
	assert.Equal(t, uint8(0x2), buf[6])
	assert.Equal(t, uint8(0x0), buf[7])
	assert.Equal(t, uint8(0x10), buf[8])
}
