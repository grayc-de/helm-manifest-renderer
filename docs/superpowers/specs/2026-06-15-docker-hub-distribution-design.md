# Docker Hub Distribution — Design

**Date:** 2026-06-15
**Branch:** `feature/docker-hub`

## Goal

Publish the `helm-manifest-renderer` container image to Docker Hub under
`docker.io/graycgmbh/helm-manifest-renderer`, alongside the existing
publish targets (grayc Gitea, `ghcr.io/grayc-de`).

No application, Dockerfile, or build changes — the same artifact is pushed to
one additional registry.

## Changes

### 1. `.woodpecker/release.yaml` (versioned `v*` tags)

Add a 4th image step `image-dockerhub`, identical in shape to the existing
`image-ghcr` steps:

- `image: kokuwaio/buildctl:v0.28.0`
- `depends_on: [build]`
- `name: "docker.io/graycgmbh/helm-manifest-renderer:${CI_COMMIT_TAG}"`
- `build_args.VERSION: ${CI_COMMIT_TAG}`
- `auth` keyed by `docker.io`:
  - `username: {from_secret: DOCKERHUB_USERNAME}`
  - `password: {from_secret: DOCKERHUB_TOKEN}`

### 2. `.woodpecker/deploy.yaml` (`:latest` on push to `main`)

Add a second step `image-dockerhub` pushing
`docker.io/graycgmbh/helm-manifest-renderer:latest`, same auth as above.

### 3. `CLAUDE.md`

Update the `release.yaml` description (currently "three registries") to list
Docker Hub as a fourth target.

## Auth key detail

The `kokuwaio/buildctl` plugin keys the `auth` map by registry hostname, and
each existing step uses the same host string for both the image name prefix and
the auth key (`ghcr.io`/`ghcr.io`).
Docker Hub follows the same convention with `docker.io`. If the plugin turns out
to require `index.docker.io`, adjust the auth key (and only the auth key — the
image name stays `docker.io/...`).

## Prerequisites (out of scope, manual)

The Woodpecker secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` must exist at
org or repo level. These cannot be created from the pipeline definition.

## Out of scope

- Dockerfile / Go code changes.
- README registry sections (they document OCI *input* sources, not publish
  targets).
