package byzcoin

import (
	"sync"
)

func newRingBuf(size int) ringBuf {
	return ringBuf{
		current: 0,
		size:    size,
		items:   make([]ringBufElem, size),
	}
}

type ringBufElem struct {
	key   string
	value string
}

type ringBuf struct {
	sync.RWMutex
	current int
	size    int
	items   []ringBufElem
}

func (b *ringBuf) add(key, value string) {
	b.Lock()
	defer b.Unlock()

	b.items[b.current] = ringBufElem{key, value}
	b.current = (b.current + 1) % b.size
}

func (b *ringBuf) get(key string) (string, bool) {
	b.RLock()
	defer b.RUnlock()
	for _, item := range b.items {
		if item.key == key {
			return item.value, true
		}
	}
	return "", false
}
