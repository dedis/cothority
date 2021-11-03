package user

import (
	"encoding/binary"
	"go.dedis.ch/cothority/v3/byzcoin"
	contracts2 "go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// Spawner can be used by users to spawn new instances of contracts on the
// chain.
type Spawner struct {
	cl         *byzcoin.Client
	costs      contracts.SpawnerStruct
	InstanceID byzcoin.InstanceID
}

// ActiveSpawner is used to collect multiple instructions in a single
// transaction to be sent to byzcoin.
type ActiveSpawner struct {
	cl           *byzcoin.Client
	iid          byzcoin.InstanceID
	cost         uint64
	instructions []byzcoin.Instruction
	costs        contracts.SpawnerStruct
	coinID       byzcoin.InstanceID
	signer       darc.Signer
}

// NewSpawner returns a new spawner and updates the costs
func NewSpawner(cl *byzcoin.Client, iid byzcoin.InstanceID) (s Spawner,
	err error) {
	repl, err := cl.GetProofFromLatest(iid[:])
	if err != nil {
		return s, xerrors.Errorf("while getting spwaner instance: %v", err)
	}
	_, v, cid, _, err := repl.Proof.KeyValue()
	if err != nil {
		return s, xerrors.Errorf("while getting value of spawner: %v", err)
	}
	if cid != contracts.ContractSpawnerID {
		return s, xerrors.Errorf("wrong contractID for this IID: %s", cid)
	}
	if err := protobuf.Decode(v, &s.costs); err != nil {
		return s, xerrors.Errorf("while decoding spawner contract: %v", err)
	}
	s.InstanceID = iid
	s.cl = cl
	return
}

// Start a new transaction - it will not be sent until SendTransaction is
// called.
func (s *Spawner) Start(coinID byzcoin.InstanceID,
	signer darc.Signer) ActiveSpawner {
	return ActiveSpawner{
		cl:     s.cl,
		iid:    s.InstanceID,
		costs:  s.costs,
		coinID: coinID,
		signer: signer,
	}
}

// SpawnDarc prepares for spawning a darc. It does not send the instruction
// to byzcoin yet.
func (as *ActiveSpawner) SpawnDarc(newDarc darc.Darc) error {
	darcBuf, err := newDarc.ToProto()
	if err != nil {
		return xerrors.Errorf("couldn't get protobuf for darc: %v", err)
	}
	as.Spawn(byzcoin.ContractDarcID, as.costs.CostDarc.Value,
		byzcoin.Argument{Name: "darc", Value: darcBuf})

	return nil
}

// SpawnDarcs prepares for spawning multiple darcs.
func (as *ActiveSpawner) SpawnDarcs(newDarcs ...darc.Darc) error {
	for _, d := range newDarcs {
		if err := as.SpawnDarc(d); err != nil {
			return xerrors.Errorf("couldn't spawn darc: %v", err)
		}
	}
	return nil
}

// SpawnCoin prepares for spawning a coin. It does not send the instruction.
func (as *ActiveSpawner) SpawnCoin(coinType byzcoin.InstanceID,
	darcID darc.ID, value uint64) (byzcoin.InstanceID, error) {
	coinIID := random.Bits(256, true, random.New())
	valueBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(valueBuf, value)
	as.Spawn(contracts2.ContractCoinID, as.costs.CostCoin.Value,
		byzcoin.Arguments{
			{Name: "coinID", Value: coinIID},
			{Name: "type", Value: coinType[:]},
			{Name: "darcID", Value: darcID},
			{Name: "coinValue", Value: valueBuf},
		}...)
	as.cost += value

	return as.instructions[len(as.instructions)-1].DeriveIDArg("", "coinID")
}

// SpawnCredential prepares for sending a credential.
// It does not send the instructions.
func (as *ActiveSpawner) SpawnCredential(cs contracts.CredentialStruct,
	darcID darc.ID) (iid byzcoin.InstanceID, err error) {
	csBuf, err := protobuf.Encode(&cs)
	if err != nil {
		return iid, xerrors.Errorf("couldn't get protobuf for credential: %v",
			err)
	}
	csIID := random.Bits(256, true, random.New())

	as.Spawn(contracts.ContractCredentialID, as.costs.CostCredential.Value,
		byzcoin.Arguments{
			{Name: "credential", Value: csBuf},
			{Name: "darcID", Value: darcID},
			{Name: "credID", Value: csIID[:]},
		}...)
	return as.instructions[len(as.instructions)-1].DeriveIDArg("", "credID")
}

// Spawn adds an instruction to spawn an instance.
func (as *ActiveSpawner) Spawn(contractID string,
	cost uint64, args ...byzcoin.Argument) {
	as.AddInstruction(
		byzcoin.Instruction{
			InstanceID: as.iid,
			Spawn: &byzcoin.Spawn{
				ContractID: contractID,
				Args:       args,
			},
		},
		cost)
}

// AddInstruction allows to add any instruction to the transaction that will
// be sent to byzcoin.
func (as *ActiveSpawner) AddInstruction(inst byzcoin.Instruction, cost uint64) {
	as.instructions = append(as.instructions, inst)
	as.cost += cost
}

// SendTransaction adds an instruction to fetch the needed coins, then
// adds all waiting instructions and sends them to byzcoin.
func (as *ActiveSpawner) SendTransaction() error {
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, as.cost)
	ctx, err := as.cl.CreateTransaction(append(byzcoin.Instructions{{
		InstanceID: as.coinID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts2.ContractCoinID,
			Command:    "fetch",
			Args: byzcoin.Arguments{
				{Name: "coins", Value: coinsBuf},
			},
		},
	}}, as.instructions...)...)
	if err != nil {
		return xerrors.Errorf("couldn't create transaction: %v", err)
	}
	if err := as.cl.SignTransaction(ctx, as.signer); err != nil {
		return xerrors.Errorf("couldn't sign transaction: %v", err)
	}
	if _, err := as.cl.AddTransactionAndWait(ctx, 10); err != nil {
		return xerrors.Errorf("while waiting for transaction to be accepted"+
			": %v", err)
	}

	return nil
}
