package darc_test

import (
	"bytes"
	"fmt"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/dedis/student_18_omniledger/omniledger/darc/expression"
)

func Example() {
	// Consider a client-server configuration. Where the client holds the
	// credentials and wants the server to check for requests or evolve
	// darcs. We begin by creating a darc on the server.
	// We can create a new darc like so.
	owner1 := darc.NewSignerEd25519(nil, nil)
	rules1 := darc.InitRules([]*darc.Identity{owner1.Identity()}, []*darc.Identity{})
	d1 := darc.NewDarc(rules1, []byte("example darc"))
	fmt.Println(d1.Verify())

	// Now the client wants to evolve the darc (change the owner), so it
	// creates a request and then sends it to the server.
	owner2 := darc.NewSignerEd25519(nil, nil)
	rules2 := darc.InitRules([]*darc.Identity{owner2.Identity()}, []*darc.Identity{})
	d2 := darc.NewDarc(rules2, []byte("example darc 2"))
	d2.EvolveFrom([]*darc.Darc{d1})
	r, err := d2.MakeEvolveRequest(owner1)
	fmt.Println(err)

	// Client sends request r to the server, and the server must verify it.
	// Usually the server will look in its database for the base ID of the
	// darc in the request and find the latest one. But in this case we
	// assume it already knows. If the check is successful, then the
	// server should add the darc in the request to its database.
	fmt.Println(d1.CheckRequest(r))
	d2Server, _ := r.MsgToDarc([]*darc.Darc{d1}) // Server stores d2Server.
	fmt.Println(bytes.Equal(d2Server.GetID(), d2.GetID()))

	// If the darcs stored on the server are trustworthy, then using
	// `CheckRequest` is enough. To do a complete verification, Darc.Verify
	// should be used. This will traverse the chain of evolution and verify
	// every evolution. However, the Darc.Path attribute must be set.
	fmt.Println(d2Server.Verify())

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

	r, _ = darc.NewRequest(d3.GetID(), action3, []byte("example request"), owner3)
	if err := d3.CheckRequest(r); err != nil {
		// not ok because the expression is created using logical and
		fmt.Println("not ok!")
	}

	r, _ = darc.NewRequest(d3.GetID(), action3, []byte("example request"), owner1, owner2, owner3)
	if err := d3.CheckRequest(r); err == nil {
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
