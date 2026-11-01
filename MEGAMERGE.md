# Local megamerge branch

Last rebuilt: 2026-10-31 (telemetry stack replacement).

## Branch foundation

Recreated local `megamerge` from `upstream/main` at `117c67181d2eece74a2a16e464bac7456296100c`. Replaced #14345 with the nine-layer telemetry stack: #14793, #14794, #14795, #14796, #14797, #14798, #14799, #14800, and #15019. Eighteen tracked PRs remain open and are merged locally; thirty-four requested PRs are inherited from upstream. All tracked PR statuses and included heads were refreshed.

The replacement deliberately drops #14345’s disk-spilled terminal cache, primary-output spool, and older archive implementation. The new stack reads logs on demand from engine/Cloud providers, bounds retained TUI logs, and adds `agent -r --no-backfill`. #14345 remains open upstream as historical reference but is no longer in the local merge set.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward. The original foundation remains at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). The integration branch was later renamed `megamerge`. #14479 corrects the original request’s #11479.

The previous branch and document are preserved at `backup/megamerge-before-telemetry-stack` (`965c8c006af4c17402d562cc3b999e5e3a812d4b`). Earlier backups remain intact. Local `main` was not moved. No remote branches or GitHub PR states were changed.

The local “use artifact agents/editor” override and extra pins remain dropped because that functionality landed upstream. The committed `dagger.toml` and `dagger.lock` match the upstream base.

Before rebuilding, 35 generated Go files and `dagger.lock` had local edits. They were saved in stash `6a32acdf6b791605796aba323c41d3311c4ac826` and restored. All 35 generated files retain their exact saved contents; 32 are now supplied by upstream. Three generated files remain modified (the cli-dev, engine-dev, and go modules’ `internal/dagger/core/core.gen.go`), along with the lockfile, whose local changes were combined with newer upstream pins. The stash remains as a backup. Historical stashes were not reapplied.

## Replacement stack, in dependency order

| Layer | PR | Change | Included head | Local merge |
| --- | --- | --- | --- | --- |
| 1 | [#14793](https://github.com/dagger/dagger/pull/14793) | telemetry: engine store and Cloud as equivalent trace sources | `bd130fa6442d5fcf10810974cd2f9740b1b3a076` | `c8bec2b5b3` |
| 2 | [#14794](https://github.com/dagger/dagger/pull/14794) | telemetry: retain every session's telemetry by trace | `6602bed88e53fd29c635f5b17c1f7a4e534d076b` | `f736299032` |
| 3 | [#14795](https://github.com/dagger/dagger/pull/14795) | telemetry: lazy historical log display for agent -r | `70f71ca2b036bda18162cd16619492da07a3be8a` | `602aa462a2` |
| 4 | [#14796](https://github.com/dagger/dagger/pull/14796) | telemetry: on-demand live log reads, presence push, search | `79a82f2e543ea9d0dfe76d1384ace89f8b2b8b0d` | `a198d70170` |
| 5 | [#14797](https://github.com/dagger/dagger/pull/14797) | telemetry: hand the CLI's own telemetry to the engine; engine alone publishes to Cloud | `e18d5807b343956016750d8651d82550ac6c246b` | `402910eac3` |
| 6 | [#14798](https://github.com/dagger/dagger/pull/14798) | telemetry: live TUI reads log bodies on demand | `0c07e71b640c2e9ea833e54e5583cb13e75da1b2` | `036f54ccf7` |
| 7 | [#14799](https://github.com/dagger/dagger/pull/14799) | telemetry: exact exit barrier for on-demand live logs | `146570bed46ea16671d82b74bd15998325206d60` | `b8bd57b27c` |
| 8 | [#14800](https://github.com/dagger/dagger/pull/14800) | telemetry: bound TUI log memory, page and search unloaded logs | `f40ed65a7114789660b986bf25dd69db1c4ee985` | `3209dae4a6` |
| 9 | [#15019](https://github.com/dagger/dagger/pull/15019) | telemetry: dagger agent -r --no-backfill | `9251398bbb9070d49e8d489230b12192dc617710` | `d36a27499f` |

## Other open upstream PRs merged locally

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `f8cb009b11` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `b12e2c8f34` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `823bd0598e5f51ab9fa298dc9e430764cbe34e9e` | `b8b11f75ae` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `5143fbdcad` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `76f90a30dd` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `f8777423f435eb6090386fb522bbb2bf04ac486c` | `cc6dcb45f5` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `491c17053c3ab62e94bd91140c735929eb2b5b8f` | `60e974304f` |
| [#14715](https://github.com/dagger/dagger/pull/14715) | core: read agent transcripts with ReadTrace | `6b48de0599c4af1846a91f79a884623e5a3fea31` | `521d2ef76a` |
| [#14997](https://github.com/dagger/dagger/pull/14997) | Tidy the agent roster: lineage for nested clients, focus-relative strip | `fce751371e0b5fafeb0631eeddcaa2f73539d7ed` | `cfdfc623ff` |

## PRs inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
| [#14930](https://github.com/dagger/dagger/pull/14930) | engine: send Cloud metrics with delta temporality | `49caafb5af3432f72e582db609758204ef1a23ec` |
| [#14739](https://github.com/dagger/dagger/pull/14739) | dagui: scope test views via the ancestry index | `49065f5de3d0db2d41cab8ca0fc81fcf322a9a95` |
| [#14637](https://github.com/dagger/dagger/pull/14637) | agents: compose expertise with owning identity | `6c28af7993e21a644e893c91321339160f15de14` |
| [#14719](https://github.com/dagger/dagger/pull/14719) | idtui: skip agent roster rebuild when unchanged | `0ba7c4c7c291bdc1ae5a69a80ce154cd24ef62db` |
| [#14716](https://github.com/dagger/dagger/pull/14716) | idtui: render markdown with glamour v2 | `62ea36621aafcb9c056f935a18d69edb8f0f1bae` |
| [#14712](https://github.com/dagger/dagger/pull/14712) | git: add GitCommit.treeSHA and diff changes with diff-tree | `18d51f8291eabe93dc2b8e93dd47c8cfa6397625` |
| [#14703](https://github.com/dagger/dagger/pull/14703) | Print the patch when a generator's /stale check fails | `ef5b22d62674d9ff4ef508199113b07e6ade2100` |
| [#14633](https://github.com/dagger/dagger/pull/14633) | dagui: place RowsView spans by local rules, not walk order | `0e3dd62eb490ebac789090348279b12ed3bc645b` |
| [#14634](https://github.com/dagger/dagger/pull/14634) | dagui: build only the trees a view shows; drop pacing | `f9d7e23a32723427178b6d09ed8d75de6cc3cafd` |
| [#14618](https://github.com/dagger/dagger/pull/14618) | idtui: keep the render loop responsive on huge traces | `ca1912936ced4f60a32330f04a083c4eeec7e506` |
| [#14620](https://github.com/dagger/dagger/pull/14620) | dagui: cut span memory by more than half | `90f71a27c6278ff662e831b56949617f7dac1c59` |
| [#14465](https://github.com/dagger/dagger/pull/14465) | llm: send configuration once, in client metadata | `cf29e0e02e8673f0bff246b6b7d37fb66b55c95f` |
| [#14463](https://github.com/dagger/dagger/pull/14463) | core/llm: map effort to thinking budget on Haiku | `6425d070f895d04b93b7019807fab7f04958e8d5` |
| [#14167](https://github.com/dagger/dagger/pull/14167) | fix(git): hold mirror locks only to copy objects out | `e119a8df81332e3fc21cc00e8d40a72f5f8c64cb` |
| [#14479](https://github.com/dagger/dagger/pull/14479) | agent: fix subagent compact/branch and OTLP crash | `e9f9a3781044d9963a99c79b2f77be510fa48510` |
| [#14576](https://github.com/dagger/dagger/pull/14576) | core: speed up workspace git checkouts, pulls and merges | `1659c5690daa8c5c0d6ae8f428432a15b3411399` |
| [#14473](https://github.com/dagger/dagger/pull/14473) | core: record tool state field-wise, not as its producing call | `b512710168444dbd3b230e6d7e0d6e8a07605869` |
| [#14466](https://github.com/dagger/dagger/pull/14466) | core: apply, diff and merge changesets by their layers | `18504114ce9008963885f57a3d98176a08112849` |
| [#14530](https://github.com/dagger/dagger/pull/14530) | workspace: prepare SSH auth for export destination | `0c586690122ce49d88c719f7f1f9e8123389f7fa` |
| [#14550](https://github.com/dagger/dagger/pull/14550) | idtui: stop live updates from scaling with session size | `55096f78e775d9689187c7cd5d9aa272e1229ee0` |
| [#14531](https://github.com/dagger/dagger/pull/14531) | workspace: freeze Git workspaces as-is, bound embedded patches | `d84226110e84281f333d9c0dfefdc8bba095538b` |
| [#14541](https://github.com/dagger/dagger/pull/14541) | archive: tolerate a single oversized row | `91111cb8ff7f9342783819068796ed525787f24c` |
| [#14460](https://github.com/dagger/dagger/pull/14460) | mcp: record tool changesets as workspace patches | `f72cf717cb054c9501a5930b5a4a65fd15e77a80` |
| [#14516](https://github.com/dagger/dagger/pull/14516) | workspace: carry user config in snapshots, add Workspace.withUserConfig | `2520b10e5a32289e878fb84e3d892f19ebb58274` |
| [#14427](https://github.com/dagger/dagger/pull/14427) | fix(workspace): handle SSH module source URLs | `228637c644f3ed5f75330dfdda3cbc81e60e5a44` |
| [#14488](https://github.com/dagger/dagger/pull/14488) | fix(core): let agents read private git URLs | `16155cdcae4b2169d24bd3a8330cac8915a5c2a0` |
| [#14514](https://github.com/dagger/dagger/pull/14514) | fix(core): resume agent sessions from private SSH checkouts | `33c6a589eae404de85c7ef503661bbb793660478` |
| [#14400](https://github.com/dagger/dagger/pull/14400) | perf(git): commit remote workspaces natively | `a34d69b2fcae77ca0d40c944ccfab8ff5e1c08ff` |
| [#14401](https://github.com/dagger/dagger/pull/14401) | perf(git): reuse approved host history on demand | `913c0faee9fff2341cdacebe7df6a8d4c183d74f` |
| [#14433](https://github.com/dagger/dagger/pull/14433) | fix(git): resolve missing refs through the upstream | `e2af618b27b822685853b514ca92bc062252bfe1` |
| [#14448](https://github.com/dagger/dagger/pull/14448) | fix(git): keep plumbing output out of span logs | `30b6955ebc62e707c6c9aa37bf28f038e9f6191e` |
| [#14498](https://github.com/dagger/dagger/pull/14498) | core/mcp: lift list-of-object tool args | `3fc43ec342d46d075b189b03f963af4a17bd57d0` |
| [#14500](https://github.com/dagger/dagger/pull/14500) | core/mcp: run continuations in written position | `2228177a1c3d37581fbaf365d1ba8e264d1a30ea` |
| [#14504](https://github.com/dagger/dagger/pull/14504) | engine: don't log redirected withExec stdout/stderr | `69ac0c2503952150a7175c4d34393f03f496611f` |

## Conflict resolutions and local changes

- All nine replacement layers merged cleanly in order. Their selected-read, provider, live-log, exit-barrier, paging, search, and no-backfill implementations are retained.
- #14715 contains an older copy of #14345 among its ancestors. Its merge incorporates the previously resolved ReadTrace feature delta without reintroducing those bundled dependencies. Old commits remain in ancestry through #14715, but their superseded implementations are absent. The obsolete terminal materialization and spilled-session cleanup adaptations were omitted.
- #14474/#14464: retain nested output, deferred error aggregation, own-output fallback, and hidden sub-agent conversations. Combine the new on-demand log visibility tracking with the report conversation filter.
- #14393: retain first-request attachables identity selection alongside asynchronous nested-client registration, exec-helper flags, and the new CLI telemetry routing.
- #14542/#14545: retain writable workspace mounts, bounded direct bundles, SSH export authentication, shared embedded-patch limits, and core SDK fixture adaptations. Mount summaries still use `toolPatchLimits.tooLarge`.
- #14546: preserve explicit hijacked-error handling and response status codes alongside the stack’s live telemetry endpoints and retained-session registration.
- #14548: retain transcript salvage, gaps/warnings, inferred checkpoints and recoverable heads while preserving the live console’s on-demand log integration.
- #14715: retain ReadTrace agent rosters/transcripts, conversation summaries, pagination, idle-agent display, and nil-safe child traversal. Adapt imported fixtures to binary protobuf storage.
- #14997: merged cleanly; retain nested-client lineage, parent-aware agent nodes, and the focus-relative roster with stopped-agent folding.
- Carry forward the nested approval cleanup and snapshot fixture fixes, plus current nested-transport signatures and workspace tool core SDK types. The replacement stack supplies the `context` import previously fixed locally. The obsolete #14345 archive-display fixture and store-opener adaptations are replaced by the stack’s own fixtures.

## Validation policy and result

No automated builds or tests were run, per the branch owner’s dogfooding policy. All eighteen included open PR heads are ancestors of this branch, and all thirty-four landed merge commits are ancestors of the fetched upstream base. Conflicts are resolved and Git whitespace checks pass. The superseded terminal cache, spool, and archive-source files are absent. Compilation and runtime behavior have not been verified.

Code tip before this tracking-document commit: `e9b06254ef95a509bde50f9fef0a8cd5c59a7ebc`.
