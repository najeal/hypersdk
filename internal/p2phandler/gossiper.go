// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p2phandler

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

var ErrNotReady = errors.New("not ready")

var _ p2p.Handler = (*TxGossipHandler)(nil)

type (
	VMIsReady       func() bool
	HandleAppGossip func(ctx context.Context, nodeID ids.NodeID, msg []byte) error
)

type TxGossipHandler struct {
	p2p.NoOpHandler
	logger          logging.Logger
	vmIsReady       VMIsReady
	handleAppGossip HandleAppGossip
}

func NewTxGossipHandler(logger logging.Logger, vmIsReady VMIsReady, handleAppGossip HandleAppGossip) *TxGossipHandler {
	return &TxGossipHandler{
		logger:    logger,
		vmIsReady: vmIsReady,
	}
}

func (t *TxGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) {
	if !t.vmIsReady() {
		t.logger.Warn("handle app gossip failed", zap.Error(ErrNotReady))
		return
	}

	if err := t.handleAppGossip(ctx, nodeID, msg); err != nil {
		t.logger.Warn("handle app gossip failed", zap.Error(err))
	}
}
