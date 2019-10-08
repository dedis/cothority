package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strings"

	"go.dedis.ch/kyber/v3/pairing"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.etcd.io/bbolt"
	"golang.org/x/xerrors"
)

// dbStatus returns the last block in the db - useful for the integration tests.
func dbStatus(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	last, err := fb.db.GetLatest(fb.genesis)
	if err != nil {
		return xerrors.Errorf("couldn't get latest block: %+v", err)
	}
	log.Infof("Last block is: %d / %x", last.Index, last.Hash)
	return nil
}

// dbCatchup uses the live byzcoin chain to update to the latest blocks
func dbCatchup(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: " +
			"conode.db byzCoinID url")
	}
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}
	err = fb.addURL(c.Args().Get(2))
	if err != nil {
		return xerrors.Errorf("couldn't add URL connection: %+v", err)
	}

	log.Info("Search for latest block")
	latestID := *fb.bcID
	lastIndex := 0
	for next := fb.db.GetByID(latestID); next != nil && len(next.ForwardLink) > 0; next = fb.db.GetByID(latestID) {
		lastIndex = next.Index
		latestID = next.ForwardLink[0].To
		if next.Index%1000 == 0 {
			log.Info("Found block", next.Index)
		}
	}
	log.Info("Last index", lastIndex)

	for {
		sb, err := fb.gbMulti(latestID)
		if err != nil {
			return xerrors.Errorf("couldn't get blocks from network: %+v", err)
		}
		if len(sb.ForwardLink) == 0 {
			break
		}
		latestID = sb.ForwardLink[0].To
	}

	return nil
}

// dbReplay re-applies all the contracts to the new blocks available.
func dbReplay(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return errors.New("couldn't initialize fetchBlocks: " + err.Error())
	}

	log.Info("Preparing db")
	start := *fb.bcID
	err = fb.boltDB.Update(func(tx *bbolt.Tx) error {
		if !fb.flagReplayCont {
			if tx.Bucket(fb.bucketName) != nil {
				err := tx.DeleteBucket(fb.bucketName)
				if err != nil {
					return err
				}
			}
		}
		_, err := tx.CreateBucketIfNotExists(fb.bucketName)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return xerrors.Errorf("couldn't add bucket: %+v", err)
	}

	if !fb.flagReplayCont {
		_, err = fb.service.ReplayStateDB(fb.boltDB, fb.bucketName, fb.genesis)
		if err != nil {
			return xerrors.Errorf("couldn't create stateDB: %+v", err)
		}
	} else {
		index, err := fb.service.ReplayStateDB(fb.boltDB, fb.bucketName, nil)
		if err != nil {
			return xerrors.Errorf("couldn't replay blocks: %+v", err)
		}

		log.Info("Searching for block with index", index+1)
		sb := fb.db.GetByID(start)
		for sb != nil && sb.Index < index+1 {
			if len(sb.ForwardLink) == 0 {
				break
			}
			sb = fb.db.GetByID(sb.ForwardLink[0].To)
			start = sb.Hash
		}
		if sb.Index <= index {
			log.Info("No new blocks available")
			return nil
		}
	}

	log.Info("Replaying blocks")
	_, err = fb.service.ReplayStateCont(start, fb.blockFetcher)
	if err != nil {
		return xerrors.Errorf("couldn't replay blocks: %+v", err)
	}
	log.Info("Successfully checked and replayed all blocks.")

	return nil
}

// dbMerge takes new blocks from a conode-db and applies them to the replay-db.
func dbMerge(c *cli.Context) error {
	if c.NArg() < 3 {
		return errors.New("please give the following arguments: " +
			"conode.db byzCoinID conode2.db")
	}

	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	dbBack, _, err := fb.openDB(c.Args().Get(2))
	if err != nil {
		return xerrors.Errorf("couldn't open DB: %+v", err)
	}
	sb := dbBack.GetByID(*fb.bcID)
	if sb != nil {
		sbLatest, err := fb.db.GetLatest(sb)
		if err != nil {
			log.Warn("Couldn't get latest block:", err)
		} else {
			sb = dbBack.GetByID(sbLatest.Hash)
		}
	}
	var latest int
	var blocks int
	maxBlocks := c.Int("blocks")
	if c.Bool("overwrite") {
		err = fb.db.RemoveSkipchain(*fb.bcID)
		if err != nil {
			return xerrors.Errorf("couldn't remove skipchain: %+v", err)
		}
		sb = dbBack.GetByID(*fb.bcID)
		for sb != nil {
			blocks++
			latest = sb.Index
			if blocks%100 == 0 {
				log.Infof("Stored %d blocks so far", blocks)
			}
			fb.db.Store(sb)
			if maxBlocks > 0 && blocks == maxBlocks+1 {
				log.Infof("Stopping after applying %d blocks", maxBlocks)
				break
			}
			if len(sb.ForwardLink) > 0 {
				sb = dbBack.GetByID(sb.ForwardLink[0].To)
			} else {
				break
			}
		}
	} else {
		for sb != nil {
			latest = sb.Index
			blocks++
			fb.db.Store(sb)
			if len(sb.ForwardLink) > 0 {
				sb = dbBack.GetByID(sb.ForwardLink[0].To)
			} else {
				break
			}
			if blocks%100 == 0 {
				log.Infof("Stored %d blocks - latest block-index is: %d",
					blocks, latest)
			}
		}
	}
	log.Infof("Found %d blocks in backup. Latest index: %d", blocks,
		latest)
	return nil
}

// dbCheck verifies all the hashes and links from the blocks.
func dbCheck(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	if fb.bcID == nil {
		return xerrors.New("need bcID")
	}

	sb := fb.db.GetByID(*fb.bcID)
	start := c.Int("start")
	for sb.Index < start {
		if len(sb.ForwardLink) == 0 {
			return xerrors.Errorf("cannot find block %d", start)
		}
		sb = fb.db.GetByID(sb.ForwardLink[0].To)
	}

	blocks := 0
	process := c.Int("process")
	for sb != nil {
		blocks++
		if blocks%process == 0 {
			log.Infof("Processed %d blocks so far", blocks)
		}
		errStr := fmt.Sprintf("found block %d with", sb.Index)
		if !sb.Hash.Equal(sb.CalculateHash()) {
			log.Errorf("%s wrong hash: %x instead"+
				" of %x", errStr, sb.Hash, sb.CalculateHash())
		}

		if sb.Index > 0 {
			if !sb.SkipChainID().Equal(*fb.bcID) {
				log.Errorf(
					"%s different skipchain-id: %x", errStr, sb.SkipChainID())
			}
			for i, bl := range sb.BackLinkIDs {
				errStrBl := fmt.Sprintf("%s with backlink at height %d",
					errStr, i)
				previous := fb.db.GetByID(bl)
				if previous == nil {
					log.Errorf(
						"%s pointing to unknown block", errStrBl)
					continue
				}
				if len(previous.ForwardLink) <= i {
					log.Warnf("%s points to block with not enough forward"+
						"-links", errStrBl)
					continue
				}
				if previous.ForwardLink[i].IsEmpty() {
					continue
				}
				if !previous.ForwardLink[i].To.Equal(sb.Hash) {
					log.Errorf(
						"%s pointing backwards to a block that doesn't"+
							" reference it", errStrBl)
				}
			}
		}
		for i, fl := range sb.ForwardLink {
			if fl.IsEmpty() {
				continue
			}
			errStrFl := fmt.Sprintf("%s forwardLink at height %d", errStr, i)
			if !fl.From.Equal(sb.Hash) {
				log.Errorf(
					"%s not originating from itself", errStrFl)
			}
			next := fb.db.GetByID(fl.To)
			if next == nil {
				log.Errorf("%s pointing to an unknown block",
					errStrFl)
				continue
			}
			err := fl.VerifyWithScheme(pairing.NewSuiteBn256(),
				sb.Roster.ServicePublics(skipchain.ServiceName),
				sb.SignatureScheme)
			if err != nil {
				log.Errorf("%s fails signature verification: %+v",
					errStrFl, err)
			}
			if fl.NewRoster != nil {
				equal, err := fl.NewRoster.Equal(next.Roster)
				if err != nil {
					log.Errorf(
						"%s cannot test if new roster is the same: %+v",
						errStrFl, err)
					continue
				}
				if !equal {
					log.Errorf("%s contains wrong roster", errStrFl)
				}
			}
		}
		if len(sb.ForwardLink) > 0 {
			sb = fb.db.GetByID(sb.ForwardLink[0].To)
		} else {
			break
		}
	}
	log.Infof("Checked successfully all blocks: %d", blocks)
	return nil
}

// fetchBlocks is used by all db-related bcadmin commands.
type fetchBlocks struct {
	cl               *skipchain.Client
	service          *byzcoin.Service
	bcID             *skipchain.SkipBlockID
	local            *onet.LocalTest
	roster           *onet.Roster
	genesis          *skipchain.SkipBlock
	latest           *skipchain.SkipBlock
	single           int
	index            int
	boltDB           *bbolt.DB
	db               *skipchain.SkipBlockDB
	bucketName       []byte
	flagCatchupBatch int
	flagReplayBlocks int
	flagReplayCont   bool
}

func newFetchBlocks(c *cli.Context) (*fetchBlocks,
	error) {
	if c.NArg() < 1 {
		return nil, errors.New("please give the following arguments: " +
			"conode.db [byzCoinID]")
	}

	fb := &fetchBlocks{
		local:            onet.NewLocalTest(cothority.Suite),
		bucketName:       []byte("replayStateBucket"),
		flagCatchupBatch: c.Int("batch"),
		flagReplayBlocks: c.Int("blocks"),
		flagReplayCont:   c.Bool("continue"),
	}

	var err error
	servers := fb.local.GenServers(1)
	fb.service = servers[0].Service(byzcoin.ServiceName).(*byzcoin.Service)

	fb.db, fb.boltDB, err = fb.openDB(c.Args().First())
	if err != nil {
		return nil, xerrors.Errorf("couldn't open DB: %+v", err)
	}

	if c.NArg() >= 2 {
		var bi skipchain.SkipBlockID
		bi, err = hex.DecodeString(c.Args().Get(1))
		fb.bcID = &bi
		fb.genesis = fb.db.GetByID(*fb.bcID)
	}

	if c.Command.Name != "catchup" {
		err := fb.needBcID()
		if err != nil {
			return nil, xerrors.Errorf("couldn't check bcID: %+v", err)
		}
	}
	return fb, nil
}

func (fb *fetchBlocks) needBcID() error {
	if fb.bcID != nil {
		return nil
	}
	var scIDs []string
	sbs, err := fb.db.GetSkipchains()
	if err != nil {
		return xerrors.Errorf("couldn't list all blocks: %+v", err)
	}
	for _, sb := range sbs {
		if sb.Index == 0 {
			scIDs = append(scIDs, hex.EncodeToString(sb.SkipChainID()))
		}
	}
	log.Info("The following chains are available in your db:",
		strings.Join(scIDs, "\n"))
	return errors.New("need byzCoinID in arguments")
}

func (fb *fetchBlocks) addURL(url string) error {
	if url == "" {
		return errors.New("cannot use empty url")
	}

	if fb.bcID == nil {
		fs := &flag.FlagSet{}
		if err := fs.Parse([]string{url}); err != nil {
			return xerrors.Errorf("couldn't parse url: %+v", err)
		}
		c := cli.NewContext(nil, fs, nil)
		err := debugList(c)
		if err != nil {
			return err
		}

		log.Info("Please provide one of the following byzcoin ID as the second argument")
		return nil
	}

	fb.cl = skipchain.NewClient()
	fb.roster = onet.NewRoster([]*network.ServerIdentity{{
		Address: network.NewAddress(network.TLS, url),
		URL:     url,
		// valid server identity must have a public so we create a fake one
		// as we are only interested in the URL.
		Public: cothority.Suite.Point().Base(),
	}})
	updateChain, err := fb.cl.GetUpdateChain(fb.roster, *fb.bcID)
	if err != nil {
		return xerrors.Errorf("couldn't get latest block: %+v", err)
	}
	fb.roster.List = []*network.ServerIdentity{}
	for _, sb := range updateChain.Update {
		for _, id := range sb.Roster.List {
			if i, _ := fb.roster.Search(id.ID); i < 0 {
				fb.roster.List = append(fb.roster.List, id)
			}
		}
	}
	fb.genesis = updateChain.Update[0]
	fb.latest = updateChain.Update[len(updateChain.Update)-1]
	fb.cl.UseNode(0)
	return nil
}

func (fb *fetchBlocks) blockFetcher(sib skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	fb.flagReplayBlocks--
	if fb.flagReplayBlocks == 0 {
		log.Info("reached end of task")
		return nil, nil
	}
	sb := fb.db.GetByID(sib)
	if sb == nil {
		return nil, nil
	}
	return sb, nil
}

func (fb *fetchBlocks) openDB(name string) (*skipchain.SkipBlockDB,
	*bbolt.DB, error) {
	db, err := bbolt.Open(name, 0600, nil)
	if err != nil {
		return nil, nil, xerrors.Errorf("couldn't open db: %+v", err)
	}
	bucketName := []byte("Skipchain_skipblocks")
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		return nil, nil, xerrors.Errorf("couldn't create bucket: %+v", err)
	}
	return skipchain.NewSkipBlockDB(db, bucketName), db, nil
}

func (fb *fetchBlocks) nextNode() {
	fb.index = (fb.index + 1) % len(fb.roster.List)
	fb.cl.UseNode(fb.index)
}

func (fb *fetchBlocks) gbMulti(startID skipchain.SkipBlockID) (
	*skipchain.SkipBlock, error) {
	for range fb.roster.List {
		blocks, err := fb.cl.GetUpdateChainLevel(fb.roster, startID,
			1, fb.flagCatchupBatch)
		log.Lvl2("got", len(blocks), "blocks")
		ok := false
		if err != nil {
			log.Warn("Got error while fetching blocks:", err)
		} else {
			ok = true
			start := blocks[0]
			for i, sb := range blocks {
				if sb.Index != start.Index+i {
					log.Warn("Blocks out of order", sb.Index, i,
						start.Index)
					ok = false
					break
				}
			}
		}

		if !ok {
			fb.nextNode()
			continue
		}

		_, err = fb.db.StoreBlocks(blocks)
		if err != nil {
			return nil, xerrors.Errorf("couldn't store blocks: %+v", err)
		}

		url := fb.roster.List[fb.index].URL
		if url == "" {
			url = fb.roster.List[fb.index].Address.String()
		}
		log.Infof("Got %d blocks from %s, starting at index %d",
			len(blocks), fb.roster.List[fb.index].Address, blocks[0].Index)
		return blocks[len(blocks)-1], nil
	}
	return fb.gbSingle(startID)
}

func (fb *fetchBlocks) gbSingle(blockID skipchain.SkipBlockID) (
	*skipchain.SkipBlock, error) {
	var sb *skipchain.SkipBlock
	for i := 0; i < fb.flagCatchupBatch; i++ {
		for range fb.roster.List {
			log.Infof("Getting single block %x from %d",
				blockID, fb.index)
			var err error
			sb, err = fb.cl.GetSingleBlock(fb.roster, blockID)
			if err != nil {
				fb.nextNode()
				continue
			}
			_, err = fb.db.StoreBlocks([]*skipchain.SkipBlock{sb})
			if err != nil {
				return nil, xerrors.Errorf("couldn't store blocks: %+v", err)
			}
			if len(sb.ForwardLink) == 0 {
				return sb, nil
			}
			break
		}
		if sb != nil && !sb.ForwardLink[0].To.Equal(blockID) {
			blockID = sb.ForwardLink[0].To
		} else {
			return sb, errors.New("couldn't fetch next block")
		}
	}
	return sb, nil
}
