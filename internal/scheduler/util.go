// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import "sync"

type taskQueue struct {
	first *node
	last  *node
	m     *sync.Mutex
}

type node struct {
	f    func()
	next *node
}

func (q *taskQueue) Pop() (func(), bool) {
	q.m.Lock()
	defer q.m.Unlock()
	if q.first == nil {
		return nil, false
	}
	result := q.first
	q.first = result.next
	return result.f, true
}

func (q *taskQueue) Insert(f func()) {
	q.m.Lock()
	defer q.m.Unlock()
	newLast := &node{
		f:    f,
		next: nil,
	}
	if q.first == nil {
		q.first = newLast
	}
	if q.last != nil {
		q.last.next = newLast
	}
	q.last = newLast
}
