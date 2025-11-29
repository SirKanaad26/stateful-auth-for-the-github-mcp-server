/**
 * SessionState WASM Module Wrapper
 * 
 * This module provides a JavaScript/TypeScript interface to the
 * sessionstate WASM module for stateful authorization enforcement.
 */

export interface SessionStateWasm {
  validateAndLock(owner: string, repo: string): { success: boolean; error?: string };
  isRepositoryLocked(): { locked: boolean };
  getLockedRepository(): { owner: string; repo: string };
  resetSession(): { success: boolean };
  lockRepository(owner: string, repo: string): { success: boolean; error?: string };
}

export interface ValidationResult {
  success: boolean;
  error?: string;
}

export interface LockStatus {
  locked: boolean;
  owner?: string;
  repo?: string;
}

/**
 * Initialize the SessionState WASM module
 * @param wasmPath Path to the compiled sessionstate.wasm file
 * @returns Promise resolving to the WASM module interface
 */
export async function initSessionStateWasm(wasmPath: string): Promise<SessionStateWasm> {
  // Import wasm_exec.js shim if in Node.js
  if (typeof window === 'undefined') {
    const go = new (require('wasm_exec.js').Go)();
    const wasmModule = await WebAssembly.instantiateStreaming(
      fetch(wasmPath),
      go.env
    );
    go.run(wasmModule.instance);
  }

  // Return the global functions injected by the WASM module
  return {
    validateAndLock: (owner: string, repo: string) => {
      return (globalThis as any).validateAndLock(owner, repo);
    },
    isRepositoryLocked: () => {
      return (globalThis as any).isRepositoryLocked();
    },
    getLockedRepository: () => {
      return (globalThis as any).getLockedRepository();
    },
    resetSession: () => {
      return (globalThis as any).resetSession();
    },
    lockRepository: (owner: string, repo: string) => {
      return (globalThis as any).lockRepository(owner, repo);
    },
  };
}

/**
 * Validate a tool call against the session policy
 * @param wasm The initialized WASM module
 * @param owner Repository owner
 * @param repo Repository name
 * @returns Validation result
 */
export function validateToolCall(
  wasm: SessionStateWasm,
  owner: string,
  repo: string
): ValidationResult {
  return wasm.validateAndLock(owner, repo);
}

/**
 * Get the current session lock status
 * @param wasm The initialized WASM module
 * @returns Lock status
 */
export function getSessionLockStatus(wasm: SessionStateWasm): LockStatus {
  const locked = wasm.isRepositoryLocked().locked;
  if (!locked) {
    return { locked: false };
  }

  const repo = wasm.getLockedRepository();
  return {
    locked: true,
    owner: repo.owner,
    repo: repo.repo,
  };
}

/**
 * Reset the session (typically called on new conversation)
 * @param wasm The initialized WASM module
 */
export function resetSessionState(wasm: SessionStateWasm): void {
  wasm.resetSession();
}
