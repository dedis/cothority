package calypso

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/protobuf"
	"testing"
)

func TestContractWrite_Invoke(t *testing.T) {
	rost := byzcoin.NewROSTSimul()

	cw := ContractWrite{
		Write: Write{
			Data:      []byte("data"),
			ExtraData: []byte("extradata"),
		},
	}
	cwID, err := rost.CreateRandomInstance(ContractWriteID, &cw, nil)
	require.NoError(t, err)
	instr := byzcoin.Instruction{
		InstanceID: cwID,
		Invoke: &byzcoin.Invoke{
			ContractID: ContractWriteID,
			Command:    "updates",
			Args: byzcoin.Arguments{{
				Name:  "data",
				Value: []byte("newData"),
			}},
		}}
	scs, _, err := cw.Invoke(rost, instr, nil)
	require.Error(t, err)

	instr.Invoke.Command = "update"
	scs, _, err = cw.Invoke(rost, instr, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	var cwNew ContractWrite
	require.NoError(t, protobuf.Decode(scs[0].Value, &cwNew))
	require.Equal(t, []byte("newData"), cwNew.Data)

	instr.Invoke.Args[0] = byzcoin.Argument{
		Name:  "extraData",
		Value: []byte("newExtraData")}
	scs, _, err = cw.Invoke(rost, instr, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(scs))
	require.NoError(t, protobuf.Decode(scs[0].Value, &cwNew))
	require.Equal(t, []byte("newExtraData"), cwNew.ExtraData)
}
