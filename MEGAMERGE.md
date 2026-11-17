# Local megamerge branch

Last rebuilt: 2026-11-17.

## Handoff: what “recreate” means

This file is the durable handoff for another agent or machine. A request to “read MEGAMERGE.md and recreate” means: rebuild the local `megamerge` branch from the latest upstream main and the latest retained PR heads, resolve integration conflicts, preserve local work, and update this file. The tables and status statements below are a snapshot from the last rebuild, not current GitHub status. Refresh them before acting. Do not reset to the recorded base or pin to the recorded PR SHAs unless explicitly asked to reproduce that historical snapshot.

Standing preferences:

- Skip builds, tests, full lint suites, and CI polling. The user dogfoods. Lightweight source inspection, `gofmt` on resolutions, `git diff --check`, and ancestry checks are appropriate; report their limits honestly.
- Work locally. Do not push, merge or close PRs on GitHub, rebase their source branches, or move local `main` as part of a recreate.
- Preserve tracked edits, staged changes, untracked files, and existing stashes. Never assume the current machine has the same dirty files listed below. Back up the existing integration branch before replacing it.
- Keep the explicitly dropped changes out: #14345, #14715 (replaced by #15154), #15163, and the local “use artifact agents/editor” override. Do not resurrect the superseded temporary diagnostics commit `598c401a54`; use the current telemetry PR heads.

### Rebuild procedure

1. Inspect the current checkout, remotes, repository instructions, and this file. Locate upstream by URL (`github.com/dagger/dagger`), not just remote name; on the original machine it is `origin`, and `vito` is the fork. Save this document outside the branch being rebuilt and create a timestamped backup ref if a local megamerge exists. An isolated detached worktree is convenient for keeping local edits untouched while integrating.
2. Fetch upstream `main`. Refresh GitHub state, title, head SHA, head branch, and base branch for the PRs in the **Open upstream PRs merged locally** table. Add any newly requested PRs; deduplicate by PR number. The inherited table is history, not a request to merge those PR heads again. Respect explicit exclusions even if their PR state later changes.
3. For each retained open PR, fetch `refs/pull/<number>/head` from upstream to a separate local ref and verify the fetched SHA matches the refreshed metadata. If a PR merged upstream, confirm its merge commit is in the new base and move it to the inherited table. If it closed without merging, inspect its replacement/context rather than silently equating closed with merged or automatically dropping requested work; seek clarification if its intended replacement cannot be established. #15163 is already explicitly dropped.
4. Start from the freshly fetched upstream base and merge retained heads in table order, adjusting for changed PR dependencies. Use separate local merge commits (`git merge --no-ff`) so the included heads remain auditable. Resolve conflicts using the integration notes below and current code. Do not merge the old megamerge wholesale, which would reintroduce dropped or superseded heads. Reusing an unchanged prefix of prior integration commits is fine only when both the upstream base and all heads in that prefix are identical. Otherwise rebuild that portion. Do not reuse entire resolved files across changed inputs without inspecting the differences.
5. Reconcile the **Integration resolutions** and local-only fixes against the resulting source, including clean auto-merges. Check for duplicated helpers and old API references. In particular, preserve the wrapped test-log trace-hint correction documented below even if no Git conflict occurs. Do not blindly cherry-pick old fixes that upstream already includes. A clean merge is not evidence that the code compiles.
6. Verify that the new base and every included head are ancestors of the result. Check the applicable dropped-head exclusions, whitespace, conflict markers, and the source around resolutions. Follow the no-build/no-test preference. Commit only integration changes and this document, with project commit conventions and sign-off. Preserve existing local modifications separately. Switch the local checkout to the completed `megamerge` without overwriting those modifications, and verify they survived; leave local `main` unchanged.
7. Rewrite this document to describe the finished branch: current base, merge order, PR titles and exact heads, local merge commits, inherited/dropped PRs, new resolutions, backup ref, preserved local edits, and actual verification. Keep these handoff instructions and the historical foundation. Report the result and any limitations; do not push.

### Moving between machines

Copy the **latest version of this file** to the new machine, or transfer a Git branch/bundle containing it. The handoff branch is `megamerge` on `vito/dagger`; fetch it explicitly rather than using an older local remote-tracking ref. A clone of upstream `dagger/dagger` alone will not contain this integration document. GitHub access and an authenticated `gh` CLI are needed to refresh PR metadata and fetch heads.

The recorded local merge SHAs and backup refs are useful only if their Git objects were transferred. They are not prerequisites: a fresh clone can reconstruct from upstream PR refs and these notes. No `/tmp/megamerge-*` scripts, JSON state files, previous chat, or local rerere cache are required. Reconstruct the workflow from this document; do not depend on those temporary artifacts.

Uncommitted files are not transported by this document or by pushing a branch. If the working changes listed below are wanted on the other machine, transfer them separately. Their preservation here does not make them part of megamerge or a prerequisite for rebuilding it.

## Current state

Local `megamerge` is rebuilt from fetched `origin/main` at `d8e43b54632d8de744d4c7687e48c2b9d4993e48`. Fourteen tracked PRs remain open and are merged locally; 63 tracked PRs are inherited from upstream. All twelve telemetry PRs (#14793–#14800, #15019, #15023, #15154, and #15184) have landed through upstream merge `d8e43b54632d8de744d4c7687e48c2b9d4993e48`. `origin` is `dagger/dagger`; `vito` is the fork.

The previous branch and tracking document are preserved at `backup/megamerge-before-rebuild-20261117-161853` (`0e881b64fa5f47f2e6f8dcc5ca8f83e5492bf447`). The checkout was clean at the start: the older snapshot's `dagger.lock`, `dang.toml`, and `prompt.dang` working changes were not present. All 38 existing stashes were preserved. Local `main` remains at `2c2cdaa056e5a7f8bc9675ef71f927c6ec171307`. No upstream PR or source branch was changed, and nothing was pushed. The fork's `megamerge` remains the handoff publication destination when explicitly requested.

## Historical foundation

This branch originally started as `upstream/main` with #14345 merged fast-forward. The original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). The integration branch was later renamed `megamerge`. #14479 corrects the original request for #11479.

The twelve-layer engine telemetry stack supersedes #14345; its older terminal disk cache, primary-output spool, and archive implementation are not separately merged. Closed #14715 is replaced by #15154. The local “use artifact agents/editor” override remains dropped.

## Dropped PR

[#15163](https://github.com/dagger/dagger/pull/15163) is closed and was explicitly dropped from the merge set. Its former head `efcfc7a313d6dac37690e60b27bd18745ada7d94` is not an ancestor of this rebuild. The approval stack now starts with #15169 against main. Current host-secret and hostless-client behavior comes from the refreshed stack; the old #15163 integration was not reapplied.

## Open upstream PRs merged locally

Rows are in merge order: five standalone PRs, eight approval/prompt layers (#15169 through #15179), then #15182. GitHub state and all heads were refreshed for this rebuild.

| PR | Title | Head branch | PR base | Included head | Local merge |
| --- | --- | --- | --- | --- | --- |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `vito/mcp-nested-output` | `main` | `d1b487db6f1dedba1beff4a0bf58df8a0053851a` | `797bdd016a2b` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `tool-result-hide-subagent-conversation` | `main` | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `e2392f6524ac` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `vito/nested-cli-prompts` | `main` | `2781761309d87a3f9a65b34165c2fd2955030bd7` | `8e174ddca9b1` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `vito/cap-capture-bundles` | `main` | `78d02073a740498671cd357c3dfc561ffcd7b8d7` | `855f2b31d8bd` |
| [#14997](https://github.com/dagger/dagger/pull/14997) | Tidy the agent roster: lineage for nested clients, focus-relative strip | `agent-roster-lineage` | `main` | `2795597fa65dbf5032dd3106f064792482cd6768` | `e676c5a4f2c5` |
| [#15169](https://github.com/dagger/dagger/pull/15169) | engine: scoped approval grants (once/session/forever) | `approval-grants` | `main` | `5686c8ebb75eb0c1744a50863fe7d81f902390f7` | `08b2047feb2c` |
| [#15171](https://github.com/dagger/dagger/pull/15171) | idtui: inline approvals under the asking tool call | `approval-inline-tui` | `approval-grants` | `c0112dd7ef9203e40c1a02190429405cc63e5fd7` | `df27f115477e` |
| [#15172](https://github.com/dagger/dagger/pull/15172) | engine: ask before agents read host secrets | `approval-secrets` | `approval-inline-tui` | `bbdeac508858ce9b787e4c67a0b4178571b91bec` | `d2b43d3140b0` |
| [#15173](https://github.com/dagger/dagger/pull/15173) | core: ask before using the user's Git identity | `approval-git-identity` | `approval-secrets` | `070e389798233871bf9ee05eed171e50f4951f1c` | `c3800af8a693` |
| [#15174](https://github.com/dagger/dagger/pull/15174) | idtui: desktop notifications for approvals and turn ends | `tui-notifications` | `approval-git-identity` | `92dfacb4f2208d1e658c5fd66b8f99f24eb00910` | `28abb9eac1be` |
| [#15175](https://github.com/dagger/dagger/pull/15175) | engine: only prompt for default-branch pushes | `push-default-branch-only` | `tui-notifications` | `d7a8c8b75eba2d49073378e64f7d459e86481080` | `b6ce17af1400` |
| [#15178](https://github.com/dagger/dagger/pull/15178) | engine: route every engine prompt through the prompt broker | `prompt-broker` | `push-default-branch-only` | `bd0299e07aefade0e264ea1a5347b5daa1946813` | `bbf644429326` |
| [#15179](https://github.com/dagger/dagger/pull/15179) | core: add Query.prompt | `prompt-api` | `prompt-broker` | `5e7232e091b49c388df50f3fa3bf547b9062bede` | `eaf304f186ee` |
| [#15182](https://github.com/dagger/dagger/pull/15182) | idtui: render tool calls while args stream | `live-tool-args` | `main` | `81091bbc1e2aed611423f8a5241e863a0e20cc67` | `b17feef7615b` |

## Integration resolutions

- #14464 against landed telemetry and #14474: retain nested tool output, ReadTrace transcript summaries and transcript-view pointers, hidden sub-agent conversations, and on-demand TUI log reads. Remove the duplicate `subAgentConversationPointer` introduced by the overlap; keep `SummarizeConversation` and `markLogsInView`.
- #14997 against landed Dang tools: preserve agent lineage, inert attachables, and caller agent identity in client scope snapshots, metadata reconstruction, cloning, and delegation.
- #15171 against #14997: combine inline approval indicators with the focus-relative roster. Include approval generation in memoization and bubble hidden agents' pending approvals to the nearest visible ancestor's needs-you count.
- #15182 against landed telemetry: retain `TraceCommand` while enabling width fitting for framed rollups in zoomed conversations; retain both unloaded-log search coverage and the search-match reflow case, with both required imports.
- Wrapped test-log correction, applied within the #15182 merge: in `dagql/idtui/frontend_tests.go`, `TestView.renderTestSummaryLogs` must call `errorWindowLines(out, rows, indent, tv.TraceCommand, traceLogsHintTarget(entry.span))` in its `final && wrap` branch. The PR still introduces stale `tv.TraceID` and `cloudLogsHintTarget(entry.span)` references. The raw-log path already uses the correct API. This replaces the need to cherry-pick historical local fix `6e0eedfe0dbda6df0dd325f43c56176c9175226e`; preserve the adaptation even if future merges are clean.
- New local-only fix `c4841f95661abe8d7af923849e91ff638bfbc2d8`: in `focusAgentThen`, the call-history fetch callback must retry against `fe.agentRosterOrder()`, not `fe.agentRosterEntries()`. Upstream's new lazy recipe fetch auto-merges cleanly with #14997, but searching only visible tabs silently loses requests to focus hidden agents reached by cycling or ID.
- Now inherited from telemetry: sequential full archive replay without an index alongside indexed reads; checkpoint recovery and conversation salvage alongside ReadTrace transcripts; bounded row-ID deduplication with replay-warning state and placement-aware filtering. These were not reapplied from old integration commits.

## Current additions and retained fixes

- #15182 is refreshed to `81091bbc1e2aed611423f8a5241e863a0e20cc67`. Besides streaming tool arguments and rich-result rendering, its new commits stabilize Anthropic required-field schema ordering, guard stale focus nodes, simplify search navigation return values, and update the prompt-edit fixture.
- The landed telemetry stack includes its newer lazy call-history fetching, agent resume/backfill behavior, archive selection, and live-log exit handling. The full telemetry implementation comes from the new base, not the old open heads.
- #15178 routes every engine prompt through the prompt broker; #15179 adds Query.prompt. The approval stack still includes scoped grants, inline approvals, secret-access and Git identity approval, desktop notifications, and default-branch push policy. It remains based away from the explicitly dropped #15163.
- #14545 retains its unused-import and core SDK fixture fixes; #14997 retains its roster complexity cleanup; #14393 retains nested-approval fixture cleanup. No old local fix was blindly cherry-picked.

## PRs inherited from upstream

The merge commits of these 63 tracked PRs are ancestors of the recorded base.

| PR | Title | Upstream merge |
| --- | --- | --- |
| [#14167](https://github.com/dagger/dagger/pull/14167) | fix(git): hold mirror locks only to copy objects out | `e119a8df8133` |
| [#14400](https://github.com/dagger/dagger/pull/14400) | perf(git): commit remote workspaces natively | `a34d69b2fcae` |
| [#14401](https://github.com/dagger/dagger/pull/14401) | perf(git): reuse approved host history on demand | `913c0faee9ff` |
| [#14427](https://github.com/dagger/dagger/pull/14427) | fix(workspace): handle SSH module source URLs | `228637c644f3` |
| [#14433](https://github.com/dagger/dagger/pull/14433) | fix(git): resolve missing refs through the upstream | `e2af618b27b8` |
| [#14448](https://github.com/dagger/dagger/pull/14448) | fix(git): keep plumbing output out of span logs | `30b6955ebc62` |
| [#14460](https://github.com/dagger/dagger/pull/14460) | mcp: record tool changesets as workspace patches | `f72cf717cb05` |
| [#14463](https://github.com/dagger/dagger/pull/14463) | core/llm: map effort to thinking budget on Haiku | `6425d070f895` |
| [#14465](https://github.com/dagger/dagger/pull/14465) | llm: send configuration once, in client metadata | `cf29e0e02e86` |
| [#14466](https://github.com/dagger/dagger/pull/14466) | core: apply, diff and merge changesets by their layers | `18504114ce90` |
| [#14473](https://github.com/dagger/dagger/pull/14473) | core: record tool state field-wise, not as its producing call | `b51271016844` |
| [#14479](https://github.com/dagger/dagger/pull/14479) | agent: fix subagent compact/branch and OTLP crash | `e9f9a3781044` |
| [#14488](https://github.com/dagger/dagger/pull/14488) | fix(core): let agents read private git URLs | `16155cdcae4b` |
| [#14498](https://github.com/dagger/dagger/pull/14498) | core/mcp: lift list-of-object tool args | `3fc43ec342d4` |
| [#14500](https://github.com/dagger/dagger/pull/14500) | core/mcp: run continuations in written position | `2228177a1c3d` |
| [#14504](https://github.com/dagger/dagger/pull/14504) | engine: don't log redirected withExec stdout/stderr | `69ac0c250395` |
| [#14514](https://github.com/dagger/dagger/pull/14514) | fix(core): resume agent sessions from private SSH checkouts | `33c6a589eae4` |
| [#14516](https://github.com/dagger/dagger/pull/14516) | workspace: carry user config in snapshots, add Workspace.withUserConfig | `2520b10e5a32` |
| [#14530](https://github.com/dagger/dagger/pull/14530) | workspace: prepare SSH auth for export destination | `0c586690122c` |
| [#14531](https://github.com/dagger/dagger/pull/14531) | workspace: freeze Git workspaces as-is, bound embedded patches | `d84226110e84` |
| [#14541](https://github.com/dagger/dagger/pull/14541) | archive: tolerate a single oversized row | `91111cb8ff7f` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `ca60691bf322` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `529efc519917` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `a0a9b1f54b31` |
| [#14550](https://github.com/dagger/dagger/pull/14550) | idtui: stop live updates from scaling with session size | `55096f78e775` |
| [#14576](https://github.com/dagger/dagger/pull/14576) | core: speed up workspace git checkouts, pulls and merges | `1659c5690daa` |
| [#14618](https://github.com/dagger/dagger/pull/14618) | idtui: keep the render loop responsive on huge traces | `ca1912936ced` |
| [#14620](https://github.com/dagger/dagger/pull/14620) | dagui: cut span memory by more than half | `90f71a27c627` |
| [#14633](https://github.com/dagger/dagger/pull/14633) | dagui: place RowsView spans by local rules, not walk order | `0e3dd62eb490` |
| [#14634](https://github.com/dagger/dagger/pull/14634) | dagui: build only the trees a view shows; drop pacing | `f9d7e23a3272` |
| [#14637](https://github.com/dagger/dagger/pull/14637) | agents: compose expertise with owning identity | `6c28af7993e2` |
| [#14703](https://github.com/dagger/dagger/pull/14703) | Print the patch when a generator's /stale check fails | `ef5b22d62674` |
| [#14712](https://github.com/dagger/dagger/pull/14712) | git: add GitCommit.treeSHA and diff changes with diff-tree | `18d51f8291ea` |
| [#14716](https://github.com/dagger/dagger/pull/14716) | idtui: render markdown with glamour v2 | `62ea36621aaf` |
| [#14719](https://github.com/dagger/dagger/pull/14719) | idtui: skip agent roster rebuild when unchanged | `0ba7c4c7c291` |
| [#14739](https://github.com/dagger/dagger/pull/14739) | dagui: scope test views via the ancestry index | `49065f5de3d0` |
| [#14891](https://github.com/dagger/dagger/pull/14891) | core: name modules by version, GraphQL-style field casing | `7e9eaacc376e` |
| [#14895](https://github.com/dagger/dagger/pull/14895) | core: rename the core API to GraphQL casing | `7e9eaacc376e` |
| [#14902](https://github.com/dagger/dagger/pull/14902) | core: version entrypoint modules via dang.toml | `7e9eaacc376e` |
| [#14930](https://github.com/dagger/dagger/pull/14930) | engine: send Cloud metrics with delta temporality | `49caafb5af34` |
| [#14958](https://github.com/dagger/dagger/pull/14958) | core: version Dang modules via dang.toml, one Dang library | `7e9eaacc376e` |
| [#14959](https://github.com/dagger/dagger/pull/14959) | core: add clientVersion to client schemas | `7e9eaacc376e` |
| [#14996](https://github.com/dagger/dagger/pull/14996) | core: add ModuleSource.naming for SDK codegen | `7e9eaacc376e` |
| [#15053](https://github.com/dagger/dagger/pull/15053) | agent: integrate provider setup and fix model switching | `a45241b20a5e` |
| [#15058](https://github.com/dagger/dagger/pull/15058) | agent: handle slash commands outside prompt queues | `a45241b20a5e` |
| [#15080](https://github.com/dagger/dagger/pull/15080) | Fresh `dagger check -l` of dagger/dagger in under 5 s: per-package Go builds, a pre-warmed engine cache, and faster module loading and check planning | `99773d51b765` |
| [#15100](https://github.com/dagger/dagger/pull/15100) | cli: fix memory, duplicate logs and dropped collector spans; fix two dagger check reports | `ce1dcebcd55a` |
| [#15128](https://github.com/dagger/dagger/pull/15128) | dang: bump vito/dang to v2.4.1 | `057dd6458b5c` |
| [#15138](https://github.com/dagger/dagger/pull/15138) | git: fetch to expand short SHAs, and record the full SHA | `917e4e2ba87d` |
| [#15156](https://github.com/dagger/dagger/pull/15156) | llm: add DangCheck and DangEval builtin tools | `2a385e9049da` |
| [#15164](https://github.com/dagger/dagger/pull/15164) | Render Dang tool calls as code in dagger agent | `2a385e9049da` |
| [#14793](https://github.com/dagger/dagger/pull/14793) | telemetry: engine store and Cloud as equivalent trace sources | `d8e43b54632d` |
| [#14794](https://github.com/dagger/dagger/pull/14794) | telemetry: retain every session's telemetry by trace | `d8e43b54632d` |
| [#14795](https://github.com/dagger/dagger/pull/14795) | telemetry: lazy historical log display for agent -r | `d8e43b54632d` |
| [#14796](https://github.com/dagger/dagger/pull/14796) | telemetry: on-demand live log reads, presence push, search | `d8e43b54632d` |
| [#14797](https://github.com/dagger/dagger/pull/14797) | telemetry: hand the CLI's own telemetry to the engine; engine alone publishes to Cloud | `d8e43b54632d` |
| [#14798](https://github.com/dagger/dagger/pull/14798) | telemetry: live TUI reads log bodies on demand | `d8e43b54632d` |
| [#14799](https://github.com/dagger/dagger/pull/14799) | telemetry: settle live log exit with the barrier; interruptible reads | `d8e43b54632d` |
| [#14800](https://github.com/dagger/dagger/pull/14800) | telemetry: bound TUI log memory, page and search unloaded logs | `d8e43b54632d` |
| [#15019](https://github.com/dagger/dagger/pull/15019) | telemetry: resume agents without backfilling history | `d8e43b54632d` |
| [#15023](https://github.com/dagger/dagger/pull/15023) | telemetry: read one-shot log queries by index | `d8e43b54632d` |
| [#15154](https://github.com/dagger/dagger/pull/15154) | core: read agent transcripts with ReadTrace | `d8e43b54632d` |
| [#15184](https://github.com/dagger/dagger/pull/15184) | telemetry: dedupe live log rows by row ID | `d8e43b54632d` |

## Validation and limits

Verified that the fetched base and all fourteen captured open PR heads are ancestors of the rebuilt branch, with each head recorded as the second parent of its own local merge commit. All 63 inherited PR merge commits are ancestors of the base. This rebuild reused no previous integration commits.

Checked that the dropped heads of #14345 (`9cdc9a698c4ad5dde180b4f1fded158374837de2`), #14715 (`8be12a16bfac965cc1ec0ee58c108b29caf9d87f`), and #15163 (`efcfc7a313d6dac37690e60b27bd18745ada7d94`) remain outside the result's ancestry. The superseded diagnostics commit `598c401a54` and known artifact-editor overrides `2806c8cdf7` and `30f276b6ec` are also absent.

Git whitespace checks passed. Files changed from the base contain no conflict markers or duplicate top-level Go function names within a file (excluding legal repeated `init` declarations). Resolved Go files were formatted and inspected; stale wrapped-log API names are absent. The source comparison against the previous integration retains the earlier conflict resolutions while incorporating the updated upstream and #15182 code. The checkout is clean, local `main` is unchanged, and all existing stashes are intact.

No builds, tests, full lint suite, or CI polling were run, per the dogfooding preference. These source checks do not establish compilation or runtime correctness. Nothing was pushed.
