# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`helm-manifest-renderer` is a single-binary Go CLI that renders a Helm chart
into a deterministic, committable manifest directory via `helm template`. It is
the replacement for an older shell-based workflow; the output is shaped to match
the GitOps `generated-manifests/` convention used across the GrayC clusters
(see the workspace-level `CLAUDE.md` and `cluster-provisioning/CLAUDE.md`).

It shells out to `helm` (must be in `PATH`), so the CLI is useless without it.
The container image (`Dockerfile`) bundles a pinned Helm version.

## Commands

`make` targets are the canonical entry points — CI runs the same ones.

| Command                                   | Purpose                                                                     |
|-------------------------------------------|-----------------------------------------------------------------------------|
| `make`                                    | `fmt vet test build` — the full local check, matches the `go.yaml` pipeline |
| `make build`                              | build `./helm-manifest-renderer` with version ldflags                       |
| `make test`                               | `go test -v ./...`                                                          |
| `make fmt` / `make fmt-check`             | apply / verify `gofmt` (CI uses `fmt-check`)                                |
| `make vet`                                | `go vet ./...`                                                              |
| `make release-linux-amd64 VERSION=vX.Y.Z` | static linux/amd64 tarball + checksum in `dist/`                            |
| `make clean`                              | remove binary, `tmp/`, `generated-manifests/`, `dist/`                      |

Run a single test: `go test ./internal/assembly/ -run TestApplyMovePaths -v`.

`make` + the Go toolchain (Go 1.26) is the canonical workflow and what CI runs.
A thin `.justfile` exists for convenience (`just build` delegates to
`make build`; `just lint` runs yamllint/markdownlint), and `.tool-versions`
pins the toolchain for asdf — but the `make` targets remain the source of truth.

## Running it

The tool operates on a *target directory* containing a `chart-source.yaml`
(and optionally `values.yaml`/`values.yml`, picked up automatically). It
`chdir`s into that directory, so all config paths are interpreted relative to it.

```bash
./helm-manifest-renderer example/metrics-server-helm        # positional target dir
./helm-manifest-renderer --stage-log .                      # verbose stage trace
```

Runnable examples live in `example/` (`metrics-server-helm`, `envoy-oci`,
`kube-prometheus-stack-helm`); the Helm/OCI ones need network access to upstream
repos. `README.md` documents the full `chart-source.yaml` schema and every flag.

## Architecture

The whole pipeline is orchestrated by `render.RunRenderWithOptions`
(`internal/render/render.go`). Stages run in this fixed order; each `internal/`
package owns one stage:

1. **`config`** — parses & validates `chart-source.yaml`. Uses
   `decoder.KnownFields(true)` so unknown keys are hard errors. Exactly one of
   `source.{local,helm,oci}` must be present and must match `sourceType`;
   `normalizeMetadata` defaults to `true` (note: it is a `*bool` so "unset" is
   distinguishable from explicit `false`).
2. **`helm`** — `GenerateHelmCommands` turns the config into the argv for
   `helm repo add/update` (helm source only) + `helm template … --output-dir`.
   It returns commands; it does **not** execute them. All sources render with
   `--include-crds --skip-tests`.
3. **render execution** — `render.go` runs each command, discarding noisy
   `helm repo` output and tolerating `helm repo add/update` failures (warn &
   continue), then locates the single rendered chart dir under `tmp/`.
4. **`yamlcleaner`** — *structured* (yaml.v3 AST) cleanup applied per-file to
   the raw render: deletes configured `deleteYamlPaths`, strips Helm-noise
   labels/annotations, prunes empty `metadata`/`status`/etc. CRD files under
   `charts/crds/` are **skipped here** (`shouldSkipStructuredCleanup`).
5. **`assembly`** — `AssembleManifests` flattens the Helm `--output-dir` tree
   (templates, subchart `charts/*/templates`, CRDs) into `generated-manifests/`,
   then applies post-render shaping **in this order**: `splitYamlDocumentsInPaths`
   → `movePaths` → `excludePaths` → remove empty dirs → generate
   `kustomization.yaml`. Order matters: excludes run *after* moves so they can
   target final paths.
6. **`assembly.TidyFiles`** — a final *text/regex* cleanup pass over the
   assembled output. CRDs get `cleanupCRDYAML`, everything else `cleanupYAMLText`
   (which preserves multi-line `expr:` blocks, e.g. Prometheus rules).

Key thing to understand: **there are two distinct cleanup passes** — a
structured AST pass (`yamlcleaner`, step 4, skips CRDs) and a textual regex pass
(`assembly.TidyFiles`, step 6, special-cases CRDs). When changing cleanup
behavior, work out which pass should own it; they exist because some fixups are
only safely expressible as text.

`buildinfo.Version` is `"dev"` by default and overridden via `-ldflags` at
build/release time (`--version` prints it).

## CI (Woodpecker, `.woodpecker/`)

- `go.yaml` — PRs/pushes: `fmt-check`, `vet`, `test`, `build`.
- `lint.yaml` — containerised yamllint / markdownlint / hadolint (kokuwaio
  images), path-filtered.
- `deploy.yaml` — push to `main` (when code/Dockerfile changes) → build & push
  `:latest` image to `git.grayc.dev` and `docker.io/graycgmbh`.
- `release.yaml` — `v*` tags → release tarball + checksum via
  `woodpecker-simple-release`, and push the versioned image to **four**
  registries (grayc Gitea, `ghcr.io/grayc-de`, `snapdevops.azurecr.io`,
  `docker.io/graycgmbh`). Keep these registry steps in sync when changing the
  image build.
