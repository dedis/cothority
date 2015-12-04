package tree_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/tree"
	"github.com/dedis/crypto/edwards"
	"testing"
)

func TestMarshalSuite(t *testing.T) {
	s, err := tree.NewSuiteJSON("25519")
	if err != nil {
		t.Fatal("Coulnd't make 25519-suite:", err)
	}
	s_json, err := s.MarshalJSON()
	if err != nil {
		t.Fatal("Couldn't marshal SuiteJSON:", err)
	}
	s_copy := &tree.SuiteJSON{}
	err = s_copy.UnmarshalJSON(s_json)
	if err != nil {
		t.Fatal("Couldn't unmarshal SuiteJSON:", err)
	}
}

func TestMarshalPoint(t *testing.T) {
	s := edwards.NewAES128SHA256Ed25519(false)
	point := tree.NewPointJSON(s, s.Point().Base())
	p_json, err := point.MarshalJSON()
	if err != nil {
		t.Fatal("Couldn't marshal PointJSON:", err)
	}
	p_copy := &tree.PointJSON{}
	err = p_copy.UnmarshalJSON(p_json)
	if err != nil {
		t.Fatal("Couldn't unmarshal PointJSON:", err)
	}
}

func TestMarshalSecret(t *testing.T) {
	s := edwards.NewAES128SHA256Ed25519(false)
	secret := tree.NewSecretJSON(s, s.Secret().Zero())
	p_json, err := secret.MarshalJSON()
	if err != nil {
		t.Fatal("Couldn't marshal PointJSON:", err)
	}
	s_copy := &tree.SecretJSON{}
	err = s_copy.UnmarshalJSON(p_json)
	if err != nil {
		t.Fatal("Couldn't unmarshal PointJSON:", err)
	}
}

func TestArithmJSON(t *testing.T) {
	s, _ := tree.NewSuiteJSON("25519")
	rand := s.Cipher([]byte("secret"))
	secret := s.Secret().Pick(rand)
	public := s.Point().Mul(nil, secret)
	dbg.Print(public.MarshalJSON())
}
