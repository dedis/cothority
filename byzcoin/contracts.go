package byzcoin

import (
	"encoding/binary"
	"errors"
	"fmt"
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
	// Verify returns nil if the instruction is valid with regard to the signature.
	VerifyDeferedInstruction(ReadOnlyStateTrie, Instruction, []byte) error
	// Spawn is used to spawn new instances
	Spawn(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Invoke only modifies existing instances
	Invoke(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Delete removes the current instance
	Delete(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
}

// ContractFn is the type signature of the instance factory functions which can be
// registered with the ByzCoin service.
type ContractFn func(in []byte) (Contract, error)

// RegisterContract stores the contract in a map and will call it whenever a
// contract needs to be done. GetService makes it possible to give either an
// `onet.Context` or `onet.Server` to `RegisterContract`.
func RegisterContract(s skipchain.GetService, contractID string, f ContractFn) error {
	scs := s.Service(ServiceName)
	if scs == nil {
		return errors.New("Didn't find our service: " + ServiceName)
	}
	return scs.(*Service).registerContract(contractID, f)
}

// BasicContract is a type that contracts may choose to embed in order to provide
// default implementations for the Contract interface.
type BasicContract struct{}

func notImpl(what string) error { return fmt.Errorf("this contract does not implement %v", what) }

// VerifyInstruction offers the default implementation of verifying an instruction. Types
// which embed BasicContract may choose to override this implementation.
func (b BasicContract) VerifyInstruction(rst ReadOnlyStateTrie, inst Instruction, ctxHash []byte) error {
	if err := inst.Verify(rst, ctxHash); err != nil {
		return err
	}
	return nil
}

// VerifyDeferedInstruction is not implemented in a BasicContract. Types which
// embed BasicContract must override this method if they want to support
// deferred executions (using the Deferred contract).
func (b BasicContract) VerifyDeferedInstruction(rst ReadOnlyStateTrie, inst Instruction, ctxHash []byte) error {
	return notImpl("VerifyDeferedInstruction")
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

func (c *contractConfig) Invoke(rst ReadOnlyStateTrie, inst Instruction, coins []Coin) (sc []StateChange, cout []Coin, err error) {
	cout = coins

	// Find the darcID for this instance.
	var darcID darc.ID
	_, _, _, darcID, err = rst.GetValues(inst.InstanceID.Slice())
	if err != nil {
		return
	}

	// There are two situations where we need to change the roster, first
	// is when it is initiated by the client(s) that holds the genesis
	// signing key, in this case we trust the client to do the right
	// thing. The second is during a view-change, so we need to do
	// additional validation to make sure a malicious node doesn't freely
	// change the roster.

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
