/*
Package expression contains the definition and implementation of a simple
language for defining complex policies. We define the language in extended-BNF notation,
the syntax we use is from: https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form

	expr = term, [ '&', term ]*
	term = factor, [ '|', factor ]*
	factor = '(', expr, ')' | id
	id = [0-9a-z]+, ':', [0-9a-f]+

Examples:

        ed25519:deadbeef // every id evaluates to a boolean
	(a:a & b:b) | (c:c & d:d)

In the simplest case, the evaluation of an expression is performed against a
set of valid ids.  Suppose we have the expression (a:a & b:b) | (c:c & d:d),
and the set of valid ids is [a:a, b:b], then the expression will evaluate to
true.  If the set of valid ids is [a:a, c:c], then the expression will evaluate
to false. However, the user is able to provide a ValueCheckFn to customise how
the expressions are evaluated.

EXTENSION - NOT YET IMPLEMENTED:
To support threshold signatures, we extend the syntax to include the following.
	thexpr = '[', id, [ ',', id ]*, ']', '/', digit
*/
package expression

import (
	"errors"
	"strings"

	parsec "github.com/prataprc/goparsec"
)

const scannerNotEmpty = "parsing failed - scanner is not empty"
const failedToCast = "evauluation failed - result is not bool"

// ValueCheckFn is a function that will be called when the parser is
// parsing/evaluating an expression.
// TODO it is useful if we can return (bool, error).
type ValueCheckFn func(string) bool

// Expr represents the unprocess expression of our DSL.
type Expr []byte

// InitParser creates the root parser
func InitParser(fn ValueCheckFn) parsec.Parser {
	// Y is root Parser, usually called as `s` in CFG theory.
	var Y parsec.Parser
	var sum, value parsec.Parser // circular rats

	// Terminal rats
	var openparan = parsec.Token(`\(`, "OPENPARAN")
	var closeparan = parsec.Token(`\)`, "CLOSEPARAN")
	var andop = parsec.Token(`&`, "AND")
	var orop = parsec.Token(`\|`, "OR")

	// NonTerminal rats
	// andop -> "&" |  "|"
	var sumOp = parsec.OrdChoice(one2one, andop, orop)

	// value -> "(" expr ")"
	var groupExpr = parsec.And(exprNode, openparan, &sum, closeparan)

	// (andop prod)*
	var prodK = parsec.Kleene(nil, parsec.And(many2many, sumOp, &value), nil)

	// Circular rats come to life
	// sum -> prod (andop prod)*
	sum = parsec.And(sumNode(fn), &value, prodK)
	// value -> id | "(" expr ")"
	value = parsec.OrdChoice(exprValueNode(fn), id(), groupExpr)
	// expr  -> sum
	Y = parsec.OrdChoice(one2one, sum)
	return Y
}

// Evaluate uses the input parser to evaluate the expression expr. It returns
// the result of the evaluate (a boolean), but the result is only valid if
// there are no errors.
func Evaluate(parser parsec.Parser, expr Expr) (bool, error) {
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
	return Evaluate(InitParser(func(s string) bool {
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
