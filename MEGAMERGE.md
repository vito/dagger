# Local main megamerge

Last rebuilt: 2026-10-10.

## Branch foundation

Recreated from `upstream/main` at `90f71a27c6278ff662e831b56949617f7dac1c59`. Ten open PRs were merged in the order below, including new additions #14633 and #14634. Twenty-five requested PRs are inherited from upstream; #14618 and #14620 landed since the previous rebuild. #14546 has an updated head. #14634 was refreshed after its rebase to `bb3b1054d399ec8a9a7ff8acf7465508f3d7a376` and merged after #14633. It replaces render pacing with lazy tree construction.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward, followed by the local artifact agents/editor configuration. That original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). Rebuilds use #14345's recorded current head.

The preceding megamerge and its tracking document are preserved at `backup/main-before-rebuild-20261010` (`c049ec3f06f5b2cb87a2532cc193c08ba1552b50`). Earlier backup branches remain intact. The artifact agents/editor configuration and applicable integration fixes were carried forward. No remote branches or GitHub PR states were changed.

Five pre-existing uncommitted `dagger.lock` entries (vito/agents main, Alpine 3.23, Go 1.24 Alpine, Node 24.13.1 Alpine, and Playwright 1.58.2 Noble) were preserved and restored as unstaged edits. A copy remains in stash `0107423a7f145bc8ee4227c217131ad411ae609e` (`megamerge 20261010: preserve local lock edits`). Historical stashes were left untouched.

## Open PRs reapplied

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14345](https://github.com/dagger/dagger/pull/14345) | tui: lazily load and bound retained trace logs | `5fe01e1a87cd9aa628b99b17bde460495c240661` | `ecd071fd73` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `6a4d53594e` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `502d77e73c` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `823bd0598e5f51ab9fa298dc9e430764cbe34e9e` | `6025d42987` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `1631460f33` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `ea40a61b98` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `f8777423f435eb6090386fb522bbb2bf04ac486c` | `6d4780e184` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `49e70574ee4e2676164cf3c031cd1eddcaaedd6f` | `016dff5af7` |
| [#14633](https://github.com/dagger/dagger/pull/14633) | dagui: place RowsView spans by local rules, not walk order | `c81f9a482867278d7f935508f6750b09984dacc0` | `660c444d3e` |
| [#14634](https://github.com/dagger/dagger/pull/14634) | dagui: build only the trees a view shows; drop pacing | `bb3b1054d399ec8a9a7ff8acf7465508f3d7a376` | `0dfdd3ea14` |

#14479 is the corrected number for the original request’s #11479.

## PRs now inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
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

All ten open heads and all twenty-five upstream merge commits are ancestors of this branch.

## Conflict resolutions and local changes

- #14345 / upstream #14541: retain lazy archive selection, the signal reader and span annotations, and oversized-row handling. Agent control records remain protected.
- #14474 / upstream tool-output changes: preserve deferred error aggregation and nested output rendering, alongside the own-output capture fallback. #14464 independently hides nested agent conversations.
- #14542 / upstream #14531: use the writable-mount patch renderer, retaining the shared `EmbeddedPatchMaxBytes` budget and `PatchNotEmbeddable` fallback. Adapt its fixtures to upstream's generated core SDK.
- #14545 / upstream #14530: stream the bounded captured bundle directly and retain `withCapturedCheckoutSSHAuth`. Keep chunk splitting/rejoining and the duplicate bundle-length check removed. Both snapshot and export pass the capture limit. Adapt the new snapshot fixture to the core SDK.
- Prior resolutions for those conflicts were reused only where both input files matched the preceding rebuild. #14393, #14546, and #14548 merged without manual conflicts.
- #14633 merged cleanly. Rebased #14634 overlapped the tree-placement implementation and search: retain its lazy walker, on-demand child/revealed trees, running/revealer indexes, and removal of pacing. Use `Vterm.SearchMatchRows()` in its DB-based search to preserve #14345's terminal materialization and locking. The resulting walker/types/span-set files match #14634's head.
- `faad5e948b`: retain the prior TUI integration fixes with now-upstream #14618/#14620: console recovery uses nil-safe `ChildSpans.Spans()`, and `PrintTail` materializes terminals evicted by #14345's bounded cache before reading their rendered rows.
- `b29a6563a0`: preserve the artifact agents/editor configuration and lock entries alongside current upstream settings.
- `2af004014d`: retain the nested approval fixture's wait for query output before stopping the outer CLI.
- `734b78b48b`: adapt the archive display fixture to upstream's generated core SDK; #14393 already includes the nested CLI adaptation.
- #14465 and #14463 now supply the metadata router, Haiku reasoning integration, and adapted fixtures upstream. The former local reasoning/catalog fixture fixes were not reapplied.
- Earlier Git lock/checkout integration (#14167) and changeset delta fixture fixes (#14466) remain supplied by upstream; old local resolutions were not reapplied over their landed implementations.

## Validation policy

The branch owner requested that future megamerge rebuilds and additions skip automated test runs because they take too long. Use dogfooding instead unless the owner explicitly requests tests. Continue resolving merge conflicts and recording branch state here.

## Validation for this rebuild

No builds or tests were run, per the branch owner's request. Runtime validation is left to dogfooding. All recorded open PR heads and upstream merge commits were verified as ancestors, merge conflicts were resolved, and `git diff --check` passed. Historical test results belong to earlier branches and remain in their backup tracking documents.

Code tip before this tracking-document commit: `734b78b48b8e3edcabd332bc5ee3c79798e6c722`.
