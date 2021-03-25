package user

import (
	"fmt"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

// DeviceSwitchKey searches the user for the device with the rule
// corresponding to the public key of oldSecret.
// This can represent the scalar created by the web-frontend for a new
// device, but it can also be used to switch to a new key.
// If the device is found, a random key is generated, and the old one is
// replaced with the new one.
// The new key is stored in the User struct.
// If the device cannot be found, an error is returned.
func (u *User) DeviceSwitchKey(oldSecret kyber.Scalar) error {
	oldPublic := cothority.Suite.Point()
	oldPublic.Mul(oldSecret, nil)
	oldSigner := darc.NewSignerEd25519(oldPublic, oldSecret)

	// Search for the old key in the device-darcs
	log.Lvl2("Getting all devices")
	devices, err := u.GetDevices()
	if err != nil {
		return xerrors.Errorf("while getting devices: %v", err)
	}
	signer := darc.NewSignerEd25519(nil, nil)
	for name, device := range devices {
		ctx, err := u.updateDevice(device, oldSigner.Identity().String(),
			signer.Ed25519.Point)
		if err != nil {
			return xerrors.Errorf("searching for device: %v", err)
		}
		if ctx != nil {
			log.Lvlf2("Found device %s to match old key, switching key",
				name)
			if err := u.cl.SignTransaction(*ctx, oldSigner); err != nil {
				return xerrors.Errorf("signing transaction: %v", err)
			}
			if _, err = u.cl.AddTransactionAndWait(*ctx, 10); err != nil {
				return xerrors.Errorf("waiting for update: %v", err)
			}
			u.Signer = signer
			return nil
		}
	}

	return xerrors.New("didn't find this old key")
}

// updateDevice checks if the oldRule matches the given device. If
// the device matches, a ClientTransaction is prepared using the newPublic
// key in the rule. The caller must sign the ClientTransaction.
// An error returned from the updateDevice is stopping,
// as it means that the device matched, but that something went wrong.
func (u User) updateDevice(device darc.Darc, oldRule string,
	newPublic kyber.Point) (*byzcoin.ClientTransaction, error) {
	action := device.Rules.GetSignExpr()
	if string(action) != oldRule {
		return nil, nil
	}

	// Create evolved Darc
	newDarc := device.Copy()
	if err := newDarc.EvolveFrom(&device); err != nil {
		return nil, xerrors.Errorf("evolving new device: %v", err)
	}
	if err := newDarc.Rules.UpdateSign(
		expression.Expr(fmt.Sprintf("ed25519:%s",
			newPublic.String()))); err != nil {
		return nil, xerrors.Errorf("couldn't update rules: %v", err)
	}

	// Create ClientTransaction
	newDarcBuf, err := newDarc.ToProto()
	if err != nil {
		return nil, xerrors.Errorf("while encoding darc: %v", err)
	}
	ctx, err := u.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(device.GetBaseID()),
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDarcID,
			Command:    "evolve",
			Args: byzcoin.Arguments{{
				Name:  "darc",
				Value: newDarcBuf,
			}},
		},
	})
	if err != nil {
		return nil, xerrors.Errorf("new device transaction: %v", err)
	}
	return &ctx, nil
}
