# Local megamerge branch

Last rebuilt: 2026-10-16.

## Branch foundation

Recreated local `megamerge` from `upstream/main` at `ff0b029a4d270a651b9a8c7286babc79089066e4`. This checkout was renamed from `main` before this rebuild; the separate local `main` branch was left unchanged. Eight open PRs were merged in the order below. Twenty-seven requested PRs are inherited from upstream. No tracked PR changed state or head since the preceding rebuild.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward, followed by the local artifact agents/editor configuration. That original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). Rebuilds use #14345's recorded current head.

The preceding megamerge and its tracking document are preserved at `backup/megamerge-before-rebuild-20261016` (`bd8169f37b33cdce81c35a285dfd39e6a6f7af74`). Earlier backup branches remain intact. The artifact agents/editor configuration and applicable integration fixes were carried forward. No remote branches or GitHub PR states were changed.

Pre-existing uncommitted `dagger.lock` edits were preserved and restored as unstaged edits: 67 removed keys and three added/updated keys. The new upstream beta.18 entry was retained. The original edits remain in stash `000ca9e3d566b6e239bbf5f25b6743267a775faa` (`megamerge 20261016: preserve local lock edits`). Historical stashes were left untouched.

## Open PRs reapplied

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14345](https://github.com/dagger/dagger/pull/14345) | tui: lazily load and bound retained trace logs | `5fe01e1a87cd9aa628b99b17bde460495c240661` | `3861612d44` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `c276290db1` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `32183dbe70` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `823bd0598e5f51ab9fa298dc9e430764cbe34e9e` | `3e8e691dae` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `893b48a82b` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `f89e6ff60b` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `f8777423f435eb6090386fb522bbb2bf04ac486c` | `02cec33be7` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `49e70574ee4e2676164cf3c031cd1eddcaaedd6f` | `815d38f037` |

#14479 is the corrected number for the original request’s #11479.

## PRs now inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
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

All eight open heads and all twenty-seven upstream merge commits are ancestors of this branch.

## Conflict resolutions and local changes

- #14345 / upstream #14541: retain lazy archive selection, the signal reader and span annotations, and oversized-row handling. Agent control records remain protected.
- #14474 / upstream tool-output changes: preserve deferred error aggregation and nested output rendering, alongside the own-output capture fallback. #14464 independently hides nested agent conversations.
- #14542 / upstream #14531: use the writable-mount patch renderer, retaining the shared `EmbeddedPatchMaxBytes` budget and `PatchNotEmbeddable` fallback. Adapt its fixtures to upstream's generated core SDK.
- #14545 / upstream #14530: stream the bounded captured bundle directly and retain `withCapturedCheckoutSSHAuth`. Keep chunk splitting/rejoining and the duplicate bundle-length check removed. Both snapshot and export pass the capture limit. Adapt the new snapshot fixture to the core SDK.
- Prior archive, tool-output, writable-mount, and capture resolutions were reused only where both input files matched the preceding rebuild. #14548 merged cleanly.
- #14345 / upstream #14634: retain DB-based lazy-view search and use `Vterm.SearchMatchRows()` for log matches, preserving terminal materialization and locking. #14633/#14634's tree placement and lazy construction are now inherited upstream.
- #14393 / upstream asynchronous nested-client registration: choose the first request's attachables client before starting registration, pass that immutable choice to the registration goroutine, and retain per-request cancellation while registration continues for the exec. Retain newer upstream session-helper error logging and timing changes.
- #14546 / upstream GraphQL error responses: retain the explicit hijacked-error marker and remove the empty-write probe so HTTP statuses are preserved. Keep upstream's JSON content type and response headers in `gqlError.WriteTo`.
- `1a0f2a0a29`: retain the prior TUI integration fixes: console recovery uses nil-safe `ChildSpans.Spans()`, and `PrintTail` materializes terminals evicted by #14345's bounded cache.
- `3e4acfc003`: preserve the artifact agents/editor configuration and local pins while keeping upstream's refreshed dependency versions, including the beta.18 lock entry. The user’s uncommitted lockfile changes are applied separately on top.
- `437c5b007a`: retain the nested approval fixture's wait for query output before stopping the outer CLI.
- `3c4bce68f5`: adapt the archive display fixture to upstream's generated core SDK; #14393 already includes the nested CLI adaptation.
- #14465 and #14463 now supply the metadata router, Haiku reasoning integration, and adapted fixtures upstream. The former local reasoning/catalog fixture fixes were not reapplied.
- Earlier Git lock/checkout integration (#14167) and changeset delta fixture fixes (#14466) remain supplied by upstream; old local resolutions were not reapplied over their landed implementations.

## Validation policy

The branch owner requested that future megamerge rebuilds and additions skip automated test runs because they take too long. Use dogfooding instead unless the owner explicitly requests tests. Continue resolving merge conflicts and recording branch state here.

## Validation for this rebuild

No builds or tests were run, per the branch owner's request. Runtime validation is left to dogfooding. All recorded open PR heads and upstream merge commits were verified as ancestors, merge conflicts were resolved, and `git diff --check` passed. Historical test results belong to earlier branches and remain in their backup tracking documents.

Code tip before this tracking-document commit: `3c4bce68f5259714a3c5db9aad1ec6561339de3d`.
