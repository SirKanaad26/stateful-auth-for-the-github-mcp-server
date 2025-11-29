//go:build wasm
// +build wasm

package sessionstate

import (
	"syscall/js"
)

// Global session instance for WASM module
var wasmSession *Session

// init initializes the WASM module and registers functions
func init() {
	wasmSession = NewSession()
	registerWasmFunctions()
}

// registerWasmFunctions registers all exported functions to the JavaScript global object
func registerWasmFunctions() {
	js.Global().Set("validateAndLock", js.FuncOf(jsValidateAndLock))
	js.Global().Set("isRepositoryLocked", js.FuncOf(jsIsRepositoryLocked))
	js.Global().Set("getLockedRepository", js.FuncOf(jsGetLockedRepository))
	js.Global().Set("resetSession", js.FuncOf(jsResetSession))
	js.Global().Set("lockRepository", js.FuncOf(jsLockRepository))
}

// jsValidateAndLock validates a tool call against the session policy
// Arguments: owner (string), repo (string)
// Returns: { success: bool, error?: string }
func jsValidateAndLock(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"success": false,
			"error":   "validateAndLock requires owner and repo arguments",
		}
	}

	owner := args[0].String()
	repo := args[1].String()

	repoContext := &RepositoryContext{
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
}

// jsIsRepositoryLocked checks if the session is locked
// Arguments: none
// Returns: { locked: bool }
func jsIsRepositoryLocked(this js.Value, args []js.Value) interface{} {
	return map[string]interface{}{
		"locked": wasmSession.IsRepositoryLocked(),
	}
}

// jsGetLockedRepository gets the repository the session is locked to
// Arguments: none
// Returns: { owner: string, repo: string }
func jsGetLockedRepository(this js.Value, args []js.Value) interface{} {
	owner, repo := wasmSession.GetLockedRepository()
	return map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
}

// jsResetSession resets the session
// Arguments: none
// Returns: { success: bool }
func jsResetSession(this js.Value, args []js.Value) interface{} {
	wasmSession = NewSession()
	return map[string]interface{}{
		"success": true,
	}
}

// jsLockRepository locks the session to a specific repository
// Arguments: owner (string), repo (string)
// Returns: { success: bool }
func jsLockRepository(this js.Value, args []js.Value) interface{} {
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
}
