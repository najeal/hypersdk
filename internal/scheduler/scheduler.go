// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"
)

type Tx interface {
	StateKeys() state.Keys
	CPUUnits() uint64
	ID() ids.ID
	Execute()
	PriorityFees() uint64
}

type TxHeap interface {
	HasNext() bool
	Push(txs []Tx)
	Next() (Tx, bool)
}

type Core interface {
	// Execute a transaction for a discrete quanta of time to simplify scheduling
	Execute(func())
	ID() int
}

type Codes interface {
	Execute(tx Tx)
}

type WaitingTxs struct {
	fromStateKeys map[string]set.Set[ids.ID]
	fromID        map[ids.ID]Tx
}

func NewWaitingTxs() *WaitingTxs {
	return &WaitingTxs{
		fromStateKeys: make(map[string]set.Set[ids.ID]),
		fromID:        make(map[ids.ID]Tx),
	}
}

func (w *WaitingTxs) WaitingFrom(keys state.Keys) bool {
	for key := range keys {
		if _, ok := w.fromStateKeys[key]; ok {
			return true
		}
	}
	return false
}

func (w *WaitingTxs) Insert(tx Tx) {
	for key := range tx.StateKeys() {
		hset, ok := w.fromStateKeys[key]
		if !ok {
			hset = set.NewSet[ids.ID](1)
			w.fromStateKeys[key] = hset
		}
		hset.Add(tx.ID())
	}
	w.fromID[tx.ID()] = tx
}

type Scheduler struct {
	txHeap             TxHeap
	coresManager       *CoresManager
	currentUsedKeys    *LockedStateKeys
	blockedTxs         *blockedTxs
	stop               bool
	releasingStateKeys chan state.Keys
	m                  *sync.Mutex
}

func NewScheduler(txHeap TxHeap, coresManager *CoresManager) *Scheduler {
	releasingStateKeys := make(chan state.Keys)
	return &Scheduler{
		txHeap:             txHeap,
		coresManager:       coresManager,
		currentUsedKeys:    &LockedStateKeys{},
		blockedTxs:         &blockedTxs{},
		stop:               false,
		m:                  &sync.Mutex{},
		releasingStateKeys: releasingStateKeys,
	}
}

func (s *Scheduler) Run() {
	for {
		select {
		case stateKeys := <-s.releasingStateKeys:
			s.executeWaitingTxs(stateKeys)
		default:
			s.executeNextTx()
		}
	}
}

func (s *Scheduler) executeNextTx() {
	s.m.Lock()
	defer s.m.Unlock()

	tx, ok := s.txHeap.Next()
	if !ok {
		s.stop = true
	}
	if !s.currentUsedKeys.AreLockable(tx.StateKeys()) ||
		s.blockedTxs.areWaitingFor(tx.StateKeys()) {
		s.blockedTxs.insert(tx)
	} else {
		s.executeTx(tx)
	}
}

// executeWaitingTxs finds the available waiting txs having dependencies on providing keys.
func (s *Scheduler) executeWaitingTxs(keys state.Keys) {
	s.m.Lock()
	defer s.m.Unlock()

	for {
		nextTx, ok := s.blockedTxs.getNextTx(keys, s.currentUsedKeys.AreLockable)
		if !ok {
			return
		}
		s.executeTx(nextTx)
	}
}

// distributeTx books the tx required keys and execute the tx.
// it must be called by an higher Scheduler function managing sync.
func (s *Scheduler) executeTx(tx Tx) {
	s.currentUsedKeys.Use(tx.StateKeys())
	f := func() {
		defer s.notifyReleasingStateKeys(tx.StateKeys())
		tx.Execute()
	}
	if err := s.coresManager.Execute(f, tx.StateKeys(), tx.CPUUnits()); err != nil {
		s.stop = true
		// cleaning scheduler
		// reintroduce current tx + blockedTxs
		s.txHeap.Push(append([]Tx{tx}, s.blockedTxs.cleanup()...))
	}
}

func (s *Scheduler) notifyReleasingStateKeys(stateKeys state.Keys) {
	s.releasingStateKeys <- stateKeys
}
