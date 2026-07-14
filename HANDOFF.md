# llm-workspace stacked PRs — CI-greening handoff

State as of 2026-07-14, after the CI-greening session for the stack. Written for
a fresh session. The stack (all `dagger/dagger`, chained bottom-up):

| PR | branch | state |
|---|---|---|
| #13632 A — engine & TUI fixes | `llm-workspace-engine-tui-fixes` → main | **GREEN** (85 pass) |
| #13633 B — Workspace glob/search | `llm-workspace-file-apis` | **GREEN** (85 pass) |
| #13634 C — LLM foundation | `llm-workspace-llm-foundation` | in progress, see §2 |
| #13635 D — LLM ⇄ Workspace | `llm-workspace-llm-binding` | untouched, needs rebase onto green C |
| #13636 tip — dev tooling | `llm-workspace-dev-tooling` | untouched, needs rebase onto D |

Old refs from the tidy session (`llm-workspace-stacked`, `llm-workspace-tidy`,
`backup/llm-workspace-fresh-2026-07-13`, `regen-workspace-overlay-base`) still
exist locally; the PR branches are the live line now. #13600 merged into main.

## 1. What landed on B (context for C/D)

- `core/schema: gate Workspace.glob and search to v1` — new public APIs MUST be
  gated `View(AfterVersion("v1.0.0-0"))` or `TestBaseSchemaAllowlist` fails.
  Fixture modules calling gated APIs need `engineVersion: v1.0.0` bumps.
- `regen go sdk module runtimes` — **vendored `**/internal/dagger/dagger.gen.go`
  clients must stay byte-identical to main.** CI's `go:generate-dagger-runtimes`
  runs a FIXED DEPLOYED engine (not PR source); local regens of those files with
  any other engine produce bogus drift. Never regenerate them on these branches.
- The gocyclo refactor of `searchWithRipgrep` is folded into the search commit.

## 2. PR C — where it stands

**Local branch `llm-workspace-llm-foundation` is at `e3cfbb3ac`, 5 commits
ahead of the pushed remote (`6c5cadb6e`) and with rewritten history below it (a
DCO fix); the next push must be `--force-with-lease`.** Worktree clean.

Local commits on top of the rebased 12-commit foundation:

```
03d0f04d0  chore(lint): resolve golangci-lint findings        (DCO signoff added)
75dd6f610  core/schema: gate the new LLM API surface to v1    KEEP
72537c2ae  test(llm): pass max-api-calls to loop              KEEP
46473615d  core/schema: restore evaluation semantics for LLM.sync   DROP (§3)
9fc6b24fd  core: decode legacy string-content LLM replays           DROP (§3)
82488a788  core: resolve golangci-lint findings in llm.go     KEEP
4508b9db1  codegen/typescript: align enum golden with acronym casing KEEP
e3cfbb3ac  core/schema: evaluate on pre-v1 LLM state getters        DROP (§3)
```

### CI failure map from C's last run (15 checks) and status

| Cluster | Checks | Status |
|---|---|---|
| DCO | DCO | fixed locally (03d0f04d0) |
| Stale regens | docs:references, golang:generate-all, php-sdk:api | **NOT DONE** — see §4 |
| Markdown lint | markdown-lint:{lint,fix} | **NOT DONE** — part of §4 |
| Go lint | golang:lint-all (5 findings in core/llm.go) | fixed (82488a788) |
| ci:bootstrap | composite of lint+markdown | clears with the above |
| TS enum golden | test-split:test-base (TestTypeEnum: EstarGz→EStarGz) | fixed (4508b9db1); the committed TS client already shipped `EStarGz`, golden follows |
| Replay + allow-llm | test-split:test-llm (13 failures) | fixed the WRONG WAY — redo per §3 |
| Flakes | elixir-sdk:{codegen-test,lint,release-dry-run,sdk-test} ("cloning repo: engine is shutting down"), golang:test-all (helm TestInstallK3S k3s timeout) | no fix needed; any push re-runs them |

## 3. DIRECTION CHANGE (decided by Alex): no backwards compatibility

The session's agents fixed two test-llm root causes by adding backwards
compatibility. **That direction is rejected. Drop these three commits** (rebase
them out):

- `46473615d` restore evaluation semantics for LLM.sync
- `e3cfbb3ac` evaluate on pre-v1 LLM state getters
- `9fc6b24fd` decode legacy string-content LLM replays

Redo both fixes by updating the tests/fixtures to the new world instead:

1. **Old-format replay goldens** — `core/integration/llmtest/{api-limit,
   hello-world,allow-llm}.golden` are pre-content-block recordings (`"content"`
   is a string; new shape is `[]*LLMContentBlock`), so `getReplay`
   (core/llm.go ~566) fails to unmarshal. Re-record them in the new format with
   `-update` (needs live provider credentials) and keep the strict decoder.
2. **TestAllowLLM / sync semantics** — the branch makes `LLM.sync` a pure
   ID-return (no evaluation), so module clients that relied on `sync` to force
   the loop never evaluate, and allow-llm enforcement never fires. Update the
   TEST (and whichever fixture module TestAllowLLM drives) to force evaluation
   the way the new API intends (e.g. `loop` / `lastReply` / whatever
   `internal/cmd/dagger` uses), rather than making sync evaluate again. While
   in there, confirm the allow-llm gate actually fires under the new flow — the
   enforcement point (`llm.allowed()`) is unchanged from main; it just needs an
   evaluating call to reach it. NOTE the fixture's vendored client must stay
   main-identical (§1); if the fixture would need new-API bindings it can't
   have, restructure the test to drive evaluation from the CLI/shell side.

Distinction that stays: the gating commit `75dd6f610` restores
`Query.llm(maxAPICalls:)` for pre-v1.0.0 **views** with a behavioral fallback.
That is NOT optional compat — removing an arg from the base view fails
`TestBaseSchemaAllowlist` outright. Leave it.

## 4. Remaining work for C (in order)

1. Rebase out the three dropped commits (§3), redo the two fixes as test/fixture
   updates.
2. Run the missing regens at C's head (an agent had this staged but never ran):
   `dagger generate -y docs golang php-sdk markdown-lint`
   - docs:references → docs/docs-graphql/schema.graphqls must GAIN the
     content-block surface (LLMMessage/LLMContentBlock, LLM.loop/messages) and
     LOSE `maxAPICalls` on Query.llm; golang → docs/current_docs/reference
     (committed cli/index.mdx predates `dagger session`); php-sdk → PHP client +
     doctum static docs; markdown-lint → whitespace fixes in
     internal/cmd/dagger/llm_branch_summary.md (verify whitespace-only; it's a
     go:embed'ed prompt template — if the fixer rewrites content, revert and add
     the file to .markdownlintignore instead).
   - Do NOT run the `go` generator (vendored clients, §1). If a generator
     dirties `**/internal/dagger/dagger.gen.go`, revert those files.
3. Local canaries before pushing (cheap, catch CI failures early):
   - `go build ./... && go vet ./core/...`
   - `go test ./core/schema -run 'TestBaseSchemaAllowlist|TestCoreModTypeDefs' -count=1`
   - `go test ./cmd/codegen/... -count=1`
   - DCO scan: `for c in $(git rev-list <base>..HEAD); do git log -1 --format='%B' $c | grep -q Signed-off-by || echo $c; done`
   - `gofmt -l core internal engine dagql | grep -v dagger.gen`
4. `git push --force-with-lease upstream llm-workspace-llm-foundation`, then
   `gh pr checks 13634 --watch` (~85 checks; expect 1 skipping).
5. Then D: rebase `llm-workspace-llm-binding` onto green C. Expect the same
   classes of work: allowlist gating for D's new API surface (@agent, object
   tools — and Env REMOVAL: deleting types/fields from the base view will need
   allowlist/view care!), regens, replay goldens. Then tip.

## 5. Operational knowledge (hard-won, read before debugging CI)

- **Traces**: always `dagger --x-release 1.0.0-beta.6 trace <id>` (plain
  `dagger trace` is the wrong CLI). Pipe to a file and grep; traces are huge.
- **Re-running one failed check**: `dagger --x-release 1.0.0-beta.6 cloud rerun
  --check <name>` reported "no Cloud checks found for the target commit" for
  every targeting combination tried (--commit head SHA, --commit merge-ref SHA,
  --pr, --org dagger) from this environment — possibly an auth/org context
  issue. Workaround used: `git commit --amend --no-edit` + force-push (SHA bump,
  same tree) re-runs everything; Cloud caching makes green checks cheap
  replays. The checks are commit STATUSES (not GitHub check runs), so `gh run
  rerun` and the check-runs rerequest API don't apply.
- **Generator cache lying**: several toolchains' generate cache keys omit the
  engine schema (e.g. go-sdk-dev's inputs are just sdk/go + cmd/codegen, and the
  engine-dev service is name-keyed "sdk"). A <1s generator run is a replay from
  whatever schema state ran first. Bust by leaving the previous output dirty in
  the worktree, or fresh dev engine (`./hack/dev` + `./hack/with-dev`). This
  cache-key bug deserves an upstream fix of its own.
- **`dagger generate` needs `-y`** in non-TTY contexts (else `huh: could not
  open a new TTY`). If export fails with "'dagql/idtui/' does not have a commit
  checked out", a prior generator session left a nested `.git`/`dagger.toml`
  under dagql/idtui — delete them and retry.
- **Latent main breakage** (independent of this stack): when Dagger Cloud's
  native-ci engine is next redeployed from current main,
  go:generate-dagger-runtimes will fail ON MAIN — committed vendored clients
  still carry the removed `Workspace.Update()` and current codegen emits new
  `<module>.gen.go` self-bindings. A coordinated regen will be needed then.
- Flake signatures seen: LLM replay divergence from `failed to emit telemetry
  ... context canceled` leaking into captured tool logs (telemetry-teardown
  race; consider hardening `captureLogs` to filter otel error-handler noise);
  elixir-sdk "cloning repo: engine is shutting down"; helm TestInstallK3S
  k3s-readiness timeout.
- Local `golangci-lint` is older than the repo's Go (1.26.1) and won't run;
  rely on the canaries above + CI.
- Two engines run locally: `dagger-engine-v0.21.7` (stock CLI's) and
  `dagger-engine.dev` (built from branch source via ./hack/with-dev). A dev
  engine built from C's HEAD may still be running.
