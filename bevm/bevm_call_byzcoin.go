package bevm

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
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

func processInstrs(instrs byzcoin.Instructions, signers []darc.Signer,
	rst byzcoin.ReadOnlyStateTrie, cin []byzcoin.Coin) ([]byzcoin.Coin,
	[]byzcoin.StateChange, error) {

	for i := range instrs {
		instrs[i].SignerIdentities = []darc.Identity{signers[i].Identity()}
		instrs[i].SignerCounter = []uint64{1} // FIXME: should increment?
	}

	instrs.SetVersion(rst.GetVersion())

	gs, ok := rst.(byzcoin.GlobalState)
	if !ok {
		return nil, nil, xerrors.Errorf("internal error: cannot convert " +
			"rst to gs")
	}

	cout := cin
	var stateChanges []byzcoin.StateChange

	for i, instr := range instrs {
		err := instr.SignWith([]byte{}, signers[i])
		if err != nil {
			return nil, nil, xerrors.Errorf("failed to sign instruction "+
				"from EVM: %v", err)
		}

		sc, c, err := gs.ExecuteInstruction(gs, cout, instr, nil)
		if err != nil {
			return nil, nil, xerrors.Errorf("failed to execute instruction "+
				"from EVM: %v", err)
		}
		cout = c
		stateChanges = append(stateChanges, sc...)
	}

	return cout, stateChanges, nil
}
