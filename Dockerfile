FROM golang:alpine AS builder

# Version comes from the git tag, passed in by the build (see Makefile). The
# build context has no reliable git available, so it is handed over explicitly.
ARG VERSION=unknown

WORKDIR /feeder

COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg \
  go build -ldflags "-X github.com/vsc-blockchain/pricefeeder/cmd.Version=${VERSION}" -o ./build/feeder .

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /feeder/build/feeder .
USER nonroot:nonroot
ENTRYPOINT ["/feeder"]
