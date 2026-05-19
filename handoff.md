# Handoff: Dang SDK `ModuleTypes`

## Status

Complete. The implementation was committed and pushed on branch `dang-self`.

- Commit: `85215c935 fix(sdk): support dang module types`
- Remote branch: `origin/dang-self`
- PR URL suggested by GitHub: <https://github.com/vito/dagger/pull/new/dang-self>

`handoff.md` itself is intentionally still untracked unless you want it
committed separately.

## What changed

### `core/sdk/dang_sdk.go`

- `dangSDK.AsModuleTypes()` now returns `sdk, true` backed by a real
  `ModuleTypes` implementation.
- Added `dangSDK.ModuleTypes(...)`, following the same scoping pattern as
  other SDKs:
  - gets the current dagql server/query,
  - scopes the module source with `scopeSourceForSDKOperation`,
  - scopes the partially initialized module with
    `ScopeModuleForSDKOperation`,
  - gets schema introspection JSON from `SchemaBuilder`,
  - starts the shared Dang eval path,
  - calls `initDangModule(ctx, dag, env)` to build typedefs.
- Extracted shared nested-client metadata creation into
  `newDangNestedClientMetadata` and reused it from runtime calls.

### `core/sdk/dang_helpers.go`

- Refactored `DangRuntime.eval` setup into `evalDangSource(...)` so runtime
  calls and `ModuleTypes` share the same behavior:
  - loopback nested-client HTTP server,
  - schema introspection decoding,
  - Dagger auto-import config,
  - telemetry stdio wiring,
  - module source mounting,
  - Dang source execution.
- Preserved existing runtime fallback behavior: synthetic module-definition
  calls with an empty parent/function still call `initDangModule`; real calls
  still call `callDangFunction`.
- Added self-call-safe module type extraction:
  - normal Dang module type extraction first tries full `dang.RunDir`,
  - if self-calls are enabled and full inference fails, it falls back to
    signature-only extraction,
  - signature-only extraction parses `.dang` files, injects auto-imports,
    infers declarations/signatures without checking function bodies, evaluates
    enough declarations for `initDangModule`, and avoids the bootstrap failure
    where `test.*` self-call fields are not available yet.
- Empty Dang source directories now return an empty prelude eval env for module
  type extraction.

### `core/integration/module_test.go`

- Added a Dang case to `TestModule/TestSelfCalls` using `--with-self-calls`.
- The regression exercises:
  - `{print(stringArg:"hello")}` → `hello\n`
  - `{printDefault}` → `Hello Self Calls\n`

## Validation

Ran via stable `dagger` as required by project workflow:

```bash
dagger --progress=dots call test-split test-specific --run TestModule/TestSelfCalls > /tmp/test-output.txt 2>&1
```

Result:

```text
ok  github.com/dagger/dagger/core/integration  34.150s
```

Also ran:

```bash
dagger --progress=dots call test-split test-specific --run TestModule/TestBuiltinDangDependencyModules > /tmp/test-output-dang-deps.txt 2>&1
```

Result:

```text
ok  github.com/dagger/dagger/core/integration  71.003s
```

And:

```bash
git diff --check
```

No whitespace errors.

## Current repo state

After committing and pushing, the tracked changes are clean. Remaining
untracked files are:

- `.pi/`
- `handoff.md`
- `toposort-modules.sh`

Do not remove or stage those unless explicitly desired.
