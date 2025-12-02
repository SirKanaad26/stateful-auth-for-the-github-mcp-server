package sessionstate

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMBridge wraps the WASM module for session state policy enforcement
type WASMBridge struct {
	runtime    wazero.Runtime
	module     api.Module
	ctx        context.Context
	closed     bool
	mu         sync.RWMutex
	validateFn api.Function
	lockFn     api.Function
	unlockFn   api.Function
	checkFn    api.Function
}

// NewWASMBridge creates and initializes a WASM bridge for session state
func NewWASMBridge(ctx context.Context, wasmPath string) (*WASMBridge, error) {
	runtime := wazero.NewRuntime(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] WASI instantiated\n")

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to read WASM file: %w", err)
	}

	module, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	validateFn := module.ExportedFunction("validate_and_lock")
	lockFn := module.ExportedFunction("lock_repository")
	unlockFn := module.ExportedFunction("unlock_repository")
	checkFn := module.ExportedFunction("check_lock_status")

	if validateFn == nil || lockFn == nil || unlockFn == nil || checkFn == nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("missing required WASM functions")
	}

	bridge := &WASMBridge{
		runtime:    runtime,
		module:     module,
		ctx:        ctx,
		closed:     false,
		validateFn: validateFn,
		lockFn:     lockFn,
		unlockFn:   unlockFn,
		checkFn:    checkFn,
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] WASM bridge initialized successfully\n")
	return bridge, nil
}

// ValidateAndLockWASM validates and locks a repository in WASM
func (wb *WASMBridge) ValidateAndLockWASM(repoContext *RepositoryContext) error {
	wb.mu.RLock()
	if wb.closed {
		wb.mu.RUnlock()
		return fmt.Errorf("WASM bridge is closed")
	}
	wb.mu.RUnlock()

	if repoContext == nil {
		return nil
	}

	ownerPtr, err := wb.writeString(repoContext.Owner)
	if err != nil {
		return fmt.Errorf("failed to write owner: %w", err)
	}

	repoPtr, err := wb.writeString(repoContext.Repo)
	if err != nil {
		return fmt.Errorf("failed to write repo: %w", err)
	}

	result, err := wb.validateFn.Call(
		wb.ctx,
		uint64(ownerPtr),
		uint64(len(repoContext.Owner)),
		uint64(repoPtr),
		uint64(len(repoContext.Repo)),
	)
	if err != nil {
		return fmt.Errorf("WASM validation failed: %w", err)
	}

	if len(result) > 0 && result[0] != 0 {
		return &PolicyViolationError{
			Message: fmt.Sprintf("WASM validation error code: %d", result[0]),
		}
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] ValidateAndLock succeeded for %s/%s\n", repoContext.Owner, repoContext.Repo)
	return nil
}

// LockRepositoryWASM locks a repository in WASM
func (wb *WASMBridge) LockRepositoryWASM(owner, repo string) error {
	wb.mu.RLock()
	if wb.closed {
		wb.mu.RUnlock()
		return fmt.Errorf("WASM bridge is closed")
	}
	wb.mu.RUnlock()

	ownerPtr, err := wb.writeString(owner)
	if err != nil {
		return err
	}

	repoPtr, err := wb.writeString(repo)
	if err != nil {
		return err
	}

	result, err := wb.lockFn.Call(
		wb.ctx,
		uint64(ownerPtr),
		uint64(len(owner)),
		uint64(repoPtr),
		uint64(len(repo)),
	)
	if err != nil {
		return fmt.Errorf("WASM lock failed: %w", err)
	}

	if len(result) > 0 && result[0] != 0 {
		return fmt.Errorf("WASM lock error code: %d", result[0])
	}

	return nil
}

// UnlockRepositoryWASM unlocks the repository in WASM
func (wb *WASMBridge) UnlockRepositoryWASM() error {
	wb.mu.RLock()
	if wb.closed {
		wb.mu.RUnlock()
		return fmt.Errorf("WASM bridge is closed")
	}
	wb.mu.RUnlock()

	result, err := wb.unlockFn.Call(wb.ctx)
	if err != nil {
		return fmt.Errorf("WASM unlock failed: %w", err)
	}

	if len(result) > 0 && result[0] != 0 {
		return fmt.Errorf("WASM unlock error code: %d", result[0])
	}

	return nil
}

// CheckLockStatusWASM checks lock status in WASM
func (wb *WASMBridge) CheckLockStatusWASM() (bool, string, error) {
	wb.mu.RLock()
	if wb.closed {
		wb.mu.RUnlock()
		return false, "", fmt.Errorf("WASM bridge is closed")
	}
	wb.mu.RUnlock()

	result, err := wb.checkFn.Call(wb.ctx)
	if err != nil {
		return false, "", fmt.Errorf("WASM check failed: %w", err)
	}

	if len(result) < 1 {
		return false, "", nil
	}

	isLocked := result[0] != 0
	return isLocked, "", nil
}

// writeString writes a string to WASM memory and returns the pointer
func (wb *WASMBridge) writeString(s string) (uint32, error) {
	memory := wb.module.Memory()
	if memory == nil {
		return 0, fmt.Errorf("module has no memory")
	}

	ptr := uint32(1024)
	buf := []byte(s)
	if ok := memory.Write(ptr, buf); !ok {
		return 0, fmt.Errorf("failed to write to memory at offset %d", ptr)
	}

	return ptr, nil
}

// Close closes the WASM bridge and releases resources
func (wb *WASMBridge) Close() error {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.closed {
		return nil
	}

	wb.closed = true
	if wb.module != nil {
		wb.module.Close(wb.ctx)
	}
	if wb.runtime != nil {
		wb.runtime.Close(wb.ctx)
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] WASM bridge closed\n")
	return nil
}
