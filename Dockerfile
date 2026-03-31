FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/helm-manifest-renderer ./cmd/helm-manifest-renderer

FROM debian:bookworm-slim

ARG HELM_VERSION=v3.19.0

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
