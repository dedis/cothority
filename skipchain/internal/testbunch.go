package internal

import (
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/cothority/skipchain/libsc"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

type TestBunch struct {
	Local      *onet.LocalTest
	Client     *skipchain.Client
	Roster     *onet.Roster
	Servers    []*onet.Server
	Genesis    *libsc.SkipBlock
	Skipblocks []*libsc.SkipBlock
	Bunch      *libsc.SkipBlockBunch
	Storage    *skipchain.SBBStorage
}

func NewTestBunch(nbrHosts, height, nbrAddSB int) *TestBunch {
	tb := &TestBunch{
		Skipblocks: make([]*libsc.SkipBlock, nbrAddSB+1),
		Storage:    skipchain.NewSBBStorage(),
	}
	tb.Local = onet.NewTCPTest()
	tb.Servers, tb.Roster, _ = tb.Local.GenTree(nbrHosts, true)

	tb.Client = skipchain.NewClient()
	log.Lvl2("Creating root and control chain")
	var cerr onet.ClientError
	tb.Genesis, cerr = tb.Client.CreateGenesis(tb.Roster, height, height, libsc.VerificationNone, nil, nil)
	log.ErrFatal(cerr)
	tb.Skipblocks[0] = tb.Genesis
	tb.Bunch = libsc.NewSkipBlockBunch(tb.Genesis)
	log.ErrFatal(cerr)
	for i := 1; i <= nbrAddSB; i++ {
		log.Lvl2("Creating skipblock", i+1)
		var cerr onet.ClientError
		_, tb.Skipblocks[i], cerr = tb.Client.AddSkipBlock(tb.Skipblocks[i-1], nil, nil)
		tb.Bunch.Store(tb.Skipblocks[i])
		log.ErrFatal(cerr)
	}
	// Get all skipblocks with all updated forward-links
	for i, sb := range tb.Skipblocks {
		sbNew, cerr := tb.Client.GetSingleBlock(tb.Roster, sb.Hash)
		log.ErrFatal(cerr)
		tb.Skipblocks[i] = sbNew
		tb.Bunch.Store(sbNew)
		tb.Storage.Store(sbNew)
	}
	return tb
}

func (tb *TestBunch) End() {
	tb.WaitPropagated(tb.Skipblocks)
	sbs := []*libsc.SkipBlock{}
	for _, sb := range tb.Bunch.SkipBlocks {
		sbs = append(sbs, sb)
	}
	tb.WaitPropagated(sbs)
	tb.Local.CloseAll()
}

func (tb *TestBunch) WaitPropagated(sbs []*libsc.SkipBlock) {
	allPropagated := false
	for !allPropagated {
		allPropagated = true
		for _, sb := range sbs {
			for _, si := range tb.Roster.List {
				reply := &skipchain.GetBlocksReply{}
				gb := &skipchain.GetBlocks{Start: nil, End: sb.Hash, MaxHeight: 0}
				cerr := tb.Client.SendProtobuf(si, gb, reply)
				if cerr != nil {
					log.Error(cerr)
					allPropagated = false
				} else if len(reply.Reply) == 0 {
					log.LLvl3("no block yet")
					allPropagated = false
				} else if !sb.Equal(reply.Reply[0]) {
					log.LLvl3("not same block")
					allPropagated = false
				}
			}
		}
	}
}
