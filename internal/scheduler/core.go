// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"sync"
)

// core is an individual processing unit responsible
// to execute provided work as a FIFO.
type core struct {
	id         int
	txs        *taskQueue
	stopStream chan struct{}
	stop       bool
	m          *sync.Mutex
}

func NewCore(id int) *core {
	stopStream := make(chan struct{})
	return &core{
		id:         id,
		txs:        &taskQueue{},
		stopStream: stopStream,
		stop:       false,
		m:          &sync.Mutex{},
	}
}

func (c *core) ID() int {
	return c.id
}

// Start core.
func (c *core) Start() {
	for {
		select {
		case <-c.stopStream:
			c.stop = true
		default:
			if !c.executeNextTask() {
				return
			}
		}
	}
}

// Stop signals the core to stop.
// It will focus on finishing the waiting tasks without accepting new ones.
func (c *core) Stop() {
	c.stopStream <- struct{}{}
}

// executesNextTask execute next task if available.
func (c *core) executeNextTask() bool {
	c.m.Lock()
	defer c.m.Unlock()
	f, ok := c.txs.Pop()
	if !ok {
		return false
	}
	f()
	return true
}

// Execute add tx in the queue.
// The function is non-blocking
func (c *core) Execute(f func()) {
	c.m.Lock()
	defer c.m.Unlock()
	if c.stop {
		return
	}
	c.txs.Insert(f)
}
