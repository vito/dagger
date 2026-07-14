# llm-workspace stacked PRs — handoff after greening C

State as of 2026-07-14, end of the PR-C greening session. Written for a fresh
session. The stack (all `dagger/dagger`, chained bottom-up):

| PR | branch | state |
|---|---|---|
| #13632 A — engine & TUI fixes | `llm-workspace-engine-tui-fixes` → main | **GREEN** |
| #13633 B — Workspace glob/search | `llm-workspace-file-apis` | **GREEN** — history rewritten this session (filesync fixes: cancellation in glob walk, grep files-only limit, search path escape) |
| #13634 C — LLM foundation | `llm-workspace-llm-foundation` @ `5bb87a5d8f` | **GREEN** (30/30 Cloud statuses) |
| #13635 D — LLM ⇄ Workspace | `llm-workspace-llm-binding` @ `1fd56dc335` | CONFLICTING — needs rebase onto green C, see §3 |
| #13636 tip — dev tooling | `llm-workspace-dev-tooling` @ `9405d9ba2b` | needs rebase onto D after |

C is 21 commits on top of B. When a base branch is rewritten, the Cloud `load`
check reports "PR has merge conflicts, cannot run checks" and nothing else
runs — that's a rebase signal, not something to debug.

## 1. What landed on C this session (context for D)

The no-backwards-compat direction was executed: the three compat commits
(sync-evaluates, legacy replay decoder, pre-v1 getter evaluation) are gone.
In the v1 world only `loop`/`step` evaluate; `sync` is a pure ID return and
all state getters (`lastReply`, `historyJSON`, `env`, ...) are passive reads.

- **Fixtures force evaluation explicitly.** The allow-llm gate checks
  `CurrentModule` inside `Loop`/`Step`, so evaluation must happen *inside* the
  module — the LLM object never escapes the fixture functions. The shared
  fixture `dagger-test-modules/llm-dir-module-depender/llm-test-module` now
  calls `.Loop()` before `LastReply`/`HistoryJSON` (pushed as `b0e5ff4`;
  main-compatible since `loop` has existed since the LLM POC and main's
  getters auto-sync, making the second evaluation a no-op). The local
  `go-programmer` fixture's `Save` got the same fix. Fixture modules in
  dagger-test-modules have no vendored client — bindings generate at load
  time under their `engineVersion` view (v0.17.0).

- **MCP object-tag aliasing fix (`3eda3e8937`) — read this before touching
  D's Env rework.** Each step runs tools on a transient MCP clone, then
  rebuilds LLM state via withResponse/withToolResponse/withObject selectors;
  the clone's ingestion registry is discarded. Two latent bugs compounded:
  the step snapshot was taken *after* `Tools()` had already ingested env
  inputs (so inputs never got withObject re-registration), and `IngestBy`
  keyed `idByHash` by recipe digest (xxh3) while `WithObject` keyed by
  `stableIDDigest` (sha256) — neither path recognized the other's entries.
  Result: a later step re-ingested the input under a freshly bumped counter
  and *stole an existing tag* (the empty input workspace became
  `ToyWorkspace#2`), so builds and `Save` ran against the wrong object. The
  fix: snapshot at the top of `step()`, `IngestBy` falls back to (and
  records) the stable ID digest, and `WithObject` raises `typeCounts` to the
  tag's number so fresh ingests can't claim occupied tags. **D reshapes the
  Env surface — preserve these invariants if it touches
  `Ingest`/`WithObject`/step materialization.** Note the bug corrupted a
  recorded golden invisibly (see replay-matching note in §4).

- **Replay goldens re-recorded** in the v1 content-block format (`content` is
  a list of typed blocks; strict `getReplay` decoder, no legacy fallback).
  hello-world was recorded twice — the first recording embedded the aliasing
  bug (`go: warning: "./..." matched no packages` in the build result).

- **Regens committed** (`27a2265bdb`): docs:references (schema.graphqls
  gains LLMMessage/LLMContentBlock/LLMToolCall, LLM.messages, loop
  maxAPICalls; Query.llm loses maxAPICalls), golang:generate-all (CLI ref
  gains `dagger llm` family), php-sdk:api (doctum static docs),
  markdown-lint:fix (llm_branch_summary.md, verified whitespace-only).
  Regen at the rebased tip re-verified drift-free against a fresh dev
  engine.

- `daggerForwardSecrets` honors `DAGGER_LLM_TEST_ENV`; the working golden
  update procedure is documented in llm_test.go (see §4).

- C was rebased onto rewritten B with
  `git rebase --onto upstream/llm-workspace-file-apis <old-B-sha>` — clean,
  but generated files must be re-verified by regen after any such rebase.

- `llmconfig` moved beneath `internal/cmd/dagger/` (`5bb87a5d8f`).

## 2. CI facts for C's green run

- 30 Cloud commit statuses, all success; `gh pr checks 13634` ~86 checks.
- Flake signatures seen this session: `golang:test-all` "Errored in 1m5s"
  (infra, too fast to be a real test run — a push re-runs it); previously
  elixir-sdk "cloning repo: engine is shutting down" and helm TestInstallK3S
  k3s timeout.

## 3. Remaining work: D, then tip

Rebase `llm-workspace-llm-binding` onto green C. Expect the same classes of
work as C needed:

1. Allowlist/view gating for D's new API surface (@agent, object tools) —
   and **Env REMOVAL**: deleting types/fields from the base view needs
   allowlist/view care or `TestBaseSchemaAllowlist` fails.
2. Regens at D's head (docs/golang/php-sdk/markdown-lint; never the `go`
   generator — vendored `**/internal/dagger/dagger.gen.go` must stay
   main-identical, §4).
3. Replay goldens for any D-affected conversations (recording procedure §4).
4. The MCP identity invariants from §1 if D touches that machinery.
5. Then rebase `llm-workspace-dev-tooling` onto D.

## 4. Operational knowledge (hard-won, read before debugging)

- **Updating LLM goldens**: run on the HOST against a dev engine so
  `-update` writes into the worktree:
  `env DAGGER_LLM_TEST_ENV=$PWD/.env ./hack/with-dev go test ./core/integration/ -run TestLLM -count=1 -update`
  `dagger call engine-dev test --update` runs `-update` inside the test
  container and DISCARDS the recordings (`Test` returns only error, unlike
  `TestTelemetry` which returns a Changeset — fixing that signature would
  dirty the vendored engine-dev bindings, so it stays).
- **Replay matching only compares TEXT blocks** (`TextContent()`), not tool
  results or tool calls. Tool-result regressions replay "successfully" and
  goldens can embed live bugs invisibly — eyeball recordings for artifacts
  like failed builds before committing them.
- **Credential-free gate/decoder testing**: `--model=replay/<base64 of a
  hand-written []LLMMessage JSON>` never contacts a provider. Roles are
  `USER`/`ASSISTANT`; a minimal user+assistant exchange exercises the full
  loop, gate, and decoder from `dagger call`/`shell`.
- **TestAllowLLM prompt_calls fails locally** if
  `~/.config/dagger/prompt-confirmations.json` has a persisted `allow_llm:`
  entry for the fixture module (prompt answers persist via `PersistentKey`;
  only "yes" persists). Delete the entry; CI is unaffected (fresh HOME).
- **.env files**: they normally hold `op://` 1Password references; each `op`
  process needs interactive desktop approval, so headless sessions block on
  them. A literal token may be present locally — never commit .env.
- **Traces**: always `dagger --x-release 1.0.0-beta.6 trace <id>` (plain
  `dagger trace` is the wrong CLI). Pipe to a file and grep. The default
  view collapses child spans — use `--span <id>` to drill down.
- **Re-running one failed check**: `cloud rerun --check` reported "no Cloud
  checks found" from this environment; workaround is a SHA bump
  (`git commit --amend --no-edit` + force-push) — Cloud caching makes green
  checks cheap replays. Checks are commit STATUSES, not check runs.
- **Generator cache lying**: several toolchains' generate cache keys omit
  the engine schema; a <1s generator run is a stale replay. Bust with a
  fresh dev engine (`./hack/dev`) and run
  `./hack/with-dev ./bin/dagger generate -y docs:references golang:generate-all php-sdk:api markdown-lint:fix`.
  The branch CLI alone tries to pull the unpublished v1.0.0 engine — always
  go through `./hack/with-dev`.
- **`dagger generate` needs `-y`** in non-TTY contexts. If export fails with
  "'dagql/idtui/' does not have a commit checked out", delete the stray
  nested `.git`/`dagger.toml` under dagql/idtui and retry.
- **Latent main breakage** (independent of this stack): when Cloud's
  native-ci engine is next redeployed from current main,
  go:generate-dagger-runtimes will fail ON MAIN — committed vendored clients
  still carry the removed `Workspace.Update()` and current codegen emits new
  `<module>.gen.go` self-bindings. A coordinated regen will be needed then.
- Local `golangci-lint` is older than the repo's Go and won't run; rely on
  build/vet/schema-test canaries + CI. `go vet ./core/...` has one
  pre-existing main finding (services.go lostcancel) — ignore it.
- Local engines: `dagger-engine.dev` (from branch source via
  `./hack/dev`/`./hack/with-dev`), plus stock `dagger-engine-v0.21.7` and
  `dagger-engine-v1.0.0-beta.6`.
