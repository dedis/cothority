package lib

import (
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/proof"
	"go.dedis.ch/kyber/v4/share/dkg/rabin"
	"go.dedis.ch/kyber/v4/shuffle"
	"go.dedis.ch/kyber/v4/util/random"
	"go.dedis.ch/onet/v4/network"

	"go.dedis.ch/cothority/v3"
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
		mixes[i] = &Mix{
			Ballots: Combine(v, w),
			Proof:   proof,
		}
		x, y = v, w
	}
	return mixes
}

// Mix contains the shuffled ballots.
type Mix struct {
	Ballots []*Ballot // Ballots are permuted and re-encrypted.
	Proof   []byte    // Proof of the shuffle.

	NodeID    network.ServerIdentityID // Node signifies the creator of the mix.
	Signature []byte                   // Signature of the public key
}

// Partial contains the partially decrypted ballots.
type Partial struct {
	Points []kyber.Point // Points are the partially decrypted plaintexts.

	NodeID    network.ServerIdentityID // NodeID is the node having signed the partial
	Signature []byte                   // Signature of the public key
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
		partials[i] = &Partial{
			Points: points,
		}
	}
	return partials
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
