package tree

import (
	"encoding/json"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/suites"
)

type PointJSON struct {
	Suite *SuiteJSON
	abstract.Point
}

type SecretJSON struct {
	Suite *SuiteJSON
	abstract.Secret
}

type SuiteJSON struct {
	abstract.Suite
}

func NewSuiteJSON(suite string) (*SuiteJSON, error) {
	s, err := suites.StringToSuite(suite)
	if err != nil {
		return nil, err
	}
	return &SuiteJSON{
		s,
	}, nil
}

func (suite *SuiteJSON) Point() *PointJSON {
	return NewPointJSON(suite.Suite, suite.Point())
}

func (suite *SuiteJSON) Secret() *SecretJSON {
	return NewSecretJSON(suite.Suite, suite.Secret())
}

func (suite *SuiteJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct{ SuiteStr string }{SuiteStr: suite.String()})
}

func (suite *SuiteJSON) UnmarshalJSON(data []byte) error {
	aux := &struct{ SuiteStr string }{}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	suite.Suite, err = suites.StringToSuite(aux.SuiteStr)
	if err != nil {
		return err
	}
	return nil
}

func NewPointJSON(s abstract.Suite, p abstract.Point) *PointJSON {
	return &PointJSON{Suite: &SuiteJSON{s}, Point: p}
}

func (p *PointJSON) Mul(poin abstract.Point, sec abstract.Secret) abstract.Point {
	return NewPointJSON(p.Suite, p.Mul(poin, sec))
}

func (p *PointJSON) MarshalJSON() ([]byte, error) {
	type Alias PointJSON
	p_binary, err := p.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		Point []byte
		*Alias
	}{
		Point: p_binary,
		Alias: (*Alias)(p),
	})
}

func (p *PointJSON) UnmarshalJSON(data []byte) error {
	type Alias PointJSON
	aux := &struct {
		Point []byte
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	p.Point = p.Suite.Suite.Point()
	err = p.UnmarshalBinary(aux.Point)
	if err != nil {
		return err
	}
	return nil
}

func NewSecretJSON(s abstract.Suite, sec abstract.Secret) *SecretJSON {
	return &SecretJSON{Suite: &SuiteJSON{s}, Secret: sec}
}

func (s *SecretJSON) MarshalJSON() ([]byte, error) {
	type Alias SecretJSON
	s_binary, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		Secret []byte
		*Alias
	}{
		Secret: s_binary,
		Alias:  (*Alias)(s),
	})
}

func (s *SecretJSON) UnmarshalJSON(data []byte) error {
	type Alias SecretJSON
	aux := &struct {
		Secret []byte
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	s.Secret = s.Suite.Suite.Secret()
	err = s.UnmarshalBinary(aux.Secret)
	if err != nil {
		return err
	}
	return nil
}
