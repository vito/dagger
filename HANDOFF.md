# llm-workspace stacked PRs — handoff after rebasing & greening-in-progress D

State as of 2026-07-14, end of the PR-D rebase session. Written for a fresh
session. The stack (all `dagger/dagger`, chained bottom-up):

| PR | branch | state |
|---|---|---|
| #13632 A — engine & TUI fixes | `llm-workspace-engine-tui-fixes` → main | **GREEN** |
| #13633 B — Workspace glob/search | `llm-workspace-file-apis` | **GREEN** |
| #13634 C — LLM foundation | `llm-workspace-llm-foundation` @ `5bb87a5d8f` | **GREEN** (30/30 Cloud statuses) |
| #13635 D — LLM ⇄ Workspace | `llm-workspace-llm-binding` @ `2ac7469458` | REBASED onto green C + two greening waves, MERGEABLE, **Cloud checks running** at session end — verify before touching |
| #13636 tip — dev tooling | `llm-workspace-dev-tooling` @ `4c4c366cf9` | rebased onto new D, pushed, MERGEABLE |

## 1. What landed on D this session

D was rebased `--onto llm-workspace-llm-foundation 9d603cb449` (old-C tip),
16 commits, with old-D's regen commit `7d36c9a290` dropped (it regenerated
vendored module clients, which violates the stay-main-identical policy).
Then 6 new commits of greening work (oldest first):

- `591b95a0ec fix(schema): gate the agent-era API surface to v1` — gates
  Agent/AgentGroup (class-level `InstallObject` + `View`, which also gates
  ID/load fields), `Function.withAgent`, `Workspace.agents`,
  `Query.currentNode` (via `FieldSpec.ViewFilter`), and
  `LLM.withWorkspace/workspace/contextTokens`. Also rewrites
  `core/schema/base_schema.json`: the base view is now main's golden + the
  intentional Env/Binding deletion + the `@agent` directive (directives have
  no view mechanism). Old-D's golden had wrongly allowlisted the content-block
  types, glob/search, and the loop(maxAPICalls) move — all view-gated during
  C's greening after D forked.
- `2491936a69 chore(sdk/go): regen dag client without Env` — restored
  `sdk/go/dag/dag.gen.go` from old-D's regen commit (drops Env/CurrentEnv
  wrappers, adds CurrentNode); the rest of sdk/go was already current.
- `aa36e5bd3d fix(core): emit changeset patches without rename entries` —
  real engine bug, found because D's docs regen renames binding.mdx →
  agent-group.mdx: `Changeset.asPatch` uses `git diff --no-prefix --no-index a b`,
  and git strips the a/ b/ mount dirs from ---/+++ but NOT from `rename
  from/to` lines, so any changeset containing a detected rename is
  unapplyable ("inconsistent old filename") and `dagger generate -y` fails at
  the apply step. Fixed with `--no-renames`. Host repro in the commit message
  path: two dirs + `git diff --no-prefix --no-index` + `git apply -p1`.
- `2985024364 chore: regen SDK clients, docs, and references` — D needs MORE
  generators than C did: `docs:references golang:generate-all php-sdk:api
  python-sdk:client-library typescript-sdk:client-library rust-sdk:apiclient
  elixir-sdk:client-library go-sdk:generate markdown-lint:fix` (the SDK
  client libraries carry the Env removal + withTools/withWorkspace/agents).
  Still NEVER `go:generate-dagger-runtimes`.
- `aff2b383d9 test(llm): pin agent fixtures to the v1 module view` — the
  testdata/agents fixtures were `engineVersion: v0.21.5`; the new gating
  hides withTools/currentNode from pre-v1 views. Bumped to `v1.0.0`
  (same convention as B's globber/searcher fixtures).
- `44740d43d3 test(llm): rewrite go-programmer for the object-tools world` —
  the fixture and hello-world.golden were Env-era (tag-addressed tools,
  `Save(name, value)` outputs). Rebuilt as a **Dang** module
  (`sdk: "dang"`, engineVersion v1.0.0, no vendored client): binds itself via
  `withTools(currentNode, except: ["run", "save", "drive"])`, tools take
  auto-injected `Workspace!` args, `write` returns the edited `Workspace`
  (rebind), `run` extracts `loop.workspace.file("main.go").contents`.
  toy-workspace dep deleted. hello-world.golden re-recorded live (procedure
  §3 below, unchanged from C's handoff) and eyeballed clean.
- `11c2787075 test(llm): pin remote LLM fixtures to llm-workspace branch` —
  see §2.

The tip (`llm-workspace-dev-tooling`, docs/skills only) rebased clean on top
and was force-pushed.

### MCP tag invariants from C are MOOT under D

C's `3eda3e8937` (tag aliasing fix) patched machinery D deletes wholesale:
there is no `IngestBy`/`WithObject`/`Snapshot`/`idByHash`/`typeCounts` in
D's mcp.go, no `Type#N` registry at all. Verified: the entire old-C→new-C
mcp.go delta sat inside the deleted code, so D's side won the conflict
byte-for-byte. The one invariant that survives in spirit: **capture bound
state at the top of `step()`, before `Tools(ctx)` runs** — D's shape is
`wsBefore/toolsBefore := llm.mcp.WorkspaceID()/BoundToolBindings()` hoisted
to the top, compared after CallBatch to emit withWorkspace/withTools
persistence selectors.

## 2. dagger-test-modules: the `llm-workspace` branch

The shared remote fixture `llm-dir-module-depender/llm-test-module` uses
`WithEnv(dag.Env().WithStringInput("CACHE_BUSTER", ...))` — builds fine on
main, **cannot build against D's engine** (Env is gone from every view;
bindings generate at load time). Fix: pushed branch **`llm-workspace`** to
`dagger/dagger-test-modules` (commit drops the buster — it's redundant since
`Query.llm` has `PerSessionInput` salting on both main and this stack), and
D's `llm_test.go` pins both fixture refs with `@llm-workspace`. Main's CI
keeps consuming the default branch. **After D merges, fold the branch into
dagger-test-modules main and unpin.**

## 3. Operational knowledge (new + carried forward)

New this session:

- **`TestAllowLLM/prompt_calls` (4 pty subtests) hang in THIS local
  environment on BOTH C and D** — A/B'd by rebuilding green-C's engine and
  running C's own tests: identical hang at `ExpectString("Allow LLM
  access?")`. The go-expect/script pty answers none of the TUI's terminal
  capability queries (OSC 11, DA1, CPR) and the prompt never renders. Not a
  D regression; Cloud CI runs them in an environment where they pass. Don't
  burn time on local repro — check CI.
- All other allow-llm subtests (allows, denials, env var, shell,
  noninteractive) pass locally against a dev engine with the pinned refs.
- `dagger generate` failing with "apply ... patch: inconsistent old
  filename" = a rename slipped into a changeset produced by an engine
  without `aa36e5bd3d`. Rebuild the dev engine before re-running generators.
- Dang fixture gotchas: local Dang types don't implement `Node` (no id) so
  they can't be passed to `withTools` — bind via `currentNode` + `except`.
  The JSON scalar is `Dagger.JSON` in Dang signatures (bare `JSON` is the
  stdlib codec module).
- Replay smoke-testing without credentials: hand-script a full history
  (USER prompt text must match exactly; matcher walks messages from index 0;
  tool-result text is not compared) and pass
  `--model=replay/$(base64 -w0 file)`. Proved the new fixture end-to-end
  before spending a live recording.

Carried forward from C's handoff (all still true): golden update procedure
(`env DAGGER_LLM_TEST_ENV=$PWD/.env ./hack/with-dev go test
./core/integration/ -run TestLLM -count=1 -update` on the HOST; engine-dev
test --update discards recordings), replay matching compares TEXT blocks
only (eyeball recordings), `--x-release 1.0.0-beta.6 trace <id>` for traces,
SHA-bump instead of `cloud rerun --check`, generator cache lying (<1s run =
stale replay; fresh `./hack/dev` busts), `dagger generate` needs `-y`,
`.env` holds op:// refs normally (a literal token existed locally this
session at `llm-workspace/.env` — never commit it), local golangci-lint too
old (build/vet/schema canaries + CI instead), `go vet` pre-existing findings
(core/services.go lostcancel, dagql/cache.go lostcancel — both on main),
latent main breakage re go:generate-dagger-runtimes vendored clients.

## 4. Remaining work

1. **Verify D's Cloud checks** on `11c2787075` (were running at session
   end; a monitor was armed in-session but dies with the session). Expect
   the known flake signatures: golang:test-all "Errored in 1m5s" (SHA bump
   re-runs), elixir-sdk "engine is shutting down", helm TestInstallK3S.
   Watch specifically: engine:testdev (runs the LLM/agents integration
   suites, including prompt_calls in CI's environment) and
   go:generate-dagger-runtimes-class checks.
2. Then #13636 tip checks (docs/skills only, low risk).
3. After D merges: fold dagger-test-modules `llm-workspace` branch into its
   main and drop the `@llm-workspace` pins in llm_test.go.
4. The `@agent` directive is visible in the base view (directives can't be
   view-gated) — flagged in the gating commit; acceptable, but worth a
   maintainer's eye on review.

## 5. Second greening wave (same session, after the first CI run)

The first full CI run on D failed 8 checks; all root causes found and fixed
(commits `4bbd0c2eb8..2ac7469458`), verified locally where possible:

- **docs:check** — hand-maintained docs still referenced the deleted Env
  reference page (coreTypes.js promoted-types list → invalid sidebar id, the
  types index table, and a core-types guidance section). All removed.
- **DCO** — one inherited old-D commit lacked signoff; fixed via reword
  rebase. Every commit in the PR range now carries Signed-off-by.
- **java-sdk:test / release-dry-run / test-modules java** — REAL SDK bug:
  `withTools(object: ID! @expectedType(Node))` generates a Node-typed Java
  parameter that Arguments.Builder can't marshal. Fix: id-providing
  interfaces are generated as `extends IDAble<ID>` (InterfaceVisitor), so the
  existing IDAble overload/marshalling covers interface values. Note the
  overload-ambiguity trap: adding a separate `add(String, Node)` overload
  instead makes every concrete-object call site ambiguous.
- **test-split:test-llm** — two causes: (1) the allow-llm gate matches repo
  URLs WITHOUT version pins, so `@llm-workspace`-pinned allow entries never
  matched — fixture refs stay pinned for `-m`, allow entries use unpinned
  variants; (2) TestAPILimit still drove the deleted `with-env` shell
  pipeline — now binds `container | from alpine` via with-tools and the
  golden was re-recorded live (model calls the container's envVariable tool).
- **test-split:test-workspaces / test-base checks family** — the
  hello-with-services and hello-with-checks fixtures called dag.CurrentEnv()
  (load-time codegen → compile error). Replaced with auto-injected
  `Workspace!` args reading Workspace.services/checks; functions renamed
  Workspace{Services,Checks}, up_test.go updated.
- **golang:lint-all / ci:bootstrap** — two lint rounds (receiver naming,
  error punctuation, stale + new gocyclo nolints, unparam drops incl.
  renderStep's always-empty prefix, dead functionRequiresArgs, unused
  DAGUI_DEBUG logger). `dagger check golang:lint-all` passes locally.
- **cmd/codegen filter tests** — dropped "Env" from ExtendableTypes
  (Binding was already gone) and regenerated the filter goldens with
  `-update`.

### KNOWN-OPEN feature gaps (not mechanical greening — need design/debugging)

1. **TestMCP (mcp_test.go)** — expects the retired ListMethods/SelectMethods
   meta-tool scheme and `--env-privileged` core exposure. D's `dagger mcp`
   serves `llm.__mcp`, which for a bare LLM exposes only builtins (ReadLogs,
   list_skills, read_skill) — verified by speaking MCP over stdio to a dev
   CLI. Needs a design call (bind workspace-module entrypoints via
   withTools? what does --env-privileged mean now?) and a test rewrite to
   the native tools/list flow.
2. **TestContextualWorkspaceCaching (3 subtests)** — in CI they fail fast
   (FUNCTION_EXECUTED where a cache hit is expected); locally they hang for
   10m each. Exercises loadWorkspaceArg cache identity / the workspace
   carry. Use the dagger-llm-workspace skill's A/B methodology.
3. **modules/evals, doug, dev, claude, evaluator** still use the Env API in
   their module code (vendored clients keep them compiling, but they are
   broken at runtime against a D engine). Not CI-blocking; needs a sweep
   when the eval harness is next used.
4. TestChecksFailFast locally reports no "context canceled" — warm-cache
   artifact (all checks replay in ~1s, nothing left to cancel); in CI it
   only ever failed on the fixture compile error. Judge by CI.
5. TestTelemetry/TestGolden (idtui) fails locally at the pushed head but
   passes in CI (golang:test-all was green) — local render-env difference;
   don't chase.
