package byzcoin

import (
	"bytes"
	"sync"
)

// newRingBuf initializes a ring buffer. It's used in byzcoin for recording
// transaction errors. But it is general enough to be used for other purposes.
func newRingBuf(size int) ringBuf {
	return ringBuf{
		current: 0,
		size:    size,
		items:   make([]ringBufElem, size),
	}
}

type ringBufElem struct {
	key   []byte
	value string
}

type ringBuf struct {
	sync.RWMutex
	current int
	size    int
	items   []ringBufElem
}

func (b *ringBuf) add(key []byte, value string) {
	b.Lock()
	defer b.Unlock()

	b.items[b.current] = ringBufElem{key, value}
	b.current = (b.current + 1) % b.size
}

func (b *ringBuf) get(key []byte) (string, bool) {
	b.RLock()
	defer b.RUnlock()
	for _, item := range b.items {
		if bytes.Equal(item.key, key) {
			return item.value, true
		}
	}
	return "", false
}
