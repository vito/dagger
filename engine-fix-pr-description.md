# Fix index out of range panic in container exec error handling

Fixes a panic that occurs during container execution error handling when there's a mismatch between mount indices and output indices.

## The Bug

```
panic: runtime error: index out of range [2] with length 2
  at /app/core/container_exec.go:302
```

The error handling code in `WithExec`'s deferred function was incorrectly assuming that iteration indices through the `results` array correspond to indices in `p.OutputRefs`. This breaks when there are mounts with no outputs (readonly mounts, cache mounts, tmpfs, secrets, sockets).

## Root Cause

There are two distinct index spaces for mounts:

1. **Mount index**: Position in the `mounts.Mounts` array (always sequential: 0, 1, 2, 3, ...)
2. **Output index**: The `mount.Output` field value (can have gaps: 0, 1, 2, -1, -1, 3, -1, ...)

The `results` array from `extractContainerBkOutputs` is indexed by **output index**, not by mount position or iteration order. The buggy code was iterating `results` and assuming indices aligned with `p.OutputRefs`, which they don't.

Example scenario that triggers the panic:
- Mount 0 (root): Output = 0
- Mount 1 (meta): Output = 1  
- Mount 2 (user bind): Output = 2
- Mount 3 (cache): Output = -1 (SkipOutput)
- Mount 4 (user bind): Output = 3

Result: `results` has length 4, but when iterating we'd try to access `p.OutputRefs[3]` when it might only have 3 entries (or be ordered differently).

## The Fix

Changed both loops to follow the same pattern as `setAllContainerMounts` in `dagop.go`:

1. **First loop** (lines 301-306): Iterate through `p.OutputRefs`, look up each mount using `outputRef.MountIndex`, then use `mount.Output` to index into `results`.

2. **Second loop** (lines 320-399): Iterate through `mounts.Mounts` by `mountIdx`, skip mounts with `Output == pb.SkipOutput`, and use `mount.Output` to index into `results`.

Both fixes properly map between mount indices and output indices, preventing the out-of-bounds access.
