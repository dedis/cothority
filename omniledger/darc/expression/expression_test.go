package expression

import (
	"testing"

	parsec "github.com/prataprc/goparsec"
)

func trueFn(s string) bool {
	return true
}

func falseFn(s string) bool {
	return false
}

func TestExprAllTrue(t *testing.T) {
	Y := InitParser(trueFn)
	s := parsec.NewScanner([]byte("a:abc & b:bb"))
	v, s := Y(s)
	if v.(bool) != true {
		t.Fatalf("Mismatch value %v\n", v)
	}
	if !s.Endof() {
		t.Fatal("Scanner did not end")
	}
}

func TestExprAllFalse(t *testing.T) {
	Y := InitParser(falseFn)
	s := parsec.NewScanner([]byte("a:abc & b:bb"))
	v, s := Y(s)
	if v.(bool) != false {
		t.Fatalf("Mismatch value %v\n", v)
	}
	if !s.Endof() {
		t.Fatal("Scanner did not end")
	}
}

func TestInitOr(t *testing.T) {
	// TODO
}

func TestInitAnd(t *testing.T) {
	// TODO
}

func TestParsing_One(t *testing.T) {
	expr := []byte("a:abc")
	fn := func(s string) bool {
		if s == "a:abc" {
			return true
		}
		return false
	}
	v, s := InitParser(fn)(parsec.NewScanner(expr))
	if v.(bool) != true {
		t.Fatalf("Mismatch value %v\n", v)
	}
	if !s.Endof() {
		t.Fatal("Scanner did not end")
	}
}

func TestParsing_Or(t *testing.T) {
	expr := []byte("a:abc | b:abc | c:abc")
	fn := func(s string) bool {
		if s == "b:abc" {
			return true
		}
		return false
	}
	v, s := InitParser(fn)(parsec.NewScanner(expr))
	if v.(bool) != true {
		t.Fatalf("Mismatch value %v\n", v)
	}
	if !s.Endof() {
		t.Fatal("Scanner did not end")
	}
}

func TestParsing_InvalidID_1(t *testing.T) {
	expr := []byte("x")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("expect an error")
	}
	if err.Error() != scannerNotEmpty {
		t.Fatalf("wrong error message, got %s", err.Error())
	}
}

func TestParsing_InvalidID_2(t *testing.T) {
	expr := []byte("a: b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("expect an error")
	}
}

func TestParsing_InvalidOp(t *testing.T) {
	expr := []byte("a:abc / b:abc")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("expect an error")
	}
	if err.Error() != scannerNotEmpty {
		t.Fatalf("wrong error message, got %s", err.Error())
	}
}

func TestParsing_Paran(t *testing.T) {
	expr := []byte("(a:b)")
	x, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
	if x != true {
		t.Fatal("wrong result")
	}
}

func TestParsing_Nesting(t *testing.T) {
	expr := []byte("(a:b | (b:c & c:d))")
	x, err := Evaluate(InitParser(func(s string) bool {
		if s == "b:c" || s == "c:d" {
			return true
		}
		return false
	}), expr)
	if err != nil {
		t.Fatal(err)
	}
	if x != true {
		t.Fatal("wrong result")
	}
}

func TestParsing_Imbalance(t *testing.T) {
	expr := []byte("(a:b | b:c & c:d))")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("error is expected")
	}
}

func TestParsing_LeftSpace(t *testing.T) {
	expr := []byte(" a:b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_RightSpace(t *testing.T) {
	expr := []byte("a:b ")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_NoSpace(t *testing.T) {
	expr := []byte("a:b|a:c&b:b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_RealIDs(t *testing.T) {
	expr := []byte("ed25519:5764e85642c3bda8748c5cf3d7f14c6d5c18e193228d70f4c58dd80ed4582748 | ed25519:bf58ca4b1ddb07a7a9bbf57fe9b856f214a38dd872b6ec07efbeb0a01003fae9")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_Empty(t *testing.T) {
	expr := []byte{}
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("empty expr should fail")
	}
}

func TestEval_DefaultParser(t *testing.T) {
	keys := []string{"a:a", "b:b", "c:c", "d:d"}
	expr := InitAndExpr(keys...)
	ok, err := DefaultParser(expr, keys...)
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatal("evaluation should return to true")
	}

	// If the expression has an extra term, then the evaluation should fail
	// because the extra term is not a part of the valid keys.
	expr = append(expr, []byte(" & e:e")...)
	ok, err = DefaultParser(expr, keys...)
	if err != nil {
		t.Fatal(err)
	}
	if ok != false {
		t.Fatal("evaluation should return false")
	}
}
