package byzcoin

import (
	"sync"
	"time"

	"golang.org/x/xerrors"
)

// heartbeat is used for monitoring signals (or heartbeats) that are suppose to
// come in periodically. The signals are received in beatChan. If a heartbeat
// is missed (when no heartbeats are heard within timeout duration), then
// another signal will be sent to timeoutChan so that the outside listener can
// react to it.
type heartbeat struct {
	beatChan          chan bool
	closeChan         chan bool
	getTimeChan       chan chan time.Time
	timeout           time.Duration
	timeoutChan       chan string
	updateTimeoutChan chan time.Duration
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
	return xerrors.New("key does not exist")
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
	return time.Unix(0, 0), xerrors.New("key does not exist")
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
	h.updateTimeoutChan <- timeout
}

func (r *heartbeats) stop(key string) {
	r.Lock()
	defer r.Unlock()
	hb, ok := r.heartbeatMap[key]
	if !ok {
		return
	}
	hb.closeChan <- true
	delete(r.heartbeatMap, key)
}

func (r *heartbeats) start(key string, timeout time.Duration, timeoutChan chan string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.heartbeatMap[key]; ok {
		return xerrors.New("key already exists")
	}

	r.heartbeatMap[key] = &heartbeat{
		beatChan:          make(chan bool),
		closeChan:         make(chan bool, 1),
		getTimeChan:       make(chan chan time.Time, 1),
		timeout:           timeout,
		timeoutChan:       timeoutChan,
		updateTimeoutChan: make(chan time.Duration),
	}

	r.wg.Add(1)
	go func(h *heartbeat) {
		defer r.wg.Done()
		lastHeartbeat := time.Now()
		timeout := time.NewTimer(h.timeout)
		for {
			// The internal state of one heartbeat monitor. It's state can be
			// changed using channels. The following changes are possible:
			// - h.beatChan: receive a heartbeat and reset the timeout
			// - timeout and send a timeout message
			// - h.getTimeChan: be asked to return the time of the last heartbeat
			// - h.closeChan: close down and return
			// - h.updateTimeoutChan: update the timeout interval
			select {
			case <-h.beatChan:
				lastHeartbeat = time.Now()
				h.resetTimeout(timeout)
			case <-timeout.C:
				select {
				// the timeoutChan channel might not be reading
				// any message when heartbeats are disabled
				case h.timeoutChan <- key:
				default:
				}

				// Because we already used the channel, we can directly reset the
				// timer and don't need to drain it: https://golang.org/pkg/time/#Timer.Reset
				timeout.Reset(h.timeout)
			case outChan := <-h.getTimeChan:
				outChan <- lastHeartbeat
			case <-h.closeChan:
				return
			case h.timeout = <-h.updateTimeoutChan:
				h.resetTimeout(timeout)
			}
		}
	}(r.heartbeatMap[key])

	return nil
}

func (h *heartbeat) resetTimeout(to *time.Timer) {
	// According to https://golang.org/pkg/time/#Timer.Reset we need to make
	// sure the channel gets drained after stopping.
	if !to.Stop() {
		<-to.C
	}
	to.Reset(h.timeout)
}
