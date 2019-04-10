package ocs

import (
	"errors"

	"go.dedis.ch/kyber/v3"

	"go.dedis.ch/onet/v3"
)

func (ocs CreateOCS) verify() error {
	if err := ocs.PolicyReencrypt.verify(ocs.Roster); err != nil {
		return err
	}
	if err := ocs.PolicyReshare.verify(ocs.Roster); err != nil {
		return err
	}
	return nil
}

func (re Reshare) verify() error {
	return errors.New("not yet implemented")
}

func (p Policy) verify(r onet.Roster) error {
	return errors.New("not yet implemented")
}

func (ar AuthReencrypt) verify(X OCSID) error {
	return errors.New("not yet implemented")
}

func (ar AuthReencrypt) Xc() kyber.Point {
	return nil
}

func (ar AuthReencrypt) C() kyber.Point {
	return nil
}
