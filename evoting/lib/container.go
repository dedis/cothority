package lib

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/proof"
	"github.com/dedis/kyber/share/dkg/rabin"
	"github.com/dedis/kyber/shuffle"
	"github.com/dedis/kyber/util/random"

	"github.com/dedis/cothority"
)

// Ballot represents an encrypted vote.
type Ballot struct {
	User uint32 // User identifier.

	// ElGamal ciphertext pair.
	Alpha kyber.Point
	Beta  kyber.Point
}

// Box is a wrapper around a list of encrypted ballots.
type Box struct {
	Ballots []*Ballot
}

// genMix generates n mixes with corresponding proofs out of the ballots.
func (b *Box) genMix(key kyber.Point, n int) []*Mix {
	mixes := make([]*Mix, n)

	x, y := Split(b.Ballots)
	for i := range mixes {
		v, w, prover := shuffle.Shuffle(cothority.Suite, nil, key, x, y, random.New())
		proof, _ := proof.HashProve(cothority.Suite, "", prover)
		mixes[i] = &Mix{Ballots: Combine(v, w), Proof: proof, Node: string(i)}
		x, y = v, w
	}
	return mixes
}

// Mix contains the shuffled ballots.
type Mix struct {
	Ballots []*Ballot // Ballots are permuted and re-encrypted.
	Proof   []byte    // Proof of the shuffle.

	Node string // Node signifies the creator of the mix.
}

// Partial contains the partially decrypted ballots.
type Partial struct {
	Points []kyber.Point // Points are the partially decrypted plaintexts.

	Flag bool   // Flag signals if the mixes could not be verified.
	Node string // Node signifies the creator of this partial decryption.
}

// genPartials generates partial decryptions for a given list of shared secrets.
func (m *Mix) genPartials(dkgs []*dkg.DistKeyGenerator) []*Partial {
	partials := make([]*Partial, len(dkgs))

	for i, gen := range dkgs {
		secret, _ := NewSharedSecret(gen)
		points := make([]kyber.Point, len(m.Ballots))
		for j, ballot := range m.Ballots {
			points[j] = Decrypt(secret.V, ballot.Alpha, ballot.Beta)
		}
		partials[i] = &Partial{Points: points, Node: string(i)}
	}
	return partials
}

// genBox generates a box of encrypted ballots.
func genBox(key kyber.Point, n int) *Box {
	ballots := make([]*Ballot, n)
	for i := range ballots {
		a, b := Encrypt(key, []byte{byte(i)})
		ballots[i] = &Ballot{User: uint32(i), Alpha: a, Beta: b}
	}
	return &Box{Ballots: ballots}
}

// Split separates the ElGamal pairs of a list of ballots into separate lists.
func Split(ballots []*Ballot) (alpha, beta []kyber.Point) {
	n := len(ballots)
	alpha, beta = make([]kyber.Point, n), make([]kyber.Point, n)
	for i := range ballots {
		alpha[i] = ballots[i].Alpha
		beta[i] = ballots[i].Beta
	}
	return
}

// Combine creates a list of ballots from two lists of points.
func Combine(alpha, beta []kyber.Point) []*Ballot {
	ballots := make([]*Ballot, len(alpha))
	for i := range ballots {
		ballots[i] = &Ballot{Alpha: alpha[i], Beta: beta[i]}
	}
	return ballots
}
