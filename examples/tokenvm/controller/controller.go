// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"
	"fmt"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	hrpc "github.com/ava-labs/hypersdk/rpc"
	hstorage "github.com/ava-labs/hypersdk/storage"
	"github.com/ava-labs/hypersdk/vm"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/config"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/orderbook"
	"github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/examples/tokenvm/version"
)

var _ vm.Controller = (*Controller)(nil)

type Controller struct {
	inner *vm.VM

	snowCtx      *snow.Context
	genesis      *genesis.Genesis
	config       *config.Config
	stateManager *StateManager

	metrics *metrics

	metaDB database.Database

	orderBook *orderbook.OrderBook
}

func New() *vm.VM {
	return vm.New(&Controller{}, version.Version)
}

func (c *Controller) Initialize(
	inner *vm.VM,
	snowCtx *snow.Context,
	gatherer ametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	vm.Config,
	vm.Genesis,
	builder.Builder,
	gossiper.Gossiper,
	database.Database,
	database.Database,
	vm.Handlers,
	chain.ActionRegistry,
	chain.AuthRegistry,
	error,
) {
	c.inner = inner
	c.snowCtx = snowCtx
	c.stateManager = &StateManager{}

	// Instantiate metrics
	var err error
	c.metrics, err = newMetrics(gatherer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Load config and genesis
	c.config, err = config.New(c.snowCtx.NodeID, configBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	c.snowCtx.Log.SetLevel(c.config.GetLogLevel())
	snowCtx.Log.Info("loaded config", zap.Any("contents", c.config))

	c.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	snowCtx.Log.Info("loaded genesis", zap.Any("genesis", c.genesis))

	// Create DBs
	blockDB, stateDB, metaDB, err := hstorage.New(snowCtx.ChainDataDir, gatherer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	c.metaDB = metaDB

	// Create handlers
	//
	// hypersdk handler are initiatlized automatically, you just need to
	// initialize custom handlers here.
	apis := map[string]*common.HTTPHandler{}
	jsonRPCHandler, err := hrpc.NewJSONRPCHandler(
		consts.Name,
		rpc.NewJSONRPCServer(c),
		common.NoLock,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	// Create builder and gossiper
	var (
		build  builder.Builder
		gossip gossiper.Gossiper
	)
	if c.config.TestMode {
		c.inner.Logger().Info("running build and gossip in test mode")
		build = builder.NewManual(inner)
		gossip = gossiper.NewManual(inner)
	} else {
		build = builder.NewTime(inner)
		gcfg := gossiper.DefaultProposerConfig()
		gcfg.GossipInterval = c.config.GossipInterval
		gcfg.GossipMaxSize = c.config.GossipMaxSize
		gcfg.GossipProposerDiff = c.config.GossipProposerDiff
		gcfg.GossipProposerDepth = c.config.GossipProposerDepth
		gcfg.BuildProposerDiff = c.config.BuildProposerDiff
		gcfg.VerifyTimeout = c.config.VerifyTimeout
		gossip = gossiper.NewProposer(inner, gcfg)
	}

	// Initialize order book used to track all open orders
	c.orderBook = orderbook.New(c, c.config.TrackedPairs)
	return c.config, c.genesis, build, gossip, blockDB, stateDB, apis, consts.ActionRegistry, consts.AuthRegistry, nil
}

func (c *Controller) Rules(t int64) chain.Rules {
	// TODO: extend with [UpgradeBytes]
	return c.genesis.Rules(t, c.snowCtx.NetworkID, c.snowCtx.ChainID)
}

func (c *Controller) StateManager() chain.StateManager {
	return c.stateManager
}

func (c *Controller) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	batch := c.metaDB.NewBatch()
	defer batch.Reset()

	results := blk.Results()
	for i, tx := range blk.Txs {
		result := results[i]
		if c.config.GetStoreTransactions() {
			err := storage.StoreTransaction(
				ctx,
				batch,
				tx.ID(),
				blk.GetTimestamp(),
				result.Success,
				result.Units,
			)
			if err != nil {
				return err
			}
		}
		if result.Success {
			switch tx.Action.GetTypeID() {
			case (&actions.CreateAsset{}).GetTypeID():
				c.metrics.createAsset.Inc()
			case (&actions.MintAsset{}).GetTypeID():
				c.metrics.mintAsset.Inc()
			case (&actions.BurnAsset{}).GetTypeID():
				c.metrics.burnAsset.Inc()
			case (&actions.ModifyAsset{}).GetTypeID():
				c.metrics.modifyAsset.Inc()
			case (&actions.Transfer{}).GetTypeID():
				c.metrics.transfer.Inc()
			case (&actions.CreateOrder{}).GetTypeID():
				c.metrics.createOrder.Inc()
				actor := auth.GetActor(tx.Auth)
				c.orderBook.Add(tx.ID(), actor, tx.Action.(*actions.CreateOrder))
			case (&actions.FillOrder{}).GetTypeID():
				c.metrics.fillOrder.Inc()
				action := tx.Action.(*actions.FillOrder)
				orderResult, err := actions.UnmarshalOrderResult(result.Output)
				if err != nil {
					// This should never happen
					return err
				}
				if orderResult.Remaining == 0 {
					c.orderBook.Remove(action.Order)
					continue
				}
				c.orderBook.UpdateRemaining(action.Order, orderResult.Remaining)
			case (&actions.CloseOrder{}).GetTypeID():
				c.metrics.closeOrder.Inc()
				c.orderBook.Remove(tx.Action.(*actions.CloseOrder).Order)
			case (&actions.ImportAsset{}).GetTypeID():
				c.metrics.importAsset.Inc()
			case (&actions.ExportAsset{}).GetTypeID():
				c.metrics.exportAsset.Inc()
			}
		}
	}
	return batch.Write()
}

func (*Controller) Rejected(context.Context, *chain.StatelessBlock) error {
	return nil
}

func (*Controller) Shutdown(context.Context) error {
	// Do not close any databases provided during initialization. The VM will
	// close any databases your provided.
	return nil
}
