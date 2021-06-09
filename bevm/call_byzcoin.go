package bevm

import (
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3/byzcoin"
	"golang.org/x/xerrors"
)

// Whitelist of EVM-spawnable Byzcoin contracts.
// This prevents replay issues in case the way to generate a new InstanceID of
// a Byzcoin contract is changed to take into account the `seed` argument
// *after* EVM-generated instances already exist in the ledger.
var evmSpawnableContracts = map[string]bool{}

const (
	byzcoinSpawnEvent  = "ByzcoinSpawn"
	byzcoinInvokeEvent = "ByzcoinInvoke"
	byzcoinDeleteEvent = "ByzcoinDelete"
)

const eventsAbiJSON = `` +
	`[` +
	`  {` +
	`    "anonymous": false,` +
	`    "inputs": [` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "instanceID",` +
	`        "type": "bytes32"` +
	`      },` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "contractID",` +
	`        "type": "string"` +
	`      },` +
	`      {` +
	`        "components": [` +
	`          {` +
	`            "name": "name",` +
	`            "type": "string"` +
	`          },` +
	`          {` +
	`            "name": "value",` +
	`            "type": "bytes"` +
	`          }` +
	`        ],` +
	`        "indexed": false,` +
	`        "name": "args",` +
	`        "type": "tuple[]"` +
	`      }` +
	`    ],` +
	`    "name": "` + byzcoinSpawnEvent + `",` +
	`    "type": "event"` +
	`  },` +
	`  {` +
	`    "anonymous": false,` +
	`    "inputs": [` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "instanceID",` +
	`        "type": "bytes32"` +
	`      },` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "contractID",` +
	`        "type": "string"` +
	`      },` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "command",` +
	`        "type": "string"` +
	`      },` +
	`      {` +
	`        "components": [` +
	`          {` +
	`            "name": "name",` +
	`            "type": "string"` +
	`          },` +
	`          {` +
	`            "name": "value",` +
	`            "type": "bytes"` +
	`          }` +
	`        ],` +
	`        "indexed": false,` +
	`        "name": "args",` +
	`        "type": "tuple[]"` +
	`      }` +
	`    ],` +
	`    "name": "` + byzcoinInvokeEvent + `",` +
	`    "type": "event"` +
	`  },` +
	`  {` +
	`    "anonymous": false,` +
	`    "inputs": [` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "instanceID",` +
	`        "type": "bytes32"` +
	`      },` +
	`      {` +
	`        "indexed": false,` +
	`        "name": "contractID",` +
	`        "type": "string"` +
	`      },` +
	`      {` +
	`        "components": [` +
	`          {` +
	`            "name": "name",` +
	`            "type": "string"` +
	`          },` +
	`          {` +
	`            "name": "value",` +
	`            "type": "bytes"` +
	`          }` +
	`        ],` +
	`        "indexed": false,` +
	`        "name": "args",` +
	`        "type": "tuple[]"` +
	`      }` +
	`    ],` +
	`    "name": "` + byzcoinDeleteEvent + `",` +
	`    "type": "event"` +
	`  }` +
	`]`

func eventByID(eventsAbi abi.ABI, eventID common.Hash) *abi.Event {
	for _, event := range eventsAbi.Events {
		if event.Id() == eventID {
			return &event
		}
	}

	return nil
}

func unpackEvent(contractAbi abi.ABI, eventID common.Hash,
	eventBytes []byte) (string, []interface{}, error) {

	event := eventByID(contractAbi, eventID)
	if event == nil {
		// Event not found
		return "", nil, nil
	}

	eventData, err := event.Inputs.UnpackValues(eventBytes)
	if err != nil {
		return "", nil, xerrors.Errorf("failed to unpack event data: %v", err)
	}

	return event.Name, eventData, err
}

func get32Byte(eventArg interface{}) ([32]byte, error) {
	value, ok := eventArg.([32]byte)
	if !ok {
		return [32]byte{}, xerrors.Errorf("received type %v, expected "+
			"[32]byte", reflect.TypeOf(eventArg))
	}

	return value, nil
}

func getString(eventArg interface{}) (string, error) {
	value, ok := eventArg.(string)
	if !ok {
		return "", xerrors.Errorf("received type %v, expected "+
			"string", reflect.TypeOf(eventArg))
	}

	return value, nil
}

func getArgs(eventArg interface{}) (byzcoin.Arguments, error) {
	value, ok := eventArg.([]struct {
		Name  string
		Value []byte
	})
	if !ok {
		return nil, xerrors.Errorf("received type %v, expected "+
			"[]struct{Name string, Value []byte}", reflect.TypeOf(eventArg))
	}

	args := byzcoin.Arguments{}

	for _, arg := range value {
		args = append(args, arg)
	}

	return args, nil
}

func getInstrForSpawnEvent(eventData []interface{}) (
	*byzcoin.Instruction, error) {
	if len(eventData) != 3 {
		return nil, xerrors.Errorf("invalid data for 'spawn' event: "+
			"got %d values, expected 3", len(eventData))
	}

	instanceID, err := get32Byte(eventData[0])
	if err != nil {
		return nil, xerrors.Errorf("invalid instanceID for 'spawn' event: "+
			"%v", err)
	}

	contractID, err := getString(eventData[1])
	if err != nil {
		return nil, xerrors.Errorf("invalid contractID for 'spawn' event: "+
			"%v", err)
	}

	args, err := getArgs(eventData[2])
	if err != nil {
		return nil, xerrors.Errorf("invalid args for 'spawn' event: "+
			"%v", err)
	}

	if !evmSpawnableContracts[contractID] {
		return nil, xerrors.Errorf("contract '%s' has not been "+
			"whitelisted to be spawned by an EVM contract",
			contractID)
	}

	return &byzcoin.Instruction{
		InstanceID: instanceID,
		Spawn: &byzcoin.Spawn{
			ContractID: contractID,
			Args:       args,
		},
	}, nil
}

func getInstrForInvokeEvent(eventData []interface{}) (
	*byzcoin.Instruction, error) {
	if len(eventData) != 4 {
		return nil, xerrors.Errorf("invalid data for 'invoke' event: "+
			"got %d values, expected 4", len(eventData))
	}

	instanceID, err := get32Byte(eventData[0])
	if err != nil {
		return nil, xerrors.Errorf("invalid instanceID for 'invoke' event: "+
			"%v", err)
	}

	contractID, err := getString(eventData[1])
	if err != nil {
		return nil, xerrors.Errorf("invalid contractID for 'invoke' event: "+
			"%v", err)
	}

	command, err := getString(eventData[2])
	if err != nil {
		return nil, xerrors.Errorf("invalid command for 'invoke' event: "+
			"%v", err)
	}

	args, err := getArgs(eventData[3])
	if err != nil {
		return nil, xerrors.Errorf("invalid args for 'invoke' event: "+
			"%v", err)
	}

	return &byzcoin.Instruction{
		InstanceID: instanceID,
		Invoke: &byzcoin.Invoke{
			ContractID: contractID,
			Command:    command,
			Args:       args,
		},
	}, nil
}

func getInstrForDeleteEvent(eventData []interface{}) (
	*byzcoin.Instruction, error) {
	if len(eventData) != 3 {
		return nil, xerrors.Errorf("invalid data for 'delete' event: "+
			"got %d values, expected 3", len(eventData))
	}

	instanceID, err := get32Byte(eventData[0])
	if err != nil {
		return nil, xerrors.Errorf("invalid instanceID for 'delete' event: "+
			"%v", err)
	}

	contractID, err := getString(eventData[1])
	if err != nil {
		return nil, xerrors.Errorf("invalid contractID for 'delete' event: "+
			"%v", err)
	}

	args, err := getArgs(eventData[2])
	if err != nil {
		return nil, xerrors.Errorf("invalid args for 'delete' event: "+
			"%v", err)
	}

	return &byzcoin.Instruction{
		InstanceID: instanceID,
		Delete: &byzcoin.Delete{
			ContractID: contractID,
			Args:       args,
		},
	}, nil
}

func getInstrForEvent(name string, eventData []interface{}) (
	*byzcoin.Instruction, error) {
	var instr *byzcoin.Instruction
	var err error

	switch name {
	case byzcoinSpawnEvent:
		instr, err = getInstrForSpawnEvent(eventData)
		if err != nil {
			return nil, xerrors.Errorf("failed to handle 'spawn' event: "+
				"%v", err)
		}

	case byzcoinInvokeEvent:
		instr, err = getInstrForInvokeEvent(eventData)
		if err != nil {
			return nil, xerrors.Errorf("failed to handle 'invoke' event: "+
				"%v", err)
		}

	case byzcoinDeleteEvent:
		instr, err = getInstrForDeleteEvent(eventData)
		if err != nil {
			return nil, xerrors.Errorf("failed to handle 'delete' event: "+
				"%v", err)
		}

	default:
		return nil, xerrors.Errorf("internal error: event '%s' not handled",
			name)
	}

	return instr, nil
}
