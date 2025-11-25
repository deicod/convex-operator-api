# Convex Operator API backend

This repository hosts the API service that fronts the Convex operator. The current milestone focuses on tooling and a minimal HTTP skeleton.

## Development workflow (TDD-first)

- Add or update tests before implementing handlers or helpers.
- Run `make test` frequently while iterating.
- Keep the static analysis gates green: `make vet` and `make lint` must pass locally and in CI before merging.
- Prefer small, focused changes; keep the `/internal` packages covered by unit tests.

## Tooling

- Go version: defined in `go.mod` (requires Go 1.25 or newer).
- `make test` runs `go test ./...`.
- `make vet` runs `go vet ./...`.
- `make lint` runs `golangci-lint run ./...` (install instructions: https://golangci-lint.run/welcome/install/).

## Running locally

```bash
go run ./cmd/api
```

The server exposes `/healthz` and `/readyz` endpoints on `:8080` by default. Set `LISTEN_ADDRESS` to override the listen address.
