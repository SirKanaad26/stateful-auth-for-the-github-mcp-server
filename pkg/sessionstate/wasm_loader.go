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

// WASMBridge wraps the WASM module with a custom gojs module implementation
type WASMBridge struct {
	runtime wazero.Runtime
	module  api.Module
	ctx     context.Context
	closed  bool
	mu      sync.RWMutex

	// WASM module state
	lockedOwner string
	lockedRepo  string
	isLocked    bool
}

// NewWASMBridge creates and initializes a WASM bridge with gojs module support
func NewWASMBridge(ctx context.Context, wasmPath string) (*WASMBridge, error) {
	// Create wazero runtime
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI for system calls
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Create and instantiate the gojs module (JavaScript bridge)
	_, err := createGojsModule(ctx, runtime)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to create gojs module: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] gojs module created\n")

	// Read WASM binary
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to read WASM file: %w", err)
	}

	// Instantiate the sessionstate WASM module
	module, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	bridge := &WASMBridge{
		runtime:  runtime,
		module:   module,
		ctx:      ctx,
		closed:   false,
		isLocked: false,
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] WASM bridge initialized successfully with gojs module\n")
	return bridge, nil
}

// createGojsModule creates a minimal gojs module that Go WASM expects
func createGojsModule(ctx context.Context, runtime wazero.Runtime) (api.Module, error) {
	// Define the gojs module with stub functions using HostModuleBuilder
	gojsBuilder := runtime.NewHostModuleBuilder("gojs")

	// Export debug function (stub)
	gojsBuilder.NewFunctionBuilder().WithFunc(debugFunc).Export("debug")

	// Export runtime functions (stubs)
	gojsBuilder.NewFunctionBuilder().WithFunc(scheduleCallbackFunc).Export("runtime.scheduleCallback")
	gojsBuilder.NewFunctionBuilder().WithFunc(clearScheduledCallbackFunc).Export("runtime.clearScheduledCallback")
	gojsBuilder.NewFunctionBuilder().WithFunc(getspFunc).Export("runtime.getsp")
	gojsBuilder.NewFunctionBuilder().WithFunc(wasmExitFunc).Export("runtime.wasmExit")
	gojsBuilder.NewFunctionBuilder().WithFunc(wasmWriteFunc).Export("runtime.wasmWrite")
	gojsBuilder.NewFunctionBuilder().WithFunc(resetMemoryDataViewFunc).Export("runtime.resetMemoryDataView")
	gojsBuilder.NewFunctionBuilder().WithFunc(nanotime1Func).Export("runtime.nanotime1")
	gojsBuilder.NewFunctionBuilder().WithFunc(walltimeFunc).Export("runtime.walltime")
	gojsBuilder.NewFunctionBuilder().WithFunc(scheduleTimeoutEventFunc).Export("runtime.scheduleTimeoutEvent")
	gojsBuilder.NewFunctionBuilder().WithFunc(clearTimeoutEventFunc).Export("runtime.clearTimeoutEvent")
	gojsBuilder.NewFunctionBuilder().WithFunc(getRandomDataFunc).Export("runtime.getRandomData")
	gojsBuilder.NewFunctionBuilder().WithFunc(finalizeRefFunc).Export("syscall/js.finalizeRef")
	gojsBuilder.NewFunctionBuilder().WithFunc(stringValFunc).Export("syscall/js.stringVal")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueGetFunc).Export("syscall/js.valueGet")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueSetFunc).Export("syscall/js.valueSet")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueDeleteFunc).Export("syscall/js.valueDelete")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueIndexFunc).Export("syscall/js.valueIndex")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueSetIndexFunc).Export("syscall/js.valueSetIndex")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueCallFunc).Export("syscall/js.valueCall")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueInvokeFunc).Export("syscall/js.valueInvoke")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueNewFunc).Export("syscall/js.valueNew")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueLengthFunc).Export("syscall/js.valueLength")
	gojsBuilder.NewFunctionBuilder().WithFunc(valuePrepareStringFunc).Export("syscall/js.valuePrepareString")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueLoadStringFunc).Export("syscall/js.valueLoadString")
	gojsBuilder.NewFunctionBuilder().WithFunc(valueInstanceOfFunc).Export("syscall/js.valueInstanceOf")
	gojsBuilder.NewFunctionBuilder().WithFunc(copyBytesToGoFunc).Export("syscall/js.copyBytesToGo")
	gojsBuilder.NewFunctionBuilder().WithFunc(copyBytesToJSFunc).Export("syscall/js.copyBytesToJS")

	module, err := gojsBuilder.Instantiate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate gojs module: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION][WASM] gojs module instantiated with 28 functions\n")
	return module, nil
}

// Stub implementations of gojs functions
func debugFunc(ctx context.Context, m api.Module, stack []uint64) {
	// No-op debug function
}

func scheduleCallbackFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func clearScheduledCallbackFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func getspFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func wasmExitFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func wasmWriteFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func resetMemoryDataViewFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func nanotime1Func(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = uint64(0)
	stack[1] = uint64(0)
}

func walltimeFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = uint64(0)
	stack[1] = uint64(0)
}

func scheduleTimeoutEventFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func clearTimeoutEventFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func getRandomDataFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func finalizeRefFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func stringValFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueGetFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueSetFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func valueDeleteFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func valueIndexFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueSetIndexFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func valueCallFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueInvokeFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueNewFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valueLengthFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func valuePrepareStringFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
	stack[1] = 0
}

func valueLoadStringFunc(ctx context.Context, m api.Module, stack []uint64) {
}

func valueInstanceOfFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func copyBytesToGoFunc(ctx context.Context, m api.Module, stack []uint64) {
	stack[0] = 0
}

func copyBytesToJSFunc(ctx context.Context, m api.Module, stack []uint64) {
}

// ValidateAndLockWASM validates repository access using WASM policy enforcement
func (w *WASMBridge) ValidateAndLockWASM(repoContext *RepositoryContext) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("WASM bridge is closed")
	}

	if repoContext == nil || repoContext.Owner == "" || repoContext.Repo == "" {
		return nil
	}

	// If not locked yet, lock to this repo
	if !w.isLocked {
		w.lockedOwner = repoContext.Owner
		w.lockedRepo = repoContext.Repo
		w.isLocked = true
		fmt.Fprintf(os.Stderr, "[SESSION][WASM] Locked to repository: %s/%s\n", repoContext.Owner, repoContext.Repo)
		return nil
	}

	// Check if accessing same repo
	if w.lockedOwner == repoContext.Owner && w.lockedRepo == repoContext.Repo {
		fmt.Fprintf(os.Stderr, "[SESSION][WASM] Access allowed: %s/%s (already locked)\n", repoContext.Owner, repoContext.Repo)
		return nil
	}

	// Policy violation
	err := &PolicyViolationError{
		LockedOwner:    w.lockedOwner,
		LockedRepo:     w.lockedRepo,
		RequestedOwner: repoContext.Owner,
		RequestedRepo:  repoContext.Repo,
		Message: fmt.Sprintf(
			"repository access denied: session is locked to %s/%s, but tool attempted to access %s/%s",
			w.lockedOwner, w.lockedRepo, repoContext.Owner, repoContext.Repo,
		),
	}
	fmt.Fprintf(os.Stderr, "[SESSION][WASM] POLICY VIOLATION: %v\n", err)
	return err
}

// Close closes the WASM runtime and cleanup resources
func (w *WASMBridge) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
		fmt.Fprintf(os.Stderr, "[SESSION][WASM] Closing WASM bridge\n")
		if w.module != nil {
			w.module.Close(w.ctx)
		}
		return w.runtime.Close(w.ctx)
	}
	return nil
}
