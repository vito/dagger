# Local megamerge branch

Last rebuilt: 2026-10-19.

## Branch foundation

Recreated local `megamerge` from `upstream/main` at `80699ac67a1c72dc38d30c4186105cd9dd3afde6`. Nine tracked PRs remain open and are merged locally; thirty-two requested PRs are inherited from upstream. All PR statuses and open heads were refreshed for this rebuild. #14637, #14719, #14716, #14712, and #14703 have landed since the previous additions. #14548 advanced to include recovery of an agent's last workspace commit.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward, followed by the local artifact agents/editor configuration. The original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). This checkout was later renamed from `main` to `megamerge`; the separate local `main` branch is unchanged.

The preceding branch and tracking document are preserved at `backup/megamerge-before-rebuild-20261019` (`caa78e3b9206869cdcef4b9d87430e5d080077a9`). Earlier backups remain intact. No remote branches or GitHub PR states were changed.

Pre-existing uncommitted `dagger.lock` and `dagger.toml` edits were restored byte-for-byte and remain unstaged. A safety copy remains in stash `b0cca366090c1b5c0eb2cf5e183ceec2e592d9bb` (`megamerge 20261019: preserve local config edits`). Historical stashes were left untouched.

## Open upstream PRs merged locally

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14345](https://github.com/dagger/dagger/pull/14345) | tui: lazily load and bound retained trace logs | `5fe01e1a87cd9aa628b99b17bde460495c240661` | `40f90f061c` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `22ee574c2b` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `131f4f1b0a` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `823bd0598e5f51ab9fa298dc9e430764cbe34e9e` | `348d8f2bc8` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `df1c121e5c` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `f10e02d196` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `f8777423f435eb6090386fb522bbb2bf04ac486c` | `29a61f7196` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `491c17053c3ab62e94bd91140c735929eb2b5b8f` | `ded591056f` |
| [#14715](https://github.com/dagger/dagger/pull/14715) | core: read agent transcripts with ReadTrace | `6b48de0599c4af1846a91f79a884623e5a3fea31` | `70b5241730` |

#14479 is the corrected number for the original request's #11479.

## PRs inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
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

- #14345: reused the archive and lazy-view search resolutions where both input blobs matched the preceding rebuild. Preserve oversized-row handling and use `Vterm.SearchMatchRows()` for bounded terminal search.
- #14474/#14464: retain nested output, deferred error aggregation, and the own-output fallback while hiding sub-agent conversations from tool results.
- #14393: retain upstream's asynchronous nested-client registration and new proxy token handling. Choose the attachables identity from the first request and pass it to the registration goroutine. Keep both upstream's token fixture and the PR's prompt fixtures.
- #14542: keep writable mount patch rendering with the shared `EmbeddedPatchMaxBytes` budget and `PatchNotEmbeddable` handling. Adapt added fixtures to the core SDK. Use `toolPatchLimits.tooLarge` for mount summaries, carrying forward the fix for the undefined `changesetTooLarge` call after #14703's refactor.
- #14545: keep bounded direct bundle capture with SSH export authentication; retain the core-SDK snapshot fixture adaptation.
- #14546: use the explicit hijacked-error marker instead of the empty-write probe, retaining upstream GraphQL content type and response headers.
- #14548: combine upstream's readable transcript decoder with loaded-telemetry fallback, explicit gaps/warnings, inferred checkpoints, and salvage support. Reapply the previously resolved console integration, then apply the PR's new recoverable-head changes, including `/head`, `/heads`, `head.json`, and addressable salvaged GitRefs. Adapt existing salvage fixtures to JSON requests and tolerant decoding.
- #14715: retain the new ReadTrace roster/transcript views, bounded summaries, pagination hints, idle-agent display, and reusable inference helper. Preserve #14548's newer head recovery when merging its duplicated dependencies; remove the duplicate old sub-agent transcript pointer. The branch carries its nil-safe child traversal and terminal materialization integration fixes.
- `e1887cd0ca`: carry forward artifact agents/editor configuration and pins. User module-source and lockfile edits remain separately unstaged.
- `785c7c9a21`: retain the nested approval fixture's wait for query output before stopping the outer CLI.
- `bb4f83956a`: retain the archive display fixture's core SDK adaptation.
- Landed PRs now supply expertise composition, roster optimization, glamour v2, native Git tree diffs, and stale-generator patch output. Prior local commits implementing these were not reapplied separately.

## Validation policy and result

The branch owner requested that rebuilds and additions skip automated builds and tests; use dogfooding unless explicitly asked otherwise. No builds or tests were run. All nine open PR heads are ancestors of this branch, and all thirty-two landed PR merge commits are ancestors of the fetched upstream base. Merge conflicts were resolved, local config files were verified byte-for-byte, and whitespace checks passed. Earlier validation records remain with the backup branches.

Code tip before this tracking-document commit: `bb4f83956a2b6e483d25795f8e33c30b99265bd6`.
