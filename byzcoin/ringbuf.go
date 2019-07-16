package byzcoin

import (
	"sync"
)

type ringBufElem struct {
	key   string
	value string
}

// TODO pre-allocate size elements and track the latest one

type ringBuf struct {
	sync.Mutex
	size  int
	items []ringBufElem
}

func (b *ringBuf) add(key, value string) {
	b.Lock()
	defer b.Unlock()

	if len(b.items) == b.size {
		b.items = append(b.items[1:], ringBufElem{key, value})
	} else {
		b.items = append(b.items, ringBufElem{key, value})
	}
}

func (b *ringBuf) get(key string) (string, bool) {
	b.Lock()
	defer b.Unlock()
	for i, item := range b.items {
		if item.key == key {
			b.items = append(b.items[:i], b.items[i+1:]...)
			return item.value, true
		}
	}
	return "", false
}
