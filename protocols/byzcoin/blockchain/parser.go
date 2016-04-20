package blockchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
)

type Parser struct {
	Path      string
	Magic     [4]byte
	CurrentId uint32
}

func NewParser(path string, magic [4]byte) (parser *Parser, err error) {
	parser = new(Parser)
	parser.Path = path
	parser.Magic = magic
	parser.CurrentId = 0
	return
}

func (p *Parser) Parse(first_block, last_block int) ([]blkparser.Tx, error) {

	Chain, _ := blkparser.NewBlockchain(p.Path, p.Magic)

	var transactions []blkparser.Tx

	for i := 0; i < last_block; i++ {
		raw, err := Chain.FetchNextBlock()

		if raw == nil || err != nil {
			if err != nil {
				return transactions, err
			}
		}

		bl, err := blkparser.NewBlock(raw[:])
		if err != nil {
			return transactions, err
		}

		// Read block till we reach start_block
		if i < first_block {
			continue
		}

		for _, tx := range bl.Txs {
			transactions = append(transactions, *tx)
		}

	}
	return transactions, nil
}

// SimulDirToBlockDir creates a path to the 'protocols/byzcoin/block'-dir by
// using 'dir' which comes from 'cothority/simul'
// If that directory doesn't exist, it will be created.
func SimulDirToBlockDir(dir string) string {
	reg, _ := regexp.Compile("simul/.*")
	blockDir := string(reg.ReplaceAll([]byte(dir), []byte("protocols/byzcoin/block")))
	if _, err := os.Stat(blockDir); os.IsNotExist(err) {
		if err := os.Mkdir(blockDir, 0777); err != nil {
			dbg.Error("Couldn't create blocks directory", err)
		}
	}
	return blockDir
}

// CheckBlockAvailable looks if the directory with the block exists or not.
// It takes 'dir' as the base-directory, generated from 'cothority/simul'.
func GetBlockName(dir string) string {
	blockDir := SimulDirToBlockDir(dir)
	m, _ := filepath.Glob(blockDir + "/*.dat")
	if m != nil {
		return m[0]
	} else {
		return ""
	}
}

// Gets the block-directory starting from the current directory - this will
// hold up when running it with 'simul'
func GetBlockDir() string {
	dir, err := os.Getwd()
	if err != nil {
		dbg.Fatal("Couldn't get working dir:", err)
	}
	return dir + "/blocks"
}

// DownloadBlock takes 'dir' as the directory where to download the block.
// It returns the downloaded file
func DownloadBlock(dir string) (string, error) {
	blockDir := SimulDirToBlockDir(dir)
	cmd := exec.Command("wget", "--no-check-certificate", "-O",
		blockDir+"/blk00000.dat", "-c",
		"https://icsil1-box.epfl.ch:5001/fbsharing/IzTFdOxf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	dbg.Lvl1("Cmd is", cmd)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return GetBlockName(dir), nil
}

// EnsureBlockIsAvailable tests if the block is already downloaded, else it will
// download it. Finally the block will be copied to the 'simul'-provided
// directory for simulation.
func EnsureBlockIsAvailable(dir string) error {
	block := GetBlockName(dir)
	if block == "" {
		var err error
		block, err = DownloadBlock(dir)
		if err != nil || block == "" {
			return err
		}
	}
	destDir := dir + "/blocks"
	if err := os.Mkdir(destDir, 0777); err != nil {
		return err
	}
	cmd := exec.Command("cp", block, destDir)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
