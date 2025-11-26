FROM golang:1.25-bullseye AS build

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOTOOLCHAIN=auto

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out && go build -o /out/convex-operator-api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/convex-operator-api /usr/local/bin/convex-operator-api
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/convex-operator-api"]
