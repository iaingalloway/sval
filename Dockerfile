FROM ghcr.io/iaingalloway/devcontainers/go:1.5.24-go1.26.0 AS build

USER root
ENV GOPATH=/go
ENV GOCACHE=/tmp/go-cache
WORKDIR /build
COPY . .

ARG VERSION=dev
RUN just version=${VERSION} publish

FROM gcr.io/distroless/static:nonroot
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY --from=build /build/bin/sval-${TARGETOS}-${TARGETARCH} /sval
ENTRYPOINT ["/sval"]
