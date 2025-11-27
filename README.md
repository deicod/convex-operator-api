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

## Provisioning defaults (S3 + Postgres)

The service now wires per-instance backing resources by default:

- S3 bucket via Rook-Ceph OBC: `S3_STORAGE_CLASS=rook-ceph-bucket`, secret name template `convex-{{name}}-s3`, endpoint scheme `https`, secret keys mapping `endpoint=endpoint,bucket=bucket,region=region,accessKeyId=accessKeyId,secretAccessKey=secretAccessKey`, timeout `S3_PROVISION_TIMEOUT=2m`, toggle `ENABLE_S3_PROVISIONING=true`.
 - Postgres on existing CNPG cluster: host `pg-rw.postgres.svc.cluster.local`, port `5432`, admin secret `pg-rw-superuser`, job namespace `postgres`, job service account `convex-operator-api-db`, job image `postgres:18-alpine`, SSL mode `require`, DB secret name template `convex-{{name}}-db`, job timeout `CNPG_JOB_TIMEOUT=2m`, toggle `ENABLE_DB_PROVISIONING=true`.
- Override via flags or env vars (see names above); leave toggles `false` to skip provisioning while testing.
