package darc

import (
	"testing"

	"github.com/dedis/student_18_omniledger/omniledger/darc/expression"
	"github.com/stretchr/testify/require"
)

func TestRules(t *testing.T) {
	// one owner
	owner := createIdentity()
	rules := InitRules([]*Identity{owner}, []*Identity{})
	expr, ok := rules[evolve]
	require.True(t, ok)
	require.Equal(t, string(expr), owner.String())

	// two owners
	owners := []*Identity{owner, createIdentity()}
	rules = InitRules(owners, []*Identity{})
	expr, ok = rules[evolve]
	require.True(t, ok)
	require.Equal(t, string(expr), owners[0].String()+" | "+owners[1].String())
}

func TestNewDarc(t *testing.T) {
	desc := []byte("mydarc")
	owner := createIdentity()

	d := NewDarc(InitRules([]*Identity{owner}, []*Identity{}), desc)
	require.Equal(t, desc, d.Description)
	require.Equal(t, string(d.Rules.GetEvolutionExpr()), owner.String())
}

func TestDarc_Copy(t *testing.T) {
	// create two darcs
	d1 := createDarc(1, "testdarc1").darc
	err := d1.Rules.AddRule("ocs:write", d1.Rules.GetEvolutionExpr())
	require.Nil(t, err)
	d2 := d1.Copy()

	// modify the first one
	d1.IncrementVersion()
	desc := []byte("testdarc2")
	d1.Description = desc
	err = d1.Rules.UpdateRule("ocs:write", []byte(createIdentity().String()))
	require.Nil(t, err)

	// the two darcs should be different
	require.NotEqual(t, d1.Version, d2.Version)
	require.NotEqual(t, d1.Description, d2.Description)
	require.NotEqual(t, d1.Rules["ocs:write"], d2.Rules["ocs:write"])

	// ID should not change if values are the same
	d2.Description = nil
	d1 = d2.Copy()
	require.Equal(t, d1.GetID(), d2.GetID())
}

func TestAddRule(t *testing.T) {
	// TODO
}

func TestUpdateRule(t *testing.T) {
	// TODO
}

func TestDeleteRule(t *testing.T) {
	// TODO
}

func TestDarc_IncrementVersion(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	previousVersion := d.Version
	d.IncrementVersion()
	require.NotEqual(t, previousVersion, d.Version)
}

// TestDarc_EvolveOne creates two darcs, the first has two owners and the
// second has one. The first darc is to be evolved into the second one.
func TestDarc_EvolveOne(t *testing.T) {
	d := createDarc(2, "testdarc").darc
	require.Nil(t, d.Verify())
	owner1 := NewSignerEd25519(nil, nil)
	owner2 := NewSignerEd25519(nil, nil)
	id1 := *owner1.Identity()
	id2 := *owner2.Identity()
	require.Nil(t, d.Rules.UpdateEvolution(expression.InitOrExpr(id1.String(), id2.String())))

	dNew := d.Copy()
	dNew.IncrementVersion()
	require.Nil(t, dNew.Rules.UpdateEvolution([]byte(id1.String())))
	// verification should fail because the signature path is not present
	require.NotNil(t, dNew.Verify())

	darcs := []*Darc{d}
	// the identity of the signer cannot be id3, it does not have the
	// evolve permission
	owner3 := NewSignerEd25519(nil, nil)
	require.Nil(t, localEvolution(dNew, darcs, owner3))
	require.NotNil(t, dNew.Verify())
	// it should be possible to sign with owner2 and owner1 because they
	// are in the first darc and have the evolve permission
	require.Nil(t, localEvolution(dNew, darcs, owner2))
	require.Nil(t, dNew.Verify())
	require.Nil(t, localEvolution(dNew, darcs, owner1))
	require.Nil(t, dNew.Verify())
	// use logical-and in the evolve expression
	// verification should if only one owner signs the darc
	require.Nil(t, dNew.Rules.UpdateEvolution(expression.InitAndExpr(owner2.Identity().String(), owner1.Identity().String())))
	require.Nil(t, localEvolution(dNew, darcs, owner2))
	require.Nil(t, dNew.Verify())
	darcs = append(darcs, dNew)
	dNew2 := dNew.Copy()
	require.Nil(t, localEvolution(dNew2, darcs, owner2))
	require.NotNil(t, dNew2.Verify())
	require.Nil(t, localEvolution(dNew2, darcs, owner1))
	require.NotNil(t, dNew2.Verify())
	require.Nil(t, localEvolution(dNew2, darcs, owner2, owner1))
	require.Nil(t, dNew2.Verify())
}

// TestDarc_EvolveMore is similar to TestDarc_EvolveOne but testing for
// multiple evolutions.
func TestDarc_EvolveMore(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify())
	prevOwner := NewSignerEd25519(nil, nil)
	require.Nil(t, d.Rules.UpdateEvolution(
		expression.InitOrExpr(prevOwner.Identity().String())))

	darcs := []*Darc{d}
	for i := 0; i < 10; i++ {
		dNew := darcs[len(darcs)-1].Copy()
		dNew.IncrementVersion()
		newOwner := NewSignerEd25519(nil, nil)
		require.Nil(t, dNew.Rules.UpdateEvolution([]byte(newOwner.Identity().String())))
		require.Nil(t, localEvolution(dNew, darcs, prevOwner))
		// require.Nil(t, dNew.Verify())
		darcs = append(darcs, dNew)
		prevOwner = newOwner
	}
	require.Nil(t, darcs[len(darcs)-1].Verify())

	// verification should fail if the Rules is tampered (ID is changed)
	darcs[len(darcs)-1].Rules.UpdateEvolution([]byte{})
	require.NotNil(t, darcs[len(darcs)-1].Verify())

	// but it should not affect an older darc
	require.Nil(t, darcs[len(darcs)-2].Verify())

	// test more failures
	darcs[1].Version = 0
	require.NotNil(t, darcs[len(darcs)-2].Verify())
	darcs[1].Version = 1
	require.Nil(t, darcs[len(darcs)-2].Verify())
	darcs[0].Rules.UpdateEvolution([]byte{})
	require.NotNil(t, darcs[len(darcs)-2].Verify())
}

func TestDarc_EvolveMoreOnline(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify())
	prevOwner := NewSignerEd25519(nil, nil)
	require.Nil(t, d.Rules.UpdateEvolution(
		expression.InitOrExpr(prevOwner.Identity().String())))

	darcs := []*Darc{d}
	for i := 0; i < 10; i++ {
		dNew := darcs[len(darcs)-1].Copy()
		dNew.IncrementVersion()
		newOwner := NewSignerEd25519(nil, nil)
		require.Nil(t, dNew.Rules.UpdateEvolution([]byte(newOwner.Identity().String())))
		require.Nil(t, localEvolution(dNew, darcs, prevOwner))
		require.Nil(t, dNew.Verify())
		darcs = append(darcs, dNew)
		prevOwner = newOwner
	}
	require.Nil(t, darcs[len(darcs)-1].Verify())

	// create some call backs
	getDarc := func(id string) *Darc {
		for _, d := range darcs {
			if d.GetIdentityString() == id {
				return darcs[len(darcs)-1]
			}
		}
		return nil
	}
	getDarcWrong0 := func(id string) *Darc {
		return darcs[0]
	}
	getDarcWrong1 := func(id string) *Darc {
		return darcs[1]
	}

	// create darcs that do not have the full path
	lightDarc1 := darcs[len(darcs)-1].Copy()
	lightDarc1.Signatures = []*Signature{&Signature{
		Signature: copyBytes(darcs[len(darcs)-1].Signatures[0].Signature),
		Signer:    darcs[len(darcs)-1].Signatures[0].Signer,
	}}
	lightDarc2 := darcs[len(darcs)-2].Copy()
	lightDarc2.Signatures = []*Signature{&Signature{
		Signature: copyBytes(darcs[len(darcs)-2].Signatures[0].Signature),
		Signer:    darcs[len(darcs)-2].Signatures[0].Signer,
	}}

	// verification should fail if the callback is not set
	require.NotNil(t, lightDarc1.Verify())
	require.NotNil(t, lightDarc2.Verify())

	// verification should fail if callback is wrong
	require.NotNil(t, lightDarc1.VerifyWithCB(getDarcWrong0))
	require.NotNil(t, lightDarc2.VerifyWithCB(getDarcWrong0))
	require.NotNil(t, lightDarc1.VerifyWithCB(getDarcWrong1))
	require.NotNil(t, lightDarc2.VerifyWithCB(getDarcWrong1))

	// verification should pass with the correct callback
	require.Nil(t, lightDarc1.VerifyWithCB(getDarc))
	require.Nil(t, lightDarc2.VerifyWithCB(getDarc))
}

// TestDarc_Rules is, other than the test, is an example of how one would use
// the Darc with a user-defined rule.
func TestDarc_Rules(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify())
	user1 := NewSignerEd25519(nil, nil)
	user2 := NewSignerEd25519(nil, nil)
	err := d.Rules.AddRule("use", expression.InitOrExpr(user1.Identity().String(), user2.Identity().String()))
	require.Nil(t, err)

	var r *Request

	// happy case with signer 1
	r, err = NewRequest(d.GetID(), "use", []byte("secrets are lies"), user1)
	require.Nil(t, err)
	require.Nil(t, d.CheckRequest(r))

	// happy case with signer 2
	r, err = NewRequest(d.GetID(), "use", []byte("sharing is caring"), user2)
	require.Nil(t, err)
	require.Nil(t, d.CheckRequest(r))

	// happy case with both signers
	r, err = NewRequest(d.GetID(), "use", []byte("privacy is theft"), user1, user2)
	require.Nil(t, err)
	require.Nil(t, d.CheckRequest(r))

	// wrong ID
	d2 := createDarc(1, "testdarc2").darc
	r, err = NewRequest(d2.GetID(), "use", []byte("all animals are equal"), user1)
	require.Nil(t, err)
	require.NotNil(t, d.CheckRequest(r))

	// wrong action
	r, err = NewRequest(d.GetID(), "go", []byte("four legs good"), user1)
	require.Nil(t, err)
	require.NotNil(t, d.CheckRequest(r))

	// wrong signer 1
	user3 := NewSignerEd25519(nil, nil)
	r, err = NewRequest(d.GetID(), "use", []byte("two legs bad"), user3)
	require.Nil(t, err)
	require.NotNil(t, d.CheckRequest(r))

	// happy case where at least one signer is valid
	r, err = NewRequest(d.GetID(), "use", []byte("four legs good"), user1, user3)
	require.Nil(t, err)
	require.Nil(t, d.CheckRequest(r))

	// tampered signature
	r, err = NewRequest(d.GetID(), "use", []byte("two legs better"), user1, user3)
	r.Signatures[0] = copyBytes(r.Signatures[1])
	require.Nil(t, err)
	require.NotNil(t, d.CheckRequest(r))
}

func TestDarc_EvolveRequest(t *testing.T) {
	td := createDarc(1, "testdarc")
	require.Nil(t, td.darc.Verify())

	dNew := td.darc.Copy()
	require.Nil(t, dNew.EvolveFrom([]*Darc{td.darc}))
	var err error
	var r *Request

	// cannot create request with nil darc
	var nilDarc *Darc
	r, err = nilDarc.MakeEvolveRequest()
	require.NotNil(t, err)
	require.Nil(t, r)

	// cannot create request with no signers
	r, err = dNew.MakeEvolveRequest()
	require.NotNil(t, err)
	require.Nil(t, r)

	// create a request with a wrong signer, the creation should succeed
	// but the verification shold fail
	badOwner := NewSignerEd25519(nil, nil)
	r, err = dNew.MakeEvolveRequest(badOwner)
	require.Nil(t, err)
	require.NotNil(t, r)
	require.NotNil(t, td.darc.CheckRequest(r))

	// create the request with the right signer and it should pass
	r, err = dNew.MakeEvolveRequest(td.owners[0])
	require.Nil(t, err)
	require.NotNil(t, r)
	require.Nil(t, td.darc.CheckRequest(r))
}

// TestDarc_Delegation in this test we test delegation. We start with two
// darcs, each has one evolution, i.e. d1 -> d2, d3 -> d4. Then, d2 adds d3 as
// one of the identities with the evolve permission. Then, d4 should have the
// permission to evolve d2 further.
func TestDarc_Delegation(t *testing.T) {
	td1 := createDarc(2, "testdarc1")
	td2 := createDarc(2, "testdarc2")
	td3 := createDarc(2, "testdarc3")
	td4 := createDarc(2, "testdarc4")

	require.Nil(t, td3.darc.Rules.UpdateSign(td3.darc.Rules.GetEvolutionExpr()))
	require.Nil(t, td4.darc.Rules.UpdateSign(td4.darc.Rules.GetEvolutionExpr()))

	require.Nil(t, localEvolution(td2.darc, []*Darc{td1.darc}, td1.owners[0]))
	require.Nil(t, td2.darc.Verify())

	require.Nil(t, localEvolution(td4.darc, []*Darc{td3.darc}, td3.owners[0]))
	require.Nil(t, td4.darc.Verify())

	id3 := NewIdentityDarc(td3.darc.GetID())
	d2Expr := []byte(id3.String())
	require.Nil(t, td2.darc.Rules.UpdateEvolution(d2Expr))
	require.NotNil(t, td2.darc.Verify())
	require.Nil(t, localEvolution(td2.darc, []*Darc{td1.darc}, td1.owners[0]))
	require.Nil(t, td2.darc.Verify())

	td5 := createDarc(2, "testdarc5")
	require.Nil(t, localEvolution(td5.darc, []*Darc{td1.darc, td2.darc}, td3.owners[0]))
	require.NotNil(t, td5.darc.Verify())
	getDarc := func(id string) *Darc {
		if id == td3.darc.GetIdentityString() {
			return td4.darc
		}
		if id == td4.darc.GetIdentityString() {
			return td4.darc
		}
		return nil
	}
	// Evolution is not allowed because the evolution is signed by
	// td3.owners which is out of date.
	require.NotNil(t, td5.darc.VerifyWithCB(getDarc))
	// If the evolution is signed by the latest darc, then it's ok.
	require.Nil(t, localEvolution(td5.darc, []*Darc{td1.darc, td2.darc}, td4.owners[0]))
	require.Nil(t, td5.darc.VerifyWithCB(getDarc))
}

// TestDarc_DelegationChain creates a chain of delegation and we will try to
// evolve the first darc using the signature of the last darc in the chain.
func TestDarc_DelegationChain(t *testing.T) {
	n := 10
	darcs := make([]*Darc, n)
	owners := make([]*Signer, n)
	// create n darcs with an empty evolve action, we'll set it later
	for i := 0; i < n; i++ {
		td := createDarc(1, "test")
		td.darc.Rules.UpdateEvolution([]byte{})
		darcs[i] = td.darc
		owners[i] = td.owners[0]
	}
	// the darc with the sign rules should have a ed25519 key
	darcs[n-1].Rules.UpdateSign([]byte(owners[n-1].Identity().String()))
	// create the chain of delegation, we have to do it backwards because
	// changing the evolution rule will change the darcID
	for i := n - 2; i >= 0; i-- {
		require.Nil(t, darcs[i].Rules.UpdateSign(
			[]byte([]byte(darcs[i+1].GetIdentityString()))))
	}
	// delegate evolution permission to the first darc
	darcs[0].Rules.UpdateEvolution([]byte(darcs[1].GetIdentityString()))
	darcs[0].Rules.UpdateSign([]byte{})

	getDarc := func(s string) *Darc {
		for _, d := range darcs {
			if d.GetIdentityString() == s {
				return d
			}
		}
		return nil
	}
	td := createDarc(1, "new")
	// cannot do evolution because owner is not in the evolve rule
	require.Nil(t, localEvolution(td.darc, []*Darc{darcs[0]}, owners[0]))
	require.NotNil(t, td.darc.VerifyWithCB(getDarc))
	// fail because only the key in the latest darc can sign
	for _, o := range owners[:n-1] {
		require.Nil(t, localEvolution(td.darc, []*Darc{darcs[0]}, o))
		require.NotNil(t, td.darc.VerifyWithCB(getDarc))
	}
	// only the last one, containing the actual key that is in the signer
	// expression, should evaluate to true.
	require.Nil(t, localEvolution(td.darc, []*Darc{darcs[0]}, owners[n-1]))
	require.Nil(t, td.darc.VerifyWithCB(getDarc))
}

func TestDarc_X509(t *testing.T) {
	// TODO
}

type testDarc struct {
	darc   *Darc
	owners []*Signer
	ids    []*Identity
}

func createDarc(nbrOwners int, desc string) *testDarc {
	td := &testDarc{}
	for i := 0; i < nbrOwners; i++ {
		s, id := createSignerIdentity()
		td.owners = append(td.owners, s)
		td.ids = append(td.ids, id)
	}
	rules := InitRules(td.ids, []*Identity{})
	td.darc = NewDarc(rules, []byte(desc))
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
	signer := NewSignerEd25519(nil, nil)
	return signer, signer.Identity()
}

func localEvolution(newDarc *Darc, path []*Darc, signers ...*Signer) error {
	if err := newDarc.EvolveFrom(path); err != nil {
		return err
	}
	r, err := newDarc.MakeEvolveRequest(signers...)
	if err != nil {
		return err
	}
	sigs := make([]*Signature, len(signers))
	for i := range r.Identities {
		sigs[i] = &Signature{
			Signature: r.Signatures[i],
			Signer:    *r.Identities[i],
		}
	}
	newDarc.Signatures = sigs
	return nil
}
