package darc

import (
	"github.com/dedis/cothority/omniledger/darc/expression"
)

// ID is the identity of a Darc - which is the sha256 of its protobuf representation
// over invariant fields [Owners, Users, Version, Description]. Signature is excluded.
// An evolving Darc will change its identity.
type ID []byte

// Action is a string that should be associated with an expression. The
// application typically will define the action but there are two actions that
// are in all the darcs, "_evolve" and "_sign". The application can modify
// these actions but should not change the semantics of these actions.
type Action string

// Rules are action-expression associations.
type Rules map[Action]expression.Expr
