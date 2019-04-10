package ocs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/protobuf"

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

func (ocs CreateOCS) CheckOCSSignature(sig []byte, X OCSID) error {
	// TODO: test signature
	return nil
	if sig == nil {
		return errors.New("no signature given")
	}
	hash := sha256.New()
	X.MarshalTo(hash)
	buf, err := protobuf.Encode(ocs)
	if err != nil {
		return erret(err)
	}
	hash.Write(buf)
	return erret(schnorr.Verify(cothority.Suite, ocs.Roster.Aggregate, hash.Sum(nil), sig))
}

func (re Reshare) verify() error {
	return errors.New("not yet implemented")
}

func (p Policy) verify(r onet.Roster) error {
	if p.X509Cert != nil {
		return p.X509Cert.verify(r)
	}
	if p.ByzCoin != nil {
		return p.ByzCoin.verify(r)
	}
	return errors.New("need to have a policy for X509 or ByzCoin")
}

func (px PolicyX509Cert) verify(r onet.Roster) error {
	// TODO: decide how to make sure the policy fits the reencryption / resharing
	return nil
}

func (px PolicyByzCoin) verify(r onet.Roster) error {
	return erret(errors.New("net yet implemented"))
}

func (ar AuthReencrypt) verify(X OCSID) error {
	return nil
	return erret(errors.New("not yet implemented"))
}

func (ar AuthReencrypt) Xc() (kyber.Point, error) {
	// TODO: takes this from the AuthReencr(X509|ByzCoin)
	return ar.Ephemeral, nil
}

func (ar AuthReencrypt) C() (kyber.Point, error) {
	if ar.X509Cert != nil {
		return ar.X509Cert.Secret, nil
	}
	if ar.ByzCoin != nil {
		return nil, errors.New("can't get secret from ByzCoin yet")
	}
	return nil, errors.New("need to have authentication for X509 or ByzCoin")
}

func erret(err error) error {
	if err == nil {
		return nil
	}
	pc, _, line, _ := runtime.Caller(1)
	errStr := err.Error()
	if strings.HasPrefix(errStr, "erret") {
		errStr += "\n\t"
	}
	return fmt.Errorf("erret at %s: %d -> %s", runtime.FuncForPC(pc).Name(), line, errStr)
}
