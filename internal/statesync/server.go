// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
)

var _ block.StateSyncableVM = (*stateSyncerServer)(nil)

// stateSyncerServer implements block.StateSyncableVM
type stateSyncerServer struct {
	vm     chain.VM
	logger logging.Logger
}

func NewStateSyncServer(vm chain.VM, logger logging.Logger) *stateSyncerServer {
	return &stateSyncerServer{
		vm:     vm,
		logger: logger,
	}
}

func (*stateSyncerServer) GetOngoingSyncStateSummary(_ context.Context) (block.StateSummary, error) {
	// Because the history of MerkleDB change proofs tends to be short, we always
	// restart syncing from scratch.
	//
	// This is unlike other DB implementations where roots are persisted
	// indefinitely (and it means we can continue from where we left off).
	return nil, database.ErrNotFound
}

func (*stateSyncerServer) StateSyncEnabled(_ context.Context) (bool, error) {
	// We always start the state syncer and may fallback to normal bootstrapping
	// if we are close to tip.
	//
	// There is no way to trigger a full bootstrap from genesis.
	return true, nil
}

// GetLastStateSummary returns the latest state summary.
// If no summary is available, [database.ErrNotFound] must be returned.
func (s *stateSyncerServer) GetLastStateSummary(context.Context) (block.StateSummary, error) {
	summary := chain.NewSyncableBlock(s.vm.LastAcceptedBlock())
	s.logger.Info("Serving syncable block at latest height", zap.Stringer("summary", summary))
	return summary, nil
}

// GetStateSummary implements StateSyncableVM and returns a summary corresponding
// to the provided [height] if the node can serve state sync data for that key.
// If not, [database.ErrNotFound] must be returned.
func (s *stateSyncerServer) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
	id, err := s.vm.GetBlockIDAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	block, err := s.vm.GetStatefulBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(block)
	s.logger.Info("Serving syncable block at requested height",
		zap.Uint64("height", height),
		zap.Stringer("summary", summary),
	)
	return summary, nil
}

func (s *stateSyncerServer) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	sb, err := chain.ParseBlock(ctx, bytes, false, s.vm)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(sb)
	s.logger.Info("parsed state summary", zap.Stringer("summary", summary))
	return summary, nil
}
