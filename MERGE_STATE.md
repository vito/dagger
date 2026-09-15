# Integration merge state

`vitoland` is a dogfooding integration branch. Push it only when explicitly requested; do not ship it. Source changes belong on their source branches; integration resolutions and this record belong here.

## September 15 integration rebuild — current

Recreated from pinned `upstream/main` **`82cc7d32d7a499a95c5b600d5bb6851d57613d96`**. Code and generated-output checkpoint: **`efc267fefd`**. Previous vitoland `2dc0d2fb8040ead77fa8f1bfe25f351763fa19f5` is retained as **`backup/vitoland-before-rebuild-20260915`**. This section supersedes all earlier source tables, pending-rebuild notes, and remainder ownership. Older dated sections are historical evidence.

### Current source inputs

| Source | Selected tip | Published tip checked | Integration | PR |
| --- | --- | --- | --- | --- |
| `workspace-git` | `69ca13c05b` | `69ca13c05b` | `219f2e972a` | [#14147](https://github.com/dagger/dagger/pull/14147), open |
| `client-lifecycle-v2` | `c50b618e1f` | `64747bfdcf` | `8f71c58359` + `efc267fefd` | [#14047](https://github.com/dagger/dagger/pull/14047), open |
| `fix/patch-apply-repo-discovery` | `20ab107ea6` | `20ab107ea6` | `cdcc5e6a64` | [#14071](https://github.com/dagger/dagger/pull/14071), open |
| `fix/mcp-mount-rebind-summary` | `1309a5386f` | `1309a5386f` | `092477bd10` | [#14074](https://github.com/dagger/dagger/pull/14074), open |

The first lifecycle merge includes the six review/lint/fixture commits after `a38b78b35f`. Full server validation then exposed a pre-existing admission-window race in the initialization-lock test. Source-owned follow-up `c50b618e1f` waits for the correct lock boundary; it is committed locally on client-lifecycle and included by `efc267fefd`, but is **not pushed**. Workspace includes the three post-rehome follow-ups through `69ca13c05b`. Source branches were not rebased or reset.

Runtime #14003 (`ab4cb5efc0`), defining schemas #14142 (`363a3c7828`), and idle telemetry #14141 (`2afd1cf7e3`) were omitted as independent merges. Their exact final PR heads and merge commits are ancestors of pinned main. Patch discovery and MCP summaries remain open and their changes still require integration.

### Retired integration patches

| Old integration home | Current implementation/regression home |
| --- | --- |
| `42d8367be9`: credential lookup leases | Lifecycle `a24cb4c440`, strengthened HTTP regression `45d97eb023` |
| Agent lifetime portions of `19511cee9f` | Lifecycle `114c38a08a` |
| Recipe lookup and shared trace helper portions of `19511cee9f` | Workspace `e0b1dd9fdc`, `5487605621` |
| `87a9e7d254` + `4374b936c0`: frozen startup, saves, previews, toolset | Workspace `eefde1438c`, `91b30a18e8`, `192f9ad6f7` |
| `87a9e7d254` + `3edb882809`: queued forms and approval tests | Workspace `4b02f94a7a`, `330cf189db` |
| `161efdaffb`: defining-schema/runtime adaptation | Main `6bce1909a0` + `82cc7d32d7`, including stronger returned-dependency tests |
| `fa3a23dbe4`: idle cache/OpenStats adaptation | Lifecycle `7fd6fbe5ba`: runtime/read references replace the idle LRU; OpenStats retained |
| `23c566161c`: runtime and reference generator | Main `82cc7d32d7`; generator implementation preserved byte-for-byte |

### Current integration remainders

| Home | Remaining work and removal condition |
| --- | --- |
| `8f71c58359` | Combine value workspaces with lifecycle caller scopes, owner metadata, current server stubs, and validated Git push owner/ancestor lookup. Retain until the independent APIs share an upstream home. Keep `core/workspace_context.go`, `core/schema/workspace.go`, `engine/server/session_workspaces.go`, and Git push coverage aligned. The combined workspace API removes host-read epochs; `workspace_epoch_test.go` remains only in standalone lifecycle, where the epoch API exists. |
| `7acac44344` | Both workspace and MCP sources independently add `MountPoints`. Keep workspace's nil-safe cloned accessor (`a912ed64eb`) and remove the other ten-line definition. This is still required despite a clean MCP merge. |
| `7681df8a1f` + `1ba589d2ce` | Preserve all 32 local module configurations, SDK scopes/settings, QA profiling/resume/tools, credential forwarding, and dependency pins. Agent stays `9750b76c7afdb4051f34a9dccfbc7959d1921401`; editor stays `5481e5ce3a8b97ffb8beaee3a569ad9100e16167`. Root configuration and local module trees match the backed-up vitoland exactly. |
| `efc267fefd` | Combined references and SDK outputs were regenerated successfully with no tree changes. On future rebuilds, regenerate from the integrated schema; retain the source-owned current/rolling generator, handwritten pages, and upstream removal of obsolete trees. |

### Delta and preservation audit

Excluding this bookkeeping file, the old rebuild changed **646 files (+82,368/−8,436)** against its main `18c593d59b`; the new checkpoint changes **399 files (+44,834/−5,681)** against main `82cc7d32d7`. This measures the reduction after merged work moved into the base. For an apples-to-apples comparison, the old tree against the same new main was **396 files (+43,803/−6,079)**: most old-to-new tree changes are newer source fixes and API evolution, rather than wholesale feature removal.

The dated extraction mapping `vitoland-rebuild-20260915.tsv` records all retired/current homes. `vitoland-rebuild-paths-20260915.tsv` accounts for every old-to-new changed or removed path. Source-exact paths were checked by Git blob identity. The combined schema/routing, tunnel, session, and test deltas were reviewed against current source changes and audit merge `2288439c7b`. All current source and remainder commits are ancestors of this checkpoint. Local configuration, intentional lock pins, and the reference generator were checked separately rather than overwritten from old whole files.

### Validation and final state

All seven runs passed sequentially through `dagger api call engine-dev test`, using the separate `dagger-engine.clientdb-idle-cache` bootstrap and the rebuild's source. Counts are runner entries; some contain nested cases.

| Check | Result | Trace |
| --- | --- | --- |
| Full server (race) | 148 passed | [trace](https://dagger.cloud/dagger/traces/f03dec518b7edb94aa3d6136b6e4dc41) |
| Full core (race) | 404 passed | [trace](https://dagger.cloud/dagger/traces/1e36671a9ca3a19bbb3c08ff691aa54d) |
| All telemetry stores (race) | 34 passed | [trace](https://dagger.cloud/dagger/traces/1e33ea0c88d91961715f6940826aa350) |
| Workspace save/reload/toolset | 2 passed | [trace](https://dagger.cloud/dagger/traces/7325c53db5c9bae6424daa2ab7755f11) |
| Recipe provenance | 3 passed | [trace](https://dagger.cloud/dagger/traces/00adccf5f8208f4749315355da6fde6b) |
| Queued prompts and console | 12 passed | [trace](https://dagger.cloud/dagger/traces/42179eec0ae7e6ce822eac0cc7cdb184) |
| Git, workspace export, runtime/spawner integration | 3 passed | [trace](https://dagger.cloud/dagger/traces/534d066ad3e67803f3b24b0c48f80005) |

The core, server, and clientdb runs used `--run '^Test' --race=true --timeout 10m`; other selections used the exact runbook regexes below with `--timeout 10m`. All eight API/SDK generation targets, CLI reference generation, current/rolling synchronization (80 generated API stubs plus sidebar entries), and combined QA module loading passed without generated tree changes. Full repository suites, benchmarks, and post-publication CI are not claimed. Validation caught and fixed the duplicate accessor and the lifecycle test admission race described above.

The extraction ledger preserves all 214 immutable inventory rows and appends the ten post-rehome source follow-ups to its mapping (124 rows total). Its existing historical submitted-head assertion still fails; it was not rewritten or presented as a passing full extraction audit. The separate tracking repository has no remote and is committed locally.

This rebuild is **local only**. Published vitoland remains `8366671a2a4ac4ff8ece8cc2047181f88a73f5b1`; no integration, source, scratch, or backup refs were pushed. The original vitoland checkout is updated only after verification. No binaries were installed and the user's running dev engine was not restarted. Scratch worktree: `/home/vito/src/dagger/vitoland-rebuild-20260915`; optional detailed logs and commands: `/tmp/vitoland-rebuild-20260915/`.

## September 14 source rehome after runtime merge

The source branches are rebased locally on `82cc7d32d7a499a95c5b600d5bb6851d57613d96`, which includes agent runtime #14003, defining schemas #14142, and idle telemetry #14141. This section supersedes the older source-status and remainder ownership tables. Vitoland's production tree is still the September 14 workspace API rebuild at `5b875b7247`; this operation only updates its bookkeeping. No refs were pushed and the user's running engine was not restarted.

### Current sources for the next rebuild

| Source | Remote | Current local tip | Published tip at start | PR |
| --- | --- | --- | --- | --- |
| `workspace-git` | `origin` | `66cb9b1d7b` | `6a358ab81f` | [#14147](https://github.com/dagger/dagger/pull/14147) |
| `client-lifecycle-v2` | `origin` | `a38b78b35f` | `876111055a` | [#14047](https://github.com/dagger/dagger/pull/14047) |
| `fix/patch-apply-repo-discovery` | `origin` | `20ab107ea6` | `20ab107ea6` | [#14071](https://github.com/dagger/dagger/pull/14071) |
| `fix/mcp-mount-rebind-summary` | `origin` | `1309a5386f` | `1309a5386f` | [#14074](https://github.com/dagger/dagger/pull/14074) |

Use these local sources, including their unpublished follow-ups, for the next rebuild. Runtime, defining schemas, and idle telemetry are represented in pinned main; do not replay their old source branches. Continue to fetch and inspect newer source/main work when actually rebuilding. The earlier completed-rebuild table describes what the unchanged vitoland code contains, rather than today's source tips.

All 59 original lifecycle and 55 original workspace commits are retained. Lifecycle's clean rebase preserves its newer nested-client, teardown, and host-tunnel fixes. Workspace's runtime conflicts move the old LLMSession behavior onto sessionAgent, including frozen startup, baseline-relative previews, serialized saves without stopping/reseeding, retries, and the selected runtime's toolset. The newer workspace export changes remain: no bound export state, explicit destinations for frozen values, and local export comparisons via `from`. Current upstream mailbox-drain, graceful-stop, defining-schema, and idle-store opener regressions remain intact.

### Integration adaptations now owned by sources

| Former integration home | Current source home | Behavior |
| --- | --- | --- |
| `42d8367be9` | lifecycle `a24cb4c440` | Credential lookup borrows a current shared-work lease and remains bounded by the routing session and timeout. |
| `19511cee9f` | lifecycle `114c38a08a` | Detached agent-loop and tombstone leases, executable agent context, dormant resume errors, and lifetime regression. |
| `19511cee9f` | workspace `e0b1dd9fdc`, `5487605621`, `a912ed64eb` | Current recipe lookup, one shared trace-capture helper, and one nil-safe cloned mount accessor. |
| `87a9e7d254` + `4374b936c0` | workspace `eefde1438c`, `91b30a18e8`, `192f9ad6f7` | Runtime workspace startup/save/preview/toolset behavior and tests, adapted during rebase. |
| `87a9e7d254` + `3edb882809` | workspace `4b02f94a7a`, `330cf189db` | Queued workspace approvals, explicit choices, draft/focus preservation, and cancellation regressions. |
| `161efdaffb` | main `82cc7d32d7` | Complete defining-schema coverage now coexists with runtime support, including stronger returned-dependency assertions. |
| `fa3a23dbe4` | main `ef97b2d159` + lifecycle `a38b78b35f` | Idle-store reuse plus lifecycle-owned OpenStats, including accurate idle-store comments and upstream opener injection. |

Do not replay these old integration deltas after merging the updated sources. Their behavior has a source or upstream home and was checked there.

### Remaining integration-only work

Keep the validated Git push delegation from `19511cee9f` and the value-workspace/client-routing resolution from `ad543ba1bd` until both independent PRs land. Preserve caller execution authority, owner metadata for host routing, and removal of host-read epochs. They require both still-open source APIs and therefore cannot move into either independent PR yet. Audit merge `2288439c7b` on `audit/post-runtime-integration-20260914` demonstrates the current resolution and was tested for push/scope routing; it is not a replacement vitoland rebuild.

Retain local development modules/settings/pins from `8ff586ec8c` and regenerate the combined references represented by `5b875b7247` during a future rebuild. All 143 path differences between the combined source audit and existing vitoland were accounted for by newer main/source work, the two small sources omitted from that focused audit, local development configuration, or generated references. Key runtime/save/approval/credential/push boundaries match the old integration with the intended newer changes.

Backups: `backup/client-lifecycle-before-runtime-merge-20260914`, `backup/workspace-git-before-runtime-merge-20260914`, and `backup/vitoland-before-runtime-rehome-20260914`. The source checkouts were clean before their verified local updates. Source mapping is durable in `~/src/dagger-extraction/source-rebase-post-runtime-20260914.tsv` (114 rows), with remainder ownership in `runtime-rehome-20260914.tsv`. The handoff, follow-on table, and idle-store landing metadata are updated. The historical ledger checker still reports its existing stale submitted-head assertion; all 214 immutable inventory rows pass. Original historical manifests are unchanged.

### Validation

| Check | Result | Trace |
| --- | --- | --- |
| Client agent/credential leases (race) | Passed | [trace](https://dagger.cloud/dagger/traces/c7a06b0bfbc15cc0846a13c46f45b0e7) |
| Client scopes, routing, and teardown (race) | Passed | [trace](https://dagger.cloud/dagger/traces/dad538014e65bd7ffc4437ea1f1af866) |
| Runtime reseed and released-spawner execution | Passed | [trace](https://dagger.cloud/dagger/traces/51ccdb8c89a06686d45a3911cdb621d9) |
| Workspace CLI saves, previews, and toolset | Passed | [trace](https://dagger.cloud/dagger/traces/2e7a7ec4be0b46db5a915d7248c7ed51) |
| Queued approvals and console endpoints | Passed | [trace](https://dagger.cloud/dagger/traces/da792b924c5c41b25a8ea665c8422d01) |
| Git remotes, pushes, and workspace exports | Passed | [trace](https://dagger.cloud/dagger/traces/557b3eceab0bc5364d3143072ecb1974) |
| Recipe/module provenance | Passed | [trace](https://dagger.cloud/dagger/traces/bb3a21b1ca2984ec545d637bb0d1e38d) |
| Recipe classification | Passed | [trace](https://dagger.cloud/dagger/traces/2cb48611b29371bcebe7aa7582aa8449) |
| Combined push delegation and scopes (race) | Passed | [trace](https://dagger.cloud/dagger/traces/364cdc6b4b360fcfaaee5e7e9324f1ba) |
| API/SDK generation (8 targets), CLI references | Passed; 89 matching current/rolling API stubs | Local generators |

Runs used the separate `dagger-engine.clientdb-idle-cache` bootstrap with `dagger api call engine-dev test`, building each tested branch. Test summaries can collapse nested tests under their suite; direct trace inspection confirmed the released-spawner regression executed. Workspace recipe classification was also tested explicitly. API/SDK and current/rolling CLI references were regenerated from the workspace branch. Whitespace and source/remainder audits pass. Full suites, benchmarks, and CI are not claimed. Detailed logs and range diffs remain under `/tmp/post-runtime-rebase-20260914/` as optional supporting artifacts.

## Rebuild procedure

This section is the runbook for a future request such as **“rebuild vitoland — see MERGE_STATE.md.”** That means reconstructing the integration branch from current main and its source branches, preserving its local behavior, validating it, and updating the bookkeeping. It does not mean installing binaries or restarting the user's running engine. A bare rebuild request in a fresh session does not request publication; honor any additional push authorization in that session.

Use the September 15 current record above and this procedure as the current authority. Older dated sections are historical evidence, not additional source lists or instructions to replay superseded behavior. The latest completed rebuild’s code checkpoint is `efc267fefd`; inspect subsequent commits too. This file and retained Git history contain the reconstruction inputs. `/tmp` scripts, logs, old scratch worktrees, and previous chat context are optional aids, not prerequisites.

### Select and preserve the inputs

1. Inspect the integration checkout, `git worktree list`, source checkouts, branch tracking configuration, and local changes. Preserve committed and uncommitted user work. Create a uniquely named backup ref for the **current** vitoland tip before changing it; keep the original checkout usable while rebuilding in a separate worktree.
2. Fetch `upstream/main` and the source remotes in the current source table above. Pin the chosen main SHA, each source SHA, and the existing published vitoland SHA for the rebuild. The recorded integrated tips are comparison baselines, not instructions to use stale heads. Include newer local commits as well as published work; never reset a source checkout to its remote to make it match. Compare rewritten histories by patch and intent. Investigate divergent heads before choosing a tip that could omit work.
3. Check the current status and actual main coverage of each source PR. Omit a source only when its intended work is represented upstream, accounting for squash merges and any later branch commits. A missing branch or a closed PR alone is not evidence that its work can be dropped. The four rows in the current source-status table are the active starting set; the older completed-rebuild table records the unchanged integration history. Archived extraction branches and unrelated local branches are not implicit rebuild inputs.
4. Audit work added to vitoland since the last completed rebuild, including new merges, direct fixes, module settings, lock pins, and notes. Give each change a source home or explicit integration remainder. Do not restrict this audit to the old remainder table. A rebuild normally merges source branches without rewriting them; rebasing all sources is a separate task.

### Reconstruct and audit the integration

Create the scratch branch at the pinned main commit, then merge active sources with `--no-ff` in table order: runtime, workspace, lifecycle, patch discovery, MCP summaries, defining schemas, idle telemetry stores. Skip only sources whose work has been verified upstream. Restore integration adaptations as their dependencies become available, then regenerate outputs. Commit coherent resolutions with sign-offs and provenance, following the repository's committing skill.

The current remainder table above is mandatory coverage, not a list to cherry-pick blindly. Ordinary remainder commits can be replayed or adapted; a merge commit also contains an entire source branch, so inspect its resolution instead of cherry-picking it with `-m 1`. Use `git show --remerge-diff <merge>` where useful and compare the affected files with the backed-up integration. Some necessary adaptations were separate follow-ups after clean merges. **A clean merge is not evidence that an integration fix survived.**

Use these file groups to check the combined behavior:

| Integration boundary | Files to inspect | Required behavior |
| --- | --- | --- |
| Workspace and runtime CLI | `internal/cmd/dagger/{agent,functions,llm,session_agent}.go`, `llm_changes_test.go`, `session_agent_test.go` | Freeze startup workspace once; preserve per-agent previews, baselines, failed-export retries, busy guards, save/reload serialization, and asynchronous refresh. Successful save advances the baseline without stopping or reseeding the runtime. Reload is explicit. Toolset follows focus and the latest runtime snapshot. |
| Runtime and lifecycle | `core/agent.go`, `core/agent_client_scope_test.go`, `core/llm_credential.go`, `core/llm_auth_test.go`, `engine/server/session.go` | Keep detached-loop and tombstone leases, credential lookup leases, executable agent contexts, and dormant-resume error handling. |
| Workspace and lifecycle | `core/schema/workspace.go`, `core/workspace_context.go`, `engine/server/session_workspaces.go`, `engine/server/git_push{,_test}.go`, `engine/server/session_test.go` | Preserve value workspaces, validated push ownership/ancestry, metadata snapshots, and current attachable routing. Do not resurrect host-read epochs or obsolete client APIs. |
| Queued prompts and workspace approvals | `dagql/idtui/frontend_pretty.go`, `frontend_command_view_test.go`, prompt-form tests | Retain queued forms, explicit confirmations, draft/focus restoration, and approval/checkpoint regressions alongside current upstream terminal handling. |
| Small cross-source adaptations | `core/workspace.go`, `core/llm_object_tools_test.go`, `dagql/recipe_classification.go`, `engine/clientdb/store_registry.go`, `core/integration/{agent_runtime,tracesink}_test.go` | Keep one nil-safe cloned mount accessor, full defining-schema coverage in runtime context, current recipe lookup, OpenStats with runtime-owned stores and immediate final-reference closure, and one shared synchronized trace-capture helper. |
| Local development environment | `dagger.toml`, `dagger.lock`, `.dagger/modules/{engine-lab,mcp-lab,go-cli,tui-qa}` | Preserve module registration, SDK scopes, settings, agent/editor pins, credential forwarding, QA profiling/resume/tools, and intentional newer source pins. Keep the lock version header first. Discard only identified incidental test/generator resolutions. |
| Generated references | `docs/plugins/dagger-api-reference/generate-stubs.js`, API stubs, rolling sidebar, CLI references, SDK outputs | Preserve the generator implementation as well as generated pages. Current and rolling live-schema references must agree; retain handwritten pages and upstream removal of obsolete reference trees. |

For a remainder that has moved upstream or to a source branch, verify its behavior and regression there and record the replacement before dropping the local adaptation. Compare the final tree against the backed-up integration and account for every changed or removed path as an upstream change, new source work, an intentional adaptation, or regeneration. In particular, review semantic changes in generator scripts rather than treating everything under `docs/` as disposable generated output. Do not restore whole old files over newer source work merely to make the comparison smaller.

### Validation commands

Read the current engine-debugging/test skills before running tests. Run from the rebuild worktree with a compatible Dagger CLI and a separate bootstrap engine; `engine-dev test` builds the engine and CLI under test from that worktree. If necessary, build a bootstrap CLI from source into a temporary path. Do not restart or replace the user's dev engine. The last run used the existing `dagger-engine.clientdb-idle-cache` bootstrap container; inspect availability and compatibility rather than assuming that container or any `/tmp` binary still exists.

Set `DAGGER_ENGINE` to the selected separate bootstrap engine. Run these selections sequentially, capturing each run to a fresh log. Use the default progress UI. Substitute the package and regex from the table into:

```sh
"$rebuild_cli" api call engine-dev test --pkg "$pkg" --run "$run" --timeout 10m > "$log" 2>&1
```

| Package | Run regex |
| --- | --- |
| `./internal/cmd/dagger` | `^(TestWorkspaceChangesRendering\|TestDaggerCMD)$/^(TestAgentWorkspaceChanges\|TestAgentWorkspaceWithoutGitBaseline\|TestAgentToolsetFollowsFocusAndSnapshot)$` |
| `./core` | `^(TestAgentClientScopeLifetime\|TestLLMEndpointCredentialOutlivesRoutingScope\|TestBoundToolsUseTheirDefiningSchemaAuthoritatively\|TestGitPushResultsAndLeases\|TestCredentialTransportDoesNotRestoreInvalidatedSDKToken\|TestApplyGitPatchIgnoresEmbeddedRepo\|TestSummarizeMountChanges\|TestUnionMountPoints)$` |
| `./engine/clientdb` | `^Test` |
| `./engine/server` | `^(TestGitPush\|TestClientScope\|TestClientAttachableWaitRouting\|TestResolveHostServiceCaller)` |
| `./dagql` | `^(TestLoadRecipeUsesModuleProvenance\|TestLoadRecipeModuleCoreCallsAndLazyRefs\|TestObjectTypeForIDUsesModuleProvenance)$` |
| `./dagql/idtui` | `^(TestConsole\|TestValidateConsoleKey\|TestPromptForm\|TestPushConfirmation\|TestPushPassphrase\|TestCheckpointSelect)` |
| `./core/integration` | `^(TestWorkspace\|TestAgentRuntime\|TestGit)$/^(TestWorkspaceExportReusesOriginBase\|TestWorkspaceExportReusesCapturedBase\|TestWorkspaceExportBaseReadinessCacheIsolation\|TestReseed\|TestSendAfterSpawnerReleased\|TestAgentArgumentAfterSpawnerReleased\|TestWithRemote\|TestPushCapturedDestination\|TestPushHTTPAuth\|TestWorkspaceGitDirectoryOriginCredentials)$` |

The backslashes before `|` in this table escape Markdown table separators; use ordinary `|` alternation in the actual regex. These are minimum regression selections for the currently recorded boundaries. Confirm they still select real tests; follow renamed/replaced tests and add focused coverage for new conflicts or source changes. Successful compilation or a zero-test run is not a substitute for the behavioral checks. Use race coverage when changes to concurrency warrant it.

Regenerate references and SDKs with the following commands, also capturing logs:

```sh
"$rebuild_cli" generate -y \
  docs:references go-client:generate python-client:client-library \
  typescript-client:client-library typescript-client:format \
  rust-client:apiclient php-client:api elixir-client:client-library
go run ./internal/cmd/dagger/docsgen \
  -out docs/current_docs/reference/cli/index.mdx -include-experimental
"$rebuild_cli" api call -m .dagger/modules/tui-qa --help
```

Check that the API generator updates both current and rolling stubs and the rolling sidebar. Synchronize the generated CLI reference into the current rolling version selected by `docs/versions.json`; its last location was `docs/versioned_docs/version-1.0-beta/reference/cli/index.mdx`. Do not hard-code an obsolete version or overwrite handwritten API pages. Inspect generated diffs and lockfile changes before committing. The direct CLI generator command avoids the nested docs module's exclusion from root `go generate` package discovery. Check `git diff --check`, module configuration/pin preservation, source ancestry or documented upstream replacements, and all remainder coverage before replacing vitoland.

### Finish and leave the next rebuild ready

Update this runbook's completed-rebuild checkpoint, the active source table, remainder mappings/removal conditions, validation results, and backup reference. Record newly discovered integration-only behavior immediately, including its files and regression. Keep previous records as history, but make the current state unambiguous.

Update `~/src/dagger-extraction/HANDOFF.md`, `vitoland-followons.tsv`, the applicable `source-rebase-*.tsv`, and a dated rebuild mapping. Update `ledger.tsv` landing metadata where relevant without changing the original inventory's ordinals, hashes, subjects, or themes. Read the tracking repository's handoff and run its read-only `scripts/ledger-check.sh`; separately verify current source/remainder mappings. Its existing stale submitted-head assertion is documented below and does not authorize silently rewriting the historical manifest or claiming a complete extraction audit. If this separate repository is unavailable, retain the complete current mappings here and explicitly record the pending ledger update; do not make reconstruction depend on temporary artifacts.

Commit the rebuild and bookkeeping, then recheck the original vitoland checkout and tip for user changes before updating it. Keep the backup. Report the new head, validation, and any unresolved limitation. If publication is authorized, use the recorded remote SHA as an explicit force-with-lease for rewritten vitoland; verify remote heads afterward. Publish only intended integration/source refs, never scratch or backup branches. The tracking repository currently has no remote and its ledger is committed locally.

## September 14 workspace API rebuild

Rebuilt from the unchanged main `18c593d59b396700273d2916409586c6bf7d409c` with the updated workspace source. Previous integration `8366671a2a4ac4ff8ece8cc2047181f88a73f5b1` is retained as `backup/vitoland-before-api-rebuild-20260914`; the runbook added after the previous rebuild is preserved. Code checkpoint before this bookkeeping: `5b875b7247`. This is the current source/remainder record; earlier dated entries below remain historical.

| Source | Remote | Integrated tip | Merge | PR |
| --- | --- | --- | --- | --- |
| `extract/agent-runtime` | `upstream` | `56f1551b0e` | `23c566161c` | [#14003](https://github.com/dagger/dagger/pull/14003) |
| `workspace-git` | `origin` | `91f50b33e6` | `87a9e7d254` | [#14147](https://github.com/dagger/dagger/pull/14147) |
| `client-lifecycle-v2` | `origin` | `80b349b11e` | `ad543ba1bd` | [#14047](https://github.com/dagger/dagger/pull/14047) |
| `fix/patch-apply-repo-discovery` | `origin` | `20ab107ea6` | `6c8fd8d80b` | [#14071](https://github.com/dagger/dagger/pull/14071) |
| `fix/mcp-mount-rebind-summary` | `origin` | `1309a5386f` | `bc700cb964` | [#14074](https://github.com/dagger/dagger/pull/14074) |
| `fix/llm-tool-schema-retention` | `origin` | `bca89d26ea` | `161efdaffb` | [#14142](https://github.com/dagger/dagger/pull/14142) |
| `fix/clientdb-idle-cache` | `origin` | `ab124f15bb` | `fa3a23dbe4` | [#14141](https://github.com/dagger/dagger/pull/14141) |

Workspace PR #14147 replaces closed, unmerged #14056. All seven sources are still open; no source was omitted because of the old PR closure. The rewritten workspace history preserves the previous 44-commit source tree exactly at `1ed15e4595` (tree-identical to `4dc466bc86`), followed by five commits: lint cleanup `29ef14cdef`, credential-safe host origin capture `009abfe0e6`, single public push URL `c3d3560216`, regenerated clients/schema `4da8d30d37`, and remote-aware checkout cache keys `91f50b33e6`.

The public API is now `GitRepository.withRemote(name, url, pushUrl)` with one optional push URL. Internal host capture still retains multiple push destinations through `__withCapturedRemote` so ambiguous captured origins require an explicit destination. Credential-bearing origins are omitted from reconstructed Git directories while safe SSH usernames are retained. Checkout cache identity includes the merged remote configuration for both ref and target-commit trees. The earlier source work, including saves and performance fixes, remains represented; `workspace-api-20260914.tsv` maps all 44 rewritten commits.

### Current integration remainders

| Current home | Previous home | Behavior to retain |
| --- | --- | --- |
| `42d8367be9` | `f3ae2933ba` | Credential lookup leases across runtime/lifecycle; retains explicit cherry-pick provenance. |
| `19511cee9f` | `5611e30474` and earlier lifecycle resolutions | Detached/tombstone agent leases, executable agent context, delegated push scope ownership, recipe lookup, cloned mount accessor, and one shared trace capture helper. The helper now belongs to workspace commit `5f5c43966b`. |
| `87a9e7d254` + `4374b936c0` | `b5ca9c9a5e`, `34d4ffe768`, `c9ff0cf6ad` | Frozen per-agent startup, runtime-preserving saves, baseline advancement/retries, busy and serialization guards, asynchronous preview, selected-runtime toolset, and their regressions. |
| `87a9e7d254` + `3edb882809` | `b5ca9c9a5e`, `96bf8fe728` | Queued forms and explicit approvals/checkpoints preserve focus and drafts; adapt tests to runtime form ownership. |
| `ad543ba1bd` | `40bcc5a1ff` | Combine value-workspace loading and client lifecycle routing, retaining both current server test stubs and attachable routing coverage without host-read epochs. |
| `161efdaffb` | `13845dfd97` | Full defining-schema regression in runtime context; portable source is still `bca89d26ea`. |
| `fa3a23dbe4` | `fa7debad31` | Idle-store reuse plus local OpenStats; portable source is still `ab124f15bb`. |
| `8ff586ec8c` + lock resolution in `ad543ba1bd` | `9b16210ee3` | All 32 module configurations, SDK bindings/settings, richer QA tools, credential forwarding, profiling/resume, and intended pins remain identical to the previous integration. |
| `23c566161c` + `5b875b7247` | `56f1551b0e`, `4b1eaeda7c` | Runtime owns the current/rolling reference generator; preserve combined type pages and sidebar when merging workspace. |

Keep each integration remainder until its behavior and regression have an equivalent source/upstream home. These homes consolidate some prior follow-ups into the new merge resolutions; they do not remove the adaptations. Exact prior/current ownership is recorded in `vitoland-api-rebuild-20260914.tsv`. Agent and editor pins remain `9750b76c7afdb4051f34a9dccfbc7959d1921401` and `5481e5ce3a8b97ffb8beaee3a569ad9100e16167`.

### API rebuild validation

Checks ran sequentially through engine-dev using the separate clientdb-idle-cache bootstrap container, with the engine and test CLI built from the rebuild source. The runbook’s seven selections passed, plus a dedicated workspace API selection; clientdb ran with the race detector. Counts are runner entries and include suites with nested cases.

| Check | Result | Trace |
| --- | --- | --- |
| Single push URL, remote-aware caches, credential-safe origin capture | 2 passed | [trace](https://dagger.cloud/dagger/traces/3da0cb73df865393864f457c5f49530d) |
| Save/reload, no-Git baseline, focused runtime toolset | 2 passed | [trace](https://dagger.cloud/dagger/traces/38469d809c86ecedbfd20c05ed2621e2) |
| Client/credential leases, defining schemas, patch discovery, MCP mounts | 8 passed | [trace](https://dagger.cloud/dagger/traces/7afc39e423466d86f5a908daa5d9e1a8) |
| All clientdb tests (race) | 34 passed | [trace](https://dagger.cloud/dagger/traces/0d5b423721ccc36ccabea6d5c29e92ad) |
| Push ownership and attachable routing | 9 passed | [trace](https://dagger.cloud/dagger/traces/76ca26a786783cc8168d8e8b3b3eeaac) |
| Cold recipes and schema provenance | 3 passed | [trace](https://dagger.cloud/dagger/traces/b34bc55c8f24931f2a5dd2b6970700e8) |
| Queued prompt and console behavior | 12 passed | [trace](https://dagger.cloud/dagger/traces/c1c5e7b9e978a0a913d18492054a818e) |
| Runtime/spawner lifetime and export reuse/cache isolation | 2 passed | [trace](https://dagger.cloud/dagger/traces/5b706fc9def89721640d6895c41a75b7) |

All eight API/SDK generation targets, the CLI reference generator, and combined QA module loading passed. Generated current/rolling API pages and the rolling sidebar agree, and the rolling CLI reference is synchronized. Full repository suites, benchmarks, and CI are not claimed.

Because main and every other source head were unchanged, the expected integration tree was derived independently by applying only the verified workspace tree delta to the backed-up vitoland. Fresh merges plus explicit remainder restoration match that expected code/configuration tree exactly. This checks preservation of all prior integration adaptations, including clean-merge omissions. Generation differences, if any, are reviewed separately. Source and ledger audits verify all seven heads, 170 source dispositions, 44 workspace rewrites, 15 cleaned follow-on homes, and 12 rebuild records.

The extraction ledger’s current source and remainder mappings, row #148’s integration landing, workspace PR authority, and handoff are updated. The immutable 214-row source inventory and historical submitted-head manifest remain unchanged; the existing stale submitted-head assertion still fails and is not claimed as resolved. The tracking repository has no remote and its changes are committed locally.

This rebuild is local; no source branches were rewritten or pushed, and the previously published vitoland remains at `8366671a2a`. The user’s running dev engine was not restarted or installed over. Artifacts are under `/tmp/vitoland-rebuild-20260914-api/`, with the isolated rebuild in `/home/vito/src/dagger/vitoland-api-rebuild-20260914`.

## September 14 integration rebuild

Rebuilt from `upstream/main` at `18c593d59b396700273d2916409586c6bf7d409c`, using fresh merges of all seven active source branches. Previous tip `1d5b1dd49e87885e8a4ce7dec85ae5f426647dba` is retained as `backup/vitoland-before-rebuild-20260914`. This section supersedes the integration tips and pending-rebuild notes in the historical entries below.

| Source | Remote | Integrated tip | Merge | PR |
| --- | --- | --- | --- | --- |
| `extract/agent-runtime` | `upstream` | `56f1551b0e` | `ece8d52592 + 1e97a37c29` | [#14003](https://github.com/dagger/dagger/pull/14003) |
| `workspace-git` | `origin` | `4dc466bc86` | `b5ca9c9a5e` | [#14056](https://github.com/dagger/dagger/pull/14056) |
| `client-lifecycle-v2` | `origin` | `80b349b11e` | `40bcc5a1ff` | [#14047](https://github.com/dagger/dagger/pull/14047) |
| `fix/patch-apply-repo-discovery` | `origin` | `20ab107ea6` | `8f975b7554` | [#14071](https://github.com/dagger/dagger/pull/14071) |
| `fix/mcp-mount-rebind-summary` | `origin` | `1309a5386f` | `f308d15d3b` | [#14074](https://github.com/dagger/dagger/pull/14074) |
| `fix/llm-tool-schema-retention` | `origin` | `bca89d26ea` | `13845dfd97` | [#14142](https://github.com/dagger/dagger/pull/14142) |
| `fix/clientdb-idle-cache` | `origin` | `ab124f15bb` | `fa7debad31` | [#14141](https://github.com/dagger/dagger/pull/14141) |

Workspace includes the user’s named-remotes, nested-repository Drop, console wait/key validation/toolset, and real-checkout changes, plus the later SDK regeneration `aa979f4ad8` and lock update `4dc466bc86`. The engine-dev client-source fix (#14143), console timings (#14144), SDK migration fixes, and beta releases are already in main and were not replayed separately.

The rebuild audit found that the earlier agent-runtime rebase had preserved the generated rolling API pages but omitted the generator from `b0af07af46`. Source follow-up `56f1551b0e` restores that implementation in PR #14003. It updates current and rolling API stubs and the rolling sidebar together; `source-rebase-20260914.tsv` now maps the original commit to this correction. This is a source-owned fix, not an untracked integration exception.

### Integration remainders to preserve

| Current commit | Previous home | Intent and removal condition |
| --- | --- | --- |
| `f3ae2933ba` | `62026681d8` | Credential lookup leases spanning runtime and client-lifecycle; retain until those APIs share a source home. |
| `34d4ffe768` | `d3a573ebaa` | Per-agent workspace saves advance the synchronization baseline without stopping or reseeding the runtime. Preserve busy-turn checks, serialization, asynchronous previews, failed-export retries, and no-Git behavior until workspace-git shares runtime sessions. |
| `5611e30474` | Previous integration resolutions | Detached-loop and tombstone leases, delegated Git push scope/ancestry validation, recipe lookup, nil-safe cloned mount accessors, and removal of the duplicate runtime trace helper. Keep while the source branches are independent. |
| `b5ca9c9a5e`, `40bcc5a1ff`, `96bf8fe728` | Previous integration resolutions | Frozen per-agent workspace startup, queued prompt forms and their approval/checkpoint tests, executable agent context, and value-workspace routing with lifecycle metadata. Do not revive host-read epochs. |
| `13845dfd97` | `4f5ed48089` | Keep the full defining-schema regression in runtime-aware test context; source fix remains `bca89d26ea`. |
| `fa7debad31` | `1572fca181` | Idle-store cache plus integration OpenStats helper/comments; source cache remains `ab124f15bb`. |
| `9b16210ee3` | `f1a9a628be` and historical module commits | Keep richer QA tools, credential forwarding, resume and profiling support alongside upstream timings and new workspace controls. All 32 root module configurations and final agent/editor pins are preserved. Shared image pins retain the user’s workspace lock update; the lock version header is first. |
| `c9ff0cf6ad` | New workspace/runtime adaptation | The console toolset resolves the selected conversation and latest committed runtime snapshot at request time. Preserve until workspace-git uses the same multi-agent session implementation. Covered by `TestAgentToolsetFollowsFocusAndSnapshot`. |

Historical configuration replays `70e655e656`, `a5836f2121`, `020d885571`, `1751809511`, `41995b8a34`, `79d9b76265`, and `b6926e65f6` are represented by the final configuration in `9b16210ee3`, rather than replaying obsolete intermediate pins. The agent pin remains `9750b76c7afdb4051f34a9dccfbc7959d1921401`; editor remains `5481e5ce3a8b97ffb8beaee3a569ad9100e16167`.

### Rebuild validation

Focused suites ran sequentially through `api call engine-dev test` using the separate clientdb-idle-cache bootstrap engine and this rebuild’s source for the tested engine and CLI. The installed dev engine was not restarted. Counts are runner entries and include suite entries containing nested cases; they are not individual assertion counts.

| Check | Result | Trace |
| --- | --- | --- |
| CLI save/reload, no-Git baselines, selected runtime toolset | 2 passed | [trace](https://dagger.cloud/dagger/traces/3a7f8fcdaf3af20e3191b415b5101244) |
| Agent leases, credential lifetime, defining-schema tools, Git push, OAuth rejection | 5 passed | [trace](https://dagger.cloud/dagger/traces/35b1a450aab2bcc8beb8db16982be0c1) |
| Git push ownership, client scopes, attachable routing | 9 passed | [trace](https://dagger.cloud/dagger/traces/e34eff2ea8818079a3a1c1155397f978) |
| Cold recipes and schema provenance | 3 passed | [trace](https://dagger.cloud/dagger/traces/1b63210f6a530dc9bb6a9374918949a7) |
| Queued prompts, approvals, checkpoint selection, console endpoints | 12 passed | [trace](https://dagger.cloud/dagger/traces/72353a1344437b7a027e4be5a5dc0cf2) |
| Runtime reseeding/spawner release and workspace export reuse/cache isolation | 2 passed | [trace](https://dagger.cloud/dagger/traces/bea869cc5888970dc3a6925bd9004f74) |

API reference generation and the Go, Python, TypeScript, Rust, PHP, and Elixir client generators passed, along with TypeScript formatting and CLI reference generation. Current and rolling references are synchronized. The combined QA module loads and exposes its existing tools plus `timings`, `wait`, and `toolset`. Full suites, benchmarks, and post-push CI were not run.

The final tree audit accounts for differences from the previous integration as upstream changes, newer workspace work, generated references, and the explicit toolset adaptation. The extraction audit checks all 170 source dispositions, all 15 cleaned follow-on source homes, and the rebuild’s explicit remainder records. The immutable 214-row inventory passes its integrity check; the historical submitted-head assertion still fails against the old workspace manifest and is not claimed as resolved.

The separate extraction repository records the current mapping in `vitoland-rebuild-20260914.tsv`, updates the remainder references in `vitoland-followons.tsv`, corrects the generator row in `source-rebase-20260914.tsv`, and updates row #148’s integration landing. Original source inventory columns remain unchanged. It has no remote; its ledger is committed locally.

Rebuild worktree, commands, logs, conflict notes, and audits are retained under `/home/vito/src/dagger/vitoland-rebuild-20260914` and `/tmp/vitoland-rebuild-20260914/`. Only vitoland and the agent-runtime generator follow-up are published; backup and scratch branches remain local.

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

## September 11 migration follow-up

The installed rebuild exposed an SDK provider mismatch in `dagger ws migrate`:
a native module's `dang` runtime was resolved through the registry to an
unversioned URL, then rejected because its existing scope belonged to the
workspace's versioned SDK provider. The initial rebuild validation did not
exercise workspace migration.

- Merged [#14119](https://github.com/dagger/dagger/pull/14119) at
  `43b1e88d7fbed73f079f268bec097ebec936de72` in `8719676afe`. This brings the
  beta.12 release follow-up, SDK scope configuration, vanity SDK sources,
  updated SDK locks, and rolling documentation into the integration.
- #14119 alone does not fix the engine's provider mismatch; migration also
  rejected its `dagger.io/sdk/dang@v1` provider. Checked the open migration
  PRs, including #14127 at `2edc41d0728defd995c636d74b5ecf29ace1a88a`, whose
  fixes address different migration issues.
- Created [#14129](https://github.com/dagger/dagger/pull/14129) from
  `vito:fix/migrate-installed-sdk`, based directly on fresh `upstream/main`
  (`ff626243dc`, reverified remotely). Its source commit `26a952eafb` resolves
  unversioned runtime names through the installed SDK first, preserving the
  provider and pin while enforcing explicit-version and ownership conflicts.
  Integrated it in `0a7ad2d901`.
- Only `vito:fix/migrate-installed-sdk` was pushed. No colleague's PR branch
  was changed. `vitoland` remains local with no remote tracking branch.
- Preserved all 32 installed modules, their settings, and the existing agent
  and editor pins. The two SDK sources now use `dagger.io/sdk/{dang,go}@v1`.
  Added the local `go-cli` SDK scope and regenerated the current and rolling
  API references and the rolling CLI reference for the integrated API.
- The pre-follow-up integration is retained as
  `backup/vitoland-before-14119-20260911` at `3ff1a31325`.

Source fix validation used `api call engine-dev test`, building the tested
engine and CLI from the isolated source branch:

- [Regression before fix](https://dagger.cloud/dagger/traces/2b430f426de5b4156e28fe95d72f8bb3)
  reproduced the provider mismatch for both native and legacy module configs.
- [Focused schema tests](https://dagger.cloud/dagger/traces/d6161c81bc0923f711bffdb138a0f273)
  passed (13 runner entries), covering versioned, vanity, pinned, and custom
  providers, plus explicit version and conflicting ownership rejection.
- [CLI migration tests](https://dagger.cloud/dagger/traces/6c9e417074fc6f4e597119bb605aa3cf)
  passed, covering preview without mutation, apply, idempotence, and the
  existing native-config cases (one suite entry).
- The merged workspace's `check -l` completed without SDK migration warnings
  (`/tmp/vitoland-14119-check-list.log`). API stub and CLI generation completed
  (`/tmp/vitoland-14119-stubs.log`, `/tmp/vitoland-14119-cli-docs.log`).
- [Full workspace migration](https://dagger.cloud/dagger/traces/a97f1daa22598dd82c04483ef2402fda)
  passed in an ephemeral engine built from the merged integration. Applied its
  generated SDK scopes for `engine-lab`, `mcp-lab`, and `tui-qa` to `dagger.toml`.
- [Repeat migration preview](https://dagger.cloud/dagger/traces/d7238ebaf99ea3f2d85055827d17de56)
  passed and reported `No migration needed.` Optional legacy examples remain
  listed for explicit migration, as intended. Logs are in
  `/tmp/vitoland-14119-migration-{smoke,noop}.log`.
- Configuration comparison confirms all 32 module entries and settings are
  preserved, with only the two intended SDK source substitutions. Discarded
  incidental build dependency lock entries and normalized the generated
  rolling CLI reference's trailing blank line. Diff whitespace checks pass.

The installed persistent development engine and CLI were not replaced by this
follow-up. Reinstall them from the updated branch to use the migration fix.

## September 12 fix redistribution

Cleaned up the seven direct fixes after `8ad5663849` and moved their source
changes back to the owning branches. All new commits have descriptive bodies
and Alex Suraci's DCO sign-off. No branches were pushed.

| Source | Commit | Purpose | Previous integration commits |
| --- | --- | --- | --- |
| `extract/agent-runtime` | `b2e04dcc73` | Portable OAuth rejection refresh | `9a865cb443` (source copy `b64b9cf70c`) |
| `extract/agent-runtime` | `faad9367a5` | Cold module recipe restoration with unit and restored-tool regressions | `734d39e62b`, `9693b12255` |
| `workspace-git` | `19c149fa33` | Author sign-off API, identity tests, and generated schema/SDK bindings | `0e31820398`, `95f3c6bf98`, `29169fba45` |
| `workspace-git` | `d01ecf62e3` | Uncommitted-edit preview and regressions | `c197af6047` |
| `vitoland` only | `62026681d8` | Client-lifecycle credential leases and regression | Lease-dependent portion of `9a865cb443` |

The existing unsigned `workspace-git` test commit `51b63b299b` was included in
`19c149fa33`; its tests match the integration's `0e31820398`.

Source validation exposed an existing problem in agent-runtime's OAuth commit
`b64b9cf70c`: it referenced client lease APIs that exist only after merging
`client-lifecycle` ([initial build failure](https://dagger.cloud/dagger/traces/f875eab5928bf1d77a80c17d786e1065)). The source commit is now `b2e04dcc73`, which retains the
session-bound resolver and explicitly carries the active request's rejection
fingerprint. The lease implementation and its lease-specific regression have
been extracted into the separate signed integration commit `62026681d8`.

**Preserve `62026681d8` when rebuilding vitoland with client-lifecycle.** It
acquires a temporary shared-work lease from the active request or agent turn,
checks that it belongs to the routing session, and bounds the lookup by session
cancellation and a timeout. The endpoint must not reuse its routing call's
released lease. `TestLLMEndpointCredentialOutlivesRoutingScope` checks this
behavior and verifies that lookup releases the temporary lease. This delta
belongs on agent-runtime only once that branch includes the client-lifecycle
APIs; until then, keep both implementation and regression on vitoland.

The preview fix uses `LLMSession` on workspace-git, where the preview code
originates. Its integration continues to use `sessionAgent`. The cold recipe
fix does not add the lifecycle branch's `dagql/recipe_classification.go` to
agent-runtime; the classifier's adjusted helper call remains an integration
merge resolution.

Replaced the seven direct integration commits with merge `164b0a3df2`
(agent-runtime at `faad9367a5`), the lease adaptation `62026681d8`, and merge
`96c9ecc84e` (workspace-git at `d01ecf62e3`). The tree at `96c9ecc84e` is
byte-for-byte identical to the previous integration tip `9693b12255`; only this
record changes afterward. Both updated source tips are ancestors of vitoland.

Original tips are retained as `backup/vitoland-before-redistribute-20260912`
(`9693b12255`), `backup/workspace-git-before-redistribute-20260912`
(`51b63b299b`), and `backup/agent-runtime-before-redistribute-20260912`
(`b64b9cf70c`). The untracked `workspace-git/refactor.md` is preserved unchanged.
The rewritten OAuth source commit changes agent-runtime's local history;
updating its remote will require a coordinated history rewrite. No remote refs
were changed by this cleanup.

Validation uses `/tmp/vitoland-dagger` with `api call engine-dev test`, building
the engine and CLI from each cleaned source worktree. Suites run sequentially.

- Workspace-git: `TestGit/TestGitRefWithCommit` and
  `TestWorkspace/TestWorkspaceWithCommitSignoff` passed (2 runner entries;
  [trace](https://dagger.cloud/dagger/traces/fb128ffa7f4c39489557aab713832805)).
- Workspace-git: `TestWorkspaceChangesRendering` and
  `TestDaggerCMD/TestAgentWorkspaceChanges` passed (2 runner entries;
  [trace](https://dagger.cloud/dagger/traces/444cb45c73b444bff8fcb4c4c994c6c5)).
- Agent-runtime: cold recipe provenance, cross-module lazy arguments, warm
  cache hits, and lazy-ref replay regressions passed (4 runner entries;
  [trace](https://dagger.cloud/dagger/traces/5ba7794c3b4585a4c402c1af9ccac213)).
  This ran before extracting the lease-specific core test; the tested dagql
  source and tests are unchanged at the final tip.
- Agent-runtime: credential source/transport, cancellation, and LLM endpoint
  tests passed (18 runner entries;
  [trace](https://dagger.cloud/dagger/traces/45030a36768701b16ae2c5d220e49866)).
- Agent-runtime: credential reload through the installed secret schema after
  the routing call ends passed (1 runner entry;
  [trace](https://dagger.cloud/dagger/traces/ae2d6d9296e96bc8d7dac7bf7992e00c)).
- Agent-runtime: restored cold module tool dispatch passed (1 runner entry;
  [trace](https://dagger.cloud/dagger/traces/02ed016312c33c02c569f8741a57973d)).
- Vitoland: routing-lease lifetime, detached resolver cancellation, and
  rejection-fingerprint propagation passed (3 runner entries;
  [trace](https://dagger.cloud/dagger/traces/1587ec35038aeebd5ea7be654a157eed)).

Tree identity, source ancestry, commit sign-offs, message formatting, and diff
whitespace checks pass. Full repository and race suites were not run. Test
logs are retained under `/tmp/dagger-redistribute-e6qvq2va/`.

## September 12 publication follow-up

After cleanup, the user explicitly requested pushing all three branches,
including vitoland. Remote inspection found `origin/workspace-git` had advanced
to `4ad953a484` (abbreviated Git SHA resolution, vito/dagger#419). Preserved those
remote commits in signed merge `10d6f65646` on workspace-git and integrated that
source tip here. The added code is patch-identical to the remote feature; the
cleanup's code-identity check above describes the state before this follow-up.
The separate credential lease commit `62026681d8` remains intact.

Publication destinations are `origin/workspace-git` at `10d6f65646`,
`upstream/extract/agent-runtime` at `faad9367a5`, and `origin/vitoland` at this
follow-up. The rewritten runtime and integration histories use explicit leases
against the inspected remote tips `b64b9cf70c` and `9693b12255`. Workspace-git
preserves its remote ancestry. Backup branches remain local.

The combined integration passed `TestGit/TestGitRefWithCommit` and
`TestGit/TestShortSHAResolution` through `engine-dev test` (1 suite entry;
[trace](https://dagger.cloud/dagger/traces/4c925f4fecf78d28c5adc19f0390911e)).
Discarded incidental test-build lock resolutions. Whitespace checks pass.

## September 13 inert-client attachables fix

Landed direct fix `3735666828` on its owning source branch,
`client-lifecycle-v2`, as signed commit `f6280d6542`, retaining cherry-pick
provenance. The source commit rejects direct `clientAttachableCaller` lookups
for inert SDK clients, including the `CallerStatFS` path that previously waited
for attachables those clients never register. The regression covers both
blocking and nonblocking direct lookups. This depends on client-lifecycle's
session-owned gateway and inert-client state, which agent-runtime does not have.

Pushed `origin/client-lifecycle-v2` from `be630ff9f2` to `f6280d6542` and merged
that source tip into vitoland. The merge changes no source code: the source
commit is patch-identical to the existing direct fix. Vitoland remains local
ahead of its published tip; it was not pushed in this follow-up.

Validation built the source branch's engine and CLI with `/tmp/vitoland-dagger`
and `api call engine-dev test --pkg ./engine/server`. The inert-client
regression, all three `TestResolveHostServiceCaller*` tests, and
`TestNestedTransportRegistrationBindsExactExecAttachables` passed (5 runner
entries; [trace](https://dagger.cloud/dagger/traces/914a53ee3da0e3cc8e62bc69db41e5ca)).
The log is `/tmp/client-lifecycle-inert-attachables-test.log`. Patch identity
and whitespace checks pass; full repository and race suites were not run.

## September 14 save cleanup and clientdb fix

The user rebased the integration, dropped the upstream/main merges, retained
only the clientdb fix as cherry-pick `2c7674a67b`, and added 17 save-performance
commits. This cleanup starts at `cb2534276f`; all earlier ancestry is preserved.
The pre-cleanup tip `5f0fad218329c6ee6e934189538219539e5d5a11` is retained locally
as `backup/vitoland-before-cleanup-20260914`. The earlier September 14 merge
`6733565738` is not part of the current history.

The 32 direct commits after the base are consolidated into 14 implementation
commits plus this documentation checkpoint. The new performance batch accounts
for five implementation commits and one observability commit. Original commit
IDs below refer to the rebased history in the backup, rather than the older IDs
in the user-supplied `cleanup.md`.

| Previous commit(s) | Cleaned commit | Change |
| --- | --- | --- |
| `7a85b62374`, `b7aedbf58a`, `57516cf35d`, `be99c75bad`, `9ab3fd1813`, `73c8b750de` | `745b92e2cb` | export to an explicit checkout |
| `a919f78a41` | `f4a9f8972d` | build clients from test source |
| `b26a8b479c` | `d58e12e6bc` | save without reloading conversations |
| `0a57764566` | `d590966585` | preserve cherry-pick messages |
| `0202897d9b` | `180bcc3612` | expose subtree timings in QA console |
| `fc4d0d0fe5` | `6300d21c8c` | honor defining object schemas |
| `f061fbfc0f`, `d13c1ef1e1` | `c0b304dee5` | preserve composed tool receivers |
| `2c7674a67b` | `5cfaa3cce3` | retain idle session stores |
| `d5077f1dcc` | `a5a8409391` | bundle local objects without fetching |
| `37694ee1b7` | `817ce68c37` | reuse unchanged snapshot trees |
| `4b03b68848` | `22de4d0226` | materialize save checkouts once |
| `84ce1c9ed9`, `140293b7f3`, `54ecb3eac9` | `61915ec71e` | trace save composition phases |
| `dcee76f157` | `fbd006bb6f` | shallow content-only checkouts |
| `576a509bd5`, `5f0fad2183` | `b12de3e1e9` | reuse owned export history |

`354ba888e9` is represented by the condensed save design note. The benchmark
parts of `54ecb3eac9` and `140293b7f3` and the benchmark attribution in
`84ce1c9ed9` are omitted. Only production spans, their privacy tests, and the
shared trace sink remain from those mixed commits.

The measurement/parser commits `8a7a62f31a` and `bdd271f7cf`, prerequisite repack
experiment/restore/evidence `783988b16f`, `191ad3796e`, and `d8856c4681`, benchmark
attribution/fetch investigation `83f23d45d6` and `c68559d3c6`, and private-index
experiment `252d7cef7c` are intentionally omitted together with their test-only
implementations. Production prerequisite import retains its fetch path.
Synthetic benchmarks, the engine-lab benchmark runner, and measurement JSON
are removed. No new benchmarks or performance investigations were run.

The retained structural export tests include their trace sink and export-scoped
operation counts. Real-origin and routing fallback tests, readiness-cache
isolation, immutable source ownership, file kinds, retries, snapshot/history
checks, and caller isolation are preserved. The save design note records the
optimizations and correctness boundaries without the experimental journal.

### PR and extraction accounting

The idle telemetry-store fix is represented by standalone
[PR #14141](https://github.com/dagger/dagger/pull/14141), source commit
`5ddc784673f1589823b8d64572f82e4296c3d62c` on `fix/clientdb-idle-cache`.
Its cleaned local integration is `5cfaa3cce3`; it has the same patch as the
pre-cleanup cherry-pick. No upstream/main merge was reintroduced.

The separate `~/src/dagger-extraction/ledger.tsv` row #148 (`1dad967b82`) and
`HANDOFF.md` now identify that PR and the current integration. The original
214-row inventory and the historical S1-3 grouping are preserved. The old
residual audit's pending status for #148 is superseded, not evidence of another
missing implementation. This does not mark the other telemetry rows complete.
The extraction checker's stale submitted-head ancestry assertion remains a
separate pre-existing issue; this update does not claim a successful full audit.

The other cleaned commits remain local follow-on work on vitoland. This cleanup
has not placed them onto owning source branches or published them in PRs.
Workspace export/history work should be reconciled with `workspace-git`;
agent-save and composed-tool behavior with `extract/agent-runtime`; engine-dev,
QA-console, and defining-schema changes still need explicit placement. The
mapping above is the durable inventory for that next extraction pass. It must
not be interpreted as proof that all local work is already covered by PRs.

Earlier save/reload integration notes are historical: the current save path
preserves the active conversation and advances its save baseline without
reloading it. No source branches or remotes were changed by this cleanup, and
nothing was pushed. The user's untracked `cleanup.md` is preserved.

### Cleanup validation

Focused tests ran sequentially through `api call engine-dev test` using the
rebuilt source and CLI, bootstrapped by the separate
`dagger-engine.clientdb-idle-cache` container. The user's running
`dagger-engine.dev` was not restarted or modified. Subsequent message and
documentation edits preserve the tested code tree. Counts are runner entries;
integration suite entries include nested tests.

| Check | Result | Trace |
| --- | --- | --- |
| Workspace export/pull/snapshot history and Git bundles | 2 passed | [trace](https://dagger.cloud/dagger/traces/a46358024c51b7bbda9d08b8c0c962d1) |
| Core bundles, snapshots, export/pull history, content depth, and tool receivers (race) | 32 passed | [trace](https://dagger.cloud/dagger/traces/2f679eb2229e14fcf2f92a451599c133) |
| Checked bundle application and file-kind/mode retries | 4 passed | [trace](https://dagger.cloud/dagger/traces/5e7161be0c3f3cf8b42a1146f3f0e746) |
| Export base qualification and reconstruction fallback | 2 passed | [trace](https://dagger.cloud/dagger/traces/e1b15c7453fb8e2f56490ea1928a7b65) |
| Full checkout/legacy metadata and reloaded LLM tools | 2 passed | [trace](https://dagger.cloud/dagger/traces/ec20f393d40b3257b9f3c2584b029040) |
| Defining-schema lookup and object rewrapping | 3 passed | [trace](https://dagger.cloud/dagger/traces/8d84d97dee0589d7b7519cd2dd9b49d3) |

The standalone clientdb source was previously validated with all 34 clientdb
tests under the race detector. Its local cherry-pick is unchanged by this
cleanup. Full repository suites and benchmarks were not run.

The history audit accounts for all 32 pre-cleanup commits, checks the 14 signed
implementation commits and their message formatting, and confirms that retained
production code matches the backup exactly. Differences are limited to the
removed benchmark runner, two test files with experiment/benchmark removals,
the deleted integration benchmark and measurement JSON, the condensed design
note, and this record. No references to removed benchmark helpers remain.
Incidental test-build lock resolutions were discarded. Whitespace checks pass.

The extraction checker confirmed all 214 immutable source ordinals, hashes,
subjects, and themes, then stopped at the existing `workspace-git` submitted-head
ancestry mismatch. The final extraction ledger update is `bfe0895` in the
separate tracking repository. Test logs, exact commands, commit mappings, and
the history audit are retained under `/tmp/vitoland-cleanup-Pc3gvE/`.

## September 14 source rehoming

Rehomed the 15 cleaned follow-on commits from `de7ac944d6` after the user
confirmed the source buckets and explicitly allowed tracked integration
remainders. Backup: `backup/vitoland-before-rehome-20260914` at `de7ac944d6`.
The previous workspace source tip is retained as
`backup/workspace-git-before-rehome-20260914` at `10d6f65646`.

### Source ownership

| Cleaned integration commit | Source branch | Source commit |
| --- | --- | --- |
| `745b92e2cb` | `workspace-git` | `dcee734be9` |
| `f4a9f8972d` | `fix/engine-dev-client-source` | `5aad15f08a` |
| `d58e12e6bc` | `workspace-git` | `befd72b9d2` |
| `d590966585` | `workspace-git` | `005cc5fea8` |
| `180bcc3612` | `feat/tui-console-timings` | `20cbdd9ebe` |
| `6300d21c8c` | `fix/llm-tool-schema-retention` | `64855b271e` |
| `c0b304dee5` | `fix/llm-tool-schema-retention` | `64855b271e` |
| `5cfaa3cce3` | `fix/clientdb-idle-cache` | `5ddc784673` |
| `a5a8409391` | `workspace-git` | `a3ab81e9bd` |
| `817ce68c37` | `workspace-git` | `378f0f65ef` |
| `22de4d0226` | `workspace-git` | `e217c1ffc4` |
| `61915ec71e` | `workspace-git` | `065e0098cd` |
| `fbd006bb6f` | `workspace-git` | `2bdf934f84` |
| `b12de3e1e9` | `workspace-git` | `95c43ce020` |
| `de7ac944d6` | `workspace-git` | `f1f6562291` |

`workspace-git` now includes eight export/history/performance commits, the
portable save design note, and the save-without-reload behavior adapted to its
`LLMSession`. The two schema/receiver commits are one standalone correctness
fix on `fix/llm-tool-schema-retention`, including unit and reloaded-conversation
regressions. The engine-dev client-source fix and QA timings endpoint each have
their own standalone source branch. `extract/agent-runtime` and
`client-lifecycle-v2` are unchanged.

Workspace save tip `befd72b9d2` is integrated by merge `01ac3c01ed`.
Source validation then exposed a trace helper defined only in runtime tests.
Follow-up `fc3a697484` puts it in the shared trace sink; merge `53fa25941d`
imports it and removes the runtime-local duplicate. The final workspace source
tip is `fc3a697484`.
Standalone fixes are cherry-picked with source provenance: `383f3006d2`
(engine-dev), `4f5ed48089` (schema/LLM), `f1a9a628be` (QA), and `1572fca181`
(clientdb). The independent branches start at main `c305ed3757`; cherry-picking
their fixes preserves the user's decision to exclude unrelated upstream/main
updates from vitoland. The clientdb source branch/PR remains unchanged.

### Integration remainders: retain on future rebuilds

| Remainder | Source counterpart | Why it remains / removal condition | Regression |
| --- | --- | --- | --- |
| `d3a573ebaa` | `workspace-git` save commit `befd72b9d2` (original `d58e12e6bc`) | Ports the save to `sessionAgent`, keeping the busy-turn guard, serialization, asynchronous preview, and active runtime. Retain until workspace-git and agent-runtime share this session implementation. | `TestDaggerCMD/TestAgentWorkspaceChanges`: source baseline, host-only cherry-picks, retries, subsequent saves, zero runtime reseeds/stops; `TestAgentWorkspaceWithoutGitBaseline` and fake runtime support. |
| `4f5ed48089` test-context resolution | `64855b271e` | Preserve existing runtime test context while importing the complete standalone defining-schema regression. Reconcile when updating/merging that source fix. | `TestBoundToolsUseTheirDefiningSchemaAuthoritatively`, including state returns and dependency attachment. |
| `f1a9a628be` wrapper-context resolution | `20cbdd9ebe` | Place timings in the existing richer QA module without removing other integration tools. Reconcile when updating/merging the source branch. | `TestConsoleTimings` and `TestConsoleTimingsHandler`; the wrapper also lives on the source branch. |
| `1572fca181` OpenStats comment adaptation | `5ddc784673` | Retain the local telemetry OpenStats helper and describe its inclusion of idle stores accurately. The standalone source lacks that helper. Remove this adaptation when the helper is shared upstream. | Existing registry refcount/OpenStats assertion; cache implementation and its tests are unchanged. |
| `53fa25941d` duplicate-helper removal | `fc3a697484` | The capture helper now belongs to the shared trace sink. Remove the runtime-local copy when integrating agent-runtime; this resolution disappears once both source branches share that helper location. | Export operation-count tests and all runtime callers retain the same locked slice-copy behavior. |

`MERGE_STATE.md` remains integration bookkeeping. The earlier credential-lease
remainder `62026681d8` and its regression remain in the preserved base; the
September 12 instructions for retaining it still apply. No other follow-on
behavior was left without a source owner or an explicit remainder above.

### Publication and ledger

At the local rehoming checkpoint, nothing had been pushed and no new PR had
been submitted. The publication update below supersedes this checkpoint status. PR #14056 still advertises `10d6f65646`; the new workspace commits
need publication. The three standalone branches need their own PRs. PR #14141
already represents the clientdb fix. Do not treat local ownership as published
PR coverage.

The separate extraction repository now has `vitoland-followons.tsv`, mapping
all 15 cleaned integration commits to source commits, publication status, and
integration remainders. It supplements the immutable 214-row source ledger;
it does not overwrite that historical inventory or claim additional parity.
Row #148 points to the current clientdb integration `1572fca181`. The historical
workspace submitted-head assertion in the old ledger checker remains stale;
this update does not silently change its historical manifest.

### Rehoming validation

Tests ran sequentially through `api call engine-dev test` using the separate
`dagger-engine.clientdb-idle-cache` bootstrap container. The user's running
`dagger-engine.dev` was not restarted or modified. Source validation temporarily
applied the independent engine-dev client-source fix to the workspace, LLM,
and QA test checkouts, then removed that harness-only delta; it is not a
product dependency of those branches. Counts below are runner entries; suite
entries include nested cases.

| Check | Result | Trace |
| --- | --- | --- |
| Workspace source: save/reload CLI behavior | 2 passed | [trace](https://dagger.cloud/dagger/traces/f31597a4c05887fd6036d610460eaf36) |
| Workspace source: bundle/snapshot/export/pull/content-depth (race) | 31 passed | [trace](https://dagger.cloud/dagger/traces/742b5055b1419fb1685d9fbf5126c2cf) |
| Workspace source: exports, real origins, cache isolation, history, bundles | 2 passed | [trace](https://dagger.cloud/dagger/traces/48359679fbeac7dd848b78f8dd9a6777) |
| Workspace source: base qualification and reconstruction fallback | 2 passed | [trace](https://dagger.cloud/dagger/traces/4d0286c3b6b023562470cb464156d5f6) |
| Standalone LLM: defining-schema receiver boundaries | 1 passed | [trace](https://dagger.cloud/dagger/traces/77babdd128aa327ab8b6907beb1174b1) |
| Standalone LLM: type lookup and object rewrapping | 3 passed | [trace](https://dagger.cloud/dagger/traces/7452d6dd1d78c76f3b3e835a0add3875) |
| Standalone LLM: reloaded tools across state returns and timeouts | 1 passed | [trace](https://dagger.cloud/dagger/traces/b96d2c447d54473998871b410243a055) |
| Standalone QA: timings endpoint/filtering/validation | 2 passed | [trace](https://dagger.cloud/dagger/traces/45eaf33a9f3d18e6069fa0c2b67453c8) |
| Standalone engine-dev: from-source engine/client build and bundle parsing | 1 passed | [trace](https://dagger.cloud/dagger/traces/6372952e56bac1b123797606911d6d06) |
| Vitoland: save baseline and runtime preservation | 2 passed | [trace](https://dagger.cloud/dagger/traces/002b2e69eb76ad49e6f82c15dc8fb181) |
| Vitoland: structural export reuse and readiness-cache isolation | 1 passed | [trace](https://dagger.cloud/dagger/traces/1d9e882b3ac1f2b88912d789ce618ab9) |

The standalone QA module also loaded successfully and exposed the `timings`
command's root, minimum-duration, and limit arguments. The first workspace
integration build failed because `capture` lived in the runtime-only test file;
`fc3a697484` and `53fa25941d` fixed that source dependency before the passing run.
No regression was dropped to make the source branches pass.

The final audit verifies ownership for all 15 cleaned commits and exact
production-code identity with `de7ac944d6`. The only test-code difference is
moving the unchanged synchronized trace-capture helper from the runtime test
file to the shared trace sink, with its imports. Other differences are confined
to this state record. The new workspace tip is an ancestor of the integration;
unrelated main tip `c305ed3757` is not. Source sign-offs, message formatting, and
whitespace checks pass. Full suites and benchmarks were not run.

The existing standalone clientdb validation remains applicable: its code and
regressions are unchanged. Earlier integration correctness validation recorded
above also applies to the unchanged production tree. Incidental test-build lock
resolutions were discarded. The user's `cleanup.md` and workspace-git's
`refactor.md` remain untracked and unchanged.

Exact commands, per-run logs, the source/integration mapping, and the final
audit are retained under `/tmp/dagger-rehome-20260914/`. The immutable source
ledger checker still reports its historical submitted-head ancestry mismatch;
the new follow-on ownership audit passes independently of that stale manifest.


## September 14 publication

The source branches are now published to the vito fork. The standalone PRs
are open and ready for review against `dagger/dagger:main`:

| Source branch | Published tip | PR |
| --- | --- | --- |
| `workspace-git` | `fc3a697484` | [#14056](https://github.com/dagger/dagger/pull/14056), updated with the save fixes and performance work |
| `fix/llm-tool-schema-retention` | `64855b271e` | [#14142](https://github.com/dagger/dagger/pull/14142) |
| `fix/engine-dev-client-source` | `5aad15f08a` | [#14143](https://github.com/dagger/dagger/pull/14143) |
| `feat/tui-console-timings` | `20cbdd9ebe` | [#14144](https://github.com/dagger/dagger/pull/14144) |
| `fix/clientdb-idle-cache` | `5ddc784673` | [#14141](https://github.com/dagger/dagger/pull/14141), already published and unchanged |

PR descriptions include the focused validation recorded above. The workspace
PR now describes explicit checkout export, saves that preserve the active
conversation, and the save-performance changes. No product code changed during
publication, and this record makes no claim about subsequent CI results.

This bookkeeping commit accompanies publication of the rebuilt `vitoland`.
Its rewritten history replaces remote tip `d13c1ef1e1c05d5a6a1b67f26fc1e5b711e267ae`
using an explicit force-with-lease. All integration remainders listed above
remain part of the branch and must survive future rebuilds. Backup and scratch
branches remain local.

The separate extraction repository's `vitoland-followons.tsv` and `HANDOFF.md`
record these PRs and published source tips. That repository has no configured
remote, so its ledger updates are committed locally. The historical 214-row
inventory and stale historical submitted-head assertion are unchanged.

## September 14 source rebase

All seven active integration source branches are rebased and published on `upstream/main` at `18c593d59b396700273d2916409586c6bf7d409c`. The original tips are preserved as `backup/<source-name-with-slashes-replaced-by-dashes>-before-main-rebase-20260914`. Pushes used explicit leases against the previously published tips.

| Source | Previous tip | Published rebased tip | PR |
| --- | --- | --- | --- |
| `workspace-git` | `b629e73545` | `19696fbd69` | [#14056](https://github.com/dagger/dagger/pull/14056) |
| `extract/agent-runtime` | `faad9367a5` | `fa426ce07e` | [#14003](https://github.com/dagger/dagger/pull/14003) |
| `client-lifecycle-v2` | `f6280d6542` | `80b349b11e` | [#14047](https://github.com/dagger/dagger/pull/14047) |
| `fix/clientdb-idle-cache` | `5ddc784673` | `ab124f15bb` | [#14141](https://github.com/dagger/dagger/pull/14141) |
| `fix/llm-tool-schema-retention` | `64855b271e` | `bca89d26ea` | [#14142](https://github.com/dagger/dagger/pull/14142) |
| `fix/patch-apply-repo-discovery` | `69474a9465` | `20ab107ea6` | [#14071](https://github.com/dagger/dagger/pull/14071) |
| `fix/mcp-mount-rebind-summary` | `9fcad2d9c9` | `1309a5386f` | [#14074](https://github.com/dagger/dagger/pull/14074) |

The engine-dev client-source fix (#14143) and console timings (#14144) have merged into main. Their branch tips remain the historical merged commits; neither needs a separate replay. Other archived, scratch, and original extraction-inventory branches were not rebased.

Workspace includes the three additional user commits after `fc3a697484`: named Git remotes (`6cef5e1f6c`), Drop for nested repositories (`46b740fde4`), and QA console tools/real checkouts (`b629e73545`). All three survive the rebase. The QA conflict retains both upstream timings and the new key validation, wait, and toolset endpoints, with both sets of tests. Core validation caught a stale submodule-test caller of `doGitCheckout`; a signed follow-up changes its empty string to an empty remote list without changing the fixture behavior.

Agent runtime retains every non-documentation patch unchanged. Upstream removed the PHP reference and relocated the rolling docs; the final synchronization commit carries the generated API stubs, CLI reference, and sidebar into the new paths. Three obsolete rolling-docs replays are accounted for by that synchronization. Client lifecycle retains current upstream lock records plus its TLA image pin, uses the session-record attachable gateway, and adapts the upstream wait-routing regression to that gateway. The historical stale-lock cleanup replay became unnecessary because its obsolete records were not reintroduced. All four small standalone fixes retain their original patch identities.

The source rebase does not rebuild the integration code: vitoland remains at the production tree recorded by `3a3e60443e`, plus this bookkeeping. The new user workspace commits and source compatibility follow-ups still need integration during the next rebuild. Keep all remainders listed above, especially runtime-preserving workspace saves and the credential lease. Historical integration hashes remain valid; new source ownership is mapped in `~/src/dagger-extraction/source-rebase-20260914.tsv`. Its 170 rows preserve each old source commit and its rebased or superseding home; `vitoland-followons.tsv` points the 15 cleaned integration follow-ons to current source commits. Original ledger row #148 now names the rebased clientdb source while retaining integration `1572fca181`.

The workspace checkout's uncommitted `dagger.lock` contents and untracked `refactor.md` are preserved. The extraction ledger is committed locally; that repository has no configured remote.

### Rebase validation

Focused tests ran sequentially through engine-dev using the separate clientdb-idle-cache bootstrap engine. The user's running dev engine was not restarted. Counts are runner entries, which can include nested cases. These tests ran on main `ecd1ec2112`; the final rebase additionally includes `18c593d59b`, whose only tree change is the docs 404-page home button. Each final source tree was checked against its tested version and differs only in that upstream documentation file.

| Check | Result | Trace |
| --- | --- | --- |
| Workspace console endpoints | 5 passed | [trace](https://dagger.cloud/dagger/traces/839bf02a2df05775ef981a4e84771dca) |
| Workspace save CLI | 2 passed | [trace](https://dagger.cloud/dagger/traces/e9051bf9d2774ddaac5f6a803d810c8b) |
| Client lifecycle and attachable routing (race) | 14 passed | [trace](https://dagger.cloud/dagger/traces/d7987f77bd855c01e25a425e565be06b) |
| Agent runtime restore/OAuth CLI | 15 passed | [trace](https://dagger.cloud/dagger/traces/e9c83fafb6e74ca258a6f585ab8ebb55) |
| Runtime credential invalidation | 1 passed | [trace](https://dagger.cloud/dagger/traces/68b26b0bbd4fa4a98c6def89686ff9df) |
| Runtime cold recipe/schema lookup | 3 passed | [trace](https://dagger.cloud/dagger/traces/1e7c4474635667b0da01fec9886e9390) |
| LLM defining-schema boundary | 1 passed | [trace](https://dagger.cloud/dagger/traces/e30f7c821bad5884810394bc2f46ae69) |
| Clientdb idle-store cache (race) | 34 passed | [trace](https://dagger.cloud/dagger/traces/0cbff028d104d20191aa14f67081dca2) |
| Workspace Git/export/remotes (race) | 36 passed | [trace](https://dagger.cloud/dagger/traces/31ac320d31c7cea0836275e374aa2dbe) |

The combined QA module also loaded successfully and advertised `timings`, `wait`, and `toolset` ([trace](https://dagger.cloud/dagger/traces/a967220cb22d556cb7d8e6fddeaa3035)). The initial workspace core build failure was fixed before the passing rerun. Whitespace checks, patch/provenance audits, reference synchronization checks, and source ancestry checks pass. Full suites, benchmarks, and post-push CI were not claimed. The historical extraction checker still has its pre-existing submitted-head assertion mismatch; the immutable inventory is unchanged.

Rebase logs, conflict-resolution history, original lockfile backup, range diffs, commands, and test logs are retained under `/tmp/dagger-source-rebase-20260914/`.
