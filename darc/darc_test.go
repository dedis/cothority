package darc

import (
	"testing"

	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDarc(t *testing.T) {
	desc := []byte("mydarc")
	var users, owner []*Identity
	owner = append(owner, createIdentity())
	for i := 0; i < 2; i++ {
		users = append(users, createIdentity())
	}
	d := NewDarc(&owner, &users, desc)
	require.Equal(t, &desc, d.Description)
	require.Equal(t, *owner[0], *(*d.Owners)[0])
	for i, user := range users {
		require.Equal(t, *user, *(*d.Users)[i])
	}
}

// Checks that when a Darc1 is copied to Darc2,
// adding a user to Darc1 does not add it to Darc2,
// and changing description and version in Darc1
// does not change them in Darc2.
func TestDarc_Copy(t *testing.T) {
	d1 := createDarc("testdarc1").darc
	d2 := d1.Copy()
	(*d1.Owners)[0] = createIdentity()
	d1.Version = 3
	desc := []byte("testdarc2")
	d1.Description = &desc
	d1.AddUser(createIdentity())
	require.NotEqual(t, (*d1.Owners)[0], (*d2.Owners)[0])
	require.NotEqual(t, len(*d1.Users), len(*d2.Users))
	require.NotEqual(t, d1.Description, d2.Description)
	require.NotEqual(t, d1.Version, d2.Version)

	d1.Description = nil
	d2 = d1.Copy()
	require.Equal(t, d1.GetID(), d2.GetID())
}

func TestDarc_AddUser(t *testing.T) {
	d := createDarc("testdarc").darc
	id := createIdentity()
	d.AddUser(id)
	require.Equal(t, id, (*d.Users)[len(*d.Users)-1])
}

func TestDarc_RemoveUser(t *testing.T) {
	d1 := createDarc("testdarc1").darc
	d2 := d1.Copy()
	id := createIdentity()
	d2.AddUser(id)
	require.NotEqual(t, len(*d1.Users), len(*d2.Users))
	d2.RemoveUser(id)
	require.Equal(t, len(*d1.Users), len(*d2.Users))
}

func TestDarc_IncrementVersion(t *testing.T) {
	d := createDarc("testdarc").darc
	previousVersion := d.Version
	d.IncrementVersion()
	require.NotEqual(t, previousVersion, d.Version)
}

func TestDarc_SetEvolution(t *testing.T) {
	d := createDarc("testdarc").darc
	log.ErrFatal(d.Verify())
	owner := NewEd25519Signer(nil, nil)
	owner2 := NewEd25519Signer(nil, nil)
	ownerI := Identity{
		Ed25519: NewEd25519Identity(owner.Point),
	}
	ownerI2 := Identity{
		Ed25519: NewEd25519Identity(owner2.Point),
	}
	d.AddOwner(&ownerI)
	dNew := d.Copy()
	dNew.IncrementVersion()
	assert.NotNil(t, dNew.Verify())

	darcs := []*Darc{d}
	signer := &Signer{
		Ed25519: owner,
	}
	signer2 := &Signer{
		Ed25519: owner2,
	}

	require.Nil(t, dNew.SetEvolution(d, NewSignaturePath(darcs, ownerI2, User), signer2))
	assert.NotNil(t, dNew.Verify())
	require.Nil(t, dNew.SetEvolution(d, NewSignaturePath(darcs, ownerI, User), signer2))
	assert.NotNil(t, dNew.Verify())
	require.Nil(t, dNew.SetEvolution(d, NewSignaturePath(darcs, ownerI, User), signer))
	require.Nil(t, dNew.Verify())
}

func TestSignatureChange(t *testing.T) {
	td1 := createDarc("testdarc")
	td2 := createDarc("testdarc")
	td2.darc.SetEvolution(td1.darc, nil, td1.owners[0])
	require.Nil(t, td2.darc.Verify())
	td2.darc.AddUser(td2.usersI[1])
	require.NotNil(t, td2.darc.Verify())

	td2.darc.SetEvolution(td1.darc, nil, td1.owners[0])
	require.Nil(t, td2.darc.Verify())

	td2.darc.AddOwner(td2.ownersI[1])
	require.NotNil(t, td2.darc.Verify())
}

func TestSignaturePath(t *testing.T) {
	td1 := createDarc("testdarc")
	td2 := createDarc("testdarc2")
	td3 := createDarc("testdarc3")
	td4 := createDarc("testdarc4")
	path := NewSignaturePath([]*Darc{td1.darc, td2.darc, td3.darc, td4.darc}, *td4.usersI[0], User)
	require.NotNil(t, path.Verify(User))
	td2.darc.SetEvolution(td1.darc, nil, td1.owners[0])
	td4.darc.SetEvolution(td3.darc, nil, td3.owners[0])
	require.NotNil(t, path.Verify(User))

	td2.darc.AddUser(&Identity{Darc: &IdentityDarc{td3.darc.GetID()}})
	require.NotNil(t, path.Verify(User))
	td2.darc.SetEvolution(td1.darc, nil, td1.owners[0])
	require.Nil(t, path.Verify(User))
	td4.darc.SetEvolution(td3.darc, nil, td3.owners[0])
	require.Nil(t, path.Verify(User))
}

func TestDarcSignature_Verify(t *testing.T) {
	msg := []byte("document")
	d := createDarc("testdarc").darc
	owner := &Signer{
		Ed25519: NewEd25519Signer(nil, nil),
	}
	ownerI := Identity{
		Ed25519: NewEd25519Identity(owner.Ed25519.Point),
	}
	path := NewSignaturePath([]*Darc{d}, ownerI, User)
	ds, err := NewDarcSignature(msg, path, owner)
	log.ErrFatal(err)
	d2 := d.Copy()
	d2.IncrementVersion()
	require.NotNil(t, ds.Verify(msg, d2))
	require.Nil(t, ds.Verify(msg, d))
}

type testDarc struct {
	darc    *Darc
	owners  []*Signer
	ownersI []*Identity
	users   []*Signer
	usersI  []*Identity
}

func createDarc(desc string) *testDarc {
	td := &testDarc{}
	for i := 0; i < 2; i++ {
		s, id := createSignerIdentity()
		td.owners = append(td.owners, s)
		td.ownersI = append(td.ownersI, id)
		s, id = createSignerIdentity()
		td.users = append(td.users, s)
		td.usersI = append(td.usersI, id)
	}
	td.darc = NewDarc(&td.ownersI, &td.usersI, []byte(desc))
	return td
}

func createSigner() *Signer {
	s, _ := createSignerIdentity()
	return s
}

func createIdentity() *Identity {
	_, id := createSignerIdentity()
	return id
}

func createSignerIdentity() (*Signer, *Identity) {
	edSigner := NewEd25519Signer(nil, nil)
	signer := &Signer{Ed25519: edSigner}
	edID := NewEd25519Identity(edSigner.Point)
	id, err := NewIdentity(nil, edID)
	log.ErrFatal(err)
	return signer, id
}
