package user

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"net/url"
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
	Signer     darc.Signer
	CredIID    byzcoin.InstanceID
	SignerDarc darc.Darc
	CoinID     byzcoin.InstanceID
	Spawner    Spawner
}

// New returns an initialized user.
// It looks the credential instance up on ByzCoin and returns an error if
// something went wrong: user not existing, wrong type of contract.
func New(cl *byzcoin.Client, credIID byzcoin.InstanceID) (u User, err error) {
	did, err := cl.GetInstance(credIID, contracts.ContractCredentialID,
		&u.credStruct)
	if err != nil {
		return u, xerrors.Errorf("while getting proof for credential instance"+
			": %v", err)
	}

	var credDarc darc.Darc
	if _, err := cl.GetInstance(byzcoin.NewInstanceID(did),
		byzcoin.ContractDarcID, &credDarc); err != nil {
		return u, xerrors.Errorf("while getting proof for credential darc: %v", err)
	}
	ids := strings.Split(string(credDarc.Rules.GetSignExpr()), "|")
	idSignerStr := strings.TrimPrefix(strings.TrimSpace(ids[0]), "darc:")
	idSignerID, err := hex.DecodeString(idSignerStr)
	if err != nil {
		return u, xerrors.Errorf("couldn't get id for signer darc: %v", err)
	}

	if _, err := cl.GetInstance(byzcoin.NewInstanceID(idSignerID),
		byzcoin.ContractDarcID, &u.SignerDarc); err != nil {
		return u, xerrors.Errorf("while getting proof for signer darc: %v", err)
	}

	u.CredIID = credIID
	u.CoinID = byzcoin.NewInstanceID(u.credStruct.GetPublic(contracts.APCoinID))
	spawnerID := u.credStruct.GetConfig(contracts.ACSpawner)
	if len(spawnerID) != 32 {
		return u, xerrors.New("couldn't get spawnerID from user")
	}
	sp, err := NewSpawner(cl, byzcoin.NewInstanceID(spawnerID))
	if err != nil {
		return u, xerrors.Errorf("couldn't get spawner: %v", err)
	}
	u.Spawner = sp
	u.cl = cl
	return
}

// NewFromByzcoin creates a new user credential,
// given a darc that is allowed to spawn any contract.
// It chooses a random private key and returns the correctly filled
// out user structure.
func NewFromByzcoin(cl *byzcoin.Client, spawnerDarcID darc.ID,
	spawnerSigner darc.Signer, name string) (*User, error) {
	ub, err := NewUserBuilder(name)
	if err != nil {
		return nil, xerrors.Errorf("couldn't create user builder: %v", err)
	}
	return ub.CreateFromDarc(cl, spawnerDarcID, spawnerSigner)
}

// NewFromURL uses the given url to create a new user by spawning a new
// device. This can only be used once for a given url, as it contains an
// ephemeral private key.
func NewFromURL(cl *byzcoin.Client, rawURL string) (u User, err error) {
	userURL, err := url.Parse(rawURL)
	if err != nil {
		return u, xerrors.Errorf("while parsing url: %v", err)
	}

	cred, err := hex.DecodeString(userURL.Query().Get("credentialIID"))
	if err != nil {
		return u, xerrors.Errorf("couldn't get credentialIID")
	}
	if len(cred) != 32 {
		return u, xerrors.New("credentialIID is not of length 32 bytes")
	}
	credIID := byzcoin.NewInstanceID(cred)

	ephemeral, err := hex.DecodeString(userURL.Query().Get("ephemeral"))
	if err != nil {
		return u, xerrors.Errorf("couldn't get ephemeral key from URL: %v", err)
	}
	if len(ephemeral) != 32 {
		return u, xerrors.New("ephemeral key is not of length 32 bytes")
	}
	ephemeralPrivate := cothority.Suite.Scalar()
	if err = ephemeralPrivate.UnmarshalBinary(ephemeral); err != nil {
		return u, xerrors.Errorf("while getting private key: %v", err)
	}
	ephemeralPublic := cothority.Suite.Point().Mul(ephemeralPrivate, nil)
	signer := darc.NewSignerEd25519(ephemeralPublic, ephemeralPrivate)
	u, err = New(cl, credIID)
	if err != nil {
		return u, xerrors.Errorf("couldn't get user: %v", err)
	}
	u.Signer = signer

	return u, u.SwitchKey()
}

// GetCredentialsCopy returns a copy of all credentials
func (u User) GetCredentialsCopy() contracts.CredentialStruct {
	return u.credStruct.Clone()
}

// GetCredential searches all credentials for the one matching the given
// name. If no match is found, it returns an empty slice.
func (u User) GetCredential(name contracts.CredentialEntry) []contracts.Attribute {
	return u.credStruct.Get(name).Attributes
}

// GetPublic returns all public attributes of this user.
func (u User) GetPublic() []contracts.Attribute {
	return u.GetCredential(contracts.CEPublic)
}

// GetConfig returns all Config attributes of this user.
func (u User) GetConfig() []contracts.Attribute {
	return u.GetCredential(contracts.CEConfig)
}

// getAttributeDarcs returns all attributes of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) getAttributeDarcs(name contracts.CredentialEntry) (map[string]darc.Darc, error) {
	if name != contracts.CEDevices && name != contracts.CERecoveries {
		return nil, xerrors.New("can only do Devices and Recoveries")
	}
	darcIDs := u.GetCredential(name)

	darcs := make(map[string]darc.Darc)
	for _, id := range darcIDs {
		log.Lvlf2("Fetching device-DARC %x", id.Value)
		var d darc.Darc
		if _, err := u.cl.GetInstance(byzcoin.NewInstanceID(id.Value), byzcoin.ContractDarcID, &d); err != nil {
			log.Errorf("Couldn't get device-DARC: %v", err)
			continue
		}
		darcs[string(id.Name)] = d
	}

	return darcs, nil
}

// GetDevices returns all devices of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) GetDevices() (map[string]darc.Darc, error) {
	return u.getAttributeDarcs(contracts.CEDevices)
}

// GetRecoveries returns all recoveries of this user as Darcs.
// It does this by querying ByzCoin for all devices.
// The method only returns an error if it couldn't get the device-list from
// the User, or if there was a network error.
// Wrong entries are ignored, but outputted as a warning.
func (u User) GetRecoveries() (map[string]darc.Darc, error) {
	return u.getAttributeDarcs(contracts.CERecoveries)
}

// GetCalypso returns all Calypso attributes of this user.
func (u User) GetCalypso() []contracts.Attribute {
	return u.GetCredential(contracts.CECalypso)
}

// CreateNewUser creates a new user with the given alias and email.
// It also adds this user to the contacts and sets the same recovery
// as the user.
func (u User) CreateNewUser(alias, email string) (*User, error) {
	ub, err := NewUserBuilder(alias)
	ub.credentialStruct.SetContacts([]byzcoin.InstanceID{u.CredIID})
	ub.credentialStruct.SetRecoveries(map[string]byzcoin.
		InstanceID{"Email Recovery": byzcoin.NewInstanceID(u.SignerDarc.GetBaseID())})
	ub.SetEmail(email)
	newUser, err := ub.CreateFromSpawner(u.getActiveSpawner())
	if err != nil {
		return nil, xerrors.Errorf("couldn't create user: %v", err)
	}
	contacts := u.credStruct.GetPublic(contracts.APContacts)
	contacts = append(contacts, newUser.CredIID[:]...)
	u.credStruct.SetPublic(contracts.APContacts, contacts)
	if err := u.SendUpdateCredential(); err != nil {
		return nil, xerrors.Errorf("couldn't update credential: %v", err)
	}

	return newUser, nil
}

// CreateLink returns a URL that can be used to transfer the user to a new
// browser instance.
func (u User) CreateLink(baseURL string) (string, error) {
	rec, err := url.Parse(baseURL)
	if err != nil {
		return "", xerrors.Errorf("couldn't parse baseURL: %v", err)
	}
	priv, err := u.Signer.GetPrivate()
	if err != nil {
		return "", xerrors.Errorf("couldn't get private key: %v", err)
	}
	eph, err := priv.MarshalBinary()
	if err != nil {
		return "", xerrors.Errorf("couldn't marshal private key: %v", err)
	}
	q := rec.Query()
	q.Set("ephemeral", hex.EncodeToString(eph))
	q.Set("credentialIID", hex.EncodeToString(u.CredIID[:]))
	rec.RawQuery = q.Encode()
	return rec.String(), nil
}

// Recover checks if a given user can be recovered by this user. If so,
// a new device is created for the user and the recovery-URL is returned.
func (u User) Recover(other byzcoin.InstanceID, baseURL string) (string, error) {
	otherUser, err := New(u.cl, other)
	if err != nil {
		return "", xerrors.Errorf("couldn't fetch user: %v", err)
	}
	if !u.canRecover(otherUser) {
		return "", xerrors.New("can't recover this user")
	}
	newSigner := darc.NewSignerEd25519(nil, nil)
	if err := u.createRecovery(baseURL, otherUser, newSigner); err != nil {
		return "", xerrors.Errorf("couldn't create recovery: %v", err)
	}

	otherUser.Signer = newSigner
	return otherUser.CreateLink(baseURL)
}

// GetActiveSpawner returns the spawner that is used to create new instances.
func (u User) GetActiveSpawner() ActiveSpawner {
	return u.Spawner.Start(u.CoinID, u.Signer)
}

func (u User) canRecover(otherUser User) bool {
	recoveries := otherUser.credStruct.Get(contracts.CERecoveries)
	for _, reco := range recoveries.Attributes {
		if bytes.Equal(reco.Value, u.SignerDarc.GetBaseID()) {
			return true
		}
	}
	return false
}

func (u User) createRecovery(baseURL string, otherUser User,
	newSigner darc.Signer) error {
	recoveryDeviceIDs := []darc.Identity{newSigner.Identity()}
	recoveryDeviceRules := darc.InitRulesWith(recoveryDeviceIDs, recoveryDeviceIDs, byzcoin.ContractDarcInvokeEvolve)
	alias := string(u.credStruct.GetPublic(contracts.APAlias))
	deviceName := "Recovered from " + alias
	recoveryDevice := darc.NewDarc(recoveryDeviceRules, []byte(deviceName))
	recoveryDeviceIdentity := darc.NewIdentityDarc(recoveryDevice.GetBaseID())

	newUserSigner := otherUser.SignerDarc.Copy()
	if err := newUserSigner.EvolveFrom(&otherUser.SignerDarc); err != nil {
		return xerrors.Errorf("couldn't evolve signer darc: %v", err)
	}
	newExpr := newUserSigner.Rules.GetSignExpr().AddOrElement(
		recoveryDeviceIdentity.String())
	if err := newUserSigner.Rules.UpdateSign(newExpr); err != nil {
		return xerrors.Errorf("couldn't update signer darc: %v", err)
	}
	if err := newUserSigner.Rules.UpdateRule(byzcoin.ContractDarcInvokeEvolve,
		newExpr); err != nil {
		return xerrors.Errorf("couldn't update signer darc: %v", err)
	}
	newUserSignerBuf, err := newUserSigner.ToProto()
	if err != nil {
		return xerrors.Errorf("couldn't create signer darc: %v", err)
	}

	devices := otherUser.credStruct.Get(contracts.CEDevices).Attributes
	devices = append(devices, contracts.Attribute{
		Name:  contracts.AttributeString(deviceName),
		Value: recoveryDevice.GetBaseID(),
	})
	otherUser.credStruct.Set(contracts.CEDevices, devices)
	credStructBuf, err := protobuf.Encode(&otherUser.credStruct)
	if err != nil {
		return xerrors.Errorf("couldn't encode credential struct: %v", err)
	}

	ap := u.getActiveSpawner()
	if err := ap.SpawnDarc(*recoveryDevice); err != nil {
		return xerrors.Errorf("couldn't spawn recovery darc: %v", err)
	}
	ap.AddInstruction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(newUserSigner.GetBaseID()),
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractDarcID,
			Command:    "evolve",
			Args: byzcoin.Arguments{{
				Name:  "darc",
				Value: newUserSignerBuf,
			}},
		},
	}, 0)
	ap.AddInstruction(byzcoin.Instruction{
		InstanceID: otherUser.CredIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCredentialID,
			Command:    "update",
			Args: byzcoin.Arguments{{
				Name:  "credential",
				Value: credStructBuf,
			}},
		},
	}, 0)
	if err := ap.SendTransaction(); err != nil {
		return xerrors.Errorf("couldn't send transaction: %v", err)
	}

	return nil
}

// SendUpdateCredential sends a request to byzcoin to update the credential.
func (u User) SendUpdateCredential() error {
	credBuf, err := protobuf.Encode(&u.credStruct)
	if err != nil {
		return xerrors.Errorf("couldn't encode credential: %v", err)
	}
	ctx, err := u.cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: u.CredIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractCredentialID,
			Command:    "update",
			Args: byzcoin.Arguments{{
				Name:  "credential",
				Value: credBuf,
			}},
		},
	})
	if err != nil {
		return xerrors.Errorf("couldn't create transaction: %v", err)
	}
	if err := u.cl.SignTransaction(ctx, u.Signer); err != nil {
		return xerrors.Errorf("couldn't sign transaction: %v", err)
	}
	_, err = u.cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return xerrors.Errorf("couldn't add transaction: %v", err)
	}
	return nil
}

// UpdateCredential fetches the latest version of the credential from byzcoin.
func (u *User) UpdateCredential() error {
	if _, err := u.cl.GetInstance(u.CredIID, contracts.ContractCredentialID,
		&u.credStruct); err != nil {
		return xerrors.Errorf("couldn't get credential: %v", err)
	}
	return nil
}

func (u User) getActiveSpawner() ActiveSpawner {
	return u.Spawner.Start(u.CoinID, u.Signer)
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
			fmt.Sprintf("ERROR while fetching devices: %+v", err))
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
	return fmt.Sprintf("User credential:\nInstanceID: %s\nSigner-DARC: %s"+
		"\nCredentials:\n%s", u.CredIID, u.SignerDarc, strings.Join(creds, "\n"))
}
