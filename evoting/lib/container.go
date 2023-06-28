package lib

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/proof"
	"go.dedis.ch/kyber/v3/share/dkg/rabin"
	"go.dedis.ch/kyber/v3/shuffle"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3/log"
	"strconv"
)

// CandidatesPerPoint indicates how many candidates fit into an ed25519 Point:
// As each candidate takes 3 bytes, and an ed25519 Point allows to embed 30 bytes,
// there is space for 10 candidates in one ed25519 Point.
// For some obscure reason (off-by-one?) the code checks for 9 candidates, so we
// keep this for not breaking previous elections.
const CandidatesPerPoint = 9

// Box is a wrapper around a list of encrypted ballots.
type Box struct {
	Ballots []*Ballot
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
func Split(ballots []*Ballot) (alpha, beta [][]kyber.Point) {
	nbrBallots := len(ballots)
	// Each encrypted point holds protocol.CandidatePerPoint choices of a voter.
	// For backward-compatibility, the first point is Ballot.Alpha / Ballot.Beta.
	nbrPoints := 1
	if nbrBallots > 0 {
		nbrPoints += len(ballots[0].AdditionalAlphas)
	}

	// This is to make alpha and beta compatible with the dedis/kyber/shuffle/sequences.go
	alpha, beta = make([][]kyber.Point, nbrPoints), make([][]kyber.Point, nbrPoints)
	for i := range alpha {
		alpha[i] = make([]kyber.Point, nbrBallots)
		beta[i] = make([]kyber.Point, nbrBallots)
	}

	for i := range ballots {
		b := ballots[i]
		alpha[0][i] = b.Alpha
		beta[0][i] = b.Beta
		for j := 1; j < nbrPoints; j++ {
			alpha[j][i] = b.AdditionalAlphas[j-1]
			beta[j][i] = b.AdditionalBetas[j-1]
		}
	}
	return
}

// Combine creates a list of ballots from two lists of points.
func Combine(alpha, beta [][]kyber.Point) []*Ballot {
	ballots := make([]*Ballot, len(alpha[0]))
	for i := range ballots {
		ballots[i] = &Ballot{Alpha: alpha[0][i], Beta: beta[0][i]}
		if len(alpha) > 1 {
			ballots[i].AdditionalAlphas = make([]kyber.Point, len(alpha)-1)
			ballots[i].AdditionalBetas = make([]kyber.Point, len(alpha)-1)
			for j := range ballots[i].AdditionalAlphas {
				ballots[i].AdditionalAlphas[j] = alpha[j+1][i]
				ballots[i].AdditionalBetas[j] = beta[j+1][i]
			}
		}
	}
	return ballots
}

// CreateShuffleProof takes the alphas and betas of a Ballot, shuffles them, and then returns the result
// as well as a proof.
// If the alphas are of length one, it uses the algorithm before July 2023.
// If the alphas are longer, it uses the shuffle.SequencesShuffle.
func CreateShuffleProof(alphas, betas [][]kyber.Point, pubKey kyber.Point) ([][]kyber.Point, [][]kyber.Point, []byte, error) {
	// Protect from missing input.
	for i := range alphas {
		for j := range alphas[i] {
			if alphas[i][j] == nil {
				alphas[i][j] = cothority.Suite.Point().Null()
			}
		}
	}
	for i := range betas {
		for j := range betas[i] {
			if betas[i][j] == nil {
				betas[i][j] = cothority.Suite.Point().Null()
			}
		}
	}

	// Backward compatible handling of creating the shuffle proof
	if len(alphas) == 1 {
		g, d, prov := shuffle.Shuffle(cothority.Suite, nil, pubKey, alphas[0], betas[0], random.New())
		shuffleProof, err := proof.HashProve(cothority.Suite, "", prov)
		return [][]kyber.Point{g}, [][]kyber.Point{d}, shuffleProof, err
	}

	// New shuffle proof using `shuffle.SequencesShuffle`
	g, d, provSeq := shuffle.SequencesShuffle(cothority.Suite, nil, pubKey, alphas, betas, random.New())
	// Creating a prover taken from the `shuffle.Shuffle` method
	beta := make([]kyber.Scalar, len(alphas))
	for i := 0; i < len(alphas); i++ {
		beta[i] = cothority.Suite.Scalar().Pick(random.New())
	}
	prov, err := provSeq(beta)
	shuffleProof, err := proof.HashProve(cothority.Suite, "", prov)
	return g, d, shuffleProof, err
}

// CreateBallot returns a ballot for a user. It makes sure that enough alphas and betas are initialized,
// even if the user votes for less than maxChoices candidates.
func CreateBallot(maxChoices int, pubKey kyber.Point, user uint32, elected []uint32) Ballot {
	totalBufs := (maxChoices-1)/9 + 1
	k := make([]kyber.Point, totalBufs)
	c := make([]kyber.Point, totalBufs)
	for buf := range k {
		end := (buf + 1) * CandidatesPerPoint
		if end > maxChoices {
			end = maxChoices
		}
		if end > len(elected) {
			end = len(elected)
		}
		start := buf * CandidatesPerPoint
		var electedSlice []uint32
		if start < maxChoices && start < len(elected) {
			electedSlice = elected[start:end]
		}
		log.Printf("electedSlice for buf %d is: %x", buf, electedSlice)
		k[buf], c[buf] = Encrypt(pubKey, scipersToBuf(electedSlice))
	}
	ballot := Ballot{
		User:  user,
		Alpha: k[0],
		Beta:  c[0],
	}
	if maxChoices > CandidatesPerPoint {
		ballot.AdditionalAlphas = k[1:]
		ballot.AdditionalBetas = c[1:]
	}
	log.Printf("ballot is: %+v", ballot)
	return ballot
}

// GenerateSignature creates a USELESS signature which should be replaced for any follow-up implementation.
// It merely signs the ID of the chain, and as such does not provide any guarantee that the user signing
// actually wanted to do anything with regard to the additional data...
// Also, if the signature fails, this method panics.
func GenerateSignature(private kyber.Scalar, ID []byte, sciper uint32) []byte {
	message := ID
	for _, c := range strconv.Itoa(int(sciper)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	sig, err := schnorr.Sign(cothority.Suite, private, message)
	if err != nil {
		panic("cannot sign:" + err.Error())
	}
	return sig
}

func scipersToBuf(scipers []uint32) []byte {
	var buf = make([]byte, len(scipers)*3)
	for i := range buf {
		s := scipers[i/3] >> ((i % 3) * 8)
		buf[i] = byte(s & 0xff)
	}
	return buf
}
