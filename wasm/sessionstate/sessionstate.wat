(module
  ;; Per-instance global state for session isolation
  ;; Each WASM instance has its own isolated copy of these globals
  (global $isLocked (mut i32) (i32.const 0))
  ;; We'll copy the locked repo info to fixed locations in memory
  ;; to ensure we're comparing actual values, not pointers
  (global $lockedOwnerLen (mut i32) (i32.const 0))
  (global $lockedRepoLen (mut i32) (i32.const 0))

  ;; Memory layout:
  ;; 0-255: locked owner string
  ;; 256-511: locked repo string
  ;; 512+: general purpose
  (memory (export "memory") 1)

  ;; Helper: Copy memory from src to dst
  (func $memcpy (param $dst i32) (param $src i32) (param $len i32)
    (local $i i32)
    (local.set $i (i32.const 0))
    (block $break
      (loop $continue
        (br_if $break (i32.ge_u (local.get $i) (local.get $len)))
        (i32.store8
          (i32.add (local.get $dst) (local.get $i))
          (i32.load8_u (i32.add (local.get $src) (local.get $i)))
        )
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $continue)
      )
    )
  )

  ;; Helper: Compare two strings in memory
  ;; Returns 1 if equal, 0 if not equal
  (func $string_equals (param $ptr1 i32) (param $len1 i32) (param $ptr2 i32) (param $len2 i32) (result i32)
    (local $i i32)

    ;; If lengths differ, strings are not equal
    (if (i32.ne (local.get $len1) (local.get $len2))
      (then (return (i32.const 0)))
    )

    ;; Compare byte by byte
    (local.set $i (i32.const 0))
    (block $break
      (loop $continue
        ;; Exit loop if we've compared all bytes
        (br_if $break (i32.ge_u (local.get $i) (local.get $len1)))

        ;; Compare current byte
        (if (i32.ne
          (i32.load8_u (i32.add (local.get $ptr1) (local.get $i)))
          (i32.load8_u (i32.add (local.get $ptr2) (local.get $i)))
        )
          (then (return (i32.const 0)))
        )

        ;; Increment counter and continue
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $continue)
      )
    )

    ;; All bytes matched
    (i32.const 1)
  )

  ;; validate_and_lock: Validates that we can access a repository
  ;; Returns 0 on success, 1 on policy violation
  (func (export "validate_and_lock") (param $ownerPtr i32) (param $ownerLen i32) (param $repoPtr i32) (param $repoLen i32) (result i32)
    ;; Check if already locked
    (if (i32.eqz (global.get $isLocked))
      (then
        ;; Not locked - copy strings to our storage and lock
        (call $memcpy (i32.const 0) (local.get $ownerPtr) (local.get $ownerLen))
        (call $memcpy (i32.const 256) (local.get $repoPtr) (local.get $repoLen))
        (global.set $lockedOwnerLen (local.get $ownerLen))
        (global.set $lockedRepoLen (local.get $repoLen))
        (global.set $isLocked (i32.const 1))
        (return (i32.const 0))
      )
    )

    ;; Already locked - check if it matches
    ;; Compare owner (stored at offset 0)
    (if (i32.eqz (call $string_equals
        (i32.const 0) (global.get $lockedOwnerLen)
        (local.get $ownerPtr) (local.get $ownerLen)))
      (then (return (i32.const 1))) ;; Policy violation
    )

    ;; Compare repo (stored at offset 256)
    (if (i32.eqz (call $string_equals
        (i32.const 256) (global.get $lockedRepoLen)
        (local.get $repoPtr) (local.get $repoLen)))
      (then (return (i32.const 1))) ;; Policy violation
    )

    ;; Everything matches
    (i32.const 0)
  )

  ;; lock_repository: Locks to a specific repository
  (func (export "lock_repository") (param $ownerPtr i32) (param $ownerLen i32) (param $repoPtr i32) (param $repoLen i32) (result i32)
    ;; Copy strings to our storage
    (call $memcpy (i32.const 0) (local.get $ownerPtr) (local.get $ownerLen))
    (call $memcpy (i32.const 256) (local.get $repoPtr) (local.get $repoLen))
    (global.set $lockedOwnerLen (local.get $ownerLen))
    (global.set $lockedRepoLen (local.get $repoLen))
    (global.set $isLocked (i32.const 1))
    (i32.const 0)
  )

  ;; unlock_repository: Unlocks the current repository
  (func (export "unlock_repository") (result i32)
    (global.set $isLocked (i32.const 0))
    (global.set $lockedOwnerLen (i32.const 0))
    (global.set $lockedRepoLen (i32.const 0))
    (i32.const 0)
  )

  ;; check_lock_status: Returns 1 if locked, 0 if not locked
  (func (export "check_lock_status") (result i32)
    (global.get $isLocked)
  )
)
