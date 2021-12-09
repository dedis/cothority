package darc

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc/expression"
)

func TestRules(t *testing.T) {
	// one owner
	owner := createIdentity()
	rules := InitRules([]Identity{owner}, []Identity{})
	expr := rules.GetEvolutionExpr()
	require.NotNil(t, expr)
	require.Equal(t, string(expr), owner.String())

	// two owners
	owners := []Identity{owner, createIdentity()}
	rules = InitRules(owners, []Identity{})
	expr = rules.GetEvolutionExpr()
	require.NotNil(t, expr)
	require.Equal(t, string(expr), owners[0].String()+" & "+owners[1].String())
}

func TestNewDarc(t *testing.T) {
	desc := []byte("mydarc")
	owner := createIdentity()

	d := NewDarc(InitRules([]Identity{owner}, []Identity{}), desc)
	require.Equal(t, desc, d.Description)
	require.Equal(t, string(d.Rules.GetEvolutionExpr()), owner.String())
}

func TestDarc_Copy(t *testing.T) {
	// create two darcs
	d1 := createDarc(1, "testdarc1").darc
	err := d1.Rules.AddRule("ocs:write", d1.Rules.GetEvolutionExpr())
	require.NoError(t, err)
	d2 := d1.Copy()

	// modify the first one
	d1.Version++
	desc := []byte("testdarc2")
	d1.Description = desc
	err = d1.Rules.UpdateRule("ocs:write", []byte(createIdentity().String()))
	require.NoError(t, err)

	// the two darcs should be different
	require.NotEqual(t, d1.Version, d2.Version)
	require.NotEqual(t, d1.Description, d2.Description)
	require.NotEqual(t, d1.Rules.Get("ocs:write"), d2.Rules.Get("ocs:write"))

	// ID should not change if values are the same
	d2.Description = nil
	d1 = d2.Copy()
	require.Equal(t, d1.GetID(), d2.GetID())
}

// TestDarc_EvolveOne creates two darcs, the first has two owners and the
// second has one. The first darc is to be evolved into the second one.
func TestDarc_EvolveOne(t *testing.T) {
	d0 := createDarc(2, "testdarc").darc
	require.Nil(t, d0.Verify(true), true)
	owner1 := NewSignerEd25519(nil, nil)
	owner2 := NewSignerEd25519(nil, nil)
	id1 := owner1.Identity()
	id2 := owner2.Identity()
	require.Nil(t, d0.Rules.UpdateEvolution(expression.InitOrExpr(id1.String(), id2.String())))

	d1 := d0.Copy()
	require.Nil(t, d1.EvolveFrom(d0))
	require.Nil(t, d1.Rules.UpdateEvolution([]byte(id1.String())))
	// verification should fail because the signature path is not present
	require.NotNil(t, d1.Verify(true))

	// the identity of the signer cannot be id3, it does not have the
	// evolve permission
	owner3 := NewSignerEd25519(nil, nil)
	require.Nil(t, localEvolution(d1, d0, owner3))
	require.NotNil(t, d1.Verify(true))
	// it should be possible to sign with owner2 and owner1 because they
	// are in the first darc and have the evolve permission
	require.Nil(t, localEvolution(d1, d0, owner2))
	require.Nil(t, d1.Verify(true))
	require.Nil(t, localEvolution(d1, d0, owner1))
	require.Nil(t, d1.Verify(true))
	// use logical-and in the evolve expression
	// verification should fail if only one owner signs the darc
	require.Nil(t, d1.Rules.UpdateEvolution(expression.InitAndExpr(owner2.Identity().String(), owner1.Identity().String())))
	require.Nil(t, localEvolution(d1, d0, owner2))
	require.Nil(t, d1.Verify(true))
	d2 := d1.Copy()
	require.Nil(t, localEvolution(d2, d1, owner2))
	require.NotNil(t, d2.Verify(true))
	require.Nil(t, localEvolution(d2, d1, owner1))
	require.NotNil(t, d2.Verify(true))
	require.Nil(t, localEvolution(d2, d0, owner2, owner1))
	require.Nil(t, d2.Verify(true))
	require.Nil(t, localEvolution(d2, d1, owner2, owner1))
	require.Nil(t, d2.Verify(true))
}

// TestDarc_EvolveMore is similar to TestDarc_EvolveOne but testing for
// multiple evolutions.
func TestDarc_EvolveMore(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify(true))
	prevOwner := NewSignerEd25519(nil, nil)
	require.Nil(t, d.Rules.UpdateEvolution(
		expression.InitOrExpr(prevOwner.Identity().String())))

	prev := d
	var darcs []*Darc
	for i := 0; i < 10; i++ {
		dNew := prev.Copy()
		dNew.EvolveFrom(prev)
		newOwner := NewSignerEd25519(nil, nil)
		require.Nil(t, dNew.Rules.UpdateEvolution([]byte(newOwner.Identity().String())))
		require.Nil(t, localEvolution(dNew, prev, prevOwner))
		require.Nil(t, dNew.Verify(true))
		darcs = append(darcs, dNew)
		prev = dNew
		prevOwner = newOwner
	}

	// verification should fail if the Rules is tampered (ID is changed)
	darcs[len(darcs)-1].Rules.UpdateEvolution([]byte{})
	require.NotNil(t, darcs[len(darcs)-1].Verify(true))

	// but it should not affect an older darc
	require.Nil(t, darcs[len(darcs)-2].Verify(true))
}

func TestDarc_EvolveMoreOnline(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify(true))
	prevOwner := NewSignerEd25519(nil, nil)
	require.Nil(t, d.Rules.UpdateEvolution(
		expression.InitOrExpr(prevOwner.Identity().String())))

	darcs := []*Darc{d}
	for i := 0; i < 10; i++ {
		dNew := darcs[len(darcs)-1].Copy()
		dNew.EvolveFrom(darcs[len(darcs)-1])
		newOwner := NewSignerEd25519(nil, nil)
		require.Nil(t, dNew.Rules.UpdateEvolution([]byte(newOwner.Identity().String())))
		require.Nil(t, localEvolution(dNew, darcs[len(darcs)-1], prevOwner))
		require.Nil(t, dNew.Verify(true))
		darcs = append(darcs, dNew)
		prevOwner = newOwner
	}

	// We create some call backs, suppose the correct one doesn't know the
	// very latest darc.
	getDarc := func(id string, latest bool) *Darc {
		if latest {
			return nil
		}
		for _, d := range darcs {
			if fmt.Sprintf("darc:%x", d.GetID()) == id {
				return d
			}
		}
		return nil
	}
	getDarcWrong0 := func(id string, latest bool) *Darc {
		return darcs[0]
	}
	getDarcWrong1 := func(id string, latest bool) *Darc {
		return darcs[1]
	}

	// create darcs that do not have the full path, i.e. we set VerificationDarcs to nil
	lightDarc1 := darcs[len(darcs)-1].Copy()
	lightDarc1.VerificationDarcs = nil
	lightDarc1.Signatures = []Signature{{
		Signature: copyBytes(darcs[len(darcs)-1].Signatures[0].Signature),
		Signer:    darcs[len(darcs)-1].Signatures[0].Signer,
	}}
	lightDarc2 := darcs[len(darcs)-2].Copy()
	lightDarc2.VerificationDarcs = nil
	lightDarc2.Signatures = []Signature{{
		Signature: copyBytes(darcs[len(darcs)-2].Signatures[0].Signature),
		Signer:    darcs[len(darcs)-2].Signatures[0].Signer,
	}}

	// verification should fail if the callback is not set
	require.NotNil(t, lightDarc1.Verify(true))
	require.NotNil(t, lightDarc2.Verify(true))

	// verification should fail if callback is wrong
	require.NotNil(t, lightDarc1.VerifyWithCB(getDarcWrong0, true))
	require.NotNil(t, lightDarc2.VerifyWithCB(getDarcWrong0, true))
	require.NotNil(t, lightDarc1.VerifyWithCB(getDarcWrong1, true))
	require.NotNil(t, lightDarc2.VerifyWithCB(getDarcWrong1, true))

	// verification should pass with the correct callback
	require.Nil(t, lightDarc1.VerifyWithCB(getDarc, true))
	require.Nil(t, lightDarc2.VerifyWithCB(getDarc, true))
}

// TestDarc_Rules is, other than the test, is an example of how one would use
// the Darc with a user-defined rule.
func TestDarc_Rules(t *testing.T) {
	d := createDarc(1, "testdarc").darc
	require.Nil(t, d.Verify(true))
	user1 := NewSignerEd25519(nil, nil)
	user2 := NewSignerEd25519(nil, nil)
	err := d.Rules.AddRule("use", expression.InitOrExpr(user1.Identity().String(), user2.Identity().String()))
	require.NoError(t, err)

	var r *Request

	// happy case with signer 1
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("secrets are lies"), user1)
	require.NoError(t, err)
	require.Nil(t, r.Verify(d))

	// happy case with signer 2
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("sharing is caring"), user2)
	require.NoError(t, err)
	require.Nil(t, r.Verify(d))

	// happy case with both signers
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("privacy is theft"), user1, user2)
	require.NoError(t, err)
	require.Nil(t, r.Verify(d))

	// wrong ID
	d2 := createDarc(1, "testdarc2").darc
	r, err = InitAndSignRequest(d2.GetID(), "use", []byte("all animals are equal"), user1)
	require.NoError(t, err)
	require.NotNil(t, r.Verify(d))

	// wrong action
	r, err = InitAndSignRequest(d.GetID(), "go", []byte("four legs good"), user1)
	require.NoError(t, err)
	require.NotNil(t, r.Verify(d))

	// wrong signer 1
	user3 := NewSignerEd25519(nil, nil)
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("two legs bad"), user3)
	require.NoError(t, err)
	require.NotNil(t, r.Verify(d))

	// happy case where at least one signer is valid
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("four legs good"), user1, user3)
	require.NoError(t, err)
	require.Nil(t, r.Verify(d))

	// tampered signature
	r, err = InitAndSignRequest(d.GetID(), "use", []byte("two legs better"), user1, user3)
	r.Signatures[0] = copyBytes(r.Signatures[1])
	require.NoError(t, err)
	require.NotNil(t, r.Verify(d))
}

func TestDarc_EvolveRequest(t *testing.T) {
	td := createDarc(1, "testdarc")
	require.Nil(t, td.darc.Verify(true))

	dNew := td.darc.Copy()
	require.Nil(t, dNew.EvolveFrom(td.darc))
	var err error
	var r *Request

	// cannot create request with nil darc
	var nilDarc *Darc
	r, _, err = nilDarc.MakeEvolveRequest()
	require.NotNil(t, err)
	require.Nil(t, r)

	// cannot create request with no signers
	r, _, err = dNew.MakeEvolveRequest()
	require.NotNil(t, err)
	require.Nil(t, r)

	// create a request with a wrong signer, the creation should succeed
	// but the verification shold fail
	badOwner := NewSignerEd25519(nil, nil)
	r, _, err = dNew.MakeEvolveRequest(badOwner)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NotNil(t, r.Verify(td.darc))

	// create the request with the right signer and it should pass
	r, dNewBuf, err := dNew.MakeEvolveRequest(td.owners[0])
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Nil(t, r.Verify(td.darc))

	// check that the evolution is actually OK
	dNew2, err := r.MsgToDarc(dNewBuf)
	require.NoError(t, err)
	require.Nil(t, dNew2.VerifyWithCB(func(s string, latest bool) *Darc {
		if latest {
			return nil
		}
		if s == NewIdentityDarc(td.darc.GetID()).String() {
			return td.darc
		}
		return nil
	}, true))
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

	require.Nil(t, localEvolution(td2.darc, td1.darc, td1.owners...))
	require.Nil(t, td2.darc.Verify(true))

	require.Nil(t, localEvolution(td4.darc, td3.darc, td3.owners...))
	require.Nil(t, td4.darc.Verify(true))

	id3 := NewIdentityDarc(td3.darc.GetID())
	d2Expr := []byte(id3.String())
	require.Nil(t, td2.darc.Rules.UpdateEvolution(d2Expr))
	require.NotNil(t, td2.darc.Verify(true))
	require.Nil(t, localEvolution(td2.darc, td1.darc, td1.owners...))
	require.Nil(t, td2.darc.Verify(true))

	td5 := createDarc(2, "testdarc5")
	require.Nil(t, localEvolution(td5.darc, td2.darc, td3.owners[0]))
	require.NotNil(t, td5.darc.Verify(true))
	getDarc := DarcsToGetDarcs([]*Darc{td1.darc, td2.darc, td3.darc, td4.darc, td5.darc})
	// Evolution is not allowed because the evolution is signed by
	// td3.owners which is out of date.
	require.NotNil(t, td5.darc.VerifyWithCB(getDarc, true))
	// If the evolution is signed by the latest darc, then it's ok.
	require.Nil(t, localEvolution(td5.darc, td2.darc, td4.owners...))
	require.Nil(t, td5.darc.VerifyWithCB(getDarc, true))
}

// TestDarc_DelegationChain creates a chain of delegation and we will try to
// evolve the first darc using the signature of the last darc in the chain.
func TestDarc_DelegationChain(t *testing.T) {
	n := 10
	darcs := make([]*Darc, n)
	owners := make([]Signer, n)
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

	getDarc := DarcsToGetDarcs(darcs)
	td := createDarc(1, "new")
	// cannot do evolution because owner is not in the evolve rule
	require.Nil(t, localEvolution(td.darc, darcs[0], owners[0]))
	require.NotNil(t, td.darc.VerifyWithCB(getDarc, true))
	// fail because only the key in the latest darc can sign
	for _, o := range owners[:n-1] {
		require.Nil(t, localEvolution(td.darc, darcs[0], o))
		require.NotNil(t, td.darc.VerifyWithCB(getDarc, true))
	}
	// only the last one, containing the actual key that is in the signer
	// expression, should evaluate to true.
	require.Nil(t, localEvolution(td.darc, darcs[0], owners[n-1]))
	require.Nil(t, td.darc.VerifyWithCB(getDarc, true))
}

// TestDarc_DelegationCycle creates n darcs and a circular delegation
func TestDarc_DelegationCycle(t *testing.T) {
	n := 5
	darcs := make([]*Darc, n)
	evolvedDarcs := make([]*Darc, n)
	owners := make([]Signer, n)
	identityStrs := make([]string, n)
	for i := 0; i < n; i++ {
		td := createDarc(1, "test cycle")
		darcs[i] = td.darc
		owners[i] = td.owners[0]
		identityStrs[i] = td.ids[0].String()
		evolvedDarcs[i] = darcs[i].Copy()
	}
	for i := 0; i < n; i++ {
		if i == n-1 {
			require.NoError(t, evolvedDarcs[i].Rules.UpdateSign([]byte(darcs[0].GetIdentityString())))
		} else {
			require.NoError(t, evolvedDarcs[i].Rules.UpdateSign([]byte(darcs[i+1].GetIdentityString())))
		}
		require.NoError(t, localEvolution(evolvedDarcs[i], darcs[i], owners[i]))
	}

	getDarc := DarcsToGetDarcs(evolvedDarcs)
	err := EvalExpr([]byte(darcs[0].GetIdentityString()), getDarc, identityStrs...)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cycle detected")
}

// TestDarc_DelegationDiamond tests the situation when there are two darcs with
// the sign rule pointing to a third darc. Evaluating darc:1 && darc:2 on the
// signer of darc:3 should succeed.
func TestDarc_DelegationDiamond(t *testing.T) {
	n := 3
	darcs := make([]*Darc, n)
	evolvedDarcs := make([]*Darc, n)
	owners := make([]Signer, n)
	identityStrs := make([]string, n)
	for i := 0; i < n; i++ {
		td := createDarc(1, "test diamond")
		darcs[i] = td.darc
		owners[i] = td.owners[0]
		identityStrs[i] = td.ids[0].String()
		evolvedDarcs[i] = darcs[i].Copy()
	}
	require.NoError(t, evolvedDarcs[0].Rules.UpdateSign([]byte(darcs[2].GetIdentityString())))
	require.NoError(t, evolvedDarcs[1].Rules.UpdateSign([]byte(darcs[2].GetIdentityString())))
	require.NoError(t, evolvedDarcs[2].Rules.UpdateSign([]byte(identityStrs[2])))
	for i := 0; i < n; i++ {
		require.NoError(t, localEvolution(evolvedDarcs[i], darcs[i], owners[i]))
	}

	getDarc := DarcsToGetDarcs(evolvedDarcs)
	expr := evolvedDarcs[0].GetIdentityString() + " & " + evolvedDarcs[1].GetIdentityString()
	// use the owner of the third darc to evaluate
	err := EvalExpr([]byte(expr), getDarc, identityStrs[2])
	require.NoError(t, err)
}

func TestDarc_X509(t *testing.T) {
	// TODO
}

func TestDarc_IsSubset(t *testing.T) {
	expr := []byte(createIdentity().String())
	supersetRules := NewRules()
	supersetRules.AddRule("rule1", expr)
	supersetRules.AddRule("rule2", expr)
	supersetRules.AddRule("rule3", expr)

	properSubsetRules := NewRules()
	properSubsetRules.AddRule("rule1", expr)
	properSubsetRules.AddRule("rule2", expr)
	properSubsetRules.AddRule("rule3", expr)

	strictSubsetRules := NewRules()
	strictSubsetRules.AddRule("rule1", expr)
	strictSubsetRules.AddRule("rule2", expr)

	wrongSubsetRules := NewRules()
	wrongSubsetRules.AddRule("rule1", expr)
	wrongSubsetRules.AddRule("rule2", expr)
	wrongSubsetRules.AddRule("rule4", expr)

	require.True(t, properSubsetRules.IsSubset(supersetRules))
	require.True(t, strictSubsetRules.IsSubset(supersetRules))
	require.False(t, wrongSubsetRules.IsSubset(supersetRules))
}

func TestDarc_Attr(t *testing.T) {
	cb := func(attr string) error {
		vals, err := url.ParseQuery(attr)
		if err != nil {
			return err
		}
		if vals.Get("pass") == "true" {
			return nil
		} else if vals.Get("pass") == "false" {
			return errors.New("fail")
		}
		return errors.New("invalid attr value")
	}
	attrFuncs := make(map[string]func(string) error)
	attrFuncs["test"] = cb

	getDarc := func(id string, latest bool) *Darc {
		return nil
	}

	id := createIdentity()
	expr := []byte(id.String() + " & attr:test:pass=true")
	require.NoError(t, EvalExprAttr(expr, getDarc, attrFuncs, id.String()))

	expr = []byte(id.String() + " | attr:test:pass=true")
	require.NoError(t, EvalExprAttr(expr, getDarc, attrFuncs, "wrong_id"))

	// fail because the callback evaluates to false
	expr = []byte(id.String() + " & attr:test:pass=false")
	err := EvalExprAttr(expr, getDarc, attrFuncs, id.String())
	require.Error(t, err)
	require.Equal(t, err.Error(), "fail")

	// fail because the attribute has a wrong format
	expr = []byte("attr::pass=true")
	err = EvalExprAttr(expr, getDarc, attrFuncs, id.String())
	require.Error(t, err)
	require.Contains(t, err.Error(), "scanner is not empty")

	// fail because the attribute has a wrong format
	expr = []byte("attr:|:pass=true")
	err = EvalExprAttr(expr, getDarc, attrFuncs, id.String())
	require.Error(t, err)
	require.Contains(t, err.Error(), "scanner is not empty")
}

func TestDarc_Threshold_Simple(t *testing.T) {
	getDarc := func(id string, latest bool) *Darc {
		return nil
	}

	id1 := createIdentity().String()
	id2 := createIdentity().String()
	id3 := createIdentity().String()

	for _, test := range []struct {
		threshold string
		pass      []bool
	}{
		{"1/1", []bool{false, false, false, true}},
		{"1/2", []bool{false, false, true, true}},
		{"1/3", []bool{false, true, true, true}},
		{"2/3", []bool{false, false, true, true}},
		{"1/0", []bool{false, false, false, false}},
		{"0/1", []bool{true, true, true, true}},
	} {
		expr := []byte(fmt.Sprintf("threshold<%s,%s,%s,%s>", test.threshold,
			id1, id2, id3))

		// Test 0 ids
		res := EvalExprAttr(expr, getDarc, nil)
		require.Equal(t, res == nil, test.pass[0],
			fmt.Sprintf("Wrong result for %+v with no id given", test))

		// Test 1 id, also test double and triple ids
		for _, ids := range [][]string{{id1}, {id2}, {id3},
			{id1, id1}, {id1, id1, id1}} {
			res := EvalExprAttr(expr, getDarc, nil, ids...)
			require.Equal(t, res == nil, test.pass[1],
				fmt.Sprintf("Wrong result for %+v with 1 id (%+v)", test, ids))
		}

		// Test 2 ids
		for _, ids := range [][]string{{id1, id2}, {id1, id3}, {id2, id3}} {
			res := EvalExprAttr(expr, getDarc, nil, ids...)
			require.Equal(t, res == nil, test.pass[2],
				fmt.Sprintf("Wrong result for %+v with 2 ids (%+v)", test, ids))
		}

		// Test 3 ids
		for _, ids := range [][]string{{id1, id2, id3}} {
			res := EvalExprAttr(expr, getDarc, nil, ids...)
			require.Equal(t, res == nil, test.pass[3],
				fmt.Sprintf("Wrong result for %+v with 3 ids (%+v)", test, ids))
		}
	}
}

func TestDarc_Threshold_Darc(t *testing.T) {

	darc1 := createDarc(1, "darc 1")
	id1 := darc1.owners[0].Identity()

	getDarc := func(id string, latest bool) *Darc {
		return darc1.darc
	}

	expr := []byte(fmt.Sprintf("threshold<1/1,%s>", darc1.darc.GetIdentityString()))

	// it shouldn't work because we didn't add the identity on the _sign
	// expression.
	err := EvalExprAttr(expr, getDarc, nil, id1.String())
	require.Error(t, err)

	darc1.darc.Rules.UpdateSign([]byte(id1.String()))

	err = EvalExprAttr(expr, getDarc, nil, id1.String())
	require.NoError(t, err)
}

func TestDarc_Threshold_Cycle_Darc(t *testing.T) {
	// let's create a loop, where darc1 delegates to darc2, which delegates to
	// darc1: darc1 -> darc2 -> darc1

	darc1 := createDarc(1, "darc 1")
	darc2 := createDarc(1, "darc 2")

	darcID1 := darc1.darc.GetIdentityString()
	darcID2 := darc2.darc.GetIdentityString()

	err := darc1.darc.Rules.UpdateSign([]byte(fmt.Sprintf("threshold<1/1,%s>", darcID2)))
	require.NoError(t, err)

	err = darc2.darc.Rules.UpdateSign([]byte(fmt.Sprintf("threshold<1/1,%s>", darcID1)))
	require.NoError(t, err)

	getDarc := func(id string, latest bool) *Darc {
		switch id {
		case darcID1:
			return darc1.darc
		case darcID2:
			return darc2.darc
		}
		return nil
	}

	expr := []byte(fmt.Sprintf("threshold<1/1,%s>", darcID1))
	err = EvalExpr([]byte(expr), getDarc, darc1.ids[0].String(), darc2.ids[0].String())
	// We shouldn't end up with an infinite loop, and the expression should not
	// be accepted.
	require.Error(t, err)
}

type testDarc struct {
	darc   *Darc
	owners []Signer
	ids    []Identity
}

func createDarc(nbrOwners int, desc string) testDarc {
	td := testDarc{}
	for i := 0; i < nbrOwners; i++ {
		s, id := createSignerIdentity()
		td.owners = append(td.owners, s)
		td.ids = append(td.ids, id)
	}
	rules := InitRules(td.ids, []Identity{})
	td.darc = NewDarc(rules, []byte(desc))
	return td
}

func createIdentity() Identity {
	_, id := createSignerIdentity()
	return id
}

func createSignerIdentity() (Signer, Identity) {
	signer := NewSignerEd25519(nil, nil)
	return signer, signer.Identity()
}

// localEvolution sets the fields of newDarc such that it's a valid evolution
// and then signs the evolution.
func localEvolution(newDarc *Darc, oldDarc *Darc, signers ...Signer) error {
	if err := newDarc.EvolveFrom(oldDarc); err != nil {
		return err
	}
	r, _, err := newDarc.MakeEvolveRequest(signers...)
	if err != nil {
		return err
	}
	sigs := make([]Signature, len(signers))
	for i := range r.Identities {
		sigs[i] = Signature{
			Signature: r.Signatures[i],
			Signer:    r.Identities[i],
		}
	}
	newDarc.Signatures = sigs
	newDarc.VerificationDarcs = append(oldDarc.VerificationDarcs, oldDarc)
	return nil
}

func TestParseIdentity(t *testing.T) {
	_, err := ParseIdentity("")
	require.Error(t, err)
	_, err = ParseIdentity(":")
	require.Error(t, err)
	_, err = ParseIdentity("wat?")
	require.Error(t, err)
	_, err = ParseIdentity("wat?:this!")
	require.Error(t, err)

	in := "ed25519:8557024ed89b420674a18121072a31d35a56125185f4f4bbbc30b06a374c1113"
	i, err := ParseIdentity(in)
	require.NoError(t, err)
	require.NotNil(t, i.Ed25519)
	require.Equal(t, in, i.String())

	in = "darc:xxx"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "darc:010203"
	i, err = ParseIdentity(in)
	require.NoError(t, err)
	require.NotNil(t, i.Darc)
	require.Equal(t, in, i.String())

	in = "x509ec:xxx"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "x509ec:010203"
	i, err = ParseIdentity(in)
	require.NoError(t, err)
	require.NotNil(t, i.X509EC)
	require.Equal(t, in, i.String())

	in = "proxy:"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "proxy:invalid point:data"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "proxy:8557024ed89b420674a18121072a31d35a56125185f4f4bbbc30b06a374c1113:data"
	i, err = ParseIdentity(in)
	require.NoError(t, err)
	require.NotNil(t, i.Proxy)
	require.Equal(t, in, i.String())

	in = "evm_contract:00"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "evm_contract:xx:0xyy"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "evm_contract:00:0x00:00"
	i, err = ParseIdentity(in)
	require.Error(t, err)

	in = "evm_contract:00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff:0x00112233445566778899aabbccddeeff00112233"
	i, err = ParseIdentity(in)
	require.NoError(t, err)
	require.NotNil(t, i.EvmContract)
	// ToLower() because common.Address uses address checksum (EIP-55)
	require.Equal(t, in, strings.ToLower(i.String()))
}

// Test any identity
func testIdentity(t *testing.T, sig Signer) {
	msg := []byte("something secret")
	signed, err := sig.Sign(msg)
	require.NoError(t, err)

	id := sig.Identity()
	require.NoError(t, id.Verify(msg, signed))
	require.Error(t, id.Verify([]byte("wrong message"), signed))
}

// Test the different identities available - currently only Ed25519.
func TestIdentities(t *testing.T) {
	testIdentity(t, NewSignerEd25519(nil, nil))
}
