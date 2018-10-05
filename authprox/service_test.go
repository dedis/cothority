package authprox

import (
	"bytes"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// a test validator that allows all auth info
type valid struct{}

func (e *valid) FindClaim(issuer string, ai []byte) (string, string, error) {
	return "dummy-claim", "dummy-extra-data", nil
}

// some zeros to send as a message
var zero64 [64]byte

func Test_EnrollAndSign(t *testing.T) {
	suite := suites.MustFind("ed25519")
	e := newEnv(t)
	defer e.local.CloseAll()

	nPartic := len(e.services)
	pris := make([]kyber.Scalar, nPartic)
	pubs := make([]kyber.Point, nPartic)
	for i := 0; i < nPartic; i++ {
		kp := key.NewKeyPair(suite)
		pris[i] = kp.Private
		pubs[i] = kp.Public
	}

	lPri := share.NewPriPoly(suite, nPartic, nil, cothority.Suite.RandomStream())
	lShares := lPri.Shares(nPartic)
	lPub := lPri.Commit(nil)
	_, lPubCommits := lPub.Info()

	rPri := share.NewPriPoly(suite, nPartic, nil, cothority.Suite.RandomStream())
	rShares := rPri.Shares(nPartic)
	rPub := rPri.Commit(nil)
	_, rPubCommits := rPub.Info()

	testType := "dummy"

	// Enroll on each proxy.
	for i, s := range e.services {
		s.registerValidator(testType, &valid{})

		lpri := PriShare{
			I: lShares[i].I,
			V: lShares[i].V,
		}
		req := &EnrollRequest{
			Type:         testType,
			Secret:       pris[i],
			Participants: pubs,
			LongPri:      lpri,
			LongPubs:     lPubCommits,
		}
		_, err := s.Enroll(req)
		require.NoError(t, err)
	}

	var partials []*share.PriShare
	for i, s := range e.services {
		rp := PriShare{
			I: rShares[i].I,
			V: rShares[i].V,
		}
		req := &SignatureRequest{
			Type:     testType,
			Message:  zero64[:],
			RandPri:  rp,
			RandPubs: rPubCommits,
		}
		resp, err := s.Signature(req)
		require.NoError(t, err)
		ps := &share.PriShare{
			I: resp.PartialSignature.Partial.I,
			V: resp.PartialSignature.Partial.V,
		}
		partials = append(partials, ps)
	}

	// Reassemble the partial signatures and validate them: this will normally
	// happen in the client, but do it here for now to see if this stuff all holds
	// together.

	// TODO: We are carelessly accepting all the shares we get back, we
	// should be doing the same validation done in dss.ProcessPartialSig.
	// The consequences of an advesary slipping in a bad signature seem to be limited to
	// fraudulently preventing us from sending a txn in, so not so serious. To
	// be able to check these, the Signer needs to have the roster or all
	// expected public keys for all Auth Proxies.

	gamma, err := share.RecoverSecret(suite, partials, nPartic, nPartic)
	require.NoError(t, err)

	// RandomPublic || gamma
	var buff bytes.Buffer
	_, _ = rPub.Commit().MarshalTo(&buff)
	_, _ = gamma.MarshalTo(&buff)
	sig := buff.Bytes()

	// Make a Darc identity for this public key and the claim we know
	// that the verifier returned.
	id := darc.IdentityProxy{
		Public: lPub.Commit(),
		Data:   "dummy-claim",
	}

	// And verify it!
	err = id.Verify(zero64[:], sig)
	require.NoError(t, err)

	// Check that Enrollments does what we expect
	resp, err := e.services[0].Enrollments(&EnrollmentsRequest{Types: []string{"other"}})
	require.NoError(t, err)
	require.Equal(t, 0, len(resp.Enrollments))
	resp, err = e.services[0].Enrollments(&EnrollmentsRequest{Types: []string{"dummy"}})
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
