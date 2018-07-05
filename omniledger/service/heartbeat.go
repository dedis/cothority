package service

import (
	"errors"
	"sync"
	"time"
)

type heartbeat struct {
	beatChan    chan bool
	closeChan   chan bool
	getTimeChan chan chan time.Time
	timeout     time.Duration
	timeoutChan chan string
}

type heartbeats struct {
	sync.Mutex
	heartbeatMap map[string]heartbeat
}

func newHeartbeats() heartbeats {
	return heartbeats{
		heartbeatMap: make(map[string]heartbeat),
	}
}

func (r *heartbeats) beat(key string) error {
	r.Lock()
	defer r.Unlock()
	if c, ok := r.heartbeatMap[key]; ok {
		c.beatChan <- true
		return nil
	}
	return errors.New("key does not exist")
}

func (r *heartbeats) getLatestHeartbeat(key string) (time.Time, error) {
	r.Lock()
	defer r.Unlock()
	if c, ok := r.heartbeatMap[key]; ok {
		resultsChan := make(chan time.Time)
		c.getTimeChan <- resultsChan
		t := <-resultsChan
		return t, nil
	}
	return time.Unix(0, 0), errors.New("key does not exist")
}

func (r *heartbeats) closeAll() {
	r.Lock()
	defer r.Unlock()
	for _, c := range r.heartbeatMap {
		c.closeChan <- true
	}
}

func (r *heartbeats) enabled() bool {
	if r.heartbeatMap == nil {
		return false
	}
	return true
}

func (r *heartbeats) exists(key string) bool {
	r.Lock()
	defer r.Unlock()
	_, ok := r.heartbeatMap[key]
	return ok
}

func (r *heartbeats) start(key string, timeout time.Duration, timeoutChan chan string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.heartbeatMap[key]; ok {
		return errors.New("key already exists")
	}

	beatChan := make(chan bool)
	closeChan := make(chan bool, 1)
	getTimeChan := make(chan chan time.Time, 1)

	go func() {
		currTime := time.Now()
		to := time.After(timeout)
		for {
			select {
			case <-beatChan:
				currTime = time.Now()
				to = time.After(timeout)
			case <-to:
				// the timeoutChan channel might not be reading
				// any message when heartbeats are disabled
				select {
				case timeoutChan <- key:
				default:
				}
				to = time.After(timeout)
			case outChan := <-getTimeChan:
				outChan <- currTime
			case <-closeChan:
				return
			}
		}
	}()

	r.heartbeatMap[key] = heartbeat{
		beatChan:    beatChan,
		closeChan:   closeChan,
		getTimeChan: getTimeChan,
		timeout:     timeout,
		timeoutChan: timeoutChan,
	}
	return nil
}
