// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"

	"github.com/ava-labs/hypersdk/chain"
)

type AuthEngine interface {
	GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier
	Cache(auth chain.Auth)
}

type StateSyncerServer interface {
	GetLastStateSummary(context.Context) (block.StateSummary, error)
	GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error)
	ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error)
}

type TxRemovedEvent struct {
	TxID ids.ID
	Err  error
}
