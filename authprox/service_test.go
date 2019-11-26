package authprox

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/dss"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// a test validator that allows all auth info
type valid struct {
	err error
}

func (e *valid) FindClaim(issuer string, ai []byte) (string, string, error) {
	// For the hash, return the ai itself, unparsed. This allows max flexability
	// for testing different cases.
	return "dummy-claim", string(ai), e.err
}

// some zeros to send as a message
var zero64 [64]byte

func Test_EnrollAndSign(t *testing.T) {
	suite := suites.MustFind("ed25519")
	e := newEnv(t)
	defer e.local.CloseAll()

	// Test with 3 out of 5 servers.
	T := 3
	nPartic := len(e.services)
	require.Equal(t, nPartic, 5)

	pubs := make([]kyber.Point, nPartic)
	for i := 0; i < nPartic; i++ {
		pubs[i] = e.roster.List[i].Public
	}

	lPri := share.NewPriPoly(suite, T, nil, cothority.Suite.RandomStream())
	lShares := lPri.Shares(nPartic)
	lPub := lPri.Commit(nil)
	_, lPubCommits := lPub.Info()

	rPri := share.NewPriPoly(suite, T, nil, cothority.Suite.RandomStream())
	rShares := rPri.Shares(nPartic)
	rPub := rPri.Commit(nil)
	_, rPubCommits := rPub.Info()

	testType := "dummy"
	validator := &valid{}

	// Enroll on each proxy.
	for i, s := range e.services {
		s.registerValidator(testType, validator)

		lpri := PriShare{
			I: lShares[i].I,
			V: lShares[i].V,
		}
		req := &EnrollRequest{
			Type:         testType,
			Participants: pubs,
			LongPri:      lpri,
			LongPubs:     lPubCommits,
		}
		_, err := s.Enroll(req)
		require.NoError(t, err)
	}

	h := sha256.Sum256(zero64[:])
	hStr := hex.EncodeToString(h[:])
	hBad := "00" + hStr[2:]
	require.Equal(t, len(hStr), len(hBad))

	var partials []*share.PriShare
	for i, s := range e.services {
		rp := PriShare{
			I: rShares[i].I,
			V: rShares[i].V,
		}

		// One time with incorrect hash in the auth info.
		req := &SignatureRequest{
			Type:     testType,
			Message:  zero64[:],
			AuthInfo: []byte(hBad),
			RandPri:  rp,
			RandPubs: rPubCommits,
		}
		resp, err := s.Signature(req)
		require.Error(t, err)

		// And now correctly.
		req = &SignatureRequest{
			Type:     testType,
			Message:  zero64[:],
			AuthInfo: []byte(hStr),
			RandPri:  rp,
			RandPubs: rPubCommits,
		}
		resp, err = s.Signature(req)
		require.NoError(t, err)

		ps := &dss.PartialSig{
			Partial: &share.PriShare{
				I: resp.PartialSignature.Partial.I,
				V: resp.PartialSignature.Partial.V,
			},
			SessionID: resp.PartialSignature.SessionID,
			Signature: resp.PartialSignature.Signature,
		}

		err = schnorr.Verify(cothority.Suite, s.ServerIdentity().Public, ps.Hash(cothority.Suite), ps.Signature)
		require.NoError(t, err)

		partials = append(partials, ps.Partial)
	}

	// Reassemble the partial signatures and validate them: this will normally
	// happen in the client, but do it here for now to see if this stuff all holds
	// together.
	gamma, err := share.RecoverSecret(suite, partials, T, nPartic)
	require.NoError(t, err)

	// RandomPublic || gamma
	var buff bytes.Buffer
	_, _ = rPub.Commit().MarshalTo(&buff)
	_, _ = gamma.MarshalTo(&buff)
	sig := buff.Bytes()

	// Make a Darc identity for this public key and some other claim.
	id := darc.IdentityProxy{
		Public: lPub.Commit(),
		Data:   "dummy-claim-other",
	}
	err = id.Verify(zero64[:], sig)
	require.Error(t, err)

	// Make a Darc identity for this public key and the claim we know
	// that the verifier returned.
	id = darc.IdentityProxy{
		Public: lPub.Commit(),
		Data:   "dummy-claim",
	}
	err = id.Verify(zero64[:], sig)
	require.NoError(t, err)

	// Test behavior of failing FindClaim: error propogates correctly to caller, and
	// then it is up to the client to see if they can still make final sig.
	partials = nil
	for i, s := range e.services {
		// Cause the first two servers to give an error from FindClaim.
		if i < 2 {
			validator.err = errors.New("FindClaim returns error for testing")
		} else {
			validator.err = nil
		}
		rp := PriShare{
			I: rShares[i].I,
			V: rShares[i].V,
		}
		req := &SignatureRequest{
			Type:     testType,
			Message:  zero64[:],
			AuthInfo: []byte(hStr),
			RandPri:  rp,
			RandPubs: rPubCommits,
		}
		resp, err := s.Signature(req)
		if i < 2 {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			ps := &share.PriShare{
				I: resp.PartialSignature.Partial.I,
				V: resp.PartialSignature.Partial.V,
			}
			partials = append(partials, ps)
		}
	}
	// Expect all the servers expect 2 to contirbute partials.
	require.Equal(t, len(e.services)-2, len(partials))

	gamma, err = share.RecoverSecret(suite, partials, T, nPartic)
	require.NoError(t, err)
	buff.Reset()
	_, _ = rPub.Commit().MarshalTo(&buff)
	_, _ = gamma.MarshalTo(&buff)
	sig = buff.Bytes()
	err = id.Verify(zero64[:], sig)
	require.NoError(t, err)

	// Check that Enrollments does what we expect
	resp, err := e.services[0].Enrollments(&EnrollmentsRequest{Types: []string{"other"}})
	require.NoError(t, err)
	require.Equal(t, 0, len(resp.Enrollments))
	resp, err = e.services[0].Enrollments(&EnrollmentsRequest{Types: []string{"dummy", "other", "dummy"}})
	require.NoError(t, err)
	require.Equal(t, 1, len(resp.Enrollments))
}

func TestService_SignatureErrors(t *testing.T) {
	e := newEnv(t)
	defer e.local.CloseAll()

	in := &SignatureRequest{
		Type: "who?",
	}
	_, err := e.services[0].Signature(in)
	require.Error(t, err)
}

type env struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*service
}

func newEnv(t *testing.T) (s *env) {
	s = &env{}
	s.local = onet.NewLocalTestT(cothority.Suite, t)
	s.hosts, s.roster, _ = s.local.GenTree(5, true)
	for _, sv := range s.local.GetServices(s.hosts, authProxID) {
		s.services = append(s.services, sv.(*service))
	}

	return
}
