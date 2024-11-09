package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	avasync "github.com/ava-labs/avalanchego/x/sync"
	"github.com/ava-labs/hypersdk/internal/p2phandler"
)

func registerStatic(vm *VM) error {
	var errs wrappers.Errs
	errs.Add(
		vm.network.AddHandler(
			rangeProofHandlerID,
			avasync.NewGetRangeProofHandler(vm.snowCtx.Log, vm.stateDB),
		),
		vm.network.AddHandler(
			changeProofHandlerID,
			avasync.NewGetChangeProofHandler(vm.snowCtx.Log, vm.stateDB),
		),
		vm.network.AddHandler(
			txGossipHandlerID,
			p2phandler.NewTxGossipHandler(vm.Logger(), vm.isReady, vm.gossiper.HandleAppGossip),
		),
	)
	return errs.Err
}
