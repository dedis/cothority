package user

import (
	"fmt"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"strings"
)

// The User structure represents a Dynacred user as defined in the document
// here: https://www.c4dt.org/wp-content/uploads/2020/09/DynaCred-101.pdf
// It is a minimal implementation used for the bcadmin command, but can be
// extended to more functionality, if needed.
type User struct {
	cl         *byzcoin.Client
	credStruct contracts.CredentialStruct
	// Signer is nil by default. It can be set by the user, so that more
	// actions are available.
	Signer   darc.Signer
	CredIID  byzcoin.InstanceID
	CredDarc darc.Darc
}

// New returns an initialized user.
// It looks the credential instance up on ByzCoin and returns an error if
// something went wrong: user not existing, wrong type of contract.
func New(cl *byzcoin.Client, credIID byzcoin.InstanceID) (u User, err error) {
	resp, err := cl.GetProof(credIID[:])
	if err != nil {
		err = xerrors.Errorf("while getting proof for credential instance: %v",
			err)
		return
	}
	buf, cid, did, err := resp.Proof.Get(credIID[:])
	if err != nil {
		err = xerrors.Errorf("reading proof: %v", err)
		return
	}
	if cid != contracts.ContractCredentialID {
		err = xerrors.Errorf("credIID points to wrong contract: %s instead of"+
			" %s", cid, contracts.ContractCredentialID)
		return
	}
	if err = protobuf.Decode(buf, &u.credStruct); err != nil {
		err = xerrors.Errorf("while decoding CredentialStruct: %v", err)
		return
	}

	resp, err = cl.GetProof(did)
	if err != nil {
		return u, xerrors.Errorf("while getting proof for credential-DARC: %v"+
			"", err)
	}
	buf, cid, _, err = resp.Proof.Get(did)
	if err != nil {
		return u, xerrors.Errorf("reading proof for credential-DARC: %v", err)
	}
	if cid != byzcoin.ContractDarcID {
		return u, xerrors.Errorf("wrong contract: %s instead of %s", cid, byzcoin.ContractDarcID)
	}
	d, err := darc.NewFromProtobuf(buf)
	if err != nil {
		return u, xerrors.Errorf("couldn't decode credential-DARC: %v", err)
	}

	u.CredDarc = *d
	u.CredIID = credIID
	u.cl = cl
	return
}

// NewFromByzcoin creates a new user credential,
// given a darc that is allowed to spawn any contract.
// It choses a random private key and returns the correctly filled
// out user structure.
func NewFromByzcoin(cl *byzcoin.Client, spawnerDarcID darc.ID,
	spawnerSigner darc.Signer, name string) (u User, err error) {
	if err := cl.UseNode(1); err != nil {
		return u, xerrors.Errorf("couldn't set UseNode: %v", err)
	}
	credSigner := darc.NewSignerEd25519(nil, nil)
	instrs, err := CreateInstructionsSpawnFromDarc(spawnerDarcID, credSigner,
		name)
	if err != nil {
		return u, xerrors.Errorf("while creating instructions: %v", err)
	}
	ctx, err := cl.CreateTransaction(instrs...)
	if err != nil {
		return u, xerrors.Errorf("while creating transaction: %v", err)
	}
	if err := cl.SignTransaction(ctx, spawnerSigner); err != nil {
		return u, xerrors.Errorf("while signing: %v", err)
	}
	if _, err := cl.AddTransactionAndWait(ctx, 10); err != nil {
		return u, xerrors.Errorf("sending transaction: %v", err)
	}
	u, err = New(cl, ctx.Instructions[2].DeriveID(""))
	if err != nil {
		return u, xerrors.Errorf("while fetching final credential: %v", err)
	}
	u.Signer = credSigner
	return
}

// CreateInstructionsSpawnFromDarc creates 3 instructions for a new
// user credential:
//   1. the device DARC
//   2. the credential DARC
//   3. the credential itself
// The caller has to put the instructions in a ClientTransaction, sign it,
// and send it to Byzcoin.
func CreateInstructionsSpawnFromDarc(spawnerDarcID darc.ID,
	device darc.Signer, name string) (byzcoin.Instructions, error) {
	ids := []darc.Identity{device.Identity()}
	deviceRules := darc.InitRules(ids, ids)
	if err := deviceRules.AddRule(darc.Action("invoke:darc.evolve"),
		expression.Expr(device.Identity().String())); err != nil {
		return nil, xerrors.Errorf("couldn't add darc.evolve rule: %v", err)
	}
	deviceDarc := darc.NewDarc(deviceRules, []byte("Initial Device"))
	credIDs := []darc.Identity{darc.NewIdentityDarc(deviceDarc.GetBaseID())}
	credRules := darc.InitRules(credIDs, credIDs)
	if err := credRules.AddRule(darc.Action("invoke:darc.evolve"),
		expression.Expr(device.Identity().String())); err != nil {
		return nil, xerrors.Errorf("couldn't add darc.evolve rule: %v", err)
	}
	credDarc := darc.NewDarc(credRules, []byte("User "+name))
	credStruct :=
		contracts.CredentialStruct{Credentials: []contracts.Credential{
			{
				Name: string(Public),
				Attributes: []contracts.Attribute{{
					Name:  Alias,
					Value: []byte(name),
				}},
			},
			{
				Name: string(Devices),
				Attributes: []contracts.Attribute{{
					Name:  "Initial",
					Value: deviceDarc.GetBaseID(),
				}},
			},
		}}

	credBuf, err := protobuf.Encode(&credStruct)
	if err != nil {
		return nil, xerrors.Errorf("while encoding credentialStruct: %v", err)
	}
	darcInstrs, err := byzcoin.ContractDarcSpawnInstructions(spawnerDarcID,
		*deviceDarc, *credDarc)
	if err != nil {
		return nil, xerrors.Errorf(
			"creating device and credential spawn instruction: %v", err)
	}
	return append(darcInstrs,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(spawnerDarcID),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractCredentialID,
				Args: byzcoin.Arguments{{
					Name:  "darcIDBuf",
					Value: credDarc.GetBaseID(),
				}, {
					Name:  "credential",
					Value: credBuf,
				}},
			},
		}), nil
}

// GetCredentialsCopy returns a copy of all credentials
func (u User) GetCredentialsCopy() (cs contracts.CredentialStruct, err error) {
	buf, err := protobuf.Encode(&u.credStruct)
	if err != nil {
		return cs, xerrors.Errorf("while encoding credentialStruct: %v", err)
	}
	err = protobuf.Decode(buf, &cs)
	return
}

// GetCredential searches all credentials for the one matching the given
// name. If no match is found, it returns an empty slice.
func (u User) GetCredential(name CredentialEntry) []contracts.Attribute {
	for _, cred := range u.credStruct.Credentials {
		if cred.Name == string(name) {
			return cred.Attributes
		}
	}
	return nil
}

// GetPublic returns all public attributes of this user.
func (u User) GetPublic() []contracts.Attribute {
	return u.GetCredential(Public)
}

// GetConfig returns all Config attributes of this user.
func (u User) GetConfig() []contracts.Attribute {
	return u.GetCredential(Config)
}

// getAttributeDarcs returns all attributes of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) getAttributeDarcs(name CredentialEntry) (map[string]darc.Darc, error) {
	if name != Devices && name != Recoveries {
		return nil, xerrors.New("can only do Devices and Recoveries")
	}
	darcIDs := u.GetCredential(name)

	darcs := make(map[string]darc.Darc)
	for _, id := range darcIDs {
		log.Lvlf2("Fetching device-DARC %x", id.Value)
		resp, err := u.cl.GetProof(id.Value)
		if err != nil {
			return nil, xerrors.Errorf("couldn't get proof: %v", err)
		}
		buf, cid, _, err := resp.Proof.Get(id.Value)
		if err != nil {
			log.Warnf("While scanning users devices: %+v", err)
			continue
		}
		if cid != byzcoin.ContractDarcID {
			log.Warnf("Found invalid device: %s instead of %s", cid,
				byzcoin.ContractDarcID)
			continue
		}
		var d darc.Darc
		if err := protobuf.Decode(buf, &d); err != nil {
			log.Warnf("Couldn't decode darc: %+v", err)
			continue
		}
		darcs[id.Name] = d
	}

	return darcs, nil
}

// GetDevices returns all devices of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) GetDevices() (map[string]darc.Darc, error) {
	return u.getAttributeDarcs(Devices)
}

// GetRecoveries returns all recoveries of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) GetRecoveries() (map[string]darc.Darc, error) {
	return u.getAttributeDarcs(Recoveries)
}

// GetCalypso returns all Calypso attributes of this user.
func (u User) GetCalypso() []contracts.Attribute {
	return u.GetCredential(Calypso)
}

func (u User) String() string {
	var creds []string
	creds = append(creds, "Public:")
	for _, att := range u.GetPublic() {
		creds = append(creds, fmt.Sprintf("  - %s: %s", att.Name,
			string(att.Value)))
	}
	creds = append(creds, "Config:")
	for _, att := range u.GetConfig() {
		creds = append(creds, fmt.Sprintf("  - %s: %s", att.Name,
			string(att.Value)))
	}
	creds = append(creds, "Devices:")
	devs, err := u.GetDevices()
	if err != nil {
		creds = append(creds,
			fmt.Sprintf("  ERROR while fetching devices: %+v", err))
	} else {
		for name, d := range devs {
			creds = append(creds, fmt.Sprintf("  - %s: %x", name,
				d.GetBaseID()))
		}
	}
	creds = append(creds, "Recoveries:")
	recoveries, err := u.GetRecoveries()
	if err != nil {
		creds = append(creds,
			fmt.Sprintf("  ERROR while fetching recoveries: %+v", err))
	} else {
		for name, d := range recoveries {
			creds = append(creds, fmt.Sprintf("  - %s: %x", name,
				d.GetBaseID()))
		}
	}
	return fmt.Sprintf("User credential:\nInstanceID: %s\nCredential-DARC: %s"+
		"\nCredentials:\n%s", u.CredIID, u.CredDarc, strings.Join(creds, "\n"))
}
