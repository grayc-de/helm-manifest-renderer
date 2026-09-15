# `sourceType: url` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `chart-source.yaml` name a released manifest file so GitOps repos
vendor upstream component YAML into `generated-manifests/` instead of fetching
it from github.com on every Flux reconcile.

**Architecture:** A fourth `sourceType` in the existing renderer. Only the
*materialise* step of `internal/render/render.go` is Helm-specific; for `url`
it downloads the release asset into the temp dir instead of running `helm
template`, and every downstream stage (`postRender` cleanup, assembly,
kustomization emission, tidy) runs unchanged. The preset's existing
`chart-source.yaml` jsonata manager gains a branch so Renovate sees the pin;
the existing renderer pipeline re-renders on the PR branch.

**Tech Stack:** Go 1.x (stdlib `net/http`, `gopkg.in/yaml.v3`), Woodpecker CI,
Renovate jsonata custom manager, kustomize, Flux.

**Spec:** `docs/superpowers/specs/2026-09-14-url-source-type-design.md`

## Global Constraints

- **Never commit or merge to `main`.** Every repo: branch, push, open a PR with
  `tea pr create --login git.grayc.dev --head <branch> --base main --title … --description "$(cat body.md)"`. Do **not** merge PRs — hand off.
- **Conventional Commits**, Jira key in a trailing `(GDO-336)`.
- **Repos and branches** (all five branch off `origin/main`):
  - `helm-manifest-renderer` — `feat/url-source-type` (already exists, carries the spec commit)
  - `woodpecker-plugins` — `feat/gdo-336-renderer-bump`
  - `renovate-config` — `feat/gdo-336-url-source`
  - `continuous-delivery` — `docs/gdo-336-vendor-capi-manifests` (already exists, carries the roadmap line, open as PR 962)
  - `cluster-provisioning` — `feat/gdo-336-vendor-ccm`
- **Vendored pins do not change:** cluster-api `v1.12.7`, cluster-api-provider-hetzner `v1.1.1`, hcloud-cloud-controller-manager `v1.30.1`. Upgrades are out of scope.
- **Go module path:** `git.grayc.dev/grayc-devops/helm-manifest-renderer`.
- **Tests:** `make test` (= `go test -v ./...`). Table-driven, `reflect.DeepEqual`, `wantErr bool` — match `internal/config/config_test.go`.
- **Markdown:** markdownlint MD060 requires padded table pipes (`| --- | --- |`, not `|---|---|`).
- **Lint:** `just lint` runs a Docker image against `$PWD`. In a sandboxed session the daemon may only see the primary working directory; if every linter reports "skipped (no matching files)", run the specific linter binary directly and say so — do not report a skipped run as green.

---

## File Structure

**`helm-manifest-renderer`**

| File | Responsibility |
| --- | --- |
| `internal/config/config.go` | Add `URLSource`, `SourceConfig.URL`, the `url` validation branch, scope `releaseName`/`namespace` to non-`url` types |
| `internal/config/config_test.go` | Table cases for the valid config and each rejection |
| `internal/fetch/fetch.go` | **New.** Build the asset URL; download it; hard-fail on non-2xx; remove partial files on error |
| `internal/fetch/fetch_test.go` | **New.** `httptest` coverage of 200 / 404 / 500 / copy failure and URL building |
| `internal/render/render.go` | Route `url` to `fetchAsset` instead of the helm commands |
| `internal/render/render_test.go` | Unit test at the `materializeSource` seam |
| `README.md` | Document the fourth source type |
| `CLAUDE.md` | Note that the materialise stage is source-dependent |

**Other repos:** one file each — `helm-manifest-pr-sync/Dockerfile` +
`.woodpecker/helm-manifest-pr-sync.yaml`; `default.json`; the three
kustomization directories plus their two `.woodpecker` pins.

---

## Task 1: `url` source in the config schema

**Files:**

- Modify: `internal/config/config.go:42-46` (SourceConfig), `:100-129` (validation)
- Test: `internal/config/config_test.go`

**Interfaces:**

- Produces: `config.URLSource{Repo, Version, Asset string}` with yaml keys
  `repo`, `version`, `asset`; `config.SourceConfig.URL *URLSource` (yaml `url`).
  Task 2 and Task 3 consume `URLSource` by value.

- [ ] **Step 1: Write the failing tests**

Add these cases to the `tests` slice in `TestParseChartConfig`
(`internal/config/config_test.go`), before the closing `}` of the slice:

```go
		{
			name: "url source config",
			content: `sourceType: url

source:
  url:
    repo: kubernetes-sigs/cluster-api
    version: v1.12.7
    asset: cluster-api-components.yaml

postRender:
  splitYamlDocumentsInPaths:
    - cluster-api-components.yaml`,
			expected: ChartSourceConfig{
				SourceType: "url",
				Source: SourceConfig{
					URL: &URLSource{
						Repo:    "kubernetes-sigs/cluster-api",
						Version: "v1.12.7",
						Asset:   "cluster-api-components.yaml",
					},
				},
				HelmArgs: []string{},
				PostRender: PostRenderConfig{
					DeleteYamlPaths:           []string{},
					ExcludePaths:              []string{},
					SplitYamlDocumentsInPaths: []string{"cluster-api-components.yaml"},
					MovePaths:                 []MovePathRule{},
					NormalizeMetadata:         boolPtr(true),
				},
			},
		},
		{
			name: "url source requires repo",
			content: `sourceType: url

source:
  url:
    version: v1.12.7
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source requires version",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source requires asset",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0`,
			wantErr: true,
		},
		{
			name: "url source rejects inactive source sections",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml
  helm:
    repoUrl: https://example.com/charts
    name: app
    version: 1.2.3`,
			wantErr: true,
		},
		{
			name: "url source rejects releaseName and namespace",
			content: `sourceType: url
releaseName: cluster-api
namespace: capi-system

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml`,
			wantErr: true,
		},
		{
			name: "url source rejects helmArgs",
			content: `sourceType: url

source:
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml

helmArgs:
  - --no-hooks`,
			wantErr: true,
		},
		{
			name: "helm source rejects an inactive url section",
			content: `sourceType: helm
releaseName: test
namespace: default

source:
  helm:
    repoUrl: https://example.com/charts
    name: app
    version: 1.2.3
  url:
    repo: owner/repo
    version: v1.0.0
    asset: components.yaml`,
			wantErr: true,
		},
```

Note the expectation that `NormalizeMetadata` defaults to `boolPtr(true)` and
the three slices default to empty — copy whatever the existing "helm source
config" case expects if it differs; those defaults are set at the end of
`ParseChartConfig` and are unchanged by this task.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd helm-manifest-renderer
go test ./internal/config/ -run TestParseChartConfig -v
```

Expected: FAIL — `undefined: URLSource` (compile error).

- [ ] **Step 3: Add the type and the struct field**

In `internal/config/config.go`, after the `OCISource` struct:

```go
type URLSource struct {
	Repo    string `yaml:"repo"`
	Version string `yaml:"version"`
	Asset   string `yaml:"asset"`
}
```

and extend `SourceConfig`:

```go
type SourceConfig struct {
	Local *LocalSource `yaml:"local"`
	Helm  *HelmSource  `yaml:"helm"`
	OCI   *OCISource   `yaml:"oci"`
	URL   *URLSource   `yaml:"url"`
}
```

- [ ] **Step 4: Add the validation branch**

In `ParseChartConfig`'s `switch c.SourceType`, add before `default:`:

```go
	case "url":
		if c.Source.URL == nil {
			return ChartSourceConfig{}, fmt.Errorf("source.url must be set for sourceType=url")
		}
		if strings.TrimSpace(c.Source.URL.Repo) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.url.repo must be set")
		}
		if strings.TrimSpace(c.Source.URL.Version) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.url.version must be set")
		}
		if strings.TrimSpace(c.Source.URL.Asset) == "" {
			return ChartSourceConfig{}, fmt.Errorf("source.url.asset must be set")
		}
		if c.Source.Local != nil || c.Source.Helm != nil || c.Source.OCI != nil {
			return ChartSourceConfig{}, fmt.Errorf("source contains inactive sections for sourceType=url")
		}
		if strings.TrimSpace(c.ReleaseName) != "" || strings.TrimSpace(c.Namespace) != "" {
			return ChartSourceConfig{}, fmt.Errorf("releaseName and namespace are Helm concepts and must not be set for sourceType=url")
		}
		if len(c.HelmArgs) > 0 {
			return ChartSourceConfig{}, fmt.Errorf("helmArgs must not be set for sourceType=url")
		}
```

Change the `default:` message to:

```go
		return ChartSourceConfig{}, fmt.Errorf("sourceType must be one of: local, helm, oci, url")
```

Extend the three existing inactive-section checks to include the new field —
in the `local` branch `if c.Source.Helm != nil || c.Source.OCI != nil` becomes
`if c.Source.Helm != nil || c.Source.OCI != nil || c.Source.URL != nil`, and
the same addition in the `helm` and `oci` branches.

- [ ] **Step 5: Scope the global required-checks**

`releaseName` and `namespace` are currently required for every source type
(`internal/config/config.go:124-129`). Wrap both:

```go
	if c.SourceType != "url" {
		if strings.TrimSpace(c.ReleaseName) == "" {
			return ChartSourceConfig{}, fmt.Errorf("releaseName must be set")
		}
		if strings.TrimSpace(c.Namespace) == "" {
			return ChartSourceConfig{}, fmt.Errorf("namespace must be set")
		}
	}
```

- [ ] **Step 6: Run the tests to verify they pass**

```bash
go test ./internal/config/ -run TestParseChartConfig -v
```

Expected: PASS, all cases including the pre-existing ones.

- [ ] **Step 7: Document the source type in `README.md`**

In the `chart-source.yaml` reference section (around the `sourceType` bullet at
`README.md:212`), add `url` to the list of valid values and add a block after
the existing examples:

````markdown
### `sourceType: url`

Vendors a released manifest file instead of rendering a chart. No Helm is
involved: `releaseName`, `namespace`, `helmArgs` and a values file are rejected.

```yaml
sourceType: url

source:
  url:
    repo: kubernetes-sigs/cluster-api
    version: v1.12.7
    asset: cluster-api-components.yaml

postRender:
  splitYamlDocumentsInPaths:
    - cluster-api-components.yaml
```

The asset is downloaded from
`https://github.com/<repo>/releases/download/<version>/<asset>`. GitHub
releases are the only supported host. Any non-2xx response fails the render
with the status code — there is no fallback.
````

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go README.md
git commit -m "feat(config): accept sourceType url for vendored release manifests (GDO-336)"
```

---

## Task 2: the `internal/fetch` package

**Files:**

- Create: `internal/fetch/fetch.go`, `internal/fetch/fetch_test.go`

**Interfaces:**

- Consumes: `config.URLSource` from Task 1.
- Produces: `fetch.AssetURL(baseURL string, src config.URLSource) string`,
  `fetch.Fetch(src config.URLSource, destDir string) error`,
  `fetch.FetchFrom(baseURL string, src config.URLSource, destDir string) error`,
  `fetch.DefaultBaseURL = "https://github.com"`. Task 3 calls `fetch.Fetch`.

- [ ] **Step 1: Write the failing tests**

Create `internal/fetch/fetch_test.go`:

```go
package fetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

func testSource() config.URLSource {
	return config.URLSource{
		Repo:    "kubernetes-sigs/cluster-api",
		Version: "v1.12.7",
		Asset:   "cluster-api-components.yaml",
	}
}

func TestAssetURL(t *testing.T) {
	got := AssetURL("https://github.com", testSource())
	want := "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.12.7/cluster-api-components.yaml"
	if got != want {
		t.Errorf("AssetURL()\n got: %s\nwant: %s", got, want)
	}
}

func TestAssetURLTrimsTrailingSlash(t *testing.T) {
	got := AssetURL("https://github.com/", testSource())
	if strings.Contains(got, "com//") {
		t.Errorf("AssetURL() double slash: %s", got)
	}
}

func TestFetchFromWritesTheAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kubernetes-sigs/cluster-api/releases/download/v1.12.7/cluster-api-components.yaml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte("kind: Namespace\n"))
	}))
	defer server.Close()

	dest := t.TempDir()
	if err := FetchFrom(server.URL, testSource(), dest); err != nil {
		t.Fatalf("FetchFrom() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dest, "cluster-api-components.yaml"))
	if err != nil {
		t.Fatalf("reading fetched asset: %v", err)
	}
	if string(content) != "kind: Namespace\n" {
		t.Errorf("content = %q", string(content))
	}
}

func TestFetchFromFailsOnNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	err := FetchFrom(server.URL, testSource(), t.TempDir())
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a 404 error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error does not name the status code: %v", err)
	}
}

func TestFetchFromFailsOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	err := FetchFrom(server.URL, testSource(), t.TempDir())
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a 500 error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error does not name the status code: %v", err)
	}
}

func TestFetchFromWritesNoFileOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	dest := t.TempDir()
	_ = FetchFrom(server.URL, testSource(), dest)

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed fetch left %d file(s) behind", len(entries))
	}
}

func TestFetchFromLeavesNoPartialFileOnCopyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}))
	defer server.Close()

	dest := t.TempDir()
	err := FetchFrom(server.URL, testSource(), dest)
	if err == nil {
		t.Fatal("FetchFrom() error = nil, want a copy failure error")
	}
	if !strings.Contains(err.Error(), "write ") {
		t.Errorf("error should be a write error, got: %v", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a failed copy left %d file(s) behind", len(entries))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/fetch/ -v
```

Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/fetch/fetch.go`:

```go
// Package fetch downloads a released manifest file for sourceType=url.
//
// The one rule that matters: a non-2xx response is a hard error. kustomize's
// own remote-file loader answers an HTTP error by reinterpreting the URL as a
// git repository, which is what made a GitHub blip read as a malformed URL
// (GDO-336). Nothing here falls back to anything.
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/config"
)

// DefaultBaseURL is the only host releases are fetched from today.
const DefaultBaseURL = "https://github.com"

// DefaultTimeout bounds Client's requests. The default http.Client has no
// timeout at all, so a stalled connection would hang a CI render step
// forever with no output. The largest real asset (cluster-api components,
// 3.2 MiB) fetched in 11s during end-to-end testing, so two minutes is ample
// headroom while still guaranteeing a render eventually fails loudly instead
// of hanging.
const DefaultTimeout = 2 * time.Minute

// Client is the HTTP client used for all fetches. It is a package-level var,
// not a const, so tests can substitute a client with a much shorter timeout
// rather than waiting out DefaultTimeout.
var Client = &http.Client{Timeout: DefaultTimeout}

// AssetURL builds the release-asset URL for src.
func AssetURL(baseURL string, src config.URLSource) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/%s",
		strings.TrimSuffix(baseURL, "/"), src.Repo, src.Version, src.Asset)
}

// Fetch downloads src into destDir from DefaultBaseURL.
func Fetch(src config.URLSource, destDir string) error {
	return FetchFrom(DefaultBaseURL, src, destDir)
}

// FetchFrom downloads src into destDir from baseURL, creating destDir if
// needed. The file is named after the asset's base name.
func FetchFrom(baseURL string, src config.URLSource, destDir string) error {
	url := AssetURL(baseURL, src)

	resp, err := Client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("fetch %s: status %d (%s)",
			url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	target := filepath.Join(destDir, filepath.Base(src.Asset))
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(target)
		return fmt.Errorf("write %s: %w", target, err)
	}

	return nil
}
```

`Client.Get` follows redirects by default, which GitHub release assets
require, and is bounded by `DefaultTimeout` so a stalled connection fails
loudly instead of hanging a CI render step forever. On any failure after the
file is created, the partial file is removed before returning.

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./internal/fetch/ -v
```

Expected: PASS (8 tests, including a timeout test that substitutes a
short-timeout `Client` and restores it via `defer`).

- [ ] **Step 5: Commit**

```bash
git add internal/fetch/
git commit -m "feat(fetch): download release assets, failing hard on non-2xx (GDO-336)"
```

---

## Task 3: route `url` through the render pipeline

**Files:**

- Modify: `internal/render/render.go:96` (stage log), `:113-138` (the helm command block)
- Test: `internal/render/render_test.go`

**Interfaces:**

- Consumes: `config.URLSource` (Task 1), `fetch.Fetch` (Task 2).
- Produces: `materializeSource(cfg config.ChartSourceConfig, tempDir, valuesFile string, stageLog func(string)) error` — package-private; puts the unrendered source into `tempDir/<name>/`. Package variable `fetchAsset = fetch.Fetch` exists so tests can stub the download.

- [ ] **Step 1: Write the failing test**

Append to `internal/render/render_test.go`:

```go
func TestMaterializeSourceFetchesURLSources(t *testing.T) {
	original := fetchAsset
	defer func() { fetchAsset = original }()

	var gotSrc config.URLSource
	var gotDest string
	fetchAsset = func(src config.URLSource, destDir string) error {
		gotSrc = src
		gotDest = destDir
		return nil
	}

	cfg := config.ChartSourceConfig{
		SourceType: "url",
		Source: config.SourceConfig{
			URL: &config.URLSource{
				Repo:    "kubernetes-sigs/cluster-api",
				Version: "v1.12.7",
				Asset:   "cluster-api-components.yaml",
			},
		},
	}

	tempDir := t.TempDir()
	if err := materializeSource(cfg, tempDir, "", nil); err != nil {
		t.Fatalf("materializeSource() error = %v", err)
	}

	if gotSrc.Repo != "kubernetes-sigs/cluster-api" {
		t.Errorf("fetched the wrong source: %+v", gotSrc)
	}
	want := filepath.Join(tempDir, "cluster-api")
	if gotDest != want {
		t.Errorf("dest = %s, want %s", gotDest, want)
	}
}

func TestMaterializeSourceReportsFetchFailure(t *testing.T) {
	original := fetchAsset
	defer func() { fetchAsset = original }()

	fetchAsset = func(src config.URLSource, destDir string) error {
		return fmt.Errorf("status 404 (Not Found)")
	}

	cfg := config.ChartSourceConfig{
		SourceType: "url",
		Source: config.SourceConfig{
			URL: &config.URLSource{Repo: "o/r", Version: "v1", Asset: "a.yaml"},
		},
	}

	err := materializeSource(cfg, t.TempDir(), "", nil)
	if err == nil {
		t.Fatal("materializeSource() error = nil, want the fetch failure")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error lost the cause: %v", err)
	}
}
```

Add `fmt`, `path/filepath`, `strings` and the `config` package to the test
file's imports if they are not already there.

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/render/ -run TestMaterializeSource -v
```

Expected: FAIL — `undefined: materializeSource`, `undefined: fetchAsset`.

- [ ] **Step 3: Extract the materialise step**

In `internal/render/render.go`, add the package variable and the function
(imports: `path`, and the `fetch` package):

```go
// fetchAsset is a variable so tests can stub the download.
var fetchAsset = fetch.Fetch

// materializeSource puts the unrendered source into tempDir: helm output for
// chart sources, the downloaded release asset for sourceType=url. Downstream
// stages only see a directory of YAML and do not care which it was.
func materializeSource(cfg config.ChartSourceConfig, tempDir string, valuesFile string, stageLog func(string)) error {
	if cfg.SourceType == "url" {
		src := *cfg.Source.URL
		dest := filepath.Join(tempDir, path.Base(src.Repo))
		stage(stageLog, fmt.Sprintf("Fetch release asset: %s", fetch.AssetURL(fetch.DefaultBaseURL, src)))
		return fetchAsset(src, dest)
	}

	cmds, err := helm.GenerateHelmCommands(cfg, tempDir, valuesFile)
	if err != nil {
		return fmt.Errorf("failed to generate helm command: %v", err)
	}
	stage(stageLog, fmt.Sprintf("Render chart with %d helm command(s)", len(cmds)))

	for _, c := range cmds {
		// … the existing loop body, moved verbatim …
	}

	return nil
}
```

Move the existing `helm.GenerateHelmCommands` call and the `for _, c := range
cmds` loop out of `Render` and into this function **unchanged** — including the
`helm repo add/update` stdout suppression and the ignore-on-failure branch.
`Render` then calls:

```go
	if err := materializeSource(cfg, tempDir, valuesFile, opts.StageLog); err != nil {
		return err
	}
```

- [ ] **Step 4: Make the Helm-only steps conditional**

Two things in `Render` must not run for `url`:

```go
	valuesFile := ""
	if cfg.SourceType != "url" {
		valuesFile, err = resolveValuesFile(opts.ValuesFile)
		if err != nil {
			return err
		}
		if valuesFile == "" {
			info("No values file found. Rendering chart with the default values from the Helm chart.")
		} else {
			stage(opts.StageLog, fmt.Sprintf("Use values file: %s", valuesFile))
		}
	}
```

and the config stage log at `render.go:96`, which prints `releaseName=` and
`namespace=` that a `url` config does not have:

```go
	if cfg.SourceType == "url" {
		stage(opts.StageLog, fmt.Sprintf("Parsed config: sourceType=%s repo=%s version=%s asset=%s",
			cfg.SourceType, cfg.Source.URL.Repo, cfg.Source.URL.Version, cfg.Source.URL.Asset))
	} else {
		stage(opts.StageLog, fmt.Sprintf("Parsed config: sourceType=%s releaseName=%s namespace=%s",
			cfg.SourceType, cfg.ReleaseName, cfg.Namespace))
	}
```

- [ ] **Step 5: Run the full suite**

```bash
make test
```

Expected: PASS — the new render tests plus every pre-existing test. The
refactor must not change Helm behaviour; if an assembly or render test fails,
the loop body was altered while moving it.

- [ ] **Step 6: Update `CLAUDE.md`**

The stage list (`CLAUDE.md:55-56`, "Stages run in this fixed order; each
`internal/` package owns one stage") gains the fetch stage. Add after the
sentence describing the render stage:

```markdown
The materialise stage is source-dependent: `internal/helm` builds and runs the
`helm template` commands for `local`/`helm`/`oci`, `internal/fetch` downloads
the release asset for `url`. Everything after it — `internal/yamlcleaner`,
`internal/assembly` — is identical for both.
```

- [ ] **Step 7: Commit**

```bash
git add internal/render/ CLAUDE.md
git commit -m "feat(render): materialise url sources by fetching the release asset (GDO-336)"
```

---

## Task 4: release the renderer

**Files:** none — this task ships Tasks 1-3.

- [ ] **Step 1: Verify the whole suite and lint**

```bash
make test
just lint
```

Expected: tests PASS. If `just lint` reports every linter "skipped (no matching
files)", the Docker daemon cannot see this directory — run
`markdownlint-cli2 '**/*.md'` and `gofmt -l .` directly and report that
`just lint` could not run.

- [ ] **Step 2: Push and open the PR**

```bash
git push -u origin feat/url-source-type
tea pr create --login git.grayc.dev --head feat/url-source-type --base main \
  --title "feat: add sourceType url for vendored release manifests (GDO-336)" \
  --description "$(cat /tmp/pr-body.md)"
```

The description should state: what the new source type does, that it is
additive (no existing `chart-source.yaml` changes behaviour), and that
`releaseName`/`namespace` are now rejected for `url` only.

- [ ] **Step 3: Report CI and stop**

Report the pipeline result. **Do not merge.** A human merges.

- [ ] **Step 4: After the merge — tag the release**

`.woodpecker/release.yaml` triggers on `refs/tags/v*`. The last tag is
`v0.1.10`, so:

```bash
git fetch origin
git tag v0.1.11 origin/main
git push origin v0.1.11
```

Expected: the release pipeline publishes the tarball, the checksum, and the
versioned image to all four registries.

---

## Task 5: bump the renderer inside the pr-sync plugin

**Repo:** `woodpecker-plugins`, branch `feat/gdo-336-renderer-bump`

**Files:**

- Modify: `helm-manifest-pr-sync/Dockerfile:1`
- Modify: `.woodpecker/helm-manifest-pr-sync.yaml:53`

**Interfaces:**

- Consumes: renderer image `v0.1.11` from Task 4.
- Produces: plugin image `git.grayc.dev/grayc-devops/woodpecker-helm-manifest-pr-sync:v0.1.13`, which Tasks 6-8 pin.

- [ ] **Step 1: Branch**

```bash
cd woodpecker-plugins
git fetch origin
git checkout -b feat/gdo-336-renderer-bump origin/main
```

- [ ] **Step 2: Bump the embedded renderer**

`helm-manifest-pr-sync/Dockerfile:1` currently reads:

```dockerfile
ARG RENDERER_IMAGE=git.grayc.dev/grayc-devops/helm-manifest-renderer:v0.1.9
```

Change `v0.1.9` to `v0.1.11`.

- [ ] **Step 3: Bump the published plugin tag**

`.woodpecker/helm-manifest-pr-sync.yaml:53`, in the `push-helm-manifest-pr-sync`
step, currently reads:

```yaml
      name: git.grayc.dev/grayc-devops/woodpecker-helm-manifest-pr-sync:v0.1.12
```

Change `v0.1.12` to `v0.1.13`. This is the only place the version lives — the
push happens on merge to `main`, not on a tag.

- [ ] **Step 4: Run the plugin's own suite**

The suite runs as a Docker build stage (`target: test`) in CI. Locally:

```bash
docker build --target test -f helm-manifest-pr-sync/Dockerfile helm-manifest-pr-sync/
```

Expected: the build succeeds (the `RUN` executing the tests is the assertion).
If the daemon cannot see the directory, say so and rely on CI.

- [ ] **Step 5: Commit, push, PR**

```bash
git add helm-manifest-pr-sync/Dockerfile .woodpecker/helm-manifest-pr-sync.yaml
git commit -m "feat(helm-manifest-pr-sync): embed renderer v0.1.11 for sourceType url (GDO-336)"
git push -u origin feat/gdo-336-renderer-bump
tea pr create --login git.grayc.dev --head feat/gdo-336-renderer-bump --base main \
  --title "feat(helm-manifest-pr-sync): embed renderer v0.1.11 (GDO-336)" \
  --description "$(cat /tmp/pr-body.md)"
```

Report CI. Do not merge.

---

## Task 6: teach the Renovate preset the new source type

**Repo:** `renovate-config`, branch `feat/gdo-336-url-source`

**Files:**

- Modify: `default.json` — the `chart-source.yaml` jsonata custom manager
- Modify: `README.md` — the custom-manager table, if it lists them

**Interfaces:**

- Consumes: the `chart-source.yaml` schema from Task 1.
- Produces: `github-releases` dependencies for every `sourceType: url` file in the 18 repos that extend the preset.

- [ ] **Step 1: Branch and locate the manager**

```bash
cd renovate-config
git fetch origin
git checkout -b feat/gdo-336-url-source origin/main
```

The manager to change is the `customType: jsonata` entry whose
`managerFilePatterns` is `/(^|/)chart-source\.ya?ml$/`.

- [ ] **Step 2: Extend the jsonata expression**

The current `matchStrings[0]` is one expression selecting on `sourceType`. Add
a `url` arm to each ternary chain so it reads:

```text
{ "depName": (sourceType = "oci" ? source.oci.registry & "/" & source.oci.path & "/" & source.oci.name : sourceType = "helm" ? source.helm.name : sourceType = "url" ? source.url.repo : undefined), "registryUrl": (sourceType = "oci" ? source.oci.registry : sourceType = "helm" ? source.helm.repoUrl : undefined), "currentValue": (sourceType = "oci" ? source.oci.version : sourceType = "helm" ? source.helm.version : sourceType = "url" ? source.url.version : undefined), "datasource": (sourceType = "oci" ? "docker" : sourceType = "helm" ? "helm" : sourceType = "url" ? "github-releases" : undefined) }
```

`registryUrl` stays undefined for `url` — the `github-releases` datasource
resolves `owner/repo` against github.com on its own.

- [ ] **Step 3: Validate the config**

```bash
npx --yes --package renovate -- renovate-config-validator default.json
```

Expected: `INFO: Config validated successfully`. A malformed jsonata string
fails here, which is the only pre-merge check this repo has.

- [ ] **Step 4: Prove the expression against a real file**

jsonata errors are silent at runtime — a wrong path yields no dependency rather
than an error. Check the expression against the actual shape before merging:

```bash
npx --yes jsonata-cli -e '<the expression>' /path/to/chart-source.yaml
```

or paste the expression and this document into <https://try.jsonata.org>:

```json
{"sourceType": "url", "source": {"url": {"repo": "kubernetes-sigs/cluster-api", "version": "v1.12.7", "asset": "cluster-api-components.yaml"}}}
```

Expected output: `depName` `kubernetes-sigs/cluster-api`, `currentValue`
`v1.12.7`, `datasource` `github-releases`, `registryUrl` absent. Verify the
`helm` and `oci` shapes still resolve too — the same expression serves all
three.

- [ ] **Step 5: Commit, push, PR**

```bash
git add default.json README.md
git commit -m "feat: track sourceType url chart sources as github-releases (GDO-336)"
git push -u origin feat/gdo-336-url-source
tea pr create --login git.grayc.dev --head feat/gdo-336-url-source --base main \
  --title "feat: track sourceType url chart sources as github-releases (GDO-336)" \
  --description "$(cat /tmp/pr-body.md)"
```

The description must name the blast radius: this preset is extended by 18
repos and takes effect on their next Renovate run.

Report CI. Do not merge.

---

## Task 7: vendor `kustomize/cluster-api`

**Repo:** `continuous-delivery`, branch `docs/gdo-336-vendor-capi-manifests`
(already exists, already carries the README roadmap line as PR 962)

**Files:**

- Modify: `.woodpecker/lint.yaml:27`, `.woodpecker/helm-manifest-renderer.yaml:8` — both pins to `v0.1.13`
- Create: `kustomize/cluster-api/chart-source.yaml`
- Create: `kustomize/cluster-api/generated-manifests/` (renderer output)
- Modify: `kustomize/cluster-api/kustomization.yaml:11`

**Interfaces:**

- Consumes: plugin image `v0.1.13` from Task 5.

- [ ] **Step 1: Capture the current build**

This is the acceptance baseline and must be taken **before** anything changes:

```bash
cd continuous-delivery
git checkout docs/gdo-336-vendor-capi-manifests
kustomize build kustomize/cluster-api > /tmp/capi-before.yaml
wc -l /tmp/capi-before.yaml
```

Expected: a large file (tens of thousands of lines). If this fails, GitHub is
having another blip — retry before continuing, because without the baseline
there is nothing to verify against.

- [ ] **Step 2: Bump both plugin pins**

Both files must name the same version or the `--check-invariants` pin-agreement
check fails:

```bash
sed -i 's|woodpecker-helm-manifest-pr-sync:v0.1.12|woodpecker-helm-manifest-pr-sync:v0.1.13|' \
  .woodpecker/lint.yaml .woodpecker/helm-manifest-renderer.yaml
grep -rn "woodpecker-helm-manifest-pr-sync" .woodpecker/
```

Expected: two lines, both `v0.1.13`.

- [ ] **Step 3: Write the chart source**

Create `kustomize/cluster-api/chart-source.yaml`:

```yaml
sourceType: url

source:
  url:
    repo: kubernetes-sigs/cluster-api
    version: v1.12.7
    asset: cluster-api-components.yaml

postRender:
  splitYamlDocumentsInPaths:
    - cluster-api-components.yaml
```

The version stays `v1.12.7`. Upgrading is a separate story.

- [ ] **Step 4: Render**

```bash
just render-helm kustomize/cluster-api
```

The recipe reads the image out of `.woodpecker/helm-manifest-renderer.yaml`, so
Step 2 is what makes this use the new renderer. Expected: `generated-manifests/`
appears with the split objects and a `kustomization.yaml` listing them.

```bash
ls kustomize/cluster-api/generated-manifests | head
ls kustomize/cluster-api/generated-manifests | wc -l
```

- [ ] **Step 5: Point the kustomization at the vendored output**

In `kustomize/cluster-api/kustomization.yaml`, replace the remote URL under
`resources:`:

```yaml
resources:
  - generated-manifests
```

Leave the `labels:` block and every `patches:` entry exactly as they are —
including the GDO-233 probe timings.

- [ ] **Step 6: Prove the build is unchanged**

```bash
kustomize build kustomize/cluster-api > /tmp/capi-after.yaml
diff /tmp/capi-before.yaml /tmp/capi-after.yaml && echo "IDENTICAL"
```

Expected: `IDENTICAL`. A diff here means the vendored bytes render a different
object set — most likely a YAML value coerced by the split's re-encode. Do not
continue until it is empty; investigate the differing objects with
`diff /tmp/capi-before.yaml /tmp/capi-after.yaml | head -40`.

- [ ] **Step 7: Prove no remote resource is left**

The build being offline *is* the point of the story, and the invariant that
makes it offline is that no `resources:` entry is a URL:

```bash
grep -rn "https\?://" \
  kustomize/cluster-api/kustomization.yaml \
  kustomize/cluster-api/generated-manifests/kustomization.yaml \
  && echo "REMOTE RESOURCE STILL PRESENT" || echo "NO REMOTE RESOURCES"
```

Expected: `NO REMOTE RESOURCES`. If a sandbox-free environment is available, the
stronger check is a build with networking disabled
(`--network=none` in a container that has kustomize installed); treat it as a
bonus, not a gate, since the image and daemon access vary by environment.

- [ ] **Step 8: Lint**

```bash
just lint
```

Expected: green, `--check-invariants` included. `.yamllint.yaml` already
exempts `**/generated-manifests/**` by glob, so the vendored YAML is not
linted.

- [ ] **Step 9: Commit**

```bash
git add .woodpecker/lint.yaml .woodpecker/helm-manifest-renderer.yaml \
  kustomize/cluster-api/
git commit -m "feat(grayc-cicd): vendor the cluster-api components manifest (GDO-336)"
```

---

## Task 8: vendor `kustomize/cluster-api-provider-hetzner`

**Repo:** `continuous-delivery`, same branch as Task 7

**Files:**

- Create: `kustomize/cluster-api-provider-hetzner/chart-source.yaml`
- Create: `kustomize/cluster-api-provider-hetzner/generated-manifests/`
- Modify: `kustomize/cluster-api-provider-hetzner/kustomization.yaml:11`

- [ ] **Step 1: Capture the current build**

```bash
kustomize build kustomize/cluster-api-provider-hetzner > /tmp/caph-before.yaml
```

- [ ] **Step 2: Write the chart source**

Create `kustomize/cluster-api-provider-hetzner/chart-source.yaml`:

```yaml
sourceType: url

source:
  url:
    repo: syself/cluster-api-provider-hetzner
    version: v1.1.1
    asset: infrastructure-components.yaml

postRender:
  splitYamlDocumentsInPaths:
    - infrastructure-components.yaml
```

- [ ] **Step 3: Render**

```bash
just render-helm kustomize/cluster-api-provider-hetzner
```

- [ ] **Step 4: Point the kustomization at the vendored output**

Replace the remote URL under `resources:` with `- generated-manifests`, leaving
the `labels:` block and all `patches:` untouched.

- [ ] **Step 5: Prove the build is unchanged**

```bash
kustomize build kustomize/cluster-api-provider-hetzner > /tmp/caph-after.yaml
diff /tmp/caph-before.yaml /tmp/caph-after.yaml && echo "IDENTICAL"
```

Expected: `IDENTICAL`.

- [ ] **Step 6: Update the roadmap line**

`README.md` already carries the GDO-336 entry from PR 962. Reword its opening
from the future tense ("Vendor the CAPI and CAPH component manifests instead of
fetching them…") to describe what landed, and mark it `[x]` **only if** the
Jira story is Done — the checkbox follows Jira status, never the other way
round. If the story is not yet Done, leave `[ ]` and only correct the tense.

- [ ] **Step 7: Lint and commit**

```bash
just lint
git add kustomize/cluster-api-provider-hetzner/ README.md
git commit -m "feat(grayc-cicd): vendor the cluster-api-provider-hetzner manifest (GDO-336)"
```

- [ ] **Step 8: Push and update the PR**

```bash
git push
```

PR 962 already exists on this branch and picks the commits up. Retitle it from
`docs(readme): …` to `feat(grayc-cicd): vendor the CAPI and CAPH component
manifests (GDO-336)` and rewrite the description to cover the whole change —
the branch name keeps its `docs/` prefix, which is cosmetic.

Report CI. Do not merge.

- [ ] **Step 9: After the merge — verify the reconciliation**

```bash
kubectl -n flux-system get gitrepository continuous-delivery \
  -o jsonpath='{.status.artifact.revision}'      # wait for the merge SHA
flux reconcile kustomization capi-system
flux reconcile kustomization caph-system
kubectl -n flux-system get kustomization capi-system caph-system
just diff                                        # expect no controller changes
```

Expected: both `READY True` on the new revision, and `just diff` showing
nothing beyond the intended move.

---

## Task 9: vendor `hetzner-ccm` in cluster-provisioning

**Repo:** `cluster-provisioning`, branch `feat/gdo-336-vendor-ccm`

**Files:**

- Modify: `.woodpecker/lint.yaml:32`, `.woodpecker/helm-manifest-renderer.yaml` — both pins to `v0.1.13`
- Create: `kustomize/base/kube-system/hetzner-ccm/chart-source.yaml`
- Create: `kustomize/base/kube-system/hetzner-ccm/generated-manifests/`
- Modify: `kustomize/base/kube-system/hetzner-ccm/kustomization.yaml:8`

- [ ] **Step 1: Branch and capture the current build**

```bash
cd cluster-provisioning
git fetch origin
git checkout -b feat/gdo-336-vendor-ccm origin/main
kustomize build kustomize/base/kube-system/hetzner-ccm > /tmp/ccm-before.yaml
```

The base is referenced by per-cluster overlays; build the **base** for the
baseline, and one overlay as well if `grep -rn "hetzner-ccm" kustomize/cluster`
shows one, so the comparison covers what Flux actually reconciles.

- [ ] **Step 2: Bump both plugin pins to `v0.1.13`**

```bash
sed -i 's|woodpecker-helm-manifest-pr-sync:v0.1.12|woodpecker-helm-manifest-pr-sync:v0.1.13|' \
  .woodpecker/lint.yaml .woodpecker/helm-manifest-renderer.yaml
grep -rn "woodpecker-helm-manifest-pr-sync" .woodpecker/
```

- [ ] **Step 3: Write the chart source**

Create `kustomize/base/kube-system/hetzner-ccm/chart-source.yaml`:

```yaml
sourceType: url

source:
  url:
    repo: hetznercloud/hcloud-cloud-controller-manager
    version: v1.30.1
    asset: ccm-networks.yaml

postRender:
  splitYamlDocumentsInPaths:
    - ccm-networks.yaml
```

The pin stays `v1.30.1`. It is seven minor lines behind `v1.37.0`; that upgrade
is its own story, and bundling it here would destroy the no-op property this
task relies on.

- [ ] **Step 4: Render, repoint, and prove the build is unchanged**

```bash
just render-helm kustomize/base/kube-system/hetzner-ccm
```

Replace the remote URL under `resources:` with `- generated-manifests`, keeping
the `labels:` block and every `patches:` entry — including the
`node.cluster.x-k8s.io/uninitialized` toleration.

```bash
kustomize build kustomize/base/kube-system/hetzner-ccm > /tmp/ccm-after.yaml
diff /tmp/ccm-before.yaml /tmp/ccm-after.yaml && echo "IDENTICAL"
```

Expected: `IDENTICAL`. Repeat for any overlay captured in Step 1.

- [ ] **Step 5: Lint, commit, push, PR**

```bash
just lint
git add .woodpecker/ kustomize/base/kube-system/hetzner-ccm/
git commit -m "feat(kustomize): vendor the hcloud-ccm networks manifest (GDO-336)"
git push -u origin feat/gdo-336-vendor-ccm
tea pr create --login git.grayc.dev --head feat/gdo-336-vendor-ccm --base main \
  --title "feat(kustomize): vendor the hcloud-ccm networks manifest (GDO-336)" \
  --description "$(cat /tmp/pr-body.md)"
```

Report CI. Do not merge.

- [ ] **Step 6: After the merge — verify the reconciliation**

```bash
kubectl -n cluster-grayc-qas get kustomization hetzner-ccm
```

Expected: `READY True` on the new revision.

---

## Task 10: prove the chain end to end

**Files:** none — this verifies Tasks 1-9 in production.

- [ ] **Step 1: Confirm Renovate now sees the pins**

After the preset (Task 6) and at least one consumer have merged, wait for the
next Renovate run, then check the Dependency Dashboard issue in each repo for
`kubernetes-sigs/cluster-api`, `syself/cluster-api-provider-hetzner` and
`hetznercloud/hcloud-cloud-controller-manager`. Or force a run:

```bash
cd continuous-delivery && just renovate
```

Expected: an upgrade PR for each of the three pins — the first ever raised for
them.

- [ ] **Step 2: Confirm the renderer pipeline re-renders on that PR**

On one of those Renovate PRs, check that `.woodpecker/helm-manifest-renderer.yaml`
fired and pushed a `chore: render helm manifests` commit **onto the same
branch**, re-rendering `generated-manifests/` for the new version.

Expected: the PR contains both the `chart-source.yaml` version bump and the
re-rendered manifests, and CI is green on the render commit. This is the
end-to-end proof that the vendoring chain works; without it a bump would leave
the committed manifests stale.

- [ ] **Step 3: Do not merge the upgrade PRs**

They are real version upgrades — CAPI across two minor lines on a management
cluster with two live workload clusters, and the CCM across seven. Leave them
for the upgrade story. Report that they exist.

---

## Self-Review Notes

Spec coverage: §1 → Task 1; §2 → Task 2; §3 → Task 3; §4 (splitting) → the
`postRender` blocks in Tasks 7-9; §5 → Task 6; §6 → Task 5; §7 → Tasks 7-9;
Acceptance → Steps 6-7 of Task 7, Step 5 of Task 8, Step 4 of Task 9, and
Task 10; Sequencing → task order and the pin-bump steps.

Known gap, deliberate: no automated test covers the jsonata expression, because
`renovate-config` has no test harness — Task 6 Step 4 substitutes a manual
evaluation against a real document, which is the best available check.
