package bizcoin

import (
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
	srv *Server
}

func NewClient(s *Server) *Client {
	return &Client{srv: s}
}

// StartClientSimulation can be called from outside (from an simulation
// implementation) to simulate a client. Parameters:
// blocksDir is the directory where to find the transaction blocks (.dat files)
// numTxs is the number of transactions the client will create
func (c *Client) StartClientSimulation(blocksDir string, numTxs uint) error {
	return c.triggerTransactions(blocksDir, numTxs)
}

func (c *Client) triggerTransactions(blocksPath string, nTxs uint) error {
	parser, err := blockchain.NewParser(blocksPath, magicNum)
	if err != nil {
		dbg.Error("Couldn't parse blocks in", blocksPath)
		return err
	}

	transactions := parser.Parse(0, ReadFirstNBlocks)
	consumed := nTxs
	for consumed > 0 {
		for _, tr := range transactions {
			// "send" transaction to server (we skip tcp connection on purpose here)
			if err := c.srv.AddTransaction(tr); err != nil {
				return err
			}
		}
		consumed--
	}

	return nil
}
