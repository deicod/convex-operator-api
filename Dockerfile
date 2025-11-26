FROM debian:bullseye AS build

ARG GO_VERSION=1.25.4
ARG GO_VERSION_DIST=1.25.4-19122936812

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl git && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://github.com/actions/go-versions/releases/download/${GO_VERSION_DIST}/go-${GO_VERSION}-linux-x64.tar.gz \
    | tar -C /usr/local -xz

ENV PATH=/usr/local/go/bin:$PATH \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOTOOLCHAIN=local

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out && go build -o /out/convex-operator-api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/convex-operator-api /usr/local/bin/convex-operator-api
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/convex-operator-api"]
