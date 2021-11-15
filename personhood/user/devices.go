package user

import (
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// SwitchKey changes the key to a new random key. It updates the first
// device with the current signer in the evolution expression, then it
// chooses a random new signer, and sends the transaction to byzcoin.
// This method returns once the transaction has been accepted, or if there
// was an error.
func (u *User) SwitchKey() error {
	// Search a device with the current signer
	var device *darc.Darc
	darcs, err := u.GetDevices()
	if err != nil {
		return xerrors.Errorf("couldn't get devices: %v", err)
	}
	exp := u.Signer.Identity().String()
	for d := range darcs {
		if string(darcs[d].Rules.Get(byzcoin.ContractDarcInvokeEvolve)) == exp {
			device = darcs[d].Copy()
		}
	}
	if device == nil {
		return xerrors.New("didn't find any device darc with existing signer")
	}

	// Create a new darc that evolves from the found one,
	// and put the new signer in all relevant rules.
	if err := device.EvolveFrom(device.Copy()); err != nil {
		return xerrors.Errorf("couldn't evolve: %v", err)
	}
	newSigner := darc.NewSignerEd25519(nil, nil)
	newExp := expression.Expr(newSigner.Identity().String())
	if err := device.Rules.UpdateSign(newExp); err != nil {
		return xerrors.Errorf("couldn't update signing expression: %v", err)
	}
	if err := device.Rules.UpdateRule(byzcoin.ContractDarcInvokeEvolve,
		newExp); err != nil {
		return xerrors.Errorf("couldn't update darc evolution expression: %v",
			err)
	}

	// Create a transaction and send it to byzcoin.
	inst, err := byzcoin.ContractDarcEvolveInstruction(*device)
	if err != nil {
		return xerrors.Errorf("couldn't create instruction: %v", err)
	}
	ctx, err := u.cl.CreateTransaction(inst)
	if err != nil {
		return xerrors.Errorf("couldn't create transaction: %v", err)
	}
	if err := u.cl.SignTransaction(ctx, u.Signer); err != nil {
		return xerrors.Errorf("couldn't sign with current signer: %v", err)
	}
	if _, err := u.cl.AddTransactionAndWait(ctx, 10); err != nil {
		return xerrors.Errorf("error while waiting for transaction to finish"+
			": %v", err)
	}

	// If all is successful, update the current signer.
	u.Signer = newSigner

	return nil
}

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
	u.Signer = darc.NewSignerEd25519(oldPublic, oldSecret)
	return u.SwitchKey()
}

// AddDevice creates a new device with an ephemeral private key for that user.
// It also adds the necessary sign and evolve rules to the signer darc.
func (u *User) AddDevice(url, name string) (string, error) {
	signer := darc.NewSignerEd25519(nil, nil)
	signerID := signer.Identity()
	rules := darc.InitRulesWith(
		[]darc.Identity{signerID}, []darc.Identity{signer.Identity()},
		darc.Action(byzcoin.ContractDarcInvokeEvolve))
	newDarc := darc.NewDarc(rules, []byte(name))
	as := u.Spawner.Start(u.CoinID, u.Signer)
	if err := as.SpawnDarc(*newDarc); err != nil {
		return "", xerrors.Errorf("couldn't create spawnDarc instruction: %v",
			err)
	}
	ndIDStr := darc.NewIdentityDarc(newDarc.GetBaseID()).String()

	newCredDarc := u.SignerDarc.Copy()
	if err := newCredDarc.EvolveFrom(&u.SignerDarc); err != nil {
		return "", xerrors.Errorf("couldn't evolve credential darc")
	}
	sign := newCredDarc.Rules.GetSignExpr().AddOrElement(ndIDStr)
	if err := newCredDarc.Rules.UpdateSign(sign); err != nil {
		return "", xerrors.Errorf("couldn't update signing action: %v", err)
	}
	darcEvolve := newCredDarc.Rules.Get(byzcoin.ContractDarcInvokeEvolve).
		AddOrElement(ndIDStr)
	if err := newCredDarc.Rules.UpdateRule(byzcoin.ContractDarcInvokeEvolve,
		darcEvolve); err != nil {
		return "", xerrors.Errorf("couldn't update darc evolve action: %v", err)
	}

	newCredDarcBuf, err := newCredDarc.ToProto()
	if err != nil {
		return "", xerrors.Errorf("couldn't convert darc to protobuf: %v", err)
	}
	as.AddInstruction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(newCredDarc.GetBaseID()),
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDarcID,
			Command:    "evolve",
			Args: byzcoin.Arguments{
				{Name: "darc", Value: newCredDarcBuf},
			},
		},
	}, 0)

	devices := u.credStruct.Get(contracts.CEDevices).Attributes
	devices = append(devices, contracts.Attribute{
		Name:  contracts.AttributeString(name),
		Value: newDarc.GetBaseID()})
	u.credStruct.Set(contracts.CEDevices, devices)
	credBuf, err := protobuf.Encode(&u.credStruct)
	if err != nil {
		return "", xerrors.Errorf("couldn't encode credential-struct: %v", err)
	}
	as.AddInstruction(byzcoin.Instruction{
		InstanceID: u.CredIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCredentialID,
			Command:    "update",
			Args: byzcoin.Arguments{
				{Name: "credential", Value: credBuf},
			},
		},
	}, 0)

	if err := as.SendTransaction(); err != nil {
		return "", xerrors.Errorf("couldn't send transaction: %v", err)
	}

	ephBuf, err := signer.Ed25519.Secret.MarshalBinary()
	if err != nil {
		return "", xerrors.Errorf("couldn't marshal private key: %v", err)
	}
	return fmt.Sprintf("%s?credentialIID=%s&ephemeral=%s", url,
		hex.EncodeToString(u.CredIID[:]),
		hex.EncodeToString(ephBuf)), nil
}
