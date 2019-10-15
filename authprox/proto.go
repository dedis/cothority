package authprox

import (
	"go.dedis.ch/kyber/v4"
)

// PROTOSTART
// package authprox;
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "AuthProxProto";

// EnrollRequest is the request sent to this service to enroll
// a user, authenticated by a certain type of external authentication.
type EnrollRequest struct {
	Type         string
	Issuer       string
	Participants []kyber.Point
	LongPri      PriShare
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
	RandPri  PriShare
	RandPubs []kyber.Point
	Message  []byte
}

// PriShare is a local copy of go.dedis.ch/kyber/v4/share.PriShare
// because we do not have proto files for Kyber objects.
type PriShare struct {
	I int          // Index of the private share
	V kyber.Scalar // Value of the private share
}

// PartialSig is a local copy of go.dedis.ch/kyber/v4/sign/dss.PartialSig
// because we do not have proto files for Kyber objects.
type PartialSig struct {
	Partial   PriShare
	SessionID []byte
	Signature []byte
}

// SignatureResponse is the response to a SignMessage request.
type SignatureResponse struct {
	PartialSignature PartialSig
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
	Type   string
	Issuer string
	Public kyber.Point
}
