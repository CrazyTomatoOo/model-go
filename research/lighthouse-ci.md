# Lighthouse CI (@lhci/cli) for the console in GitHub Actions

Research ticket: [#27](https://github.com/CrazyTomatoOo/model-go/issues/27). Repo: `CrazyTomatoOo/model-go`. Target: `web/apps/console` (React 18 + antd 5 + Vite 7 SPA, routes `/login`, `/change-password`, `/console` shell).

Version basis: `@lhci/cli` **0.16.0** (npm dist-tag `latest`, published 2026-11-02; `engines: {node: ">=22.19.0"}`), so the runner's Node 24 qualifies. Docs cited below are the Lighthouse CI `main` branch at commit `3ead206a96a5ba714c386affe274576ccb8c09a9` (2026-11-03). GitHub Actions runner `ubuntu-latest` currently ships Chrome 156.0.8078.47 (image version 20261026.354.1, [runner-images Ubuntu2404 README](https://github.com/actions/runner-images/blob/ubuntu24/20261026.354.1/images/ubuntu/Ubuntu2404-Readme.md)), so `collect` needs no Chrome download. Every factual claim below links to the primary source that owns it.

## 1. Q1: static build artifacts or a served app? — static is enough for the gate page

### 1.1 `lhci collect` has two ways to reach the app

`lhci collect` "Runs Lighthouse n times and stores the LHRs in a local `.lighthouseci/` folder" ([configuration.md, `collect`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#collect)). The docs define the two modes:

- A built static directory, served by Lighthouse CI itself:

  > `staticDistDir` — The path to the directory where the project's productionized static assets are kept. Lighthouse CI uses this to spin up a static server on your behalf that will be used to load your site. Use this option when the project is a static website to be hosted locally that does not require a separate server. **DO NOT** use this option if `url` will point to an origin that isn't `localhost` or the project uses `startServerCommand` to start a separate server. ([configuration.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#staticdistdir))

- A separately started server: "If your site is not static and requires the use of a custom server, you'll need to teach Lighthouse CI how to start your server" via `startServerCommand` + `url` ([getting-started.md, "Modifications for Sites with a Custom Server"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/getting-started.md#modifications-for-sites-with-a-custom-server)).

The console is a pure static build: Vite's default `outDir` is `dist` ([Vite build options](https://vite.dev/config/build-options#outdir); `web/apps/console/vite.config.ts` sets no `outDir`, so the build lands in `web/apps/console/dist`), and production is served today as static files by nginx with an SPA fallback (`docker/console/nginx.conf`). So for anything that does not require the backend, **`staticDistDir` needs no `startServerCommand` and no extra serve step**: lhci's own static server is the server.

### 1.2 Client-side routing: set `isSinglePageApplication: true`

With `staticDistDir`, URLs are audited against the built-in server and their port is rewritten to the server's port ([configuration.md, `url`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#url)). A route URL like `/login` has no matching file in `dist`, so the static server must fall back to `index.html` for the SPA to render:

> `isSinglePageApplication` — Boolean that controls whether the static server started in `staticDistDir` should act like a single-page application that serves `index.html` instead of a 404 for unrecognized paths. ([configuration.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#issingleepageapplication))

The server source confirms the flag is what turns the fallback on: static routes are registered for `/` and `/app`, and only when `isSinglePageApplication` is set does the server answer unknown paths with `index.html` ([packages/cli/src/collect/fallback-server.js:21-28](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/collect/fallback-server.js#L21-L28)). Without it, `/login` 404s — a failure mode users actually hit upstream: [lighthouse-ci#800](https://github.com/GoogleChrome/lighthouse-ci/issues/800) "Impossible to run specific URLs in SPA with staticDistDir" (a `/test-page` URL 404s against the static server unless the SPA fallback is on). The flag itself was added in [PR #159](https://github.com/GoogleChrome/lighthouse-ci/pull/159). The console uses `BrowserRouter` (`web/apps/console/src/App.tsx`), so with the fallback on, requesting `/login` renders the login page client-side exactly as it does under the nginx `try_files $uri $uri/ /index.html` rule.

When `url` is given alongside `staticDistDir`, URL auto-discovery from HTML files on disk is disabled and only the listed URLs are audited ([configuration.md, `url`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#url)). Without `url`, lhci auto-discovers the first 5 HTML files ([CLI option `--maxAutodiscoverUrls`, default 5](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#collect)) — which for this app is just `dist/index.html`, i.e. `/`.

### 1.3 The login wall decides which pages are measurable

Routes in `web/apps/console/src/App.tsx`:

- `/login` — public.
- `/change-password` and `/` (console shell) — wrapped in `RequireAuth`.
- `*` — `<Navigate to="/login" replace />`.

An unauthenticated request to `/change-password` or `/` renders the shell, immediately redirects to `/login`, and Lighthouse scores the login page — so auditing the two authed routes duplicates `/login` rather than covering them. Three realistic options:

1. **Gate `/login` only (recommended for #25's acceptance).** One extra step in the existing `frontend` job; no backend, no credentials. This is the page every visitor lands on first, and the acceptance criterion is "Lighthouse ≥ 90" over the polished pages; the two authed pages are functional-covered by the Playwright E2E job that already exists.
2. **Cover all three pages against the real backend**, mirroring the existing `console-e2e` job (`.github/workflows/console-e2e.yml`): Postgres+Redis services, `admin bootstrap` accounts (initial password `Bootstrap@Passw0rd`, and the first login forces a password change), apiserver on `:8080`, then serve the production build (`pnpm build && pnpm preview` or a static server) and log in with a `puppeteerScript`. Cost: a second ~10-minute job and a password-change dance in the script (the bootstrap account cannot reach the shell without completing `/change-password`). `puppeteerScript` is the documented mechanism for "a page behind authentication": "you'll need to authorize the browser that Lighthouse CI will be using" and "you need to install `puppeteer` yourself" ([configuration.md, `puppeteerScript`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#puppeteerscript)).
3. **Measure only `/login` in CI now, run the authed pages manually (report-only) until the backend-in-CI story is wanted.** Cheapest path to the acceptance evidence; the E2E job is the template for option 2 later.

Note on language: the app defaults to Chinese when the browser's `navigator.language` starts with `zh` (detector `order: ['navigator', 'localStorage']`, key in `web/apps/console/src/i18n/index.ts`). lhci's Chrome defaults to English, so a zh-CN measurement needs a `puppeteerScript` that overrides `navigator.language` before load (the same script mechanism as option 2). The gate draft below audits the page as-is; whether the acceptance score applies to the zh-CN render is an open decision.

### 1.4 Chrome on the runner

Chrome resolution order is `chromePath` option → `CHROME_PATH` env → `puppeteer`'s executable → highest-priority `chrome-launcher` install ([configuration.md, `chromePath`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#chromepath)). The `ubuntu-latest` runner already ships Chrome 156 (section above), so nothing needs installing. Two drift notes: `ubuntu-latest` floats (Chrome version moves with runner image updates), and `@lhci/cli`'s own breaking-change policy lists "New minimum Node version" at roughly 1/year and "Change of the default value for a flag or option" at high impact ([version-policy.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/version-policy.md)) — pin `@lhci/cli@0.16.x` in the workflow so scores move only when the pin moves.

## 2. Q2: asserting performance ≥ 90 in `lighthouserc.json`

### 2.1 Assertion syntax

"Assertions are keyed by the Lighthouse audit ID and follow an eslint-style format of `level | [level, options]`… When no options are set, the default options of `{"aggregationMethod": "optimistic", "minScore": 1}` are used" ([configuration.md, `assertions`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#assertions)). Category scores are asserted with the `categories:<categoryId>` key:

```jsonc
"categories:performance": ["warn", {"minScore": 0.9}],
"categories:accessibility": ["error", {"minScore": 1}]
```

Levels: `off` (default), `warn` ("failure will not result in a non-zero exit code"), `error` ("failure will result in a non-zero exit code") ([same section, "Levels"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#levels)). Aggregation across the 3 runs is selectable per assertion via `aggregationMethod`: `median` / `optimistic` / `pessimistic` / `median-run` ([same section, "Aggregation Methods"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#aggregation-methods)). The presets are `lighthouse:all`, `lighthouse:recommended` ("every audit outside performance received a perfect score… and warns when metric values drop below a score of `90`"), and `lighthouse:no-pwa` ([configuration.md, `preset`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#preset)).

For the acceptance "Lighthouse ≥ 90" the meaningful assertion is the category score, not the recommended preset (the preset hard-fails many individual audits this app will not be in a position to satisfy, e.g. perfect scores on every non-performance audit). The draft (section 4) asserts:

- `categories:performance` — `["error", {"minScore": 0.9}]` (the hard gate, fails the job),
- `categories:accessibility`, `categories:best-practices`, `categories:seo` — `["warn", {"minScore": 0.9}]` (visible in logs, not blocking).

#25's wording "Lighthouse ≥ 90" does not say which category; performance is the ticket's emphasis ("性能 ≥90"), so the draft makes performance the only error-level gate. If the team wants all four categories to block, move the other three from `warn` to `error`. The default `aggregationMethod` is `optimistic`; if CI noise becomes a problem, set `"aggregationMethod": "median"` per assertion — that is a flakiness trade-off, not a correctness one.

### 2.2 Regression protection: job failure, surfaced as a GitHub status

When `lhci autorun` runs, `lhci assert` failure exits non-zero, which fails the workflow step, which fails the job, which blocks the PR if `frontend` is a required check — that is the regression gate. On top of that, `lhci upload` "sets a GitHub status check when the GitHub token is available and target is not `filesystem`" ([configuration.md, `upload`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#upload)). The status is a **commit status** (not a Check Run): the CLI POSTs to `repos/{owner}/{repo}/statuses/{sha}` ([packages/cli/src/upload/upload.js:172](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/upload/upload.js#L172)), and its state derives from the assertion results — `failedResults.length ? 'failure' : 'success'` with a description like "Failed 1 assertion(s)" / "Passed" ([upload.js, `postStatusToGitHub`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/upload/upload.js)). It shows up in the PR's checks list under the context label `lhci/lhci` by default (`githubStatusContextSuffix` — "by default `lhci` is used as the root of the status check label", [configuration.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#githubstatuscontextsuffix)).

So: "防回退是否用 GitHub Check" — the GitHub status is the presentation layer and a per-commit signal; the enforcement is the workflow job's exit code. Branch protection should require the `frontend` job (or the whole workflow) rather than the `lhci/lhci` status alone, because the status is written only when an upload target is configured and the token works, while the job failure always fires.

## 3. Q3: PR surface — check + report link, or artifact

### 3.1 The upload targets

`upload.target` choices ([configuration.md, `target`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#target)):

- `lhci` (default) — requires a Lighthouse CI server (own infrastructure: "Node v22 LTS" + a database; "LHCI really needs to own the entire origin", [server.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/server.md)) plus a per-project build token. Heaviest option; not needed for this ticket.
- `temporary-public-storage` — zero infrastructure, but: "reports being available to anyone on the internet with the link", "automatically deleted after a few days", stored on GCP Cloud Storage, and you must "read and agree to the [full terms of service and privacy policy](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/services-disclaimer.md#temporary-public-storage)" ([configuration.md, `target`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#target)). It gives the PR check a direct, clickable report link.
- `filesystem` — "You don't need detailed status checks in GitHub" ([configuration.md, `target`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#target)); raw LHRs land in `.lighthouseci/` and you attach them to the PR via `actions/upload-artifact` (the repo already does exactly this for Playwright reports in `console-e2e.yml`, retention 7 days).

Token methods for the status check: the official GitHub App (env `LHCI_GITHUB_APP_TOKEN`, "Recommended" in [getting-started.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/getting-started.md#github-status-checks)) or a personal access token with `repo:status` scope (env `LHCI_GITHUB_TOKEN`, [getting-started.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/getting-started.md#github-status-checks)). The lhci-recommended Actions recipe uses the App token as a secret:

```yaml
      - run: |
          npm install -g @lhci/cli@0.16.x
          lhci autorun
        env:
          LHCI_GITHUB_APP_TOKEN: ${{ secrets.LHCI_GITHUB_APP_TOKEN }}
```

([getting-started.md, "Additional configuration for GitHub Actions as CI Provider"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/getting-started.md#additional-configuration-for-github-actions-as-ci-provider))

### 3.2 Recommendation: `temporary-public-storage` + the automatic token — with an artifact fallback

For "最小 workflow 复杂度" the honest two-step scale is:

1. **Status check with report link (needs one consent + one permission line):** `target: temporary-public-storage` and `LHCI_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` in the step env. This uses GitHub's automatic token (no secret to provision); because the token's permissions default to read-only, the job must declare `permissions: { contents: read, statuses: write }` (GitHub Actions `permissions` reference, [workflow-syntax](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions#permissions); the endpoint lhci writes to is the statuses API, upload.js:172 above). The only human decision is accepting the temporary-public-storage terms; if that is vetoed, skip the upload target entirely.
2. **No consent version:** `target: filesystem` (or no upload at all) + an `actions/upload-artifact` step for `web/.lighthouseci`, retention 7 days. No token, no status, no public URL — the report is downloadable from the run's artifacts, and the gate is still enforced by the job failure.

One nuance on upload reliability: through `autorun`, an upload failure is **not** fatal by default — "running upload through `autorun` only warns to avoid noisy failures from temporary connectivity loss" (["failOnUploadFailure"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#failonuploadfailure)). So the status check may occasionally be missing on a run without failing the job; the `assert` step's exit code is the regression gate either way.

A self-hosted `lhci server` (persistent reports, controlled access) is available later if the team outgrows both; nothing about the draft config prevents switching targets later.

## 4. Draft `lighthouserc.json` (ready to paste)

lighthouserc files are auto-discovered in the working directory in the order `.lighthouserc.js` → `lighthouserc.js` → … → `lighthouserc.json` → … → `lighthouserc.yaml` ([configuration.md, "Configuration File"](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#configuration-file)). With the job's `defaults.run.working-directory: web` (section 5), place it at `web/lighthouserc.json` and reference the build output relative to `web/`.

```jsonc
{
  "ci": {
    "collect": {
      // built by `pnpm --filter @model-go/console build`; vite default outDir = dist
      "staticDistDir": "apps/console/dist",
      // /login is a BrowserRouter route with no file in dist: without this it 404s
      "isSinglePageApplication": true,
      // disabling autodiscovery: audit the public gate page only (see 1.3)
      "url": ["http://localhost/login"],
      "numberOfRuns": 3
    },
    "assert": {
      "assertions": {
        // the #25 acceptance gate: performance >= 90, fails the job
        "categories:performance": ["error", { "minScore": 0.9 }],
        // visible but non-blocking; promote to "error" if the team wants all four
        "categories:accessibility": ["warn", { "minScore": 0.9 }],
        "categories:best-practices": ["warn", { "minScore": 0.9 }],
        "categories:seo": ["warn", { "minScore": 0.9 }]
      }
    },
    "upload": {
      // report link in the PR status; delete if the storage terms are not accepted
      // and use the artifact fallback (section 5, variant B) instead
      "target": "temporary-public-storage"
    }
  }
}
```

Notes: `numberOfRuns: 3` is lhci's default and mitigates "fluctuations due to natural page variability" ([configuration.md, `numberOfRuns`](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md#numberofruns)); each run is a full page load, expect the step to take roughly 2-5 minutes total in CI (a small SPA runs fast; this is an estimate, unmeasured). If PR CI time matters more than noise resistance, `numberOfRuns: 1` is a legitimate start. The token never goes in this file — lhci reads `LHCI_GITHUB_TOKEN` (or `LHCI_GITHUB_APP_TOKEN`) from the environment (getting-started.md, section 3.1).

## 5. Workflow step draft (ready to paste)

The existing job ([.github/workflows/ci.yml](https://github.com/CrazyTomatoOo/model-go/blob/main/.github/workflows/ci.yml)) is:

```yaml
  frontend:
    name: web console (install + lint + test + build)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 12.10.1
      - uses: actions/setup-node@v4
        with:
          node-version: '24'
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: Install
        run: pnpm install --frozen-lockfile
      - name: Lint
        run: pnpm --filter @model-go/console lint
      - name: Test
        run: pnpm --filter @model-go/console test
      - name: Build
        run: pnpm --filter @model-go/console build
```

Variant A — status check + report link (needs the temporary-public-storage consent, section 3.2):

```yaml
  frontend:
    name: web console (install + lint + test + build + lighthouse)
    runs-on: ubuntu-latest
    # automatic token is read-only by default; the statuses API needs write
    permissions:
      contents: read
      statuses: write
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
        with:
          # lhci's own recipe audits and status-checks the PR head commit
          ref: ${{ github.event.pull_request.head.sha }}
      # ... pnpm/action-setup, setup-node, Install, Lint, Test, Build unchanged ...

      - name: Lighthouse CI
        run: pnpm dlx @lhci/cli@0.16.0 autorun
        env:
          LHCI_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Why `pnpm dlx`: the ticket forbids touching `package.json`, and `@lhci/cli` is a CI-only tool; `pnpm dlx` fetches the pinned version without adding a dependency or changing the lockfile, and `autorun` finds `lighthouserc.json` in the step's working directory (`web/`). The lhci docs' canonical recipe uses `npm install -g @lhci/cli@0.16.x` instead — equivalent effect, just a global install outside pnpm's store. `ref:` is needed so the status lands on the PR's head commit (the recipe's own choice); on `push` events the expression is empty and checkout falls back to the default branch.

Variant B — no consent, artifact-only (status check omitted):

```yaml
      # ... same steps through Build ...

      - name: Lighthouse CI (collect + assert)
        run: pnpm dlx @lhci/cli@0.16.0 autorun --upload.target=filesystem
        env:
          LHCI_GITHUB_TOKEN: ""   # or drop the env line; no status is posted for filesystem
```

(No `target: filesystem` line in the config file needed: `--upload.target=filesystem` overrides the draft's `temporary-public-storage` — lhci CLI flags are `--<section>.<key>=<value>` and override config-file values.)

```yaml
      - name: Upload Lighthouse reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: lighthouse-reports
          path: web/.lighthouseci
          retention-days: 7
```

(`.lighthouseci/` is created relative to the step's working directory — `web/` — mirroring the Playwright report artifact already in `.github/workflows/console-e2e.yml`.)

## 6. Open decisions

- **Category set for "≥90"**: draft gates `performance` only; #25's single "Lighthouse ≥ 90" could mean all categories. Decide before wiring branch protection.
- **zh-CN render**: the app renders Chinese for zh browsers; lhci's Chrome defaults to English. A `puppeteerScript` overriding `navigator.language` (and, for authed pages, logging in) is the mechanism; only worth it if the acceptance score must reflect the zh-CN layout.
- **Authed pages** (`/change-password`, `/console` shell): need the backend-in-CI pattern from `console-e2e.yml` plus a login `puppeteerScript` that also completes the forced password change. Recommend deferring; `/login` is the CI gate.
- **Storage consent**: accepting the temporary-public-storage terms is a repo-owner decision (section 3.2).
- **First-run verification**: the `permissions: statuses: write` behavior of the automatic token should be confirmed on the first CI run; the doc's mechanism evidence is the statuses endpoint in `upload.js:172`, but the token's effective scopes are enforced by GitHub at runtime.

## Sources

- Lighthouse CI docs at main (`3ead206a96a5ba714c386affe274576ccb8c09a9`): [configuration.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/configuration.md), [getting-started.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/getting-started.md), [server.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/server.md), [version-policy.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/version-policy.md), [services-disclaimer.md](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/docs/services-disclaimer.md)
- Lighthouse CI source: [packages/cli/src/collect/fallback-server.js](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/collect/fallback-server.js), [packages/cli/src/collect/collect.js](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/collect/collect.js), [packages/cli/src/upload/upload.js](https://github.com/GoogleChrome/lighthouse-ci/blob/3ead206a96a5ba714c386affe274576ccb8c09a9/packages/cli/src/upload/upload.js)
- npm registry: `@lhci/cli` 0.16.0 metadata (`https://registry.npmjs.org/@lhci/cli`; `dist-tags.latest`, `engines.node >=22.19.0`)
- GitHub Actions: [workflow-syntax `permissions`](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions#permissions); [automatic-token-authentication](https://docs.github.com/en/actions/security-guides/automatic-token-authentication)
- GitHub runner images: [Ubuntu2404-Readme.md](https://github.com/actions/runner-images/blob/ubuntu24/20261026.354.1/images/ubuntu/Ubuntu2404-Readme.md)
- Repo files: `.github/workflows/ci.yml`, `.github/workflows/console-e2e.yml`, `web/apps/console/src/App.tsx`, `web/apps/console/src/i18n/index.ts`, `web/apps/console/vite.config.ts`, `docker/console/nginx.conf`, `web/pnpm-lock.yaml`
- Vite: [build options `outDir`](https://vite.dev/config/build-options#outdir)

## Verification limits

- Nothing was executed: the worktree has no `pnpm install`, no build, no lhci run (the ticket forbids touching `package.json`/lockfile and the sandbox worktree stays clean). The draft config and workflow are therefore un-tested: `staticDistDir` path reasoning rests on Vite's default `outDir`, and the actual scores on `/login` are unmeasured.
- `ubuntu-latest` floats; the quoted Chrome 156 is the image version at writing time and will drift.
- The `permissions: statuses: write` behavior of the automatic token is asserted from the statuses-endpoint source plus GitHub's permissions model, not from a run (see Open decisions).
- The temporary-public-storage terms have not been accepted on the repo's behalf; that decision belongs to the repo owners.
