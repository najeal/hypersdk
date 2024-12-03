// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"github.com/ava-labs/hypersdk/internal/heap"
	"github.com/ava-labs/hypersdk/state"
)

// blockedTxs tracks txs blocked by their stateKeys.
type blockedTxs map[string]*heap.Heap[Tx, uint64]

// cleanup delete and return all stored txs.
func (b *blockedTxs) cleanup() []Tx {
	txs := make([]Tx, 0)
	for _, txHeap := range *b {
		for {
			txEntry := txHeap.Pop()
			if txEntry == nil {
				break
			}
			txs = append(txs, txEntry.Item)
		}
	}
	return txs
}

// areWaitingFor returns `true` when txs are blocked by the stateKeys.
func (b *blockedTxs) areWaitingFor(stateKeys state.Keys) bool {
	for key := range stateKeys {
		if _, ok := (*b)[key]; ok {
			return true
		}
	}
	return false
}

// insert adds the tx into the blocked txs db.
func (b *blockedTxs) insert(tx Tx) {
	for key := range tx.StateKeys() {
		var txHeap *heap.Heap[Tx, uint64]
		var ok bool
		txHeap, ok = (*b)[key]
		if !ok {
			txHeap = heap.New[Tx, uint64](1, false)
		}
		txHeap.Push(&heap.Entry[Tx, uint64]{
			ID:   tx.ID(),
			Item: tx,
			Val:  tx.PriorityFees(),
		})
		(*b)[key] = txHeap
	}
}

// getNextTx finds the next highest tx with provided keys
func (b *blockedTxs) getNextTx(stateKeys state.Keys, stateKeysAreLockableFn func(keys state.Keys) bool) (Tx, bool) {
	var nextTx Tx
	var highestPriority uint64
	for triggeredKey := range stateKeys {
		txHeap, ok := (*b)[triggeredKey]
		if !ok {
			continue
		}
		txEntry := txHeap.First()
		if txEntry == nil {
			continue
		}
		// check no opponent with higher priority tx is waiting on its keys
		greaterPriorityExists := false
		for txKey := range txEntry.Item.StateKeys() {
			if txKey == triggeredKey {
				continue // we know current tx is first
			}
			opponentHeap, ok := (*b)[txKey]
			if !ok {
				continue
			}
			if opponentTx := opponentHeap.First(); opponentTx != nil && opponentTx.Val > txEntry.Val {
				greaterPriorityExists = true
			}
		}
		if greaterPriorityExists {
			continue
		}
		tx, priority := txEntry.Item, txEntry.Val
		if stateKeysAreLockableFn(tx.StateKeys()) {
			if priority > highestPriority {
				nextTx = tx
				highestPriority = priority
			}
		}
	}
	if nextTx != nil {
		// clean tx presence from each heap
		for key := range nextTx.StateKeys() {
			txHeap, ok := (*b)[key]
			if ok {
				entry, found := txHeap.Get(nextTx.ID())
				if found {
					txHeap.Remove(entry.Index)
				}
			}
		}
	}
	return nextTx, nextTx != nil
}
