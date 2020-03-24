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
	assert.Equal(t, ballots[0].Alpha, a[0])
	assert.Equal(t, ballots[0].Beta, b[0])
	assert.Equal(t, ballots[1].Alpha, a[1])
	assert.Equal(t, ballots[1].Beta, b[1])
}

func TestCombine(t *testing.T) {
	_, X1 := RandomKeyPair()
	_, X2 := RandomKeyPair()

	a, b := []kyber.Point{X1, X1}, []kyber.Point{X2, X2}
	ballots := Combine(a, b)

	assert.Equal(t, X1, ballots[0].Alpha)
	assert.Equal(t, X1, ballots[1].Alpha)
	assert.Equal(t, X2, ballots[0].Beta)
	assert.Equal(t, X2, ballots[1].Beta)
}
