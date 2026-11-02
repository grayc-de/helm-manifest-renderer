FROM golang:1.26 AS builder

ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X git.grayc.dev/grayc-devops/helm-manifest-renderer/internal/buildinfo.Version=${VERSION}" \
  -o /out/helm-manifest-renderer ./cmd/helm-manifest-renderer

FROM debian:trixie-slim

# Helm 4 is a major release with its own changes to `helm template` output; stay
# on 3.x until the rendered manifests have been compared.
# renovate: datasource=github-releases packageName=helm/helm versioning=semver
ENV HELM_VERSION=v3.22.1

# hadolint ignore=DL3008
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl tar \
  && mkdir -p /work \
  && curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" -o /tmp/helm.tar.gz \
  && tar -xzf /tmp/helm.tar.gz -C /tmp \
  && install /tmp/linux-amd64/helm /usr/local/bin/helm \
  && rm -rf /var/lib/apt/lists/* /tmp/helm.tar.gz /tmp/linux-amd64

COPY --from=builder /out/helm-manifest-renderer /usr/local/bin/helm-manifest-renderer

ENV HOME=/tmp
WORKDIR /work
USER 1000:1000

ENTRYPOINT ["/usr/local/bin/helm-manifest-renderer"]
