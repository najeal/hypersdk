// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/heap"
	"github.com/ava-labs/hypersdk/state"
)

var ErrCPULimitReached = errors.New("cpu limit reached")

// CoresManager distributes work on multiple cores
// until it is closed.
type CoresManager struct {
	cores  []core
	heap   *heap.Heap[Core, uint64]
	gauges Gauges
	wgroup *sync.WaitGroup
}

// NewCoresManager initializes a priority a core MinHeap and CPU usage gauges.
// It start all cores.
func NewCoresManager(cores []*core, maxCPUUsage uint64) *CoresManager {
	coreHeap := heap.New[Core, uint64](len(cores), true)
	coreManager := &CoresManager{
		gauges: NewGauges(len(cores), maxCPUUsage),
		heap:   coreHeap,
		wgroup: &sync.WaitGroup{},
	}

	for _, core := range cores {
		coreHeap.Push(&heap.Entry[Core, uint64]{
			ID:    ids.GenerateTestID(),
			Item:  core,
			Val:   0,
			Index: coreHeap.Len(),
		})

		coreManager.wgroup.Add(1)
		go func() {
			defer coreManager.wgroup.Done()
			core.Start()
		}()
	}

	return coreManager
}

// Close indicates not more txs will be provided, it lets the cores terminate their work and stop.
func (cm *CoresManager) Close() {
	for _, core := range cm.cores {
		core.Stop()
	}
	cm.wgroup.Wait()
}

// Execute distributes the work to the least used core.
func (cm *CoresManager) Execute(f func(), stateKeys state.Keys, cpuUnits uint64) error {
	entry := cm.heap.Pop()
	core := entry.Item
	forcastCPU, incremented := cm.gauges.Increment(core.ID(), stateKeys, cpuUnits)
	if !incremented {
		// the less used core has reached limit
		return ErrCPULimitReached
	}
	cm.heap.Push(&heap.Entry[Core, uint64]{
		ID:    ids.GenerateTestID(),
		Item:  core,
		Val:   forcastCPU,
		Index: cm.heap.Len(),
	})
	core.Execute(f)
	return nil
}
