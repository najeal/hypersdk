// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"sync"

	"github.com/ava-labs/hypersdk/state"
)

// Gauges track cpu usage for x core.
// It limits core usage to the upper bound limit.
type Gauges struct {
	time         []uint64
	releasedKeys map[string]releasedHistory
	maxCPUUsage  uint64
	m            *sync.Mutex
}

func NewGauges(nCores int, maxCPUUsage uint64) Gauges {
	return Gauges{
		time:         make([]uint64, nCores),
		releasedKeys: make(map[string]releasedHistory),
		maxCPUUsage:  maxCPUUsage,
		m:            &sync.Mutex{},
	}
}

// IncrementCoreTIme returns incremnted core time unless max cpu usage has been reached
func (g *Gauges) Increment(gaugeID int, keys state.Keys, units uint64) (uint64, bool) {
	g.m.Lock()
	defer g.m.Unlock()
	coreTime := g.time[gaugeID]
	for key, perms := range keys {
		history, ok := g.releasedKeys[key]
		if ok {
			if history.releasedTime > coreTime && history.perms != perms {
				// coreTime must be mark as used/wasted
				coreTime = history.releasedTime
			}
		}
	}
	coreTime = coreTime + units
	if coreTime > g.maxCPUUsage {
		return 0, false
	}
	for key, perms := range keys {
		// keep higher released time
		higherReleaseTime := coreTime
		history, ok := g.releasedKeys[key]
		if !ok || higherReleaseTime >= history.releasedTime {
			g.releasedKeys[key] = releasedHistory{
				perms:        perms,
				releasedTime: coreTime,
			}
		}
	}
	g.time[gaugeID] = coreTime
	return coreTime, true
}

type releasedHistory struct {
	perms        state.Permissions
	releasedTime uint64
}
