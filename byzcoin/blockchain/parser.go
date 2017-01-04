package blockchain

import (
	"os"
	"os/exec"
	"path/filepath"

	"crypto/tls"
	"net/http"

	"io"

	"crypto/sha256"

	"encoding/hex"

	"github.com/dedis/cothority/byzcoin/blockchain/blkparser"
	"github.com/dedis/onet/log"
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

// CheckBlockAvailable looks if the directory with the block exists or not.
// It takes 'dir' as the base-directory, generated from 'cothority/simul'.
func GetBlockName(dir string) string {
	m, _ := filepath.Glob(dir + "/*.dat")
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
		log.Fatal("Couldn't get working dir:", err)
	}
	return dir + "/blocks"
}

// DownloadBlock takes 'dir' as the directory where to download the block.
// It returns the downloaded file
func DownloadBlock(dir string) (string, error) {
	log.Info("Downloading block-file")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("https://pop.dedis.ch/blk00000.dat")
	if err != nil {
		return "", err
	}

	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return "", err
	}
	out, err := os.Create(dir + "/blk00000.dat")
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	// Calculate SHA-256 to make sure we downloaded the correct file
	out.Seek(0, 0)
	hash := sha256.New()
	if _, err := io.Copy(hash, out); err != nil {
		log.Fatal(err)
	}
	sha256 := hex.EncodeToString(hash.Sum(nil))
	if sha256 != "3fb2b64a3aaadbc67268113ba4791cde76ea18205fff3862fc3ac0de47a7cdbb" {
		log.Fatal("sha256 of downloaded file is wrong: ", sha256)
	}

	return GetBlockName(dir), nil
}

// EnsureBlockIsAvailable tests if the block is already downloaded, else it will
// download it. Finally the block will be copied to the 'simul'-provided
// directory for simulation.
func EnsureBlockIsAvailable(dir string) error {
	tmpdir := "/tmp/byzcoin"
	block := GetBlockName(tmpdir)
	if block == "" {
		var err error
		block, err = DownloadBlock(tmpdir)
		if err != nil || block == "" {
			return err
		}
	}
	destDir := dir + "/blocks"
	os.RemoveAll(destDir)
	if err := os.Mkdir(destDir, 0777); err != nil {
		return err
	}
	cmd := exec.Command("cp", block, destDir)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
