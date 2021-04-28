package bypros

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	// register the sql driver
	_ "github.com/jackc/pgx/stdlib"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/bypros/browse/paginate"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

const (
	pingWsInterval = time.Minute * 1

	defaultPageSize = 200
	defaultNumPages = 40

	maxRetry = 5
)

// Follow starts following a node, which means it will listen to every new
// blocks and update the database accordingly.
func (s *Service) Follow(req *Follow) (*EmptyReply, error) {
	select {
	case <-s.follow:
		log.Lvl1("proxy following")
	default:
		return nil, xerrors.Errorf("already following")
	}

	if s.scID == nil {
		s.scID = req.ScID
	}

	if !s.scID.Equal(req.ScID) {
		s.follow <- struct{}{}
		return nil, xerrors.Errorf("wrong skipchain ID: expected '%x', got '%x'",
			s.scID, req.ScID)
	}

	s.following = true
	s.stopFollow = make(chan struct{}, 1)
	s.normalUnfollow = make(chan struct{}, 1)
	s.followReq = req

	wire, err := subscribe(req)
	if err != nil {
		s.follow <- struct{}{}
		s.following = false

		return nil, xerrors.Errorf("failed to start following: %v", err)
	}

	waitDone := sync.WaitGroup{}
	stopPing := make(chan struct{})

	// When the stop signal is received, close the ws connection and allow new
	// call to follow. Note that the ws could already be closed.
	go func() {
		<-s.stopFollow

		close(stopPing)

		wire.client.Close()

		waitDone.Wait()
		log.Lvl1("done following")
		s.following = false
		s.follow <- struct{}{}
	}()

	// keep the ws connection alive
	waitDone.Add(1)
	go func() {
		defer waitDone.Done()
		keepAlive(stopPing, wire)
	}()

	// listen to new blocks and save them in the database
	waitDone.Add(1)
	go func() {
		defer waitDone.Done()
		s.listenBlocks(wire)
	}()

	return &EmptyReply{}, nil
}

// subscribe send a request to start listening on new added blocks. It return a
// ws connection that will be filled with each new block.
func subscribe(req *Follow) (*wire, error) {
	client := onet.NewClient(cothority.Suite, byzcoin.ServiceName)

	streamReq := byzcoin.StreamingRequest{
		ID: req.ScID,
	}

	var conn onet.StreamingConn
	var err error

	retryWait := time.Second * 1 // 1 / 3 / 9 / 27 / 81
	for retry := 0; retry < maxRetry; retry++ {
		conn, err = client.Stream(req.Target, &streamReq)
		if err == nil {
			break
		}

		log.Warnf("failed to connect, retrying in %d", retryWait)
		time.Sleep(retryWait)
		retryWait *= 3
	}

	if err != nil {
		return nil, xerrors.Errorf("failed to create conn: %v", err)
	}

	return &wire{
		client: client,
		conn:   conn,
	}, nil
}

// keepAlive sends regularly a ping on the connection to keep it alive
func keepAlive(stop chan struct{}, wire *wire) {
	for {
		select {
		case <-time.After(pingWsInterval):
			err := wire.conn.Ping(nil, time.Now().Add(time.Second*10))
			if err != nil {
				log.Warnf("failed to ping: %v", err)
				return
			}
		case <-stop:
			return
		}
	}
}

// listenBlocks reads for new blocks and parse them.
func (s *Service) listenBlocks(wire *wire) {
	streamResp := byzcoin.StreamingResponse{}

	for {
		readOpt := onet.StreamingReadOpts{
			Deadline: time.Time{}, // a zero time will make it block
		}

		err := wire.conn.ReadMessageWithOpts(&streamResp, readOpt)
		if err != nil {
			log.Lvl1("stops listening on blocks")
			s.notifyStop()

			select {
			case <-s.normalUnfollow:
				log.Lvl1("normal close")
			default:
				log.Warn("connection dropped, retying")

				_, err = s.Follow(s.followReq)
				if err != nil {
					log.Errorf("failed to Follow again: %v", err)
				}
			}

			break
		}

		s.followCallback(streamResp, nil)
	}
}

// notifyStop can be called multiple times to stop the current following
// session, the one that listens to new blocks. There can be only one following
// session at a time.
func (s *Service) notifyStop() {
	select {
	case s.stopFollow <- struct{}{}:
	default:
	}
}

// CatchUP updates the database from a given block until the end of the chain.
// It uses the streaming service to periodically send back the catch up state to
// the client. This is appropriate since a catch up can be quite long.
func (s *Service) CatchUP(req *CatchUpMsg) (chan *CatchUpResponse, chan bool, error) {
	if s.scID == nil {
		s.scID = req.ScID
	}

	if !s.scID.Equal(req.ScID) {
		return nil, nil, xerrors.Errorf("wrong skipchain ID: "+
			"expected '%x', got '%x'", s.scID, req.ScID)
	}

	if req.UpdateEvery < 1 {
		return nil, nil, xerrors.Errorf("wrong 'updateEvery' value: %d", req.UpdateEvery)
	}

	wsAddr, err := getWsAddr(req.Target)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get ws addr: %v", err)
	}

	outChan := make(catchUpOut)
	stopChan := make(chan bool)

	s.doCatchUP(outChan, stopChan, req, wsAddr)

	return outChan, stopChan, nil
}

// doCatchUP uses a browse implementation to browse the chain and parse each
// block.
func (s *Service) doCatchUP(outChan catchUpOut, stopChan chan bool, req *CatchUpMsg, url string) {
	ctx, cancel := context.WithCancel(context.Background())
	count := 0

	browseDone := make(chan struct{})

	// cancel the context in case we receive a stop signal
	go func() {
		<-stopChan
		cancel()
		<-browseDone // wait for the "done" message to be sent
		close(outChan)
	}()

	// this function will be called for each block
	browseHandler := func(block *skipchain.SkipBlock) error {
		err := s.parseBlock(block)
		if err != nil {
			return xerrors.Errorf("failed to parse block: %v", err)
		}

		count++
		if count%req.UpdateEvery == 0 {
			outChan.statusf(block.Index, block.Hash)
		}

		return nil
	}

	browseSrv := paginate.NewService(defaultPageSize, defaultNumPages)
	browser := browseSrv.GetBrowser(browseHandler, req.ScID, req.Target)

	go func() {
		err := browser.Browse(ctx, req.FromBlock)
		if err != nil {
			outChan.errf("browsing failed: %v", err)
			return
		}
		outChan.done()
		close(browseDone)
	}()
}

// streamCallback is called each time a new block is added. Used when we follow.
func (s *Service) followCallback(sr byzcoin.StreamingResponse, err error) {
	if err != nil {
		_, ok := err.(*websocket.CloseError)
		if ok {
			return // This is a normal close
		}

		log.Errorf("error from stream: %v", err)
		s.notifyStop()
		return
	}

	err = s.parseBlock(sr.Block)
	if err != nil {
		log.Errorf("failed to parse block: %v", err)
		s.notifyStop()
		return
	}
}

// parseBlock updates the database with the block is not already found.
func (s *Service) parseBlock(block *skipchain.SkipBlock) error {
	log.Lvl3("parsing block", block.Index)

	blockID, err := s.storage.GetBlock(block.Hash)
	if err != nil {
		return xerrors.Errorf("failed to get block: %v", err)
	}

	if blockID != -1 {
		log.Lvlf3("block with index %d already exist: skipping", block.Index)
		return nil
	}

	_, err = s.storage.StoreBlock(block)
	if err != nil {
		return xerrors.Errorf("failed to store block; %v", err)
	}

	return nil
}

// Unfollow stop the following session.
func (s *Service) Unfollow(req *Unfollow) (*EmptyReply, error) {
	if !s.following {
		return nil, xerrors.Errorf("not following")
	}

	s.normalUnfollow <- struct{}{}
	s.notifyStop()

	return &EmptyReply{}, nil
}

// Query queries the storage and sends back the result.
func (s *Service) Query(req *Query) (*QueryReply, error) {
	res, err := s.storage.Query(req.Query)
	if err != nil {
		res = []byte(xerrors.Errorf("ERROR: failed to query: %v", err).Error())
	}

	return &QueryReply{
		Result: res,
	}, nil
}

// catchUpOut is used to send back responses to the client during a catch up
type catchUpOut chan *CatchUpResponse

func (o catchUpOut) errf(format string, a ...interface{}) {
	o <- &CatchUpResponse{
		Err: fmt.Sprintf(format, a...),
	}
}

func (o catchUpOut) statusf(blockIndex int, blockHash []byte) {
	o <- &CatchUpResponse{
		Status: CatchUpStatus{
			Message:    fmt.Sprintf("parsed block %d", blockIndex),
			BlockIndex: blockIndex,
			BlockHash:  blockHash,
		},
	}
}

// done sends a done message if someone is willing to take it, otherwise it does
// nothing. That could be the case when the client just wants to stop listening.
func (o catchUpOut) done() {
	select {
	case o <- &CatchUpResponse{
		Done: true,
	}:
	case <-time.After(time.Second * 10):
	}
}

// wire bundles the read/write capabilities of the streaming onet
type wire struct {
	client *onet.Client
	conn   onet.StreamingConn
}
