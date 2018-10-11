package byzcoin

import (
	"errors"
	"sync"
	"time"
)

// heartbeat is used for monitoring signals (or heartbeats) that are suppose to
// come in periodically. The signals are received in beatChan. If a heartbeat
// is missed (when no heartbeats are heard within timeout duration), then
// another signal will be sent to timeoutChan so that the outside listener can
// react to it.
type heartbeat struct {
	beatChan    chan bool
	closeChan   chan bool
	getTimeChan chan chan time.Time
	timeout     time.Duration
	timeoutChan chan string
	updateTO    chan time.Duration
}

type heartbeats struct {
	sync.Mutex
	wg           sync.WaitGroup
	heartbeatMap map[string]*heartbeat
}

func newHeartbeats() heartbeats {
	return heartbeats{
		heartbeatMap: make(map[string]*heartbeat),
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
	r.wg.Wait()
	r.heartbeatMap = make(map[string]*heartbeat)
}

func (r *heartbeats) exists(key string) bool {
	r.Lock()
	defer r.Unlock()
	_, ok := r.heartbeatMap[key]
	return ok
}

// updateTimeout stores the new timeout and resets the timer.
func (r *heartbeats) updateTimeout(key string, timeout time.Duration) {
	r.Lock()
	defer r.Unlock()
	h, ok := r.heartbeatMap[key]
	if !ok {
		return
	}
	if h.timeout != timeout {
		h.updateTO <- timeout
	}
}

func (r *heartbeats) start(key string, timeout time.Duration, timeoutChan chan string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.heartbeatMap[key]; ok {
		return errors.New("key already exists")
	}

	r.heartbeatMap[key] = &heartbeat{
		beatChan:    make(chan bool),
		closeChan:   make(chan bool, 1),
		getTimeChan: make(chan chan time.Time, 1),
		timeout:     timeout,
		timeoutChan: timeoutChan,
		updateTO:    make(chan time.Duration),
	}

	r.wg.Add(1)
	go func(h *heartbeat) {
		defer r.wg.Done()
		currTime := time.Now()
		to := time.NewTimer(h.timeout)
		for {
			select {
			case <-h.beatChan:
				currTime = time.Now()
				h.resetTimer(to)
			case <-to.C:
				// the timeoutChan channel might not be reading
				// any message when heartbeats are disabled
				select {
				case h.timeoutChan <- key:
				default:
				}
				// Because we already used the channel, we can directly reset the
				// timer
				to.Reset(h.timeout)
			case outChan := <-h.getTimeChan:
				outChan <- currTime
			case <-h.closeChan:
				return
			case h.timeout = <-h.updateTO:
				h.resetTimer(to)
			}
		}
	}(r.heartbeatMap[key])

	return nil
}

func (h *heartbeat) resetTimer(to *time.Timer) {
	if !to.Stop() {
		<-to.C
	}
	to.Reset(h.timeout)
}
