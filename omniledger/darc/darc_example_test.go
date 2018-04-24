package darc_test

import (
	"fmt"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	"github.com/dedis/student_18_omniledger/omniledger/darc/expression"
)

func Example() {
	// We can create a new darc like so.
	owner1 := darc.NewSignerEd25519(nil, nil)
	rules1 := darc.InitEvolutionRule(owner1.Identity())
	d1 := darc.NewDarc(rules1, []byte("example darc"))
	fmt.Println(d1.Verify())

	// Create another one and set the first to evolve to the second. The
	// second darc will have a different evolution rule.
	owner2 := darc.NewSignerEd25519(nil, nil)
	rules2 := darc.InitEvolutionRule(owner2.Identity())
	d2 := darc.NewDarc(rules2, []byte("example darc 2"))
	fmt.Println(d2.Verify())
	d2.Evolve([]*darc.Darc{d1}, owner1)
	fmt.Println(d2.Verify())

	// The above illustrates the basic use of darcs, in the following
	// examples, we show how to create custom rules to enforce custom
	// policies.
	owner3 := darc.NewSignerEd25519(nil, nil)
	action3 := darc.Action("custom_action")
	expr3 := expression.InitAndExpr(
		owner1.Identity().String(),
		owner2.Identity().String(),
		owner3.Identity().String())
	d3 := d2.Copy()
	d3.Rules.AddRule(action3, expr3)
	d3.Evolve([]*darc.Darc{d1, d2}, owner2)
	fmt.Println(d3.Verify())

	r, _ := darc.NewRequest(d3.GetID(), action3, []byte("example request"), owner3)
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
	// <nil>
	// not ok!
	// ok!
}
