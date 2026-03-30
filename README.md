# helm-manifest-renderer

`helm-manifest-renderer` renders a Helm chart into a deterministic manifest directory using `helm template`.

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

## Build

```bash
make build
```

This produces the binary:

```bash
./helm-manifest-renderer
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
  normalizeMetadata: true
```

`deleteYamlPaths`

- optional list of YAML paths to remove from rendered documents

`excludePaths`

- optional list of files or directories to remove from the final output directory

`normalizeMetadata`

- optional boolean
- defaults to `true`
- removes common noisy metadata and applies targeted cleanup for CRDs and selected multi-line expressions

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
