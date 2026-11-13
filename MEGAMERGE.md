# Local megamerge branch

Last rebuilt: 2026-11-13.

## Current state

Local `megamerge` is rebuilt from fetched `origin/main` at `3991a28b9dbcfb6e9badbf35fa3b54e9419d07f2`. Twenty-five tracked PRs remain open and are merged locally; forty-nine tracked PRs are already inherited from upstream. `origin` is `dagger/dagger`; `vito` is the fork. The base includes #15133’s host-policy support for session clients and #15144’s Dang SDK reference correction.

The previous branch and tracking document are preserved at `backup/megamerge-before-rebuild-20261113-184058` (`bcb1cdd2fd1f38dae3ed830682cb7ef23aada488`). The pre-existing uncommitted `dagger.lock` change is preserved in the working tree and is not part of the merge commits. Local `main` is unchanged. Nothing was pushed and no upstream PR or stack branch was changed by this rebuild.

## Historical foundation

This branch originally started as `upstream/main` with #14345 merged fast-forward. The original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). The integration branch was later renamed `megamerge`. #14479 corrects the original request for #11479.

The eleven-layer telemetry stack below supersedes #14345. Its older terminal disk cache, primary-output spool, and archive implementation are not separately merged. Closed #14715 is replaced by #15154 at the top of that stack. The local “use artifact agents/editor” override remains dropped.

## Open upstream PRs merged locally

Rows are in local merge order. The first eleven form the engine telemetry stack (native stack #14801). Five standalone PRs follow, then #15156 and its rendering PR #15164. The approval stack follows in order: #15163, #15169, #15171, #15172, #15173, #15174, #15175.

| PR | Title | Included head | Local merge |
| --- | --- | --- | --- |
| [#14793](https://github.com/dagger/dagger/pull/14793) | telemetry: engine store and Cloud as equivalent trace sources | `10681158ddaf3df85b90f1d1e53d4863614b8d3f` | `a12c8a137640` |
| [#14794](https://github.com/dagger/dagger/pull/14794) | telemetry: retain every session's telemetry by trace | `4dd9270e91a30b49edd403f8137fd16025361ce0` | `2b3c40309f18` |
| [#14795](https://github.com/dagger/dagger/pull/14795) | telemetry: lazy historical log display for agent -r | `288c65175683bd29ec04dbb8a50633b1677b9dc2` | `849c7ebf1cc0` |
| [#14796](https://github.com/dagger/dagger/pull/14796) | telemetry: on-demand live log reads, presence push, search | `21b1404aca7a540b61f4e8a6fc891544f311b297` | `fdc1012dae1e` |
| [#14797](https://github.com/dagger/dagger/pull/14797) | telemetry: hand the CLI's own telemetry to the engine; engine alone publishes to Cloud | `053008c75452ec730e8abe37335177f7b8be554b` | `87ea3b960833` |
| [#14798](https://github.com/dagger/dagger/pull/14798) | telemetry: live TUI reads log bodies on demand | `39307484934e58f8472ab44f9f6c00f14399d212` | `397e98e80e38` |
| [#14799](https://github.com/dagger/dagger/pull/14799) | telemetry: exact exit barrier for on-demand live logs | `179348b96b79971d711cd41e534d3aeb471717fa` | `d08dfd11e19e` |
| [#14800](https://github.com/dagger/dagger/pull/14800) | telemetry: bound TUI log memory, page and search unloaded logs | `598c401a5460da8a8fe04b6b3a831e4121fa826d` | `e428de0c5e48` |
| [#15019](https://github.com/dagger/dagger/pull/15019) | telemetry: dagger agent -r --no-backfill | `f1409dfc568194a4840fda9e30f4af2c9bad014d` | `74e8582daf6e` |
| [#15023](https://github.com/dagger/dagger/pull/15023) | telemetry: read one-shot log queries by index | `0381d30cac0f504871f82b9931de07adad8a740d` | `db28d0982a96` |
| [#15154](https://github.com/dagger/dagger/pull/15154) | core: read agent transcripts with ReadTrace | `45fcfac842bddbba08ee959037c14e1a5f592106` | `798eb0018689` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `d1b487db6f1dedba1beff4a0bf58df8a0053851a` | `33199b6810dc` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `e7512e11189a` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `2781761309d87a3f9a65b34165c2fd2955030bd7` | `dc83478b8ecf` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `78d02073a740498671cd357c3dfc561ffcd7b8d7` | `4b3fc4536566` |
| [#14997](https://github.com/dagger/dagger/pull/14997) | Tidy the agent roster: lineage for nested clients, focus-relative strip | `2795597fa65dbf5032dd3106f064792482cd6768` | `6e45d9fedb5c` |
| [#15156](https://github.com/dagger/dagger/pull/15156) | llm: add DangCheck and DangEval builtin tools | `afab68b35b15a665b48676279964569609490636` | `8490d4fce759` |
| [#15164](https://github.com/dagger/dagger/pull/15164) | Render Dang tool calls as code in dagger agent | `b5cb8d1f3d76603e54f71d7808319916a27020de` | `340244e12fc3` |
| [#15163](https://github.com/dagger/dagger/pull/15163) | core: restored workers can't read host secrets | `efcfc7a313d6dac37690e60b27bd18745ada7d94` | `517f05db6500` |
| [#15169](https://github.com/dagger/dagger/pull/15169) | engine: scoped approval grants (once/session/forever) | `27a7edc28073d59eebbf6c9ca4a733c0140aee35` | `18457bd90ffd` |
| [#15171](https://github.com/dagger/dagger/pull/15171) | idtui: inline approvals under the asking tool call | `1e535bd3ce1587de043c2e624ecb523cbdd799ea` | `7cfacca965cd` |
| [#15172](https://github.com/dagger/dagger/pull/15172) | engine: ask before agents read host secrets | `2ded3d1bc929ed23eab999851cdbd5bde086f415` | `3d18a6fd3ef8` |
| [#15173](https://github.com/dagger/dagger/pull/15173) | core: ask before using the user's Git identity | `a2e05d30ba185eedf7a2665fe03fe26c217422c3` | `c0591e1bcb57` |
| [#15174](https://github.com/dagger/dagger/pull/15174) | idtui: desktop notifications for approvals and turn ends | `1880ac1f34a743af670490684ed8198484b40c1d` | `9bbb480e8797` |
| [#15175](https://github.com/dagger/dagger/pull/15175) | engine: only prompt for default-branch pushes | `a6bba87b5b6225eff58310b59112846eb51cd847` | `53865a37980e` |

## Integration resolutions

- #15171 and #14997: combine inline approval indicators with the focus-relative roster. Include approval generation in roster memoization so cues refresh when requests change, and bubble hidden agents’ pending approvals to the nearest visible ancestor’s needs-you count.

- #15156 and #14997: retain `InertAttachables`, caller agent identity, and agent lineage in the client scope snapshot, metadata reconstruction, cloning, and delegation.

- #14796 and #15023: preserve #14793’s sequential full archive replay when the store has no log index, while retaining indexed display/selected reads and the newer cursor and row-count contract.
- #15154: combine ReadTrace transcript support with the landed #14548 checkpoint recovery and conversation salvage code. Remove duplicate checkpoint decoding helpers introduced by the overlapping changes.
- #14464: preserve #14474’s nested tool output and #15154’s ReadTrace transcript pointers while retaining hidden sub-agent conversations and on-demand TUI log reads. Remove the duplicate conversation-pointer helper.

## Recent fixes included

- Added #15171 inline TUI approvals, #15172 secret-access approval, #15173 Git identity approval, #15174 desktop notifications, and #15175 default-branch-only push prompts. #15163 and #15169 were already included. The new PR heads form a verified ancestry chain above #15169; #15172–#15175 merged without additional conflicts.

- #14800 now includes `598c401a5460da8a8fe04b6b3a831e4121fa826d`, “cli: DIAGNOSTICS for live log replays.” The temporary instrumentation is intentionally retained for dogfooding. It records rate-limited diagnostics in the session trace while hiding them from the TUI and primary output. The commit message calls for dropping it before upstream merge.
- #15156 adds the DangCheck and DangEval builtin LLM tools. Refreshed head `afab68b35b15a665b48676279964569609490636` adds chat and me bindings and lets Dang snippets act for their agent, while retaining bounded results and module scoping.
- #15164 is refreshed to `b5cb8d1f3d76603e54f71d7808319916a27020de` on the updated Dang tools branch. Dang code rendering and Timeout presentation remain included.
- #15163 (`efcfc7a313d6dac37690e60b27bd18745ada7d94`) prevents restored workers from reading host secrets, including its binding identity correction.
- #15169 (`27a7edc28073d59eebbf6c9ca4a733c0140aee35`) adds scoped approval grants (once/session/forever) on top of #15163. Both PRs merged cleanly when first included.

- #14793 includes sequential full archive replay and engine connection selection for `dagger trace`. These fixes exist on the first layer; descendant upstream branches have not been rebased onto them. This integration preserves both fixes alongside all later layers.
- #14545 includes the unused-import removal and the oversized-bundle fixture’s core SDK adaptation.
- #14997 includes the roster-neighborhood extraction addressing the lint complexity failure.
- #14393 includes the nested-approval fixture cleanup previously carried locally.
- #14546, #14548, and #14542 are now inherited from upstream, including their HTTP error handling, checkpoint/console test corrections, and writable-mount fixture corrections. #15128 and #15138 are also inherited. Historical local fix commits were not blindly reapplied.

## PRs inherited from upstream

The merge commit for each of these forty-nine tracked PRs is an ancestor of the recorded base.

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

## Validation and limits

Verified that the upstream base and all twenty-five captured PR heads are ancestors of the rebuilt branch. The base and previous twenty PR heads were unchanged, so their existing integration commits were reused. Inspected and formatted the inline-approval/roster conflict resolution. Git whitespace checks and inspection of changed files found no conflict markers or duplicate top-level Go function names. The diagnostics commit remains an ancestor, and its full patch passes a reverse-application check. No upstream branches were modified.

No builds, tests, or full lint suite were run, per the dogfooding preference. This document records integration state, not CI or review readiness.
