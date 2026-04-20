# helm-manifest-renderer

`helm-manifest-renderer` renders a Helm chart into a deterministic manifest
directory using `helm template`.

## What It Does

The tool:

- reads a `chart-source.yaml` file
- renders a chart from a local path, a Helm repository, or an OCI registry
- applies optional post-processing cleanup
- assembles the rendered output into a single manifest directory
- generates a `kustomization.yaml` for the final output

By default, manifests are written to `generated-manifests`.

## Prerequisites

Local execution requires:

- Go 1.26 or newer to build the binary
- `helm` available in `PATH`

Depending on the chart source, you may also need:

- network access to the Helm repository
- network access and authentication for OCI registries

For containerized or CI execution, the image must also provide `helm` in `PATH`.
The provided container image runs as a non-root user by default.

## Install

To install the latest released binary locally:

```bash
tar -xzf helm-manifest-renderer_<version>_linux_amd64.tar.gz
sudo mv helm-manifest-renderer /usr/local/bin/
sudo chmod +x /usr/local/bin/helm-manifest-renderer
```

Verify the installation:

```bash
helm-manifest-renderer --help
```

To build and install from source instead:

```bash
make build
sudo mv helm-manifest-renderer /usr/local/bin/
sudo chmod +x /usr/local/bin/helm-manifest-renderer
```

## Build

```bash
make build
```

This produces the binary:

```bash
./helm-manifest-renderer
```

## CI And Releases

The repository is structured around three pipeline paths:

- pull requests run formatting, vetting, tests, and a build
- pushes to `main` build and publish the container image as `:latest`
- version tags such as `v0.1.0` create a release with a packaged binary and
  build the container image with the same version tag

The release artifact currently includes a Linux `amd64` tarball and a checksum
file.

## Container Usage

Build the image locally:

```bash
docker build -t helm-manifest-renderer:dev .
```

Run it against a mounted workload directory:

```bash
docker run --rm \
  -v "$PWD/example/metrics-server-helm:/work" \
  -w /work \
  helm-manifest-renderer:dev
```

## Run

Run the tool in a directory that contains a `chart-source.yaml`.

If a `values.yaml` or `values.yml` file is present, it is used automatically.
If no values file is present, the chart is rendered with the default values from
the Helm chart.

```bash
./helm-manifest-renderer
```

You can also pass an explicit target directory:

```bash
./helm-manifest-renderer path/to/workload
```

Available flags:

- `--config`: path to the chart source configuration file
- `--values-file`: optional path to the Helm values file
- `--output-dir`: output directory for generated manifests
- `--temp-dir`: temporary render directory

Example:

```bash
./helm-manifest-renderer \
  --config chart-source.yaml \
  --values-file values.yaml \
  --output-dir generated-manifests \
  .
```

If no values file is found and `--values-file` is not set, the renderer prints a
notice and continues with the chart defaults.

## Example

A few runnable examples are included under [example](/home/budi/_dev/grayc/grayc-devops/helm-manifest-renderer/example):

- [metrics-server-helm](/home/budi/_dev/grayc/grayc-devops/helm-manifest-renderer/example/metrics-server-helm)
  A minimal Helm repository example.
- [kube-prometheus-stack-helm](/home/budi/_dev/grayc/grayc-devops/helm-manifest-renderer/example/kube-prometheus-stack-helm)
  A Helm repository example that also demonstrates `postRender` cleanup options.
- [envoy-oci](/home/budi/_dev/grayc/grayc-devops/helm-manifest-renderer/example/envoy-oci)
  An OCI chart example.

The `metrics-server-helm` example can be rendered like this:

```bash
./helm-manifest-renderer \
  --config example/metrics-server-helm/chart-source.yaml \
  --values-file example/metrics-server-helm/values.yaml \
  example/metrics-server-helm
```

It also works without an explicit values file:

```bash
./helm-manifest-renderer \
  --config example/metrics-server-helm/chart-source.yaml \
  example/metrics-server-helm
```

The Helm-based examples require network access to the upstream chart repositories.

## Configuration

The tool expects a `chart-source.yaml` with this structure:

```yaml
sourceType: helm
releaseName: my-release
namespace: my-namespace

source:
  helm:
    repoUrl: https://example.com/charts
    name: my-chart
    version: 1.2.3
    repoName: example

helmArgs: []

postRender:
  deleteYamlPaths:
    - metadata.annotations."example.com/remove-me"
  excludePaths:
    - some/path/to/file.yaml
  splitYamlDocumentsInPaths:
    - crds/crds.yaml
  movePaths:
    - from: "*.yaml"
      to: grouped
  normalizeMetadata: true
```

### Top-Level Fields

`sourceType`

- selects how the chart is resolved
- supported values: `local`, `helm`, `oci`

`releaseName`

- Helm release name used for `helm template`

`namespace`

- namespace passed to `helm template`

`source`

- contains the source-specific chart definition
- exactly one section must be active, matching `sourceType`

`helmArgs`

- optional list of additional arguments passed to `helm template`
- use this for Helm-specific flags only

`postRender`

- optional cleanup and output-shaping rules applied after rendering

### `source.local`

```yaml
source:
  local:
    chartPath: ../../charts/my-chart
    version: 0.1.0
```

`chartPath`

- required
- relative or absolute path to the local chart directory

`version`

- optional metadata
- not used to resolve the chart

### `source.helm`

```yaml
source:
  helm:
    repoUrl: https://example.com/charts
    name: my-chart
    version: 1.2.3
    repoName: example
```

`repoUrl`

- required
- Helm repository URL

`name`

- required
- chart name inside the repository

`version`

- required
- chart version passed to Helm

`repoName`

- optional
- local alias used for `helm repo add`

### `source.oci`

```yaml
source:
  oci:
    registry: ghcr.io
    path: my-org/charts
    name: my-chart
    version: 1.2.3
```

`registry`

- required
- OCI registry host

`path`

- required
- OCI repository path below the registry

`name`

- required
- chart name inside the OCI path

`version`

- required
- chart version passed to Helm

### `postRender`

```yaml
postRender:
  deleteYamlPaths:
    - metadata.annotations."example.com/remove-me"
  excludePaths:
    - some/path/to/file.yaml
  movePaths:
    - from: "*.yaml"
      to: grouped
  normalizeMetadata: true
```

`deleteYamlPaths`

- optional list of YAML paths to remove from rendered documents

`excludePaths`

- optional list of files or directories to remove from the final output directory

`movePaths`

- optional ordered list of file move rules applied in `generated-manifests`
- `from` is a glob pattern relative to `generated-manifests`
- `to` is a destination directory relative to `generated-manifests`
- only file matches are moved; directories are ignored

`normalizeMetadata`

- optional boolean
- defaults to `true`
- removes common noisy metadata and applies targeted cleanup for CRDs and
  selected multi-line expressions

### Supported Source Types

`sourceType: local`

```yaml
sourceType: local
releaseName: my-chart
namespace: default

source:
  local:
    chartPath: ../../charts/my-chart
    version: 0.1.0
```

`sourceType: helm`

```yaml
sourceType: helm
releaseName: my-chart
namespace: default

source:
  helm:
    repoUrl: https://example.com/charts
    name: my-chart
    version: 1.2.3
    repoName: example
```

`sourceType: oci`

```yaml
sourceType: oci
releaseName: my-chart
namespace: default

source:
  oci:
    registry: ghcr.io
    path: my-org/charts
    name: my-chart
    version: 1.2.3
```

## Post-Render Behavior

`postRender.deleteYamlPaths`

- removes matching YAML paths from rendered documents

`postRender.excludePaths`

- removes matching files or directories from the assembled output directory

`postRender.movePaths`

- moves matching files inside the assembled output directory before excludes
- rules are applied in order
- patterns are standard non-recursive glob patterns relative to `generated-manifests`

`postRender.excludePaths`

- removes matching files or directories from the final assembled output layout
- this runs after `movePaths`, so excludes can target the final destination paths

`postRender.splitYamlDocumentsInPaths`

- splits multi-document YAML files in the assembled output directory
- each entry may point to a YAML file or a directory relative to `generated-manifests`
- generated files are named deterministically as:
  `metadata.name` for CRDs, original basename plus `kind` and `metadata.name`
  for other resources

`postRender.normalizeMetadata`

- enables metadata cleanup such as removing empty annotations and other noisy
  rendered metadata

The renderer also applies targeted cleanup for CRDs and Prometheus-style `expr`
blocks to keep the output closer to the current shell-based workflow.

## Development

Format, vet, test, and build:

```bash
make
```

Run tests only:

```bash
make test
```

Format code:

```bash
make fmt
```
