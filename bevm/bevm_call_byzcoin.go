package bevm

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3/byzcoin"
	"golang.org/x/xerrors"
)

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
	eventBytes []byte) (string, interface{}, error) {

	event := eventByID(contractAbi, eventID)
	if event == nil {
		// Event not found
		return "", nil, nil
	}

	eventData, err := unpackData(contractAbi, event.Name, eventBytes,
		event.Inputs)

	return event.Name, eventData, err
}

func convertArgs(eventArgs []struct {
	Name  string
	Value []byte
}) byzcoin.Arguments {
	args := byzcoin.Arguments{}

	for _, arg := range eventArgs {
		args = append(args, arg)
	}

	return args
}

func getInstrForEvent(name string, iface interface{}) (
	*byzcoin.Instruction, error) {
	var instr *byzcoin.Instruction

	switch name {
	case byzcoinSpawnEvent:
		event, ok := iface.(struct {
			InstanceID [32]byte
			ContractID string
			Args       []struct {
				Name  string
				Value []byte
			}
		})
		if !ok {
			return nil, xerrors.Errorf("failed to cast 'spawn' event")
		}

		instr = &byzcoin.Instruction{
			InstanceID: event.InstanceID,
			Spawn: &byzcoin.Spawn{
				ContractID: event.ContractID,
				Args:       convertArgs(event.Args),
			},
		}

	case byzcoinInvokeEvent:
		event, ok := iface.(struct {
			InstanceID [32]byte
			ContractID string
			Command    string
			Args       []struct {
				Name  string
				Value []byte
			}
		})
		if !ok {
			return nil, xerrors.Errorf("failed to cast 'invoke' event")
		}

		instr = &byzcoin.Instruction{
			InstanceID: event.InstanceID,
			Invoke: &byzcoin.Invoke{
				ContractID: event.ContractID,
				Command:    event.Command,
				Args:       convertArgs(event.Args),
			},
		}

	case byzcoinDeleteEvent:
		event, ok := iface.(struct {
			InstanceID [32]byte
			ContractID string
			Args       []struct {
				Name  string
				Value []byte
			}
		})
		if !ok {
			return nil, xerrors.Errorf("failed to cast 'delete' event")
		}

		instr = &byzcoin.Instruction{
			InstanceID: event.InstanceID,
			Delete: &byzcoin.Delete{
				ContractID: event.ContractID,
				Args:       convertArgs(event.Args),
			},
		}

	default:
		return nil, xerrors.Errorf("internal error: event '%s' not handled",
			name)
	}

	return instr, nil
}
