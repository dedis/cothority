package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go.dedis.ch/cothority/v3/byzcoin/trie"
	"strconv"
	"strings"
	"time"

	"go.dedis.ch/protobuf"

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
	if c.NArg() < 1 {
		return xerrors.New("please give the following arguments: " +
			"conode.db [byzCoinID [url]]")
	}
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	askAllNodes := true
	if c.NArg() == 3 {
		askAllNodes = false
		err = fb.addURL(c.Args().Get(2))
		if err != nil {
			return xerrors.Errorf("couldn't add URL connection: %+v", err)
		}
	}

	log.Info("Search for latest block in local db")
	latestID := *fb.bcID
	lastIndex := 0
	for next := fb.db.GetByID(latestID); next != nil && len(next.ForwardLink) > 0; next = fb.db.GetByID(latestID) {
		lastIndex = next.Index
		latestID = next.ForwardLink[0].To
		if next.Index%1000 == 0 {
			log.Info("Found block", next.Index)
		}
	}
	log.Info("Last index in local db", lastIndex)

	fb.setNode(0)
	for {
		sb, err := fb.gbMulti(latestID)
		if err != nil {
			return xerrors.Errorf("couldn't get blocks from network: %+v", err)
		}
		if sb == nil {
			break
		}
		if len(sb.ForwardLink) == 0 {
			if askAllNodes {
				// If no further blocks exist,
				// and no node is given on the command-line,
				// ask all other nodes first.
				fb.roster = sb.Roster
				fb.index++
				if fb.index >= len(sb.Roster.List) {
					break
				}
				log.Info("Got possible latest block - getting confirmation"+
					" from", fb.roster.List[fb.index].URL)
				fb.cl.UseNode(fb.index)
				latestID = sb.Hash
				continue
			} else {
				break
			}
		} else {
			fb.index = 0
		}
		latestID = sb.ForwardLink[0].To
	}

	log.Info("Downloaded all available blocks from the chain")
	return nil
}

// dbReplay re-applies all the contracts to the new blocks available.
func dbReplay(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't initialize fetchBlocks: %v", err)
	}

	log.Info("Preparing db")
	start := *fb.bcID
	err = fb.boltDB.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket(fb.bucketName) != nil {
			err := tx.DeleteBucket(fb.bucketName)
			if err != nil {
				return err
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

	log.Lvl2("Copying blocks to skipchain's DB")
	fb.skipchain.GetDB().DB = fb.db.DB

	log.Info("Replaying blocks")
	rso := byzcoin.ReplayStateOptions{
		MaxBlocks:   c.Int("blocks"),
		VerifyFLSig: c.Bool("verifyFLSig"),
	}
	if c.Bool("continue") {
		rso.StartingTrie = fb.trieDB
	}
	st, err := fb.service.ReplayState(start, newSumFetcher(c), rso)
	if err != nil {
		return xerrors.Errorf("couldn't replay blocks: %+v", err)
	}
	log.Info("Successfully checked and replayed all blocks.")
	if c.Bool("write") {
		log.Info("Writing new stateTrie to DB")
		err := fb.boltDB.Update(func(tx *bbolt.Tx) error {
			if tx.Bucket(fb.trieBucketName) != nil {
				err := tx.DeleteBucket(fb.trieBucketName)
				if err != nil {
					return fmt.Errorf("while deleting bucket: %v", err)
				}
			}
			bucket, err := tx.CreateBucket(fb.trieBucketName)
			if err != nil {
				return fmt.Errorf("while creating bucket: %v", err)
			}
			return st.View(func(b trie.Bucket) error {
				return b.ForEach(func(k, v []byte) error {
					return bucket.Put(k, v)
				})
			})
		})
		if err != nil {
			return fmt.Errorf("couldn't update bucket: %v", err)
		}
	}

	return nil
}

type sumFetcher struct {
	summarizeBlocks int
	totalTXs        int
	accepted        int
	seenBlocks      int
	timeLastBlock   int64
	timeLastSum     int64
	maxTPS          float64
	maxBlockSize    int
	totalBlockSize  int
}

func newSumFetcher(c *cli.Context) *sumFetcher {
	return &sumFetcher{summarizeBlocks: c.Int("summarize")}
}

func (sf sumFetcher) LogNewBlock(sb *skipchain.SkipBlock) {
	if sf.summarizeBlocks == 1 {
		log.Infof("Replaying block at index %d", sb.Index)
	}
}

func (sf sumFetcher) LogWarn(sb *skipchain.SkipBlock, msg, dump string) {
	log.Infof("Warning for block %d: %s", sb.Index, msg)
}

func (sf *sumFetcher) LogAppliedBlock(sb *skipchain.SkipBlock,
	head byzcoin.DataHeader, body byzcoin.DataBody) {

	for _, tx := range body.TxResults {
		if tx.Accepted {
			sf.accepted++
		}
	}
	buf, err := protobuf.Encode(sb)
	log.ErrFatal(err, "while encoding block")
	if sf.summarizeBlocks > 1 {
		sf.totalBlockSize += len(buf)
		if len(buf) > sf.maxBlockSize {
			sf.maxBlockSize = len(buf)
		}
	}
	sf.totalTXs += len(body.TxResults)
	sf.seenBlocks++
	if sf.timeLastBlock == 0 {
		sf.timeLastBlock = head.Timestamp
		sf.timeLastSum = head.Timestamp
		sf.maxTPS = 0
	}

	if sf.summarizeBlocks > 1 && head.Timestamp != sf.timeLastBlock {
		tpsBlock := float64(len(body.TxResults)) /
			float64((head.Timestamp-sf.timeLastBlock)/1e9)
		if tpsBlock > sf.maxTPS {
			sf.maxTPS = tpsBlock
		}
		sf.timeLastBlock = head.Timestamp
	}

	if sb.Index%sf.summarizeBlocks == (sf.summarizeBlocks - 1) {
		tStr := time.Unix(head.Timestamp/1e9, 0).String()
		tpsMean := float64(sf.totalTXs) /
			float64((head.Timestamp-sf.timeLastSum)/1e9)
		if sf.summarizeBlocks > 1 {
			log.Infof("Processed blocks %d.."+
				"%d [%s]: Txs total/accepted = %d/%d - "+
				"tps max/mean: %.1f/%.1f\n"+
				"\troster-size: %d, max/mean block size: %d/%d",
				sb.Index-sf.seenBlocks+1, sb.Index, tStr,
				sf.totalTXs, sf.accepted, sf.maxTPS, tpsMean,
				len(sb.Roster.List),
				sf.maxBlockSize, sf.totalBlockSize/sf.summarizeBlocks)
		} else {
			log.Infof("Got correct block from %s with %d txs, "+
				"out of which %d txs got accepted. Tps: %.1f",
				tStr, sf.totalTXs, sf.accepted, tpsMean)
		}
		sf.totalTXs = 0
		sf.accepted = 0
		sf.seenBlocks = 0
		sf.timeLastBlock = head.Timestamp
		sf.timeLastSum = head.Timestamp
		sf.maxTPS = 0.0
		sf.maxBlockSize = 0
		sf.totalBlockSize = 0
	}
}

// dbMerge takes new blocks from a conode-db and applies them to the replay-db.
func dbMerge(c *cli.Context) error {
	if c.NArg() < 3 {
		return xerrors.New("please give the following arguments: " +
			"conode.db byzCoinID conode2.db")
	}

	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	// Open backup db and get starting block
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
	if sb == nil {
		return xerrors.New("something went wrong - didn't find skipblock")
	}

	// Get how many blocks to copy
	var blocks int
	copyBlocks := c.Int("blocks")
	if copyBlocks == 0 {
		if c.Bool("append") {
			latest, err := dbBack.GetLatestByID(sb.SkipChainID())
			if err != nil {
				return xerrors.Errorf(
					"couldn't get latest block from backup: %v", err)
			}
			copyBlocks = latest.Index - sb.Index
			if copyBlocks < 0 {
				return xerrors.New("no blocks to append in backup-db")
			}
		} else {
			copyBlocks = sb.Index
		}
	}

	// Apply the flags
	if c.Bool("wipe") {
		err = fb.db.RemoveSkipchain(*fb.bcID)
		if err != nil {
			return xerrors.Errorf("couldn't remove skipchain: %+v", err)
		}
	}
	if !c.Bool("append") {
		sb = dbBack.GetByID(*fb.bcID)
	}

	// Copy blocks
	log.Infof("Going to copy up to %d blocks, starting at index %d",
		copyBlocks, sb.Index)
	for sb != nil {
		blocks++
		if blocks%1000 == 0 {
			log.Infof("Stored %d blocks so far", blocks)
		}
		fb.db.Store(sb)
		if blocks > copyBlocks {
			log.Infof("Stopping after applying %d blocks", copyBlocks)
			break
		}
		if len(sb.ForwardLink) > 0 {
			sb = dbBack.GetByID(sb.ForwardLink[0].To)
		} else {
			break
		}
	}

	log.Infof("Copied %d blocks from backup.", blocks)
	return nil
}

// dbReset removes dangling forward-links from the db
func dbReset(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	if fb.bcID == nil {
		return xerrors.New("need bcID")
	}

	latest := *fb.bcID
	var previous skipchain.SkipBlockID
	for {
		sb := fb.db.GetByID(latest)
		if sb == nil {
			if previous == nil {
				return xerrors.New("didn't find first block")
			}
			sb = fb.db.GetByID(previous)
			log.Info("Found dangling forward-link in block", sb.Index)
			sb.ForwardLink = []*skipchain.ForwardLink{}
			err = fb.db.RemoveBlock(previous)
			if err != nil {
				return xerrors.Errorf("couldn't remove block: %v", err)
			}
			fb.db.Store(sb)
			log.Info("DB updated")
			break
		}
		if len(sb.ForwardLink) == 0 {
			return xerrors.New("no dangling forward-links")
		}
		previous = latest
		latest = sb.ForwardLink[len(sb.ForwardLink)-1].To
	}

	return nil
}

// dbRemove deletes a number of blocks from the end
func dbRemove(c *cli.Context) error {
	fb, err := newFetchBlocks(c)
	if err != nil {
		return xerrors.Errorf("couldn't create fetchBlock: %+v", err)
	}

	if fb.bcID == nil {
		return xerrors.New("need bcID")
	}

	latest, err := fb.db.GetLatestByID(*fb.bcID)
	if err != nil {
		return fmt.Errorf("couldn't get latest block: %v", err)
	}

	blocks := 1
	if c.NArg() > 2 {
		blocks, err = strconv.Atoi(c.Args().Get(2))
		if err != nil {
			return fmt.Errorf("couldn't convert %s to int: %v",
				c.Args().Get(2), err)
		}
		if blocks >= latest.Index {
			return errors.New("cannot delete up the genesis block or earlier")
		}
	}

	// Checking the removal of blocks will not lead to an unrecoverable state
	// of the node.
	tr := trie.NewDiskDB(fb.boltDB, fb.trieBucketName)
	err = tr.View(func(b trie.Bucket) error {
		buf := b.Get([]byte("trieIndexKey"))
		if buf == nil {
			return errors.New("couldn't get index key")
		}
		if latest.Index-blocks <= int(binary.LittleEndian.Uint32(buf)) {
			return errors.New("need to keep blocks up to the trie-state." +
				"\nUse `bcadmin db replay --write` to replay the db to a" +
				" previous state")
		}
		return nil
	})
	if err != nil {
		return err
	}

	for i := 0; i < blocks; i++ {
		log.Info("deleting block", latest.Index)
		if err := fb.db.RemoveBlock(latest.Hash); err != nil {
			return fmt.Errorf("couldn't remove block: %v", err)
		}
		for i, bl := range latest.BackLinkIDs {
			tmp := fb.db.GetByID(bl)
			if tmp == nil {
				return errors.New("didn't find backlink")
			}
			log.Info("Removing backlink", i, tmp.Index)
			if err := fb.db.RemoveBlock(bl); err != nil {
				return fmt.Errorf("couldn't remove block: %v", err)
			}
			tmp.ForwardLink = tmp.ForwardLink[:i]
			fb.db.Store(tmp)
		}
		latest = fb.db.GetByID(latest.BackLinkIDs[0])
		if latest == nil {
			return errors.New("couldn't get previous block")
		}
	}

	return fb.db.Close()
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

	last, err := fb.db.GetLatestByID(sb.SkipChainID())
	if err != nil {
		return fmt.Errorf("couldn't get last block: %v", err)
	}

	blocks := 0
	process := c.Int("process")
	skipSig := c.Bool("skipSig")
	for sb != nil {
		blocks++
		if blocks%process == 0 {
			log.Infof("Processed %d blocks so far", blocks)
		}

		// Basic sanity check of the block header
		errStr := fmt.Sprintf("found block %d with", sb.Index)
		if !sb.Hash.Equal(sb.CalculateHash()) {
			log.Errorf("%s wrong hash: %x instead"+
				" of %x", errStr, sb.Hash, sb.CalculateHash())
		}

		// Check backward-links for all blocks except the genesis-block,
		// as its backward-link only serves as random input to the hash of
		// the genesis block.
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
					continue
				}
				if previous.ForwardLink[i].IsEmpty() {
					continue
				}
				if !previous.ForwardLink[i].To.Equal(sb.Hash) {
					log.Errorf(
						"%s pointing backwards to a block that references"+
							" another block, which indicates a fork", errStrBl)
				}
			}
		}

		// Verify available forward-links to match the backward-links of the
		// blocks they point to.
		// If enabled, also verify the signature of the forward-links.
		indexes := sb.GetFLIndexes()
		for i, index := range indexes {
			if index > last.Index {
				break
			}
			errStrFl := fmt.Sprintf("%s forwardLink at height %d", errStr, i)

			if i >= sb.GetForwardLen() || sb.ForwardLink[i].IsEmpty() {
				log.Warnf("%s missing: should point to %d", errStrFl,
					index)
				continue
			}

			fl := sb.ForwardLink[i]
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
			if len(next.BackLinkIDs) <= i {
				log.Errorf("%s points to a block with too few backlinks", errStrFl)
				continue
			}
			if !next.BackLinkIDs[i].Equal(sb.Hash) {
				log.Errorf("%s points to block %d which doesn't point back",
					errStrFl, next.Index)
			}

			if !skipSig {
				err := fl.VerifyWithScheme(pairing.NewSuiteBn256(),
					sb.Roster.ServicePublics(skipchain.ServiceName),
					sb.SignatureScheme)
				if err != nil {
					log.Errorf("%s fails signature verification: %+v",
						errStrFl, err)
				}
			}

			if fl.NewRoster != nil {
				equal, err := fl.NewRoster.Equal(next.Roster)
				if err != nil {
					log.Errorf(
						"%s cannot test if roster in forward-link is the same"+
							" as the roster in the block it points to: %+v",
						errStrFl, err)
					continue
				}
				if !equal {
					log.Errorf("%s contains wrong roster", errStrFl)
				}
			}
		}

		// Get lowest forwardLink available.
		if len(sb.ForwardLink) > 0 {
			for i, fl := range sb.ForwardLink {
				if !fl.IsEmpty() {
					if i > 0 {
						log.Errorf("%s has empty level-0 forward link, "+
							"trying to continue anyway", errStr)
					}
					sb = fb.db.GetByID(fl.To)
				}
			}
		} else {
			break
		}
	}
	log.Infof("Checked successfully all blocks: %d", blocks)
	return nil
}

// dbCheck verifies all the hashes and links from the blocks.
func dbOptimize(c *cli.Context) error {
	dbArgs, err := newDbOptArgs(c)
	if err != nil {
		return xerrors.Errorf("couldn't create dbOptArgs: %v", err)
	}

	log.Infof("Starting to check blocks from %d to %d", dbArgs.currentSB.Index,
		dbArgs.stop)
	for dbArgs.currentSB.Index <= dbArgs.stop {
		if dbArgs.currentSB.Index%1000 == 0 {
			log.Infof("Checking block %d", dbArgs.currentSB.Index)
		}

		if err := dbArgs.optimizeBlock(); err != nil {
			return xerrors.Errorf("couldn't optimize block: %v", err)
		}

		if err := dbArgs.getNextBlock(); err != nil {
			return xerrors.Errorf("couldn't get next block: %v", err)
		}
	}

	log.Infof("Done optimizing blockchain %x", *dbArgs.fb.bcID)
	log.Infof("Updated blocks with missing links from network: %d",
		dbArgs.updatedFromRoster)
	log.Infof("Optimized blocks by creating new forward lins: %d",
		dbArgs.optimized)

	return nil
}

// Returns the optimal length given a latest block index.
func getOptimalHeight(block *skipchain.SkipBlock, latest int) int {
	indexes := block.GetFLIndexes()
	for i, index := range indexes {
		if index > latest {
			return i
		}
	}
	return len(indexes)
}

// fetchBlocks is used by all db-related bcadmin commands.
type fetchBlocks struct {
	cl               *skipchain.Client
	service          *byzcoin.Service
	skipchain        *skipchain.Service
	bcID             *skipchain.SkipBlockID
	local            *onet.LocalTest
	roster           *onet.Roster
	genesis          *skipchain.SkipBlock
	latest           *skipchain.SkipBlock
	index            int
	boltDB           *bbolt.DB
	db               *skipchain.SkipBlockDB
	bucketName       []byte
	flagCatchupBatch int
	trieDB           trie.DB
	trieBucketName   []byte
}

func newFetchBlocks(c *cli.Context) (*fetchBlocks,
	error) {
	if c.NArg() < 1 {
		return nil, xerrors.New("please give the following arguments: " +
			"conode.db [byzCoinID]")
	}

	fb := &fetchBlocks{
		local:            onet.NewLocalTest(cothority.Suite),
		bucketName:       []byte("replayStateBucket"),
		flagCatchupBatch: c.Int("batch"),
	}

	var err error
	servers := fb.local.GenServers(1)
	fb.service = servers[0].Service(byzcoin.ServiceName).(*byzcoin.Service)
	fb.skipchain = servers[0].Service(skipchain.ServiceName).(*skipchain.Service)

	log.Info("Opening database", c.Args().First())
	fb.db, fb.boltDB, err = fb.openDB(c.Args().First())
	if err != nil {
		return nil, xerrors.Errorf("couldn't open DB: %+v", err)
	}

	if c.NArg() >= 2 {
		var bi skipchain.SkipBlockID
		bi, err = hex.DecodeString(c.Args().Get(1))
		if err != nil {
			return nil, fmt.Errorf("couldn't decode skipchain-ID: %v", err)
		}
		fb.bcID = &bi
		fb.genesis = fb.db.GetByID(*fb.bcID)
		if fb.genesis != nil {
			fb.latest, err = fb.db.GetLatestByID(fb.genesis.SkipChainID())
			if err != nil {
				return nil, fmt.Errorf("couldn't get latest block: %v", err)
			}
			fb.roster = fb.latest.Roster
		}
	} else {
		err = fb.getBCID()
		if err != nil {
			return nil, fmt.Errorf("couldn't auto-detect skipchain-id: %v", err)
		}
	}

	if c.Command.Name != "catchup" {
		err := fb.needBcID()
		if err != nil {
			return nil, xerrors.Errorf("couldn't check bcID: %+v", err)
		}
	}

	fb.trieBucketName = []byte(fmt.Sprintf("ByzCoin_%x", *fb.bcID))
	fb.trieDB = trie.NewDiskDB(fb.boltDB, fb.trieBucketName)
	fb.cl = skipchain.NewClient()

	return fb, nil
}

func (fb *fetchBlocks) getBCID() error {
	log.Info("Searching for available skipchains")
	sbs, err := fb.db.GetSkipchains()
	if err != nil {
		return fmt.Errorf("couldn't search skipchains: %v", err)
	}
	switch len(sbs) {
	case 0:
		return errors.New("no skipchain stored - give id and URL")
	case 1:
		for sbID, sb := range sbs {
			bcID := skipchain.SkipBlockID(sbID)
			fb.bcID = &bcID
			fb.genesis = fb.db.GetByID(sb.GenesisID)
			fb.latest, err = fb.db.GetLatest(sb)
			if err != nil {
				return fmt.Errorf("couldn't get latest skipblock: %v", err)
			}
			fb.roster = fb.latest.Roster
		}
		log.Infof("Using single skipchain: %x", *fb.bcID)
	default:
		log.Info("More than one skipchain found - please chose one:")
		for sb := range sbs {
			log.Infof("SkipchainID: %x", []byte(sb))
		}
		return errors.New("more than 1 skipchain present in db")
	}
	return nil
}

func (fb *fetchBlocks) needBcID() error {
	if fb.bcID != nil {
		return nil
	}
	var scIDs []string
	log.Info("No bcID given - searching for all skipchains")
	sbs, err := fb.db.GetSkipchains()
	if err != nil {
		return xerrors.Errorf("couldn't list all blocks: %+v", err)
	}
	for _, sb := range sbs {
		scIDs = append(scIDs, hex.EncodeToString(sb.SkipChainID()))
	}
	log.Info("The following chains are available in your db:",
		strings.Join(scIDs, "\n"))
	return xerrors.New("need byzCoinID in arguments")
}

func (fb *fetchBlocks) addURL(url string) error {
	if url == "" {
		return xerrors.New("cannot use empty url")
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

func (fb *fetchBlocks) setNode(i int) {
	fb.index = i % len(fb.roster.List)
	fb.cl.UseNode(fb.index)
}

func (fb *fetchBlocks) nextNode() {
	fb.setNode(fb.index + 1)
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
			if fb.index == len(fb.roster.List)-1 {
				return nil, nil
			}
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
		log.Infof("Got %d blocks from %s (not sure), starting at index %d",
			len(blocks), url, blocks[0].Index)
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
			return sb, xerrors.New("couldn't fetch next block")
		}
	}
	return sb, nil
}
