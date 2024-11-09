// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
)

// StateSyncClientVM is the minimal VM feature required by the statesync client.
type StateSyncClientVM interface {
	GetDiskIsSyncing() (bool, error)
	LastAcceptedBlock() *chain.StatefulBlock
	GetStateSyncMinBlocks() uint64
	GetStateSyncParallelism() int
	Genesis() genesis.Genesis
	State() (merkledb.MerkleDB, error)
	NewNetworkClient(handlerID uint64, options ...p2p.ClientOption) *p2p.Client
	PutDiskIsSyncing(v bool) error
}
