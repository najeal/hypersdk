// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scheduler

import (
	"sync"

	"github.com/ava-labs/hypersdk/state"
)

type LockedStateKeys struct {
	stateKeys    state.Keys
	keysRefCount map[string]uint64

	m *sync.RWMutex
}

// AreAvailable checks if provided keys/perms are compatible with the ones under usage.
func (u *LockedStateKeys) AreLockable(stateKeys state.Keys) bool {
	u.m.RLock()
	defer u.m.RUnlock()
	for key, askingPerm := range stateKeys {
		usingPerm, ok := u.stateKeys[key]
		if ok && (usingPerm != state.Read || askingPerm != state.Read) {
			return false
		}
	}
	return true
}

// ReleaseKeys counts down the state keys usage.
func (u *LockedStateKeys) ReleaseKeys(keys state.Keys) {
	u.m.Lock()
	defer u.m.Unlock()

	for key := range keys {
		count, ok := u.keysRefCount[key]
		if !ok || count == 0 {
			continue
		}
		u.keysRefCount[key] = count - 1
		delete(u.stateKeys, key)
	}
}

// Use checks every keys can be marked as used, then it marks them.
func (u *LockedStateKeys) Use(keys state.Keys) bool {
	u.m.Lock()
	defer u.m.Unlock()

	for key := range keys {
		count, ok := u.keysRefCount[key]
		if !ok || count == 0 {
			continue
		}
		if perms, ok := u.stateKeys[key]; ok && perms != state.Read {
			return false
		}
	}

	for key, perms := range keys {
		count := u.keysRefCount[key]
		u.keysRefCount[key] = count + 1

		u.stateKeys[key] = perms
	}
	return true
}
