package unicore

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/log"
)

type contract struct {
	byzcoin.BasicContract
}

// Spawn is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support spawning.
func (c *contract) Spawn(rst byzcoin.ReadOnlyStateTrie, instr byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	_, _, _, darcID, err := rst.GetValues(instr.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	v := instr.Spawn.Args.Search("binary")

	sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instr.DeriveID(""), contractName, v, darcID))

	return
}

// Invoke is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support invoking.
func (c *contract) Invoke(rst byzcoin.ReadOnlyStateTrie, instr byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	bin, _, _, _, err := rst.GetValues(instr.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	tmpDir, err := ioutil.TempDir("", "unicore_")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpDir)

	err = ioutil.WriteFile(path.Join(tmpDir, "exec"), bin, 0700)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(path.Join(tmpDir, "exec"), "1", "2")

	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return nil, nil, err
	}

	log.Lvlf1("Command output: %s", out.String())

	return
}

// Delete is not implmented in a BasicContract. Types which embed BasicContract
// must override this method if they support deleting.
func (c *contract) Delete(rst byzcoin.ReadOnlyStateTrie, instr byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins
	return
}
