package expression

import (
	"strings"
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
	s := parsec.NewScanner([]byte("ed25519:abc & x509ec:bb"))
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
	s := parsec.NewScanner([]byte("ed25519:abc & x509ec:bb"))
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
	expr := []byte("ed25519:abc")
	fn := func(s string) bool {
		if s == "ed25519:abc" {
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
	expr := []byte("ed25519:abc | ed25519:abc | ed25519:abc")
	fn := func(s string) bool {
		if s == "ed25519:abc" {
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
	if !strings.HasPrefix(err.Error(), errScannerNotEmpty.Error()) {
		t.Fatalf("wrong error message, got %v", err.Error())
	}
}

func TestParsing_InvalidID_2(t *testing.T) {
	expr := []byte("ed25519: b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("expect an error")
	}
}

func TestParsing_InvalidOp(t *testing.T) {
	expr := []byte("ed25519:abc / ed25519:abc")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("expect an error")
	}
	if !strings.HasPrefix(err.Error(), errScannerNotEmpty.Error()) {
		t.Fatalf("wrong error message, got %s", err.Error())
	}
}

func TestParsing_Paran(t *testing.T) {
	expr := []byte("(ed25519:b)")
	x, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
	if x != true {
		t.Fatal("wrong result")
	}
}

func TestParsing_Nesting(t *testing.T) {
	expr := []byte("(ed25519:b | (ed25519:c & ed25519:d))")
	x, err := Evaluate(InitParser(func(s string) bool {
		if s == "ed25519:c" || s == "ed25519:d" {
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
	expr := []byte("(ed25519:b | ed25519:c & ed25519:d))")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("error is expected")
	}
}

func TestParsing_LeftSpace(t *testing.T) {
	expr := []byte(" ed25519:b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_RightSpace(t *testing.T) {
	expr := []byte("ed25519:b ")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_NoSpace(t *testing.T) {
	expr := []byte("ed25519:b|ed25519:c&ed25519:b")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_RealIDs(t *testing.T) {
	expr := []byte("darc:5764e85642")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
	expr = []byte("ed25519:5764e85642c3bda8748c5cf3d7f14c6d5c18e193228d70f4c58dd80ed4582748")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
	expr = []byte("proxy:5764e85642c3bda8748c5cf3d7f14c6d5c18e193228d70f4c58dd80ed4582748:admin_user@example.com")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParsing_Attr(t *testing.T) {
	expr := []byte("attr:xyz_-:zys=123&sdy=234")
	_, err := Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}

	expr = []byte("attr:xyz_-:zys=123&sdy=234 & ed25519:abc")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}

	expr = []byte("attr:xyz:")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err != nil {
		t.Fatal(err)
	}

	expr = []byte("attr:xyz*:zys=123")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("attr name cannot have an *")
	}

	expr = []byte("attr::zys=123")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("attr name cannot be empty")
	}

	expr = []byte("attr:abc")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("attr value cannot be empty")
	}

	expr = []byte("attr:abc:zys = 123")
	_, err = Evaluate(InitParser(trueFn), expr)
	if err == nil {
		t.Fatal("attr value cannot have spaces")
	}

	expr = []byte(`attr:myattr:{"purpose":"work","age":10} & attr:abc:efg`)
	_, err = Evaluate(InitParser(trueFn), expr)
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
	keys := []string{"ed25519:a", "ed25519:b", "x509ec:c", "darc:d"}
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
	expr = append(expr, []byte(" & x509ec:e")...)
	ok, err = DefaultParser(expr, keys...)
	if err != nil {
		t.Fatal(err)
	}
	if ok != false {
		t.Fatal("evaluation should return false")
	}
}
