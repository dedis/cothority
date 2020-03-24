package bftcosi

import (
	"crypto/sha512"
	"errors"
	"time"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	for _, i := range []interface{}{
		BFTSignature{},
		Announce{},
		Commitment{},
		ChallengePrepare{},
		ChallengeCommit{},
		Response{},
	} {
		network.RegisterMessage(i)
	}
}

// RoundType is a type to know if we are in the "prepare" round or the "commit"
// round
type RoundType int32

const (
	// RoundPrepare is the first round (prepare)
	RoundPrepare RoundType = iota
	// RoundCommit is the final round (Commit)
	RoundCommit
)

// BFTSignature is what a bftcosi protocol outputs. It contains the signature,
// the message and some possible exceptions.
type BFTSignature struct {
	// cosi signature
	Sig []byte
	Msg []byte
	// List of peers that did not want to sign.
	Exceptions []Exception
}

// Verify returns whether the verification of the signature succeeds or not.
// Specifically, it adjusts the signature according to the exception in the
// signature, so it can be verified by dedis/crypto/cosi.
// publics is a slice of all public signatures, and the msg is the msg
// being signed.
func (bs *BFTSignature) Verify(s network.Suite, publics []kyber.Point) error {
	if bs == nil || bs.Sig == nil || bs.Msg == nil {
		return errors.New("Invalid signature")
	}
	// compute the aggregate key of all the signers
	aggPublic := s.Point().Null()
	for i := range publics {
		aggPublic.Add(aggPublic, publics[i])
	}
	// compute the reduced public aggregate key (all - exception)
	aggReducedPublic := aggPublic.Clone()

	// compute the aggregate commit of exception
	aggExCommit := s.Point().Null()
	for _, ex := range bs.Exceptions {
		aggExCommit = aggExCommit.Add(aggExCommit, ex.Commitment)
		aggReducedPublic.Sub(aggReducedPublic, publics[ex.Index])
	}
	// get back the commit to recreate  the challenge
	origCommit := s.Point()
	pointLen := s.PointLen()
	sigLen := pointLen + s.ScalarLen()

	if len(bs.Sig) < sigLen {
		return errors.New("signature too short")
	}
	if err := origCommit.UnmarshalBinary(bs.Sig[0:pointLen]); err != nil {
		return err
	}

	// re create challenge
	h := sha512.New()
	if _, err := origCommit.MarshalTo(h); err != nil {
		return err
	}
	if _, err := aggPublic.MarshalTo(h); err != nil {
		return err
	}
	if _, err := h.Write(bs.Msg); err != nil {
		return err
	}

	// redo like in cosi -k*A + r*B == C
	// only with C being the reduced version
	k := s.Scalar().SetBytes(h.Sum(nil))
	minusPublic := s.Point().Neg(aggReducedPublic)
	ka := s.Point().Mul(k, minusPublic)
	r := s.Scalar().SetBytes(bs.Sig[pointLen:sigLen])
	rb := s.Point().Mul(r, nil)
	left := s.Point().Add(rb, ka)

	right := s.Point().Sub(origCommit, aggExCommit)

	if !left.Equal(right) {
		return errors.New("Commit recreated is not equal to one given")
	}
	return nil
}

// Announce is the struct used during the announcement phase (of both
// rounds)
type Announce struct {
	TYPE    RoundType
	Timeout time.Duration
}

// announceChan is the type of the channel that will be used to catch
// announcement messages.
type announceChan struct {
	*onet.TreeNode
	Announce
}

// Commitment is the commitment packets that is sent for both rounds
type Commitment struct {
	TYPE       RoundType
	Commitment kyber.Point
}

// commitChan is the type of the channel that will be used to catch commitment
// messages.
type commitChan struct {
	*onet.TreeNode
	Commitment
}

// ChallengePrepare is the challenge used by ByzCoin during the "prepare" phase.
// It contains the basic challenge plus the message from which the challenge has
// been generated.
type ChallengePrepare struct {
	Msg       []byte
	Data      []byte
	Challenge kyber.Scalar
}

// ChallengeCommit  is the challenge used by BftCoSi during the "commit"
// phase. It contains the basic challenge (out of the block we want to sign) +
// the signature of the "prepare" round. It also contains the exception list
// coming from the "prepare" phase. This exception list has been collected by
// the root during the response of the "prepare" phase and broadcast it through
// the challenge of the "commit". These are needed in order to verify the
// signature and to see how many peers did not sign. It's not spoofable because
// otherwise the signature verification will be wrong.
type ChallengeCommit struct {
	// Challenge for the current round
	Challenge kyber.Scalar
	// Signature is the signature response generated at the previous round (prepare)
	Signature *BFTSignature
}

// challengeChan is the type of the channel that will be used to catch the
// challenge messages.
type challengePrepareChan struct {
	*onet.TreeNode
	ChallengePrepare
}

// challengeCommitChan is the type of the channel used to catch the response messages.
type challengeCommitChan struct {
	*onet.TreeNode
	ChallengeCommit
}

// Response is the struct used by ByzCoin during the response. It
// contains the response + the basic exception list.
type Response struct {
	Response   kyber.Scalar
	Exceptions []Exception
	TYPE       RoundType
}

// responseChan is the type of the channel used to catch the response messages.
type responseChan struct {
	*onet.TreeNode
	Response
}

// Exception represents the exception mechanism used in BFTCosi to indicate a
// signer did not want to sign.
// The index is the index of the public key of the cosigner that do not want to
// sign.
// The commit is needed in order to be able to
// correctly verify the signature
type Exception struct {
	Index      int
	Commitment kyber.Point
}
