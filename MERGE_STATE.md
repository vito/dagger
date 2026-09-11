# Integration merge state

`vitoland` is a local dogfooding branch. Do not push or ship it. Source changes belong on their source branches; integration resolutions and this record belong here.

## September 11 rebuild

Base: `upstream/main` at `ff626243dca48bad2ff5c18e2f2b401f58f3c83d`, verified against the remote on September 11, 2026.

Previous integration tip: `d02bab6afb18820e85b2e133bcf4db9e28a3b172`, retained as `backup/vitoland-2026-09-11`. Its `MERGE_STATE.md` contains the September 10 rebuild and validation history. The previous tip also included the later lockfile update, form-focus fix, Git-author routing fix, and agent bump, which were not recorded there.

Rebuilt in `/home/vito/src/dagger/vitoland-rebuild-20260911` before replacing the original checkout. The rebuilt `vitoland` has no remote tracking branch. No source branches or remotes were changed or pushed; the two stale patch/MCP remote-tracking refs were refreshed during planning.

| Order | Source | Integrated tip | Merge |
| --- | --- | --- | --- |
| 1 | `extract/agent-runtime` | `fba469284ab48e99b1574085ba71bb2f101dd3f9` | `bb5103142c` |
| 2 | `workspace-git` | `5d5af85bd26bcc32c2ed51cfe027273fe0d94e86` | `847d6e1ccc` |
| 3 | `origin/client-lifecycle-v2` | `be630ff9f25ace13507fd03855bad7074f678a9d` | `9e2a5d7ab4` |
| 4 | `origin/fix/patch-apply-repo-discovery` (#14071) | `69474a94654b382a8bc40ab9b4240a0529114f2b` | `e74c76e9be` |
| 5 | `origin/fix/mcp-mount-rebind-summary` (#14074) | `9fcad2d9c9781e6f2b4f979b57fc5d27c804de08` | `e41d7a5a70` |

Agent runtime, workspace-git, patch application, and MCP were rewritten: their previously integrated tips are not ancestors of these heads. This rebuild uses fresh merges from main rather than extending the old integration graph. The source tips are pinned to the approved plan.

## Omitted and already preserved changes

- Dump-ID modes (#14073) are now upstream, alongside the previously upstream mount search (#14068), patch preview width (#14069), changeset diff headers (#14070), and legacy toolchain loading in value workspaces.
- Workspace reloaded identity (#14072) remains superseded. The current workspace API uses explicit `Workspace.snapshot`; it does not restore `Workspace.sync`, `reloaded`, or host-read epochs.
- Form-focus restoration (`25683e5e68` on the old integration) is now in agent runtime as `701e6bb392`.
- Git-author routing through the non-module parent client (`ba5bc02694`) is included in workspace-git, including the regression test.
- Workspace-git also includes detached-HEAD bundle transport handling, acceptance of ancestor bundle prerequisites, thin-pack and sparse-pack fixes, and the SDL diff declaration-order behavior. Their old commits were not replayed separately.
- The separate `sdl-schema-order` branch was not an integration source and was not added.

## Preserved direct commits

The five historical configuration commits were preserved, followed by the two later lockfile commits. Each replay retains cherry-pick provenance.

| Previous replay or direct commit | New replay | Purpose |
| --- | --- | --- |
| `1de3e6b180` (original `c6867f7063`) | `70e655e656` | Install editor |
| `b04c206a9e` (original `f6c2266746`) | `a5836f2121` | Sync local development modules and install contributor |
| `53df956f95` (original `0d082a75ea`) | `020d885571` | Install and polish go-cli |
| `33598691ec` (original `193c6be7cf`) | `1751809511` | Install staff and committer |
| `84dc3b7286` (original `dd4ca5a411`) | `41995b8a34` | Historical agent bump |
| `2d91b46802` | `79d9b76265` | Update lockfile format and pins |
| `d02bab6afb` | `b6926e65f6` | Latest agent bump |

All 32 module configurations are preserved. The only differences from the previous integration's root module configuration are upstream's explicit `@v0.1` selectors for the Go and Dang SDK modules. Existing migration comments, module settings, and the tla-check declaration remain intact.

The final agent pin is `9750b76c7afdb4051f34a9dccfbc7959d1921401`, and the editor pin is `5481e5ce3a8b97ffb8beaee3a569ad9100e16167`. Lock reconciliation (`a2746eb2fb`) retains the later v1 SDK selections, adds the pins for upstream's explicit v0.1 selectors, and retains four new upstream Git entries with normalized addresses. Incidental test/generator image resolutions were discarded. Lockfile comments remain supported and intact.

## Integration resolutions

- Ported the prior workspace integration onto the current per-agent runtime. Startup composition and save/reload materialize `currentWorkspace.snapshot.id` once and bind the returned workspace by ID. Plain conversations without workspaces remain supported.
- Retained per-agent previews and synchronization baselines. Save integrates commits into the current workspace and merges pending edits before exporting; successful save/reload advances the baseline and reseeds the runtime. Failed exports preserve the agent workspace and baseline.
- Preserved per-agent save/reload serialization, busy-turn checks, asynchronous UI refresh, and the runtime's new visible save/reload activity spans.
- Retained queued prompt forms, explicit confirmations, creation on the UI goroutine, placement above the shell draft, and scoped focus restoration. Ported the prior approval-form and workspace-preview regressions.
- Preserved lifecycle tombstone and detached-loop leases, dormant-resume error propagation, delegated push authorization using held scopes and validated ancestry, cloned metadata, and current caller routing. Retained the current workspace/value loading paths without reviving removed host-read epochs. The old attachables regression remains ported to the session-owned resolver.
- Resolved the duplicate `MountPoints` methods introduced independently by workspace-git and MCP. The final accessor is nil-safe and returns a cloned slice (`cb09012ee1`).
- Retained both workspace Git export tests and upstream's CLI export tests.
- Respected upstream removal of the generated PHP reference, including newly added agent pages. Regenerated current and rolling API stubs (`303e596a18`). The CLI reference and six SDK generators completed successfully and required no further changes.

## Validation

Tests ran through the rebuilt CLI at `/tmp/vitoland-dagger`, using `DAGGER_ENGINE=container://dagger-engine.dev` as the bootstrap engine and `api call engine-dev test` to build and test the integration's own engine and CLI. The installed beta was not used to run these tests. Suites ran sequentially. Source under test includes the final mount-accessor correction; later commits only record that correction, reconcile the lockfile, and update generated documentation and this record.

Counts below are the runner's reported entries; suite entries include selected nested tests and should not be read as a count of individual assertions or subtests.

| Check | Result | Trace |
| --- | --- | --- |
| `go build ./cmd/dagger ./cmd/engine ./cmd/dump-id` | Passed | Local |
| `TestCaptureGit*` | 23 passed | [capture](https://dagger.cloud/dagger/traces/9b2e1b3c4e6ceae5644c51fbc07ebcd2) |
| Core agent leases/wait guards, bundle refs/validation, MCP mount summaries | 9 passed | [core](https://dagger.cloud/dagger/traces/5692ef2f863b99ee1850f212cce77ae5) |
| CLI preview rendering, reseeding, rewind, debug-listener cancellation | 4 passed | [CLI](https://dagger.cloud/dagger/traces/e82e1207821d217d71df6ab736fdcb02) |
| `TestDaggerCMD/TestAgentWorkspace*` | Passed: selected save/reload and no-Git tests, 1 suite entry | [CLI workspace](https://dagger.cloud/dagger/traces/5902fcd9bb16778e26440c87b9b3b46d) |
| Push approvals, client scopes, caller routing | 13 passed | [server](https://dagger.cloud/dagger/traces/c2504579925478e8618ac7387d8e38e3) |
| Agent lifecycle/reseed/spawner-release, workspace snapshots/exports/Git directory/author, delegated pushes, CLI export | Passed: selected tests across 4 suite entries | [integration](https://dagger.cloud/dagger/traces/b9bc5f5064ddc93ab1bcc754b47caa67) |
| Prompt focus, push confirmations, passphrases, checkpoint selection | 7 passed | [TUI](https://dagger.cloud/dagger/traces/f6f0dad075a040c8cba44d321875560d) |
| API reference plus Go, TypeScript, Python, PHP, Rust, Elixir client generators | 7 generators passed; no further changes | `/tmp/vitoland-generate.log` |
| CLI reference generator | Passed; no changes | `/tmp/vitoland-cli-docs.log` |

The first engine-dev build caught the duplicate mount accessor before tests ran ([trace](https://dagger.cloud/dagger/traces/249b30644d98b810561255acf8c1b962)); the corrected build and all listed checks pass. The previously failing `TestAgentDebugServerContextCancellation` passes in this rebuild without a test exclusion or unrelated debug-server fix.

An initial broader integration selection also passed ([trace](https://dagger.cloud/dagger/traces/e742c698849633c4414effa42de85b5d)); the final anchored selection additionally covers `TestWorkspace/TestExportCLI`. All pinned source ancestry checks, module configuration comparisons, TOML/lock parsing, and diff whitespace checks pass. The full repository and race suites were not run.
