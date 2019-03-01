package unicore

import (
	"crypto/sha256"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3/log"
)

type contract struct {
	byzcoin.BasicContract
}

// Spawn is
func (c *contract) Spawn(rst byzcoin.ReadOnlyStateTrie, instr byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	_, _, _, darcID, err := rst.GetValues(instr.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	bin := instr.Spawn.Args.Search("binary")

	h := sha256.New()
	h.Write(instr.DeriveID("").Slice())
	vid := byzcoin.NewInstanceID(h.Sum(nil))

	// create the binary instance
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, instr.DeriveID(""), contractName, bin, darcID))
	// create the state instance
	sc = append(sc, byzcoin.NewStateChange(byzcoin.Create, vid, contractName, []byte{}, darcID))

	return
}

// Invoke is
func (c *contract) Invoke(rst byzcoin.ReadOnlyStateTrie, instr byzcoin.Instruction, coins []byzcoin.Coin) (sc []byzcoin.StateChange, cout []byzcoin.Coin, err error) {
	cout = coins

	bin, _, _, darcID, err := rst.GetValues(instr.InstanceID.Slice())
	if err != nil {
		return nil, nil, err
	}

	h := sha256.New()
	h.Write(instr.InstanceID.Slice())
	vid := h.Sum(nil)

	v, _, _, _, err := rst.GetValues(vid)
	if err != nil {
		return nil, nil, err
	}

	switch instr.Invoke.Command {
	case "exec":
		out, err := execBinary(bin, v, instr)
		if err != nil {
			return nil, nil, err
		}
		sc = append(sc, byzcoin.NewStateChange(byzcoin.Update, byzcoin.NewInstanceID(vid), contractName, out, darcID))
	default:
		return nil, nil, errors.New("unknown command for unicore contract")
	}

	return
}

func execBinary(bin []byte, value []byte, instr byzcoin.Instruction) ([]byte, error) {
	tmpDir, err := ioutil.TempDir("", "unicore_")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	err = ioutil.WriteFile(path.Join(tmpDir, "exec"), bin, 0700)
	if err != nil {
		return nil, err
	}

	args := []string{}
	for _, a := range instr.Invoke.Args {
		args = append(args, string(a.Value))
	}

	cmd := exec.Command(path.Join(tmpDir, "exec"), args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	// Write the previous state in the standard input
	_, err = stdin.Write(value)
	if err != nil {
		return nil, err
	}

	// prevent the execution to stall
	stdin.Close()

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Lvlf1("stderr: %s", string(out))
		return nil, err
	}

	return out, nil
}
