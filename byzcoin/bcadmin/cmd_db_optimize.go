package main

import (
	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

type dbOptArgs struct {
	fb                *fetchBlocks
	currentSB         *skipchain.SkipBlock
	skipMissing       bool
	updatedFromRoster int
	optimized         int
	start             int
	stop              int
}

func newDbOptArgs(c *cli.Context) (dba dbOptArgs, err error) {
	dba.fb, err = newFetchBlocks(c)
	if err != nil {
		return dba, xerrors.Errorf("couldn't create fetchBlocks: %v", err)
	}

	fb := dba.fb
	if fb.bcID == nil {
		return dba, xerrors.New("need bcID")
	}
	fb.cl.UseNode(len(fb.latest.Roster.List))

	currentSB := fb.genesis
	if dba.start = c.Int("start"); dba.start > 0 {
		log.Infof("Getting start block %d", dba.start)
		reply, err := fb.cl.GetSingleBlockByIndex(fb.latest.Roster, *fb.bcID,
			dba.start)
		if err != nil {
			return dba, xerrors.Errorf("Couldn't get index %d: %v", dba.start,
				err)
		}
		currentSB = reply.SkipBlock
		fb.db.Store(currentSB)
	}

	dba.stop = fb.latest.Index - 1
	if stopFlag := c.Int("stop"); stopFlag >= 0 {
		if stopFlag < currentSB.Index {
			return dba, xerrors.New("stop cannot be smaller than start")
		}
		if stopFlag > dba.stop {
			log.Warn("given stop value is after known chain")
		}
		dba.stop = stopFlag
	}

	dba.skipMissing = c.Bool("skipMissing")
	dba.currentSB = currentSB
	return
}

// Tries the best to optimize a block.
// First it checks if there is an updated block in the chain.
// Then it queries all nodes with a request to update the forward-links.
func (dba *dbOptArgs) optimizeBlock() error {
	fb := dba.fb

	currentSB := dba.currentSB
	optimal := getOptimalHeight(currentSB, fb.latest.Index)
	if currentSB.GetForwardLen() < optimal {
		log.Infof("Block %d only has %d out of %d links.",
			currentSB.Index, currentSB.GetForwardLen(), optimal)

		updated := dba.queryOptimizedBlock(optimal)
		if !updated {
			if err := dba.askOptimization(optimal); err != nil {
				log.Warnf("Failed to optimize: %v", err)
			}
		}
	}

	return nil
}

func (dba *dbOptArgs) getNextBlock() error {
	currentSB := dba.currentSB

	if currentSB.GetForwardLen() == 0 {
		return xerrors.Errorf("Found block with no forward-links before" +
			" stop reached.")
	}

	fb := dba.fb
	nextSB := fb.db.GetByID(currentSB.ForwardLink[0].To)
	if nextSB != nil {
		dba.currentSB = nextSB
		return nil
	}

	if dba.skipMissing {
		if currentSB.GetForwardLen() == 1 {
			return xerrors.Errorf(
				"cannot skip missing block %d because no forward links"+
					" available", currentSB.Index+1)
		}
		for _, fl := range currentSB.ForwardLink {
			if currentSB = fb.db.GetByID(fl.To); currentSB != nil {
				dba.currentSB = currentSB
				return nil
			}
		}
		return xerrors.Errorf(
			"tried all forward links to jump over missing blocks")
	}

	newSB, err := fb.cl.GetSingleBlock(fb.latest.Roster, currentSB.Hash)
	if err != nil {
		return xerrors.Errorf("couldn't get next block: %v", err)
	}
	dba.currentSB = newSB
	return nil
}

// Try all nodes in the latest roster to get an up-to-date block.
func (dba *dbOptArgs) queryOptimizedBlock(optimal int) bool {
	log.Info("Fetching block from roster")
	fb := dba.fb

	for _, si := range fb.latest.Roster.List {
		roster := onet.NewRoster([]*network.ServerIdentity{si})
		reply, err := fb.cl.GetSingleBlock(roster, dba.currentSB.Hash)
		if err != nil {
			log.Warnf("Node %s returns error: %v", si, err)
			continue
		}

		if reply.GetForwardLen() == optimal {
			fb.db.Store(reply)
			dba.updatedFromRoster++
			dba.currentSB = reply
			return true
		}
	}

	return false
}

// Tries to update the block.
func (dba *dbOptArgs) askOptimization(optimal int) error {
	log.Infof("Roster doesn't know of a better block - optimizing")
	fb := dba.fb
	currentSB := dba.currentSB
	updates, err := dba.optimizeProof()
	if err != nil {
		return xerrors.Errorf("couldn't optimize proof; %v", err)
	}

	// The optimization might fail because the roster is not
	// online anymore.
	updatedSB := updates.Search(currentSB.Index)
	if updatedSB == nil || updatedSB.GetForwardLen() < optimal {
		return xerrors.New("the optimization failed")
	}
	if _, err := fb.db.StoreBlocks(updates); err != nil {
		return xerrors.Errorf(
			"couldn't store updated blocks: %v", err)
	}

	dba.optimized++
	dba.currentSB = updatedSB
	return nil
}

// This tries to call all nodes until one can optimize the block.
func (dba *dbOptArgs) optimizeProof() (skipchain.Proof, error) {
	sb := dba.currentSB
	cl := dba.fb.cl
	for _, si := range sb.Roster.List {
		roster := onet.NewRoster([]*network.ServerIdentity{si})
		update, err := cl.OptimizeProof(roster, sb.Hash)
		if err != nil {
			log.Warnf("Couldn't contact %s: %v", si.Address, err)
			continue
		}
		if update.Proof.Search(sb.Index).GetForwardLen() <= sb.GetForwardLen() {
			log.Warnf("Node %s didn't create higher forward link", si.Address)
			continue
		}
		return update.Proof, nil
	}
	return nil, xerrors.New("couldn't get any of the nodes to optimize")
}
