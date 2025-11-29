//go:build wasm
// +build wasm

package main

import (
	"syscall/js"

	"github.com/github/github-mcp-server/pkg/sessionstate"
)

// Global session instance for WASM module
var wasmSession *sessionstate.Session

func main() {
	// Initialize global session
	wasmSession = sessionstate.NewSession()

	// Register functions with JavaScript
	registerFunctions()

	// Block main from exiting
	select {}
}

func registerFunctions() {
	// Register validateAndLock function
	js.Global().Set("validateAndLock", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 2 {
			return map[string]interface{}{
				"success": false,
				"error":   "validateAndLock requires owner and repo arguments",
			}
		}

		owner := args[0].String()
		repo := args[1].String()

		repoContext := &sessionstate.RepositoryContext{
			Owner: owner,
			Repo:  repo,
		}

		err := wasmSession.ValidateAndLock(repoContext)
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
		}

		return map[string]interface{}{
			"success": true,
		}
	}))

	// Register isRepositoryLocked function
	js.Global().Set("isRepositoryLocked", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return map[string]interface{}{
			"locked": wasmSession.IsRepositoryLocked(),
		}
	}))

	// Register getLockedRepository function
	js.Global().Set("getLockedRepository", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		owner, repo := wasmSession.GetLockedRepository()
		return map[string]interface{}{
			"owner": owner,
			"repo":  repo,
		}
	}))

	// Register resetSession function
	js.Global().Set("resetSession", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		wasmSession = sessionstate.NewSession()
		return map[string]interface{}{
			"success": true,
		}
	}))

	// Register lockRepository function
	js.Global().Set("lockRepository", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 2 {
			return map[string]interface{}{
				"success": false,
				"error":   "lockRepository requires owner and repo arguments",
			}
		}

		owner := args[0].String()
		repo := args[1].String()

		wasmSession.LockRepository(owner, repo)
		return map[string]interface{}{
			"success": true,
		}
	}))
}
