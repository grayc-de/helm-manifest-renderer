# Vendored Release Manifests — `sourceType: url` — Design

**Date:** 2026-09-14
**Branch:** `feat/url-source-type`
**Story:** [GDO-336](https://grayc.atlassian.net/browse/GDO-336)

## Goal

Let a `chart-source.yaml` name a **released manifest file** instead of a Helm
chart, so a GitOps repo can vendor upstream component YAML into
`generated-manifests/` and stop fetching it from github.com on every Flux
reconcile.

The renderer already owns the shape this needs — `chart-source.yaml` in, a
deterministic `generated-manifests/` directory plus `kustomization.yaml` out.
A released manifest is simply a source that needs no templating.

## Why

Three kustomizations across two repos list a GitHub release asset directly
under `resources:`:

| Repo | Path | Asset |
| --- | --- | --- |
| `continuous-delivery` | `kustomize/cluster-api/` | `kubernetes-sigs/cluster-api` `cluster-api-components.yaml` |
| `continuous-delivery` | `kustomize/cluster-api-provider-hetzner/` | `syself/cluster-api-provider-hetzner` `infrastructure-components.yaml` |
| `cluster-provisioning` | `kustomize/base/kube-system/hetzner-ccm/` | `hetznercloud/hcloud-cloud-controller-manager` `ccm-networks.yaml` |

Nothing caches them, so each is a live HTTP dependency on github.com at
reconcile time — on the cluster that is the CAPI management cluster for
`grayc-qas` and `grayc-play`. Two consequences, both observed:

**It breaks, and the error is misleading.** On 2026-09-14 `capi-system` went
`BuildFailed` at 14:35:12 and 14:36:15 UTC and self-healed on the 1 min
`retryInterval`. kustomize emits `URL is a git repository` **only** inside the
non-2xx branch of `api/internal/loader/fileloader.go` (v5.8.1, lines 318-330):
on an HTTP error it asks `git.NewRepoSpecFromURL(path)`, and if the URL also
parses as a repo spec it returns that string *instead of the status code*,
which is then never logged. It then falls back to treating the URL as a git
base, clones the upstream repo into `/tmp/kustomize-<random>`, and reports
`evalsymlink failure on '/tmp/kustomize-…/releases/download/…'`. The real
cause — upstream returned non-2xx — appears nowhere in the message.

**Renovate cannot see the pins, so they rot.** No Renovate manager reads a
release-asset URL out of `resources:` (the kustomize manager handles git-style
remote bases and OCI). Every change to these files in git history is a human
commit, never a `chore(deps)` one:

| Dependency | Pinned | Released | Latest | Released |
| --- | --- | --- | --- | --- |
| `cluster-api` | v1.12.7 | 2026-04-21 | v1.14.2 | 2026-09-08 |
| `cluster-api-provider-hetzner` | v1.1.1 | 2026-05-01 | v1.1.8 | 2026-08-03 |
| `hcloud-cloud-controller-manager` | v1.30.1 | 2026-02-20 | v1.37.0 | 2026-09-11 |

CAPI is two minor lines and ~5 months behind, the Hetzner CCM is **seven**
minor lines and ~7 months behind, and no PR has ever proposed any of it.
Vendoring is what makes these dependencies visible at all.

## Changes

### 1. `internal/config` — a fourth source variant

`SourceConfig` gains `URL *URLSource`, and `ChartSourceConfig` validation gains
a `url` case alongside `local` / `helm` / `oci`:

```yaml
sourceType: url

source:
  url:
    repo: kubernetes-sigs/cluster-api
    version: v1.12.7
    asset: cluster-api-components.yaml
```

`repo`, `version` and `asset` are all required and non-empty. The URL is built
as:

```text
https://github.com/<repo>/releases/download/<version>/<asset>
```

GitHub releases are the only supported host. `github-release` would describe
today's behaviour more exactly; `url` is chosen so that adding another host
later means adding fields rather than renaming the type and every
`chart-source.yaml` that uses it. Revisit only if a second host appears.

Strictness matches the existing style:

- `source contains inactive sections for sourceType=url` when `local`, `helm`
  or `oci` are also set — the existing cross-check, extended.
- `releaseName`, `namespace` and `helmArgs` are **rejected** for `sourceType:
  url`. They are Helm concepts; a released manifest carries its own namespaces
  and has no release. This requires moving the currently global `releaseName`
  / `namespace` required-checks (`internal/config/config.go:124-129`) into the
  three Helm-ish branches. `decoder.KnownFields(true)` already rejects unknown
  keys, so this closes the remaining silent-nonsense gap.
- `sourceType must be one of: local, helm, oci, url` at `config.go:121`.

Tests extend the existing table in `internal/config/config_test.go`: one valid
`url` config, plus one case per rejection above.

### 2. `internal/fetch` — new package

```go
func Fetch(src config.URLSource, destDir string) error
```

Downloads the asset to `destDir/<asset>`. Two rules worth stating because they
are the entire bug being fixed:

**Any non-2xx response is a hard error naming the status code and the URL.**
No fallback, no retry-into-a-different-mechanism, no interpretation. Redirects
are followed (GitHub release assets redirect to object storage). A test server
covers 200, 404 and 500.

**A failure during the transfer (non-2xx, connection drop, write error) removes
any partial file left behind.** Nothing is left in an inconsistent state.

### 3. `internal/render` — a materialise step

`Render` currently calls `helm.GenerateHelmCommands` and executes the result.
For `sourceType: url` it calls `fetch.Fetch` into `tempDir/<releaseRoot>/`
instead. Everything downstream is untouched: the `postRender` YAML walk,
`assembly.AssembleManifests`, `assembly.TidyFiles`.

`render.go:96`'s stage log drops `releaseName=`/`namespace=` for `url` and
prints the resolved URL instead.

No values file is resolved for `url`.

### 4. `postRender` — split the multi-document stream

`postRender.splitYamlDocumentsInPaths` already exists and is source-agnostic.
CAPI's components file is 3.0 MiB in one document stream; committed unsplit,
every future upgrade is an unreviewable diff, which throws away half the point
of vendoring. All three consumers therefore split.

`assembly.buildSplitFileName` (`internal/assembly/split.go:111`) gives stable
per-object names — `<name>.yaml` for a CRD, `<base>--<kind>--<name>.yaml`
otherwise, with a `--N` suffix on collision — so an upgrade shows as per-object
diffs and a removed object shows as a deleted file. No renderer change needed.

### 5. `renovate-config` — one jsonata branch

The `chart-source.yaml` custom manager in `default.json` gains a
`sourceType = "url"` branch emitting `depName: source.url.repo`,
`currentValue: source.url.version`, `datasource: "github-releases"`. No new
manager, no new file convention, and it reaches all 18 repos that extend the
preset.

### 6. `woodpecker-plugins/helm-manifest-pr-sync` — renderer bump

`Dockerfile:1` pins `ARG RENDERER_IMAGE=…/helm-manifest-renderer:v0.1.9` and
copies the binary out. It needs a release carrying the new renderer.
`--check-invariants` needs no change: its six conditions are about wiring
(pipeline present, preset order, trigger coverage, chart root, pin agreement),
none of which the new source type alters.

### 7. The three consumers

Each directory becomes the standard shape, identical to
`kustomize/flux-system/deployment/`:

```text
kustomize/cluster-api/
  chart-source.yaml
  generated-manifests/
    …split objects…
    kustomization.yaml
  kustomization.yaml      # resources: [generated-manifests], patches unchanged
```

The existing JSON6902 patches move across untouched — including the GDO-233
probe timings on the CAPI controllers.

**Vendored at the current pins, not the latest.** v1.12.7 / v1.1.1 / v1.30.1
stay exactly as they are, which makes each conversion provably a no-op (see
Acceptance). Upgrading CAPI across two minor lines on a live management cluster
is separate work with its own rollout.

## Acceptance

Per converted directory, the test that matters:

```sh
kustomize build <dir>            # before, from the URL      > /tmp/before.yaml
kustomize build <dir>            # after, from generated-manifests > /tmp/after.yaml
diff /tmp/before.yaml /tmp/after.yaml    # must be empty
```

An empty diff proves the vendored bytes render the identical object set — and
catches the one real risk in splitting, which re-encodes YAML through
`gopkg.in/yaml.v3` (a value coerced by re-encoding would show up here).

Then:

- `kustomize build` succeeds with **no network access** for all three.
- Renovate raises a PR for each pin after the preset lands (verified on the
  dependency dashboard, not assumed).
- That first Renovate PR gets its `generated-manifests/` re-rendered onto its
  own branch by the existing `helm-manifest-renderer` pipeline — which is the
  end-to-end proof of the whole chain.
- `just lint` green in both consuming repos, `--check-invariants` included.
- `capi-system`, `caph-system` and `hetzner-ccm` reconcile green, and the
  running controller deployments are unchanged (`just diff` shows nothing
  beyond the intended move).

## Sequencing

Five human-authored PRs; Renovate handles the image-pin bumps in between.

1. `helm-manifest-renderer` — `sourceType: url` + tests; release.
2. `woodpecker-plugins` — renderer bump in `helm-manifest-pr-sync`; release.
3. `renovate-config` — the jsonata branch. Independent of 1 and 2; can land any
   time, does nothing until a `url` chart-source exists.
4. `continuous-delivery` — both CAPI directories, plus the GDO-336 roadmap line
   (currently open as its own PR 962, to be folded in here).
5. `cluster-provisioning` — `hetzner-ccm`.

Steps 4 and 5 need the plugin pins in those repos (`.woodpecker/lint.yaml` and
`.woodpecker/helm-manifest-renderer.yaml`, both `v0.1.12` today) to have moved
to the release from step 2 — the two pins must agree or `--check-invariants`
fails.

## Out of scope

- **Upgrading any of the three pins** (CAPI v1.12.7 → v1.14.2, CAPH v1.1.1 →
  v1.1.8, CCM v1.30.1 → v1.37.0). Each is its own story with its own rollout —
  CAPI especially, on a management cluster with two live workload clusters
  downstream. Vendoring makes the upgrades *visible*; it does not perform
  them.
- Other hosts than GitHub releases.
- Auditing the rest of the estate for remote `resources:` URLs. The three known
  ones are covered; a sweep is worth its own task.
- **Integrity checking of the vendored bytes.** There is no checksum
  verification — a fetched asset is trusted as-is, the same trust boundary
  `git` already draws once it is committed.
- **Pointing the fetcher anywhere but GitHub.** `DefaultBaseURL` is fixed, so
  there is no way to redirect fetches to a mirror or to an offline fixture for
  testing without editing code.

## Risks

- **Repo size.** CAPI's 3.0 MiB lands in git, split across several hundred
  files. This matches what `generated-manifests/` already does elsewhere and is
  the cost of reviewable upgrades.
- **A large one-time diff** on each conversion PR. Unavoidable; the
  `kustomize build` diff is what makes it reviewable despite that.
- **The renderer grows a non-Helm responsibility.** Accepted deliberately: the
  alternative is a second parallel convention beside `chart-source.yaml`, which
  is exactly what GDO-243, GDO-257 and GDO-256 spent three stories removing.
