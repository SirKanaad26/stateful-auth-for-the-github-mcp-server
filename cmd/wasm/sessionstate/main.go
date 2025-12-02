//go:build wasm
// +build wasm

package main

import (
	"sync"
	"unsafe"
)

// Per-instance state - each WASM module instance has its own isolated copy
// When wazero instantiates this module, each instance gets separate linear memory,
// ensuring complete session isolation between different clients
var (
	lockedRepository   string
	isRepositoryLocked bool
	mu                 sync.RWMutex
)

// main is required for WASM but does nothing - wazero loads via exported functions
func main() {}

//export validate_and_lock
func validateAndLock(ownerPtr, ownerLen, repoPtr, repoLen uint32) uint32 {
	owner := readString(ownerPtr, ownerLen)
	repo := readString(repoPtr, repoLen)

	mu.Lock()
	defer mu.Unlock()

	fullRepoName := owner + "/" + repo

	// Check if already locked to a different repository
	if isRepositoryLocked && lockedRepository != fullRepoName {
		// Policy violation: trying to access different repository
		return 1
	}

	// Lock to this repository
	lockedRepository = fullRepoName
	isRepositoryLocked = true

	return 0 // success
}

//export lock_repository
func lockRepository(ownerPtr, ownerLen, repoPtr, repoLen uint32) uint32 {
	owner := readString(ownerPtr, ownerLen)
	repo := readString(repoPtr, repoLen)

	mu.Lock()
	defer mu.Unlock()

	lockedRepository = owner + "/" + repo
	isRepositoryLocked = true

	return 0 // success
}

//export unlock_repository
func unlockRepository() uint32 {
	mu.Lock()
	defer mu.Unlock()

	if !isRepositoryLocked {
		return 0 // already unlocked
	}

	lockedRepository = ""
	isRepositoryLocked = false

	return 0 // success
}

//export check_lock_status
func checkLockStatus() uint32 {
	mu.RLock()
	defer mu.RUnlock()

	if isRepositoryLocked {
		return 1
	}
	return 0
}

// readString reads a string from WASM linear memory at the given pointer and length
func readString(ptr, length uint32) string {
	if length == 0 {
		return ""
	}
	// Convert pointer to byte slice using unsafe
	// This reads from the WASM module's linear memory
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
	return string(bytes)
}
