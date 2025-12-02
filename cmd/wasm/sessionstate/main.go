//go:build wasm
// +build wasm

package main

import (
	"fmt"
	"sync"
	"syscall/js"
)

var (
	lockedRepository   string
	isRepositoryLocked bool
	mu                 sync.RWMutex
)

func main() {
	fmt.Println("[WASM] Session state validation module initialized")

	// Register functions to JavaScript global
	js.Global().Set("validateAndLock", js.FuncOf(jsValidateAndLock))
	js.Global().Set("lockRepository", js.FuncOf(jsLockRepository))
	js.Global().Set("unlockRepository", js.FuncOf(jsUnlockRepository))
	js.Global().Set("getLockStatus", js.FuncOf(jsGetLockStatus))
	js.Global().Set("isLocked", js.FuncOf(jsIsLocked))

	fmt.Println("[WASM] Functions registered successfully")

	// Keep the program alive
	select {}
}

// jsValidateAndLock validates that we can access a repository
func jsValidateAndLock(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return js.ValueOf(1) // error
	}

	owner := args[0].String()
	repo := args[1].String()

	mu.Lock()
	defer mu.Unlock()

	if isRepositoryLocked && lockedRepository != fmt.Sprintf("%s/%s", owner, repo) {
		fmt.Printf("[WASM] Policy violation: attempt to access %s/%s while locked to %s\n", owner, repo, lockedRepository)
		return js.ValueOf(1) // policy violation
	}

	lockedRepository = fmt.Sprintf("%s/%s", owner, repo)
	isRepositoryLocked = true

	fmt.Printf("[WASM] ValidateAndLock succeeded for %s/%s\n", owner, repo)
	return js.ValueOf(0) // success
}

// jsLockRepository locks a specific repository
func jsLockRepository(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return js.ValueOf(1) // error
	}

	owner := args[0].String()
	repo := args[1].String()

	mu.Lock()
	defer mu.Unlock()

	lockedRepository = fmt.Sprintf("%s/%s", owner, repo)
	isRepositoryLocked = true

	fmt.Printf("[WASM] Repository locked: %s\n", lockedRepository)
	return js.ValueOf(0) // success
}

// jsUnlockRepository unlocks the current repository
func jsUnlockRepository(this js.Value, args []js.Value) any {
	mu.Lock()
	defer mu.Unlock()

	if !isRepositoryLocked {
		return js.ValueOf(0) // already unlocked
	}

	fmt.Printf("[WASM] Repository unlocked: %s\n", lockedRepository)
	lockedRepository = ""
	isRepositoryLocked = false

	return js.ValueOf(0) // success
}

// jsGetLockStatus returns the current lock status
func jsGetLockStatus(this js.Value, args []js.Value) any {
	mu.RLock()
	defer mu.RUnlock()

	result := js.Global().Get("Object").New()
	result.Set("isLocked", js.ValueOf(isRepositoryLocked))
	result.Set("lockedRepository", js.ValueOf(lockedRepository))

	return result
}

// jsIsLocked returns whether a repository is locked
func jsIsLocked(this js.Value, args []js.Value) any {
	mu.RLock()
	defer mu.RUnlock()

	return js.ValueOf(isRepositoryLocked)
}
