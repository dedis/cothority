package bizcoin

import (
	"errors"
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

// ReadFirstNBlocks: only read the first ReadFirstNBlocks in the the BlocksDir
// (so you only have to copy the first blocks to deterLab)
const ReadFirstNBlocks = 400

// Client is a client simulation. At the moment we do not measure the
// communication between client and server. Hence, we do not even open a real
// network connection
type Client struct {
	// holds the sever as a struct
	srv BlockServer
}

func NewClient(s BlockServer) *Client {
	return &Client{srv: s}
}

// StartClientSimulation can be called from outside (from an simulation
// implementation) to simulate a client. Parameters:
// blocksDir is the directory where to find the transaction blocks (.dat files)
// numTxs is the number of transactions the client will create
func (c *Client) StartClientSimulation(blocksDir string, numTxs int) error {
	return c.triggerTransactions(blocksDir, numTxs)
}

func (c *Client) triggerTransactions(blocksPath string, nTxs int) error {
	dbg.Lvl1("BizCoin Client will trigger up to", nTxs, " transactions")
	parser, err := blockchain.NewParser(blocksPath, magicNum)
	if err != nil {
		dbg.Error("Couldn't parse blocks in", blocksPath)
		return err
	}

	transactions, err := parser.Parse(0, ReadFirstNBlocks)
	if err != nil {
		return fmt.Errorf("Error while parsing transactions %v", err)
	}
	if len(transactions) == 0 {
		return errors.New("Couldn't read any transactions.")
	}
	if len(transactions) != nTxs {
		return fmt.Errorf("Read only %v but caller wanted %v", len(transactions), nTxs)
	}
	consumed := nTxs
	for consumed > 0 {
		for _, tr := range transactions {
			// "send" transaction to server (we skip tcp connection on purpose here)
			c.srv.AddTransaction(tr)
		}
		consumed--
	}
	return nil
}
