# Local main megamerge

Last rebuilt: 2026-10-07 (third rebuild).

## Branch foundation

Recreated from `upstream/main` at `676a73c20a817152404db7ebf202cb05eb2d9080`. Fifteen open PRs were merged in the order below, including new additions #14550 and #14548. Fourteen requested PRs are now inherited from upstream; #14531 landed since the previous rebuild.

Historically, this branch started as `upstream/main` with #14345 merged fast-forward, followed by the local artifact agents/editor configuration. That original foundation is preserved at `backup/main-before-megamerge-20261004` (`fb1ca4d899c27040e9e28b16340a276d9924bb50`). The rebuilds use #14345's recorded current head.

The immediately preceding megamerge and its tracking document are preserved at `backup/main-before-rebuild-20261007-3` (`f3cb51a42a6c95bd4797cf46516668060b15fb57`). All earlier backup branches remain intact. The artifact agents/editor configuration and existing integration fixes were carried forward. No remote branches or GitHub PR states were changed.

The pre-existing uncommitted `dagger.lock` entry for Alpine 3.23 was restored unchanged as an unstaged edit. A copy is also retained in stash `90915b57258024e4eec0fb66d1656c881ea83a7a` (`megamerge 20261007-3: preserve local dagger.lock edit`). It is not part of the rebuilt commits.

## Open PRs reapplied

| PR | Change | Included head | Local merge |
| --- | --- | --- | --- |
| [#14345](https://github.com/dagger/dagger/pull/14345) | tui: lazily load and bound retained trace logs | `5fe01e1a87cd9aa628b99b17bde460495c240661` | `b2cb13e942` |
| [#14479](https://github.com/dagger/dagger/pull/14479) | agent: fix subagent compact/branch and OTLP crash | `94f36d46d8ace753fd6a55d96c856a59f4c436fd` | `e8aaa8cfac` |
| [#14474](https://github.com/dagger/dagger/pull/14474) | core/mcp: keep nested output in tool results | `40e6e1dc6c6f270d48fe6aa2bace63f42bba0cfe` | `48568c613a` |
| [#14473](https://github.com/dagger/dagger/pull/14473) | core: record tool state field-wise, not as its producing call | `80a2bc147a227e853874666162c220152fed2987` | `f64ca39149` |
| [#14466](https://github.com/dagger/dagger/pull/14466) | core: apply and diff small changesets cheaply | `eb54e8d86c9602d30ce77350ce79e4eb34858641` | `1a4bf9114a` |
| [#14465](https://github.com/dagger/dagger/pull/14465) | llm: send configuration once, in client metadata | `b4e4eb1f48972455f09737503aa49bf9cc0b9154` | `833c3348d5` |
| [#14464](https://github.com/dagger/dagger/pull/14464) | core/mcp: keep sub-agent chats out of tool results | `f931c9636ceea2963d8a32d932d1a5edb3c3cca8` | `7b52107830` |
| [#14463](https://github.com/dagger/dagger/pull/14463) | core/llm: map effort to thinking budget on Haiku | `8fb5c405ae100a656acdece45127c50d0eccac02` | `c632a434d8` |
| [#14393](https://github.com/dagger/dagger/pull/14393) | fix(client): serve prompts from nested interactive CLI | `975ffaec17b92db402127b0af4a59fd6e523de3f` | `7f3d4d3332` |
| [#14530](https://github.com/dagger/dagger/pull/14530) | workspace: prepare SSH auth for export destination | `598027c64cd7c2c89b89d004b80777b3ae2218b9` | `6378b127f3` |
| [#14542](https://github.com/dagger/dagger/pull/14542) | core: make workspace mounts writable | `829cee91a0e3089be50a2cf7b98dfdaf1d3ae548` | `5f5044500d` |
| [#14545](https://github.com/dagger/dagger/pull/14545) | workspace: cap captured bundles, drop the split-and-rejoin | `4ae3ff33a070de8398d7e8fe26887be707ba196a` | `a61970d975` |
| [#14546](https://github.com/dagger/dagger/pull/14546) | engine: keep handler error status codes; let nested clients read archives | `b8a5d6bcb1b34d3a80ba1b1f8a0a931769f8734e` | `89c1d61f8d` |
| [#14550](https://github.com/dagger/dagger/pull/14550) | idtui: stop live updates from scaling with session size | `59c101b51f9ffe0b9ba387e0d693b09c46000262` | `ff95ac86fa` |
| [#14548](https://github.com/dagger/dagger/pull/14548) | tui console: salvage agents from broken traces | `49e70574ee4e2676164cf3c031cd1eddcaaedd6f` | `453a7befc6` |

#14479 is the corrected number for the original request’s #11479.

## PRs now inherited from upstream

| PR | Change | Upstream merge |
| --- | --- | --- |
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

All fifteen open heads and all fourteen upstream merge commits are ancestors of this branch.

## Conflict resolutions and local changes

- #14345 / upstream #14541: retain lazy archive selection, its signal reader and span annotations, together with oversized-row shrinking/skipping in the batch loop. Agent control records remain protected.
- #14464 / #14474: preserve nested tool output and hide nested agent conversations independently; retain scoped checks, tests, and services comments.
- #14463 / #14465: keep the client-metadata router and apply catalog reasoning mode in `core/llm_router.go`, where routing now lives.
- #14393 / upstream nested-session support: retain the new-session HTTP method on the test handler and the separate nested CLI attachables test.
- #14542 / upstream #14531: use the writable-mount patch renderer, retaining the shared `EmbeddedPatchMaxBytes` budget and `PatchNotEmbeddable` fallback.
- #14545 / #14530: stream the bounded captured bundle directly, retaining the shared `withCapturedCheckoutSSHAuth` helper. Capture validates bundle length, so the removed chunk-splitting/rejoining path and its duplicate length check stay removed. Both snapshot and export pass the capture limit.
- #14550 and #14548 merged cleanly, adding live-update performance improvements and console recovery for agents in broken traces. #14546 and #14530 also merged without manual conflicts.
- `2806c8cdf7`: preserve the local artifact agents/editor configuration and lock entries, alongside current upstream settings.
- `9c23d31bad` and `d7c850459b`: adapt Haiku reasoning and catalog fixtures to the client-metadata LLM configuration.
- `e949776a95`: adapt workspace patch fixtures to the changeset delta fast-path signature.
- `b2c33d2b75`: make the nested approval test wait for the query result before stopping the outer CLI.

## Validation policy

The branch owner requested that future megamerge rebuilds and additions skip automated test runs because they take too long. Use dogfooding instead unless the owner explicitly requests tests. Continue resolving merge conflicts and recording branch state here.

## Validation for this rebuild

No builds or tests were run, per the branch owner's request. Validation is by dogfooding. The recorded open PR heads and upstream merge commits were verified as ancestors, merge conflicts were resolved, and `git diff --check` passed. Historical test results belong to earlier branches and remain in their backup tracking documents.

Code tip before this tracking-document commit: `b2c33d2b7525da842e3666b4cfe75c6c650db204`.
