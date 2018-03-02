package authentication

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/auth/darc"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_GetPolicy(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, _, genService := local.MakeSRS(cothority.Suite, 1, authID)
	s := genService.(*Service)

	reply1, err := s.GetPolicy(&GetPolicy{
		Policy: "",
	})
	require.Nil(t, err)
	require.NotNil(t, reply1.Latest)
	require.True(t, (*reply1.Latest.Owners)[0].Ed25519.Point.Equal(s.ServerIdentity().Public))

	reply2, err := s.GetPolicy(&GetPolicy{
		Policy: "test",
	})
	require.Nil(t, err)
	require.True(t, (*reply2.Latest.Owners)[0].Ed25519.Point.Equal(s.ServerIdentity().Public))
	require.True(t, reply1.Latest.Equal(reply2.Latest))
}

func TestService_UpdatePolicy(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	servers, _, genService := local.MakeSRS(cothority.Suite, 1, authID)
	s := genService.(*Service)

	root := darc.NewSignerEd25519(local.GetPrivate(servers[0]))
	reply1, err := s.GetPolicy(&GetPolicy{
		Policy: "",
	})
	require.Nil(t, err)
	user := key.NewKeyPair(cothority.Suite)
	userSigner := darc.NewSignerEd25519(user.Private)
	darc2 := reply1.Latest.Copy()
	darc2.AddOwner(darc.NewIdentityEd25519(user.Public))
	darc2.AddUser(darc.NewIdentityEd25519(user.Public))

	// Sign by wrong key
	require.Nil(t, darc2.SetEvolution(reply1.Latest, userSigner))
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:  "",
		NewDarc: darc2,
	})
	require.NotNil(t, err)

	// Use correct key but wrong policy
	require.Nil(t, darc2.SetEvolution(reply1.Latest, root))
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:  "test",
		NewDarc: darc2,
	})
	require.NotNil(t, err)

	// Use correct key and correct policy
	require.Nil(t, darc2.SetEvolution(reply1.Latest, root))
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:  "",
		NewDarc: darc2,
	})
	require.Nil(t, err)

	// Store new darc for new policy
	user2 := key.NewKeyPair(cothority.Suite)
	user2Signer := darc.NewSignerEd25519(user2.Private)
	user2ids := &[]*darc.Identity{user2Signer.Identity()}
	darc3 := &darc.Darc{
		Owners:  user2ids,
		Users:   user2ids,
		Version: 0,
	}

	// Cannot replace an existing darc with another one.
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:  "",
		NewDarc: darc3,
	})
	require.NotNil(t, err)

	// Cannot add an unsigned darc.
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:  "test",
		NewDarc: darc3,
	})
	require.NotNil(t, err)

	// Cannot add a wrongly signed darc.
	sigpath := &darc.SignaturePath{Signer: *user2Signer.Identity(), Role: darc.Owner}
	darcSig, err := darc.NewDarcSignature(darc3.GetID(), sigpath, userSigner)
	require.Nil(t, err)
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:    "test",
		NewDarc:   darc3,
		Signature: darcSig,
	})
	require.NotNil(t, err)

	// Add a new darc signed by a responsible darc
	sigpath = &darc.SignaturePath{Signer: *userSigner.Identity(), Role: darc.Owner}
	darcSig, err = darc.NewDarcSignature(darc3.GetID(), sigpath, userSigner)
	require.Nil(t, err)
	_, err = s.UpdatePolicy(&UpdatePolicy{
		Policy:    "test",
		NewDarc:   darc3,
		Signature: darcSig,
	})
	require.Nil(t, err)

	// Verify our verification function
	msg := []byte("message")
	sigpath = &darc.SignaturePath{Signer: *user2Signer.Identity(), Role: darc.User}

	// Wrong service for user2
	darcSig, err = darc.NewDarcSignature(msg, sigpath, user2Signer)
	require.Nil(t, err)
	auth := Auth{Signature: *darcSig}
	require.NotNil(t, Verify(*s.Context, "fail", "it", auth, msg))

	// Correct service
	darcSig, err = darc.NewDarcSignature(msg, sigpath, user2Signer)
	require.Nil(t, err)
	auth = Auth{Signature: *darcSig}
	require.Nil(t, Verify(*s.Context, "test", "it", auth, msg))

	// For user this should work with any service
	sigpath = &darc.SignaturePath{Signer: *userSigner.Identity(), Role: darc.User}
	darcSig, err = darc.NewDarcSignature(msg, sigpath, userSigner)
	require.Nil(t, err)
	auth = Auth{Signature: *darcSig}
	require.Nil(t, Verify(*s.Context, "any", "it", auth, msg))
}

func TestService_UpdatePolicyPIN(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, _, genService := local.MakeSRS(cothority.Suite, 1, authID)
	s := genService.(*Service)

	// Create new darc
	user := key.NewKeyPair(cothority.Suite)
	userSigner := darc.NewSignerEd25519(user.Private)
	userids := &[]*darc.Identity{userSigner.Identity()}
	darc := &darc.Darc{
		Owners:  userids,
		Users:   userids,
		Version: 0,
	}

	// Create PIN
	_, err := s.UpdatePolicyPIN(&UpdatePolicyPIN{
		Policy:  "",
		NewDarc: darc,
		PIN:     "",
	})
	require.NotNil(t, err)

	// Overwrite existing darc
	_, err = s.UpdatePolicyPIN(&UpdatePolicyPIN{
		Policy:  "",
		NewDarc: darc,
		PIN:     s.pin,
	})
	require.Nil(t, err)
}
