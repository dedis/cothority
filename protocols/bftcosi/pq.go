// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This example demonstrates a Priority queue built using the heap interface.
package bftcosi

import (
	"container/heap"
	"github.com/dedis/cothority/log"
)

// // An Item is something we manage in a Priority queue.
// type Item struct {
// 	value    string // The value of the item; arbitrary.
// 	Priority int    // The Priority of the item in the queue.
// 	// The index is needed by update and is maintained by the heap.Interface methods.
// 	index int // The index of the item in the heap.
// }

type Item struct {
	Priority   int
	NotifyChan chan bool
	index      int
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, Priority so we use greater than here.
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Peak() int {
	old := *pq
	n := len(old)
	if n == 0 {
		log.Lvl1("empty queue")
		return -1
	}
	item := old[n-1]
	return item.Priority
}

// update modifies the Priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, value chan bool, Priority int) {
	item.NotifyChan = value
	item.Priority = Priority
	heap.Fix(pq, item.index)
}
