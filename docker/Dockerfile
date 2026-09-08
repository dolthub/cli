# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26.0
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG DH_VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=${DH_VERSION}" -o /out/dh ./cmd/dh

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && \
    addgroup -g 1001 dh && adduser -D -u 1001 -G dh dh
COPY --from=builder /out/dh /usr/local/bin/dh
ARG DH_VERSION=dev
ARG VCS_REF
LABEL org.opencontainers.image.title="DoltHub CLI" \
      org.opencontainers.image.source="https://github.com/dolthub/cli" \
      org.opencontainers.image.version=$DH_VERSION \
      org.opencontainers.image.revision=$VCS_REF
ENV HOME=/home/dh
USER dh
WORKDIR /home/dh
ENTRYPOINT ["dh"]
CMD ["--help"]
