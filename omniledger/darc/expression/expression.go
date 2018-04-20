package expression

import (
	"errors"
	"strings"

	parsec "github.com/prataprc/goparsec"
)

/*
We define an expression language for defining complex policies. First, we
clarify a few definitions.  A _policy_ is represented by the Darc structure.
Unlike the darc in the current implementation, we remove owners and users and
use a general technique to specify the policy calles _rules_.

There are many _rules_ for each policy, each rules defines an action. The
actions can be, for example, 'READ', 'WRITE', DELETE' and so on, they are
specified by the application. Examples of the data structures are shown below.

	type Darc struct {
		Version int
		Description *[]byte
		BaseID *ID // ID of the first Darc
		Rules map[string]Expr // key is the action, one rule per action, should we include a resource in Rules?
		Signature *Signature // signature over the fields above
	}

	type Expr {
		AST
	}

Finally, an expression is a DSL that defines, we define the DSL in extended-BNF.
Notation used are from https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form

	expr = term, [ '&', term ]*
	term = factor, [ '|', factor ]*
	factor = '(' expr ')' | id
	id = hex-string

Examples:

	id1 // every identifier evaluates to a boolean
	(id1 & id2) | (id3 & id4)

The evaluation of expressions is performed against requests. The rule referring
to the action in the request is used to check expression. In brief, the service
(who is responsible for checking the request) evaluates the expression by first
checking whether the action exists, otherwise the evaluation fails.  Otherwise,
the service looks at the signatures in the request, and sets the unevaluated
identities to true if the signature is valid and the signer matches the ID in
the expression.

	type Request struct {
		DarcID         Identity // for identifying the darc
		Signatures [][]byte // we need multi signatures because expression
		Action     string   // do we need this, also specific to the application?
		Msg        []byte   // what the request wants to do, application specific
		Darcs      *[]*Darc // for offline verification
	}

	// should we define the message like this? then Action won't be needed anymore
	type MsgWithAction interface {
		func GetActions() []string
		func GetMsg() [] []byte
	}
*/

const scannerNotEmpty = "parsing failed - scanner is not empty"
const failedToCast = "evauluation failed - result is not bool"

// ValueCheckFn TODO
type ValueCheckFn func(string) bool

// Expr TODO
type Expr []byte

// InitParser creates the root parser
func InitParser(fn ValueCheckFn) parsec.Parser {
	// Y is root Parser, usually called as `s` in CFG theory.
	var Y parsec.Parser
	var sum, value parsec.Parser // circular rats

	// Terminal rats
	var openparan = parsec.Token(`\(`, "OPENPARAN")
	var closeparan = parsec.Token(`\)`, "CLOSEPARAN")
	var addop = parsec.Token(`&`, "AND")
	var subop = parsec.Token(`\|`, "OR")

	// NonTerminal rats
	// addop -> "&" |  "|"
	var sumOp = parsec.OrdChoice(one2one, addop, subop)

	// value -> "(" expr ")"
	var groupExpr = parsec.And(exprNode, openparan, &sum, closeparan)

	// (addop prod)*
	var prodK = parsec.Kleene(nil, parsec.And(many2many, sumOp, &value), nil)

	// Circular rats come to life
	// sum -> prod (addop prod)*
	sum = parsec.And(sumNode(fn), &value, prodK)
	// value -> id | "(" expr ")"
	value = parsec.OrdChoice(exprValueNode(fn), id(), groupExpr)
	// expr  -> sum
	Y = parsec.OrdChoice(one2one, sum)
	return Y
}

// ParseExpr TODO
func ParseExpr(parser parsec.Parser, expr Expr) (bool, error) {
	v, s := parser(parsec.NewScanner(expr))
	_, s = s.SkipWS()
	if !s.Endof() {
		return false, errors.New(scannerNotEmpty)
	}
	vv, ok := v.(bool)
	if !ok {
		return false, errors.New(failedToCast)
	}
	return vv, nil
}

// DefaultParser creates a parser and evaluates the expression expr, every id
// in pks will evaluate to true.
func DefaultParser(expr Expr, ids ...string) (bool, error) {
	return ParseExpr(InitParser(func(s string) bool {
		for _, k := range ids {
			if k == s {
				return true
			}
		}
		return false
	}), expr)
}

// InitAndExpr creates an expression where & (and) is used to combine all the
// IDs.
func InitAndExpr(ids ...string) Expr {
	return Expr(strings.Join(ids, " & "))
}

// InitOrExpr creates an expression where | (or) is used to combine all the
// IDs.
func InitOrExpr(ids ...string) Expr {
	return Expr(strings.Join(ids, " | "))
}

func id() parsec.Parser {
	return func(s parsec.Scanner) (parsec.ParsecNode, parsec.Scanner) {
		_, s = s.SkipAny(`^[  \n\t]+`)
		p := parsec.Token(`[0-9a-z]+:[0-9a-f]+`, "ID")
		return p(s)
	}
}

func sumNode(fn ValueCheckFn) func(ns []parsec.ParsecNode) parsec.ParsecNode {
	return func(ns []parsec.ParsecNode) parsec.ParsecNode {
		if len(ns) > 0 {
			val := ns[0].(bool)
			for _, x := range ns[1].([]parsec.ParsecNode) {
				y := x.([]parsec.ParsecNode)
				n := y[1].(bool)
				switch y[0].(*parsec.Terminal).Name {
				case "AND":
					val = val && n
				case "OR":
					val = val || n
				}
			}
			return val
		}
		return nil
	}
}

func exprValueNode(fn ValueCheckFn) func(ns []parsec.ParsecNode) parsec.ParsecNode {
	return func(ns []parsec.ParsecNode) parsec.ParsecNode {
		if len(ns) == 0 {
			return nil
		} else if term, ok := ns[0].(*parsec.Terminal); ok {
			return fn(term.Value)
		}
		return ns[0]
	}
}

func exprNode(ns []parsec.ParsecNode) parsec.ParsecNode {
	if len(ns) == 0 {
		return nil
	}
	return ns[1]
}

func one2one(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) == 0 {
		return nil
	}
	return ns[0]
}

func many2many(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) == 0 {
		return nil
	}
	return ns
}
