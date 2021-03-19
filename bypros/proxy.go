package bypros

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/stdlib"
	"go.dedis.ch/cothority/v3/bypros/browse/paginate"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

const (
	pingWsInterval = time.Minute * 1

	defaultPageSize = 200
	defaultNumPages = 40
)

// Follow starts following a node, which means it will listen to every new
// blocks and update the database accordingly.
func (s *Service) Follow(req *Follow) (*EmptyReply, error) {
	select {
	case <-s.follow:
		log.LLvl1("proxy following")
	default:
		return nil, xerrors.Errorf("already following")
	}

	s.following = true
	s.stopFollow = make(chan struct{}, 1)

	waitDone := sync.WaitGroup{}

	apiEndpoint, err := getWsAddr(req.Target)
	if err != nil {
		return nil, xerrors.Errorf("failed to get ws addr: %v", err)
	}

	apiURL := fmt.Sprintf("%s/%s/%s", apiEndpoint, byzcoin.ServiceName, "StreamingRequest")
	c, _, err := websocket.DefaultDialer.Dial(apiURL, nil)
	if err != nil {
		return nil, xerrors.Errorf("failed to dial %s: %v", apiURL, err)
	}

	streamReq := byzcoin.StreamingRequest{
		ID: req.ScID,
	}

	buf, err := protobuf.Encode(&streamReq)
	if err != nil {
		return nil, xerrors.Errorf("failed to encode streaming request: %v", err)
	}

	err = c.WriteMessage(websocket.BinaryMessage, buf)
	if err != nil {
		return nil, xerrors.Errorf("failed to send streaming request: %v", err)
	}

	stopPing := make(chan struct{})

	// When the stop signal is received, cancel the context and allow a new call
	// to start following.
	go func() {
		<-s.stopFollow

		close(stopPing)

		err := c.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second*5))
		if err != nil {
			log.Warnf("failed to write close: %v", err)
		}

		c.Close()

		waitDone.Wait()
		log.LLvl1("done following")
		s.follow <- struct{}{}
		s.following = false
	}()

	// listen to new blocks and save them in the database
	waitDone.Add(1)
	go func() {
		defer waitDone.Done()

		for {
			_, buf, err := c.ReadMessage()
			if err != nil {
				_, ok := err.(*websocket.CloseError)
				if !ok {
					// that can happen when we close the connection
					log.Warnf("failed to read request: %v", err)
				}
				s.notifyStop()
				return
			}

			streamResp := byzcoin.StreamingResponse{}
			err = protobuf.Decode(buf, &streamResp)
			if err != nil {
				log.Errorf("failed to decode request: %v", err)
				s.notifyStop()
				return
			}

			s.followCallback(streamResp, nil)
		}
	}()

	// keep the connection alive be regularly sending a ping
	waitDone.Add(1)
	go func() {
		defer waitDone.Done()

		for {
			select {
			case <-time.After(pingWsInterval):
				err := c.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Warnf("failed to ping: %v", err)
					return
				}
			case <-stopPing:
				return
			}

		}
	}()

	return &EmptyReply{}, nil
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

	// cancel the context in case we receive a stop signal
	go func() {
		<-stopChan
		cancel()
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
	browser := browseSrv.GetBrowser(browseHandler, req.ScID, url)

	go func() {
		err := browser.Browse(ctx, req.FromBlock)
		if err != nil {
			outChan.errf("browsing failed: %v", err)
			return
		}

		outChan.done()
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
	log.LLvl3("parsing block", block.Index)

	blockID, err := s.storage.GetBlock(block.Hash)
	if err != nil {
		return xerrors.Errorf("failed to get block: %v", err)
	}

	if blockID != -1 {
		log.LLvlf3("block with index %d already exist: skipping", block.Index)
		return nil
	}

	_, err = s.storage.StoreBlock(block)
	if err != nil {
		return xerrors.Errorf("failed to store block; %v", err)
	}

	return nil
}

// UnFollow stop the following session.
func (s *Service) UnFollow(req *UnFollow) (*EmptyReply, error) {
	if !s.following {
		return nil, xerrors.Errorf("not following")
	}

	s.notifyStop()

	return &EmptyReply{}, nil
}

// Query queries the storage and sends back the result.
func (s *Service) Query(req *Query) (*QueryReply, error) {
	res, err := s.storage.Query(req.Query)
	if err != nil {
		return nil, xerrors.Errorf("failed to query: %v", err)
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

func (o catchUpOut) done() {
	o <- &CatchUpResponse{
		Done: true,
	}
}
