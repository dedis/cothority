package authprox

import (
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/dss"
)

// EnrollRequest is the request sent to this service to enroll
// a user, authenticated by a certain type of external authentication.
type EnrollRequest struct {
	Type   string
	Issuer string

	Secret       kyber.Scalar
	Participants []kyber.Point
	LongPri      share.PriShare
	LongPubs     []kyber.Point
}

// EnrollResponse is returned when an enrollment has been done correctly.
type EnrollResponse struct {
}

// SignatureRequest is the request sent to this service to request that
// the Authentication Proxy check the authentication information and
// generate a signature connecting some information identifying the
// holder of the AuthInfo to the message.
type SignatureRequest struct {
	Type     string
	Issuer   string
	AuthInfo []byte
	RandPri  share.PriShare
	RandPubs []kyber.Point
	Message  []byte
}

// SignatureResponse is the response to a SignMessage request.
type SignatureResponse struct {
	PartialSignature dss.PartialSig
}

// EnrollmentsRequest gets a list of enrollments, optionally limited
// by the Types and Issuers list. If an enrollment matches any of
// the strings in Types or Issuers, it will be returned. If Types
// or Issuers are empty, then all enrollments are considered to match.
type EnrollmentsRequest struct {
	Types   []string
	Issuers []string
}

// EnrollmentsResponse is the returned list of enrollments.
type EnrollmentsResponse struct {
	Enrollments []EnrollmentInfo
}

// EnrollmentInfo is public info about an enrollment.
type EnrollmentInfo struct {
	Type, Issuer string
	Public       kyber.Point
}
