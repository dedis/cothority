package user

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	contracts2 "go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// Builder allows to create a new user either directly from a DARC with
// the appropriate permissions, or from a Spawner contract.
type Builder struct {
	credentialStruct contracts.CredentialStruct
	darcDevice       *darc.Darc
	darcSigner       *darc.Darc
	darcCoin         *darc.Darc
	darcCred         *darc.Darc
	alias            string
	coinID           byzcoin.InstanceID
	spawnerID        byzcoin.InstanceID
	signer           darc.Signer
	UserID           byzcoin.InstanceID
}

// NewUserBuilder creates a new Builder.
func NewUserBuilder(alias string) (*Builder, error) {
	ub := &Builder{
		alias:            alias,
		signer:           darc.NewSignerEd25519(nil, nil),
		credentialStruct: contracts.CredentialStruct{},
	}
	ub.SetAlias(alias)
	ub.credentialStruct.SetConfig(contracts.ACView, []byte("c4dt_user"))
	return ub, nil
}

// CreateFromDarc creates a new user from a darc that has the necessary
// rules to create all elements.
func (ub *Builder) CreateFromDarc(cl *byzcoin.Client, spawnerDarcID darc.ID,
	spawnerSigner darc.Signer) (*User, error) {
	if err := ub.createDarcs(); err != nil {
		return nil, xerrors.Errorf("creating darcs: %v", err)
	}
	instrs, err := ub.instrsFromDarc(spawnerDarcID)
	if err != nil {
		return nil, xerrors.Errorf("couldn't create instructions: %v", err)
	}
	instrs = append(instrs, ub.createCoin(spawnerDarcID)...)

	return ub.createUser(cl, instrs, spawnerSigner)
}

// CreateFromSpawner creates a new user from a spawner instance.
func (ub *Builder) CreateFromSpawner(spawner ActiveSpawner) (*User, error) {
	ub.SetSpawnerID(spawner.iid)
	if err := ub.createDarcs(); err != nil {
		return nil, xerrors.Errorf("creating darcs: %v", err)
	}
	if err := spawner.SpawnDarcs(*ub.darcCoin, *ub.darcCred, *ub.darcSigner,
		*ub.darcDevice); err != nil {
		return nil, xerrors.Errorf("couldn't spawn darcs: %v", err)
	}

	coinIID, err := spawner.SpawnCoin(contracts.SpawnerCoin,
		ub.darcCoin.GetBaseID(), 2000)
	if err != nil {
		return nil, xerrors.Errorf("couldn't spawn coin: %v", err)
	}
	ub.SetCoinID(coinIID)

	credIID, err := spawner.SpawnCredential(ub.credentialStruct,
		ub.darcCred.GetBaseID())
	if err != nil {
		return nil, xerrors.Errorf("couldn't spawn credential: %v", err)
	}
	ub.UserID = credIID

	if err := spawner.SendTransaction(); err != nil {
		return nil, xerrors.Errorf("couldn't send transaction: %v", err)
	}

	u, err := New(spawner.cl, ub.UserID)
	if err != nil {
		return nil, xerrors.Errorf("couldn't get finished user: %v", err)
	}
	u.Signer = ub.signer

	return &u, nil
}

// createUser creates a new user from the instructions.
func (ub *Builder) createUser(cl *byzcoin.Client,
	instrs byzcoin.Instructions, spawnerSigner darc.Signer) (*User, error) {
	ctx, err := cl.CreateTransaction(instrs...)
	if err != nil {
		return nil, xerrors.Errorf("while creating transaction: %v", err)
	}
	if err := cl.SignTransaction(ctx, spawnerSigner); err != nil {
		return nil, xerrors.Errorf("while signing: %v", err)
	}
	if _, err := cl.AddTransactionAndWait(ctx, 10); err != nil {
		return nil, xerrors.Errorf("sending transaction: %v", err)
	}
	u, err := New(cl, ub.UserID)
	if err != nil {
		return nil, xerrors.Errorf("while fetching final credential: %v", err)
	}
	u.Signer = ub.signer
	return &u, nil
}

// createDarcs creates the darcs for the user.
func (ub *Builder) createDarcs() error {
	// Create DARCs
	signerID := ub.signer.Identity()
	signerIDs := []darc.Identity{signerID}
	deviceRules := darc.InitRulesWith(signerIDs, signerIDs, byzcoin.ContractDarcInvokeEvolve)
	ub.darcDevice = darc.NewDarc(deviceRules, []byte("Initial Device"))
	deviceDarcID := darc.NewIdentityDarc(ub.darcDevice.GetBaseID())

	devicesExp := expression.Expr(deviceDarcID.String())
	signerRules := darc.NewRules()
	if err := signerRules.AddRule("_sign", devicesExp); err != nil {
		return xerrors.Errorf("while updating sign: %v", err)
	}
	for _, recovery := range ub.credentialStruct.Get(contracts.CERecoveries).
		Attributes {
		devicesExp = devicesExp.AddOrElement(
			darc.NewIdentityDarc(recovery.Value).String())
	}
	if err := signerRules.AddRule(byzcoin.ContractDarcInvokeEvolve,
		devicesExp); err != nil {
		return xerrors.Errorf("while updating rule: %v", err)
	}
	ub.darcSigner = darc.NewDarc(signerRules, []byte("Signer for Credential"))
	signerDarcID := darc.NewIdentityDarc(ub.darcSigner.GetBaseID())
	signerDarcIDs := []darc.Identity{signerDarcID}

	credRules := darc.InitRulesWith(signerDarcIDs, signerDarcIDs, byzcoin.ContractDarcInvokeEvolve)
	if err := credRules.AddRule(darc.Action("invoke:credential.update"),
		expression.Expr(signerDarcID.String())); err != nil {
		return xerrors.Errorf("couldn't add darc rule for credential update"+
			": %v", err)
	}
	ub.darcCred = darc.NewDarc(credRules, []byte("User "+ub.alias))

	coinRules := darc.InitRulesWith(signerDarcIDs, signerDarcIDs, byzcoin.ContractDarcInvokeEvolve)
	for _, rule := range []string{"transfer", "store", "fetch"} {
		action := darc.Action("invoke:coin." + rule)
		if err := coinRules.AddRule(action,
			expression.Expr(signerDarcID.String())); err != nil {
			return xerrors.Errorf("couldn't add darc.evolve rule: %v", err)
		}
	}
	ub.darcCoin = darc.NewDarc(coinRules, []byte("CoinDarc for "+ub.alias))

	devices := ub.credentialStruct.GetDevices()
	devices["Initial"] = ub.darcDevice.GetBaseID()
	ub.credentialStruct.SetDevices(devices)

	return nil
}

// instrsFromDarc returns the instructions to create all instances from a
// darc with the appropriate spawn-rules.
// It creates the following instructions for a new user credential:
//   1. a new spawner contract
//   2. all DARCs: device, signer, credential, coin
//   3. the coin
//   4. the credential itself
// The caller has to put the instructions in a ClientTransaction, sign it,
// and send it to Byzcoin.
func (ub *Builder) instrsFromDarc(spawnerDarcID darc.ID) (instrs byzcoin.
	Instructions, err error) {
	spdID := byzcoin.NewInstanceID(spawnerDarcID)

	cost100 := byzcoin.Coin{Name: contracts.SpawnerCoin, Value: 100}
	cost100buf, err := protobuf.Encode(&cost100)
	if err != nil {
		return nil, xerrors.Errorf("couldn't encode coin: %v", err)
	}
	cost500 := byzcoin.Coin{Name: contracts.SpawnerCoin, Value: 500}
	cost500buf, err := protobuf.Encode(&cost500)
	if err != nil {
		return nil, xerrors.Errorf("couldn't encode coin: %v", err)
	}
	spawnerInst := byzcoin.Instruction{
		InstanceID: spdID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractSpawnerID,
			Args: byzcoin.Arguments{
				{Name: "preID", Value: random.Bits(256, true, random.New())},
				{Name: "costDarc", Value: cost100buf},
				{Name: "costCoin", Value: cost100buf},
				{Name: "costCredential", Value: cost500buf},
				{Name: "costParty", Value: cost500buf},
				{Name: "costRoPaSci", Value: cost100buf},
				{Name: "costCWrite", Value: cost500buf},
				{Name: "costCRead", Value: cost100buf},
				{Name: "costValue", Value: cost100buf},
			},
		},
	}
	spawnerID, err := spawnerInst.DeriveIDArg("", "preID")
	if err != nil {
		return instrs, xerrors.Errorf("couldn't get spawnerID: %v", err)
	}
	instrs = append(instrs, spawnerInst)
	ub.SetSpawnerID(spawnerID)

	darcInstrs, err := byzcoin.ContractDarcSpawnInstructions(spawnerDarcID,
		*ub.darcSigner, *ub.darcDevice, *ub.darcCred, *ub.darcCoin)
	if err != nil {
		return instrs, xerrors.Errorf(
			"creating device and credential spawn instruction: %v", err)
	}
	instrs = append(instrs, darcInstrs...)

	// Append a coin
	coinInstr := byzcoin.Instruction{
		InstanceID: spdID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts2.ContractCoinID,
			Args: byzcoin.Arguments{
				{Name: "coinID", Value: random.Bits(256, true, random.New())},
				{Name: "darcID", Value: ub.darcCoin.GetBaseID()},
				{Name: "type", Value: contracts.SpawnerCoin[:]},
			},
		},
	}
	coinID, err := coinInstr.DeriveIDArg("", "coinID")
	if err != nil {
		return instrs, xerrors.Errorf("couldn't derive coinID: %v", err)
	}
	instrs = append(instrs, coinInstr)
	ub.SetCoinID(coinID)

	// Create credential
	credBuf, err := protobuf.Encode(&ub.credentialStruct)
	if err != nil {
		return instrs, xerrors.Errorf("while encoding credentialStruct: %v", err)
	}
	userCredID := random.Bits(256, true, random.New())
	credInstr := byzcoin.Instruction{
		InstanceID: spdID,
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractCredentialID,
			Args: byzcoin.Arguments{{
				Name:  "darcIDBuf",
				Value: ub.darcCred.GetBaseID(),
			}, {
				Name:  "credential",
				Value: credBuf,
			}, {
				Name:  "credentialID",
				Value: userCredID,
			}},
		},
	}
	ub.UserID = contracts.CredentialIID(userCredID)
	instrs = append(instrs, credInstr)

	return
}

// createCoin is used for a new user from a DARC,
// so that the new user can have some coins to start other transactions.
func (ub *Builder) createCoin(spawnerDarc darc.ID) byzcoin.Instructions {
	coinInstr, coinID := contracts2.ContractCoinSpawn(spawnerDarc,
		&contracts.SpawnerCoin)
	coinMintInstr := contracts2.ContractCoinMint(coinID, 1e9)
	coinTransferInstr := contracts2.ContractCoinTransfer(coinID, ub.coinID, 1e9)
	return byzcoin.Instructions{coinInstr, coinMintInstr, coinTransferInstr}
}

// SetAlias sets the alias of the user in the credential.
func (ub *Builder) SetAlias(alias string) {
	ub.alias = alias
	ub.credentialStruct.SetPublic(contracts.APAlias, []byte(alias))
}

// SetCoinID sets the coinID of the user in the credential.
func (ub *Builder) SetCoinID(coinID byzcoin.InstanceID) {
	ub.coinID = coinID
	ub.credentialStruct.SetPublic(contracts.APCoinID, coinID[:])
}

// SetSpawnerID sets the spawnerID of the user in the credential.
func (ub *Builder) SetSpawnerID(spawnerID byzcoin.InstanceID) {
	ub.spawnerID = spawnerID
	ub.credentialStruct.SetConfig(contracts.ACSpawner, spawnerID[:])
}

// SetEmail sets the email of the user in the credential.
func (ub *Builder) SetEmail(email string) {
	ub.credentialStruct.SetPublic(contracts.APEmail, []byte(email))
}
