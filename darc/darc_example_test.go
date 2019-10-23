package darc_test

import (
	"bytes"
	"fmt"

	"go.dedis.ch/cothority/v4/darc"
	"go.dedis.ch/cothority/v4/darc/expression"
)

func Example() {
	// Consider a client-server configuration. Where the client holds the
	// credentials and wants the server to check for requests or evolve
	// darcs. We begin by creating a darc on the server.
	// We can create a new darc like so.
	owner1 := darc.NewSignerEd25519(nil, nil)
	rules1 := darc.InitRules([]darc.Identity{owner1.Identity()}, []darc.Identity{})
	d1 := darc.NewDarc(rules1, []byte("example darc"))
	fmt.Println(d1.Verify(true))

	// Now the client wants to evolve the darc (change the owner), so it
	// creates a request and then sends it to the server.
	owner2 := darc.NewSignerEd25519(nil, nil)
	rules2 := darc.InitRules([]darc.Identity{owner2.Identity()}, []darc.Identity{})
	d2 := darc.NewDarc(rules2, []byte("example darc 2"))
	d2.EvolveFrom(d1)
	r, d2Buf, err := d2.MakeEvolveRequest(owner1)
	fmt.Println(err)

	// Client sends request r and serialised darc d2Buf to the server, and
	// the server must verify it. Usually the server will look in its
	// database for the base ID of the darc in the request and find the
	// latest one. But in this case we assume it already knows. If the
	// verification is successful, then the server should add the darc in
	// the request to its database.
	fmt.Println(r.Verify(d1)) // Assume we can find d1 given r.
	d2Server, _ := r.MsgToDarc(d2Buf)
	fmt.Println(bytes.Equal(d2Server.GetID(), d2.GetID()))

	// If the darcs stored on the server are trustworthy, then using
	// `Request.Verify` is enough. To do a complete verification,
	// Darc.Verify should be used. This will traverse the chain of
	// evolution and verify every evolution. However, the Darc.Path
	// attribute must be set.
	fmt.Println(d2Server.VerifyWithCB(func(s string, latest bool) *darc.Darc {
		if s == darc.NewIdentityDarc(d1.GetID()).String() {
			return d1
		}
		return nil
	}, true))

	// The above illustrates the basic use of darcs, in the following
	// examples, we show how to create custom rules to enforce custom
	// policies. We begin by making another evolution that has a custom
	// action.
	owner3 := darc.NewSignerEd25519(nil, nil)
	action3 := darc.Action("custom_action")
	expr3 := expression.InitAndExpr(
		owner1.Identity().String(),
		owner2.Identity().String(),
		owner3.Identity().String())
	d3 := d1.Copy()
	d3.Rules.AddRule(action3, expr3)

	// Typically the Msg part of the request is a digest of the actual
	// message. For simplicity in this example, we put the actual message
	// in there.
	r, _ = darc.InitAndSignRequest(d3.GetID(), action3, []byte("example request"), owner3)
	if err := r.Verify(d3); err != nil {
		// not ok because the expression is created using logical and
		fmt.Println("not ok!")
	}

	r, _ = darc.InitAndSignRequest(d3.GetID(), action3, []byte("example request"), owner1, owner2, owner3)
	if err := r.Verify(d3); err == nil {
		fmt.Println("ok!")
	}

	// Output:
	// <nil>
	// <nil>
	// <nil>
	// true
	// <nil>
	// not ok!
	// ok!
}
