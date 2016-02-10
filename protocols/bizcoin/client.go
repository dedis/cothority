package bizcoin

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

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
// implementation) to simulate a client
func (c *Client) StartClientSimulation() {
	// these are the constants from lefteris' current code
	// (see https://github.com/LefKok/cothority/blob/BitCoSi_round/app/skeleton/client.go#L51-L52)
	// XXX put into a config file?
	c.triggerTransactions("blocks", 400, 1000)
}

func (c *Client) triggerTransactions(blocksPath string, readNumBlocks, iterations int) error {
	parser, err := blockchain.NewParser(blocksPath, magicNum)
	if err != nil {
		dbg.Error("Couldn't parse blocks in", blocksPath)
		return err
	}

	for i := 0; i < iterations; i++ {
		transactions := parser.Parse(0, readNumBlocks)
		for _, tr := range transactions {
			if err := c.srv.AddTransaction(tr); err != nil {
				return err
			}
		}
	}

	return nil
}
