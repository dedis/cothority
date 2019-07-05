package byzcoin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/blscosi/protocol"
	"go.dedis.ch/cothority/v3/byzcoin/viewchange"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// Contract is the interface that an instance needs
// to implement to be callable as a pre-compiled smart
// contract.
type Contract interface {
	// Verify returns nil if the instruction is valid with regard to the signature.
	VerifyInstruction(ReadOnlyStateTrie, Instruction, []byte) error
	// VerifyDeferredInstruction should be implemented if one wants to support
	// the deferred execution of the contract. It should do the same verify
	// process as the VerifyInstruction method but, instead of calling
	// inst.Verify(), it should use inst.VerifyWithOption() with the
	// "checkCounters" parameter set to false. See the value contract.
	VerifyDeferredInstruction(ReadOnlyStateTrie, Instruction, []byte) error
	// Spawn is used to spawn new instances
	Spawn(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Invoke only modifies existing instances
	Invoke(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Delete removes the current instance
	Delete(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Print ...
	FormatMethod(Instruction) string
}

// FormatMethod prints the method of a given instruction (ie. "Spawn", "Invoke",
// or "Delete"). This basic function simply calls "strconv.Quote" on the args of
// the method. It should be overrided by contracts that have more complex
// arguments. See the config contract for an example.
func (b BasicContract) FormatMethod(instr Instruction) string {
	out := new(strings.Builder)
	var instArgs Arguments

	switch instr.GetType() {
	case SpawnType:
		out.WriteString("- Spawn:\n")
		fmt.Fprintf(out, "-- ContractID: %s\n", instr.Spawn.ContractID)
		instArgs = instr.Spawn.Args
	case InvokeType:
		out.WriteString("- Invoke:\n")
		fmt.Fprintf(out, "-- ContractID: %s\n", instr.Invoke.ContractID)
		fmt.Fprintf(out, "-- Command: %s\n", instr.Invoke.Command)
		instArgs = instr.Invoke.Args
	case DeleteType:
		out.WriteString("- Delete:\n")
		fmt.Fprintf(out, "-- ContractID: %s\n", instr.Delete.ContractID)
		instArgs = []Argument{}
	}

	out.WriteString("-- Args:\n")
	for _, name := range instArgs.Names() {
		fmt.Fprintf(out, "--- %s:\n", name)
		fmt.Fprintf(out, "---- %s\n", strconv.Quote(string(instArgs.Search(name))))
	}
	return out.String()
}

// ReadOnlyContractRegistry is the read-only interface for the contract registry.
type ReadOnlyContractRegistry interface {
	Search(contractID string) (ContractFn, bool)
}

// ContractWithRegistry is an interface to detect contracts that need a reference
// to the registry.
type ContractWithRegistry interface {
	SetRegistry(ReadOnlyContractRegistry)
}

// ContractFn is the type signature of the instance factory functions which can be
// registered with the ByzCoin service.
type ContractFn func(in []byte) (Contract, error)

// contractRegistry maps a contract ID with its constructor function. As soon
// as the first cloning happens, the registry will be locked and no new contract
// can be added for the global call.
type contractRegistry struct {
	registry map[string]ContractFn
	locked   bool
	sync.Mutex
}

// register tries to store the contract inside the registry. It will fail if the
// registry is locked and ignoreLock is set to false. It will also fail if the
// contract already exists.
// Because of backwards compatibility, the ignoreLock parameter can be set to
// true to register a contract after module initialization.
func (cr *contractRegistry) register(contractID string, f ContractFn, ignoreLock bool) error {
	cr.Lock()
	if cr.locked && !ignoreLock {
		cr.Unlock()
		return errors.New("contract registry is locked")
	}

	_, exists := cr.registry[contractID]
	if exists {
		cr.Unlock()
		return errors.New("contract already registered")
	}

	cr.registry[contractID] = f
	cr.Unlock()
	return nil
}

// Search looks up the contract ID and returns the constructor function
// if it exists and nil otherwise.
func (cr *contractRegistry) Search(contractID string) (ContractFn, bool) {
	cr.Lock()
	fn, exists := cr.registry[contractID]
	cr.Unlock()
	return fn, exists
}

// Clone returns a copy of the registry and locks the source so that
// static registration is not allowed anymore. This is to prevent
// registration of a contract at runtime and limit it only to the
// initialization phase.
func (cr *contractRegistry) clone() *contractRegistry {
	cr.Lock()
	cr.locked = true

	clone := newContractRegistry()
	// It is locked for outsiders but the package can manually update
	// the registry (e.g. tests)
	clone.locked = true
	for key, value := range cr.registry {
		clone.registry[key] = value
	}
	cr.Unlock()

	return clone
}

func newContractRegistry() *contractRegistry {
	return &contractRegistry{
		registry: make(map[string]ContractFn),
		locked:   false,
	}
}

var globalContractRegistry = newContractRegistry()

// RegisterGlobalContract stores the contract in the global registry. This should
// be called during module initialization as the registry will be locked down
// after the first cloning.
func RegisterGlobalContract(contractID string, f ContractFn) error {
	return globalContractRegistry.register(contractID, f, false)
}

// RegisterContract stores the contract in the service registry which
// makes it only available to byzcoin.
//
// Deprecated: Use RegisterGlobalContract during the module initialization
// for a global access to the contract.
func RegisterContract(s skipchain.GetService, contractID string, f ContractFn) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}

	return scs.(*Service).contracts.register(contractID, f, true)
}

// GetContractRegistry clones the global registry and returns a read-only one.
// Caution: calling this during the initialization will lock the registry.
func GetContractRegistry() ReadOnlyContractRegistry {
	return globalContractRegistry.clone()
}

// BasicContract is a type that contracts may choose to embed in order to provide
// default implementations for the Contract interface.
type BasicContract struct{}

func notImpl(what string) error { return fmt.Errorf("this contract does not implement %v", what) }

// VerifyInstruction offers the default implementation of verifying an instruction. Types
// which embed BasicContract may choose to override this implementation.
func (b BasicContract) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, ctxHash []byte) error {
	return inst.Verify(rst, ctxHash)
}

// VerifyDeferredInstruction is not implemented in a BasicContract. Types which
// embed BasicContract must override this method if they want to support
// deferred executions (using the Deferred contract).
func (b BasicContract) VerifyDeferredInstruction(rst ReadOnlyStateTrie, inst Instruction, ctxHash []byte) error {
	return notImpl("VerifyDeferredInstruction")
}

// Spawn is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support spawning.
func (b BasicContract) Spawn(ReadOnlyStateTrie, Instruction, []Coin) (sc []StateChange, c []Coin, err error) {
	err = notImpl("Spawn")
	return
}

// Invoke is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support invoking.
func (b BasicContract) Invoke(ReadOnlyStateTrie, Instruction, []Coin) (sc []StateChange, c []Coin, err error) {
	err = notImpl("Invoke")
	return
}

// Delete is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support deleting.
func (b BasicContract) Delete(ReadOnlyStateTrie, Instruction, []Coin) (sc []StateChange, c []Coin, err error) {
	err = notImpl("Delete")
	return
}

//
// Built-in contracts necessary for bootstrapping the ledger.
//  * Config
//  * SecureDarc
//

// ContractConfigID denotes a config-contract
const ContractConfigID = "config"

// ConfigInstanceID represents the 0-id of the configuration instance.
var ConfigInstanceID = InstanceID{}

type contractConfig struct {
	BasicContract
	ChainConfig
}

var _ Contract = (*contractConfig)(nil)

func contractConfigFromBytes(in []byte) (Contract, error) {
	c := &contractConfig{}
	err := protobuf.DecodeWithConstructors(in, &c.ChainConfig, network.DefaultConstructors(cothority.Suite))

	if err != nil {
		return nil, err
	}
	return c, nil
}

type darcContractIDs struct {
	IDs []string
}

// We need to override BasicContract.Verify because of the genesis config special case.
func (c *contractConfig) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, msg []byte) (err error) {
	pr, err := rst.GetProof(ConfigInstanceID.Slice())
	if err != nil {
		return
	}
	ok, err := pr.Exists(ConfigInstanceID.Slice())
	if err != nil {
		return
	}

	// The config does not exist yet, so this is a genesis config creation. No need/possiblity of verifying it.
	if !ok {
		return nil
	}

	return inst.Verify(rst, msg)
}

// This is the same as the VerifyInstruction function, but it uses
// VerifyWithOption() instead of Verify(). We need to implement it in order to
// use deferred config contract.
func (c *contractConfig) VerifyDeferredInstruction(rst ReadOnlyStateTrie, inst Instruction, msg []byte) (err error) {
	pr, err := rst.GetProof(ConfigInstanceID.Slice())
	if err != nil {
		return
	}
	ok, err := pr.Exists(ConfigInstanceID.Slice())
	if err != nil {
		return
	}

	// The config does not exist yet, so this is a genesis config creation. No need/possiblity of verifying it.
	if !ok {
		return nil
	}

	return inst.VerifyWithOption(rst, msg, false)
}

// FormatMethod overrides the implementation from the BasicContract in order to
// proprely print "invoke:config.update_config"
func (c *contractConfig) FormatMethod(instr Instruction) string {
	out := new(strings.Builder)
	if instr.GetType() == InvokeType && instr.Invoke.Command == "update_config" {
		out.WriteString("- Invoke:\n")
		fmt.Fprintf(out, "-- ContractID: %s\n", instr.Invoke.ContractID)
		fmt.Fprintf(out, "-- Command: %s\n", instr.Invoke.Command)

		contractConfig := ChainConfig{}
		err := protobuf.Decode(instr.Invoke.Args.Search("config"), &contractConfig)
		if err != nil {
			return "[!!!] failed to decode contractConfig: " + err.Error()
		}

		out.WriteString("-- Args:\n")
		out.WriteString(eachLine.ReplaceAllString(contractConfig.String(), "--$1"))

		return out.String()
	}
	return c.BasicContract.FormatMethod(instr)
}

// Spawn expects those arguments:
//   - darc           darc.Darc
//   - block_interval int64
//   - max_block_size int64
//   - roster         onet.Roster
//   - darc_contracts darcContractID
func (c *contractConfig) Spawn(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins
	darcBuf := inst.Spawn.Args.Search("darc")
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		log.Errorf("couldn't decode darc: %v", err)
		return
	}
	if d.Rules.Count() == 0 {
		return nil, nil, errors.New("don't accept darc with empty rules")
	}
	if err = d.Verify(true); err != nil {
		log.Error("couldn't verify darc")
		return
	}

	intervalBuf := inst.Spawn.Args.Search("block_interval")
	interval, _ := binary.Varint(intervalBuf)
	bsBuf := inst.Spawn.Args.Search("max_block_size")
	maxsz, _ := binary.Varint(bsBuf)

	rosterBuf := inst.Spawn.Args.Search("roster")
	roster := onet.Roster{}
	err = protobuf.DecodeWithConstructors(rosterBuf, &roster, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return
	}

	// create the config to be stored by state changes
	c.BlockInterval = time.Duration(interval)
	c.Roster = roster
	c.MaxBlockSize = int(maxsz)
	if err = c.sanityCheck(nil); err != nil {
		return
	}

	// get the darc contracts
	darcContractIDsBuf := inst.Spawn.Args.Search("darc_contracts")
	dcIDs := darcContractIDs{}
	err = protobuf.Decode(darcContractIDsBuf, &dcIDs)
	if err != nil {
		return
	}
	c.DarcContractIDs = dcIDs.IDs

	configBuf, err := protobuf.Encode(c)
	if err != nil {
		return
	}

	id := d.GetBaseID()
	sc = []StateChange{
		NewStateChange(Create, ConfigInstanceID, ContractConfigID, configBuf, id),
		NewStateChange(Create, NewInstanceID(id), ContractDarcID, darcBuf, id),
	}
	return
}

// Invoke offers the following functions:
//   - Invoke:update_config
//   - Invoke:view_change
//
// Invoke:update_config should have the following input argument:
//   - config ChainConfig
//
// Invoke:view_change sould have the following input arguments:
//   - newview viewchange.NewViewReq
//   - multisig []byte
func (c *contractConfig) Invoke(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// There are two situations where we need to change the roster:
	// 1. When it is initiated by the client(s) that holds the genesis
	//    signing key. In this case, we trust the client to do the right thing.
	// 2. During a view-change. In this case, we need to do additional
	//    validation to make sure a malicious node doesn't freely change the
	//    roster.

	switch inst.Invoke.Command {
	case "update_config":
		configBuf := inst.Invoke.Args.Search("config")
		newConfig := ChainConfig{}
		err = protobuf.DecodeWithConstructors(configBuf, &newConfig, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return
		}

		var oldConfig *ChainConfig
		oldConfig, err = LoadConfigFromTrie(rst)
		if err != nil {
			return
		}
		if err = newConfig.sanityCheck(oldConfig); err != nil {
			return
		}
		var val []byte
		val, _, _, _, err = rst.GetValues(darcID)
		if err != nil {
			return
		}
		var genesisDarc *darc.Darc
		genesisDarc, err = darc.NewFromProtobuf(val)
		if err != nil {
			return
		}
		var rules []string
		for _, p := range newConfig.Roster.Publics() {
			rules = append(rules, "ed25519:"+p.String())
		}
		genesisDarc.Rules.UpdateRule("invoke:"+ContractConfigID+".view_change", expression.InitOrExpr(rules...))
		var genesisBuf []byte
		genesisBuf, err = genesisDarc.ToProto()
		if err != nil {
			return
		}
		sc = []StateChange{
			NewStateChange(Update, NewInstanceID(nil), ContractConfigID, configBuf, darcID),
			NewStateChange(Update, NewInstanceID(darcID), ContractDarcID, genesisBuf, darcID),
		}
		return
	case "view_change":
		var req viewchange.NewViewReq
		err = protobuf.DecodeWithConstructors(inst.Invoke.Args.Search("newview"), &req, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return
		}
		// If everything is correctly signed, then we trust it, no need
		// to do additional verification.
		sigBuf := inst.Invoke.Args.Search("multisig")
		err = protocol.BlsSignature(sigBuf).Verify(pairingSuite, req.Hash(), req.Roster.ServicePublics(ServiceName))
		if err != nil {
			return
		}

		sc, err = updateRosterScs(rst, darcID, req.Roster)
		return
	default:
		err = errors.New("invalid invoke command: " + inst.Invoke.Command)
		return
	}
}

func updateRosterScs(rst ReadOnlyStateTrie, darcID darc.ID, newRoster onet.Roster) (StateChanges, error) {
	config, err := LoadConfigFromTrie(rst)
	if err != nil {
		return nil, err
	}
	config.Roster = newRoster
	configBuf, err := protobuf.Encode(config)
	if err != nil {
		return nil, err
	}

	return []StateChange{
		NewStateChange(Update, NewInstanceID(nil), ContractConfigID, configBuf, darcID),
	}, nil
}

// LoadConfigFromTrie loads the configuration data from the trie.
func LoadConfigFromTrie(st ReadOnlyStateTrie) (*ChainConfig, error) {
	// Find the genesis-darc ID.
	val, _, contract, _, err := GetValueContract(st, NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	if string(contract) != ContractConfigID {
		return nil, errors.New("did not get " + ContractConfigID)
	}

	config := ChainConfig{}
	err = protobuf.DecodeWithConstructors(val, &config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// GetValueContract gets all the information in an instance, an error is
// returned if the instance does not exist.
func GetValueContract(st ReadOnlyStateTrie, key []byte) (value []byte, version uint64, contract string, darcID darc.ID, err error) {
	value, version, contract, darcID, err = st.GetValues(key)
	if err != nil {
		return
	}
	if value == nil {
		err = errKeyNotSet
		return
	}
	return
}

func getInstanceDarc(c ReadOnlyStateTrie, iid InstanceID, darcContractIDs []string) (*darc.Darc, error) {
	// conver the string slice to a map
	m := make(map[string]bool)
	for _, id := range darcContractIDs {
		m[id] = true
	}

	// From instance ID, find the darcID that controls access to it.
	_, _, _, dID, err := c.GetValues(iid.Slice())
	if err != nil {
		return nil, err
	}

	// Fetch the darc itself.
	value, _, contract, _, err := c.GetValues(dID)
	if err != nil {
		return nil, err
	}

	if _, ok := m[string(contract)]; !ok {
		return nil, fmt.Errorf("for instance %v, \"%v\" is not a contract ID that decodes to a DARC", iid, string(contract))
	}
	return darc.NewFromProtobuf(value)
}

// LoadDarcFromTrie loads a darc which should be stored in key.
func LoadDarcFromTrie(st ReadOnlyStateTrie, key []byte) (*darc.Darc, error) {
	darcBuf, _, contract, _, err := st.GetValues(key)
	if err != nil {
		return nil, err
	}
	config, err := LoadConfigFromTrie(st)
	if err != nil {
		return nil, err
	}
	var ok bool
	for _, id := range config.DarcContractIDs {
		if contract == id {
			ok = true
		}
	}
	if !ok {
		return nil, errors.New("the contract \"" + contract + "\" is not in the set of DARC contracts")
	}
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		return nil, err
	}
	return d, nil
}
