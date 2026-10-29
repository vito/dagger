# Local megamerge branch

Last rebuilt: 2026-10-28.

## Branch foundation

Recreated local `megamerge` from `upstream/main` at `5c4cdd243bed2e274c6c08a75e4971965d9735a2`. Ten tracked PRs are listed as open and merged locally; thirty-three requested PRs are inherited from upstream. PR statuses and open heads were refreshed for this rebuild. #14739 has landed, and #14345 has a newer head. Subsequently added #14930 at its fetched head without recreating the branch or refreshing the other PRs.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward, followed by the local artifact agents/editor configuration. The original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). The integration branch was later renamed `megamerge`.

The preceding branch and tracking document are preserved at `backup/megamerge-before-rebuild-20261028` (`f821cf18df09be4fd3aea2fcdea1a3b25a7adab9`). Earlier backups remain intact. This checkout started on local `main` at `49065f5de3d0db2d41cab8ca0fc81fcf322a9a95`; that branch pointer was left unchanged. The checkout now uses rebuilt `megamerge`. No remote branches or GitHub PR states were changed.

Uncommitted changes to `dagger.lock`, `go.mod`, and `go.sum` were saved and restored as uncommitted changes. These include Dang v2.2.0, the context-module lookup, and the local Alpine 3.22 pin, which takes precedence over the older artifact-config pin. The safety stash `d2e9f711ec0a55cc85b647ba332058fbfd6d3ebc` remains available. Historical stashes were not reapplied.

## Open upstream PRs merged locally

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14345](https://github.com/dagger/dagger/pull/14345) | tui: lazily load and bound retained trace logs | `9cdc9a698c4ad5dde180b4f1fded158374837de2` | `6fb89f77f0` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `e372051de0` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `232265738b` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `823bd0598e5f51ab9fa298dc9e430764cbe34e9e` | `006b5df5fd` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `128b10da67` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `f54ec175ff` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `f8777423f435eb6090386fb522bbb2bf04ac486c` | `e5fd647d14` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `491c17053c3ab62e94bd91140c735929eb2b5b8f` | `8eb21f1317` |
| [#14715](https://github.com/dagger/dagger/pull/14715) | core: read agent transcripts with ReadTrace | `6b48de0599c4af1846a91f79a884623e5a3fea31` | `eae944e5d3` |
| [#14930](https://github.com/dagger/dagger/pull/14930) | engine: send Cloud metrics with delta temporality | `b3f5dbe9128be9701fe6fe9939380597a5bdfa7f` | `23adc87d50` |

#14479 is the corrected number for the original request's #11479.

## PRs inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
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

- #14930: merged cleanly. Engine Cloud metrics now use delta temporality; existing local dependency edits were preserved byte-for-byte.

- #14345: retain the refreshed bounded log caches, archive topology/log indexes and idle store holds. Combine these with upstream’s lightweight append-only reopen path and deferred index construction. Keep oversized archive-row handling, current archive lookup flushing, and bounded terminal search.
- #14474/#14464: retain nested output, deferred error aggregation and own-output fallback while hiding sub-agent conversations from tool results.
- #14393: preserve upstream’s asynchronous nested-client registration and exec-helper flag. Select attachables identity from the first request. Keep both upstream and prompt-handling fixtures.
- #14542/#14545: retain writable workspace mount patches and bounded direct bundle capture, SSH export authentication, the shared embedded-patch budget, and core SDK fixture adaptations. Mount summaries use `toolPatchLimits.tooLarge`, carrying forward the previous undefined-`changesetTooLarge` fix.
- #14546: use the explicit hijacked-error marker, preserving upstream GraphQL response headers.
- #14548: retain transcript salvage, gaps/warnings, inferred checkpoints, recoverable heads and addressable salvaged GitRefs.
- #14715: retain ReadTrace agent rosters/transcripts, bounded summaries, pagination and idle-agent display. Preserve the newer #14345 implementation and #14548 recovery features when merging older dependency copies. Keep session cleanup, terminal materialization, nil-safe child traversal and upstream original-module ownership.
- Adapt archive classification and imported fixtures to upstream’s binary protobuf storage format. Update nested transport fixture signatures for the exec-helper flag.
- Carry forward artifact agents/editor configuration and pins from `084386dc83`, keeping newer upstream dependency pins and lock-file header order.
- Carry forward the nested approval fixture cleanup fix from `f100b437df`, archive display core SDK adaptation from `b45b119809`, and snapshot fixture adaptation from `96dee1bb2a`.

## Validation policy and result

The branch owner requested skipping automated builds and tests in favor of dogfooding. No builds or tests were run. All ten included open PR heads are ancestors of this branch; all thirty-three landed PR merge commits are ancestors of the fetched upstream base. Merge conflicts are resolved and Git whitespace checks pass. The only uncommitted changes are the three restored dependency files. Runtime and compilation behavior have not been verified.

Code tip before this tracking-document update: `23adc87d502600ebcfdde3f3bf0fc73915549cdd`.
