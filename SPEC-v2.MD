# Convex Operator API Backend – Spec v2 (Rook-Ceph + CNPG provisioning)

## Objectives
- Extend the API to provision per-instance backing resources before creating a `ConvexInstance` CR:
  - Create an S3-compatible bucket via Rook-Ceph `ObjectBucketClaim` (OBC) using the `rook-ceph-bucket` StorageClass.
  - Create a Postgres user and database on the existing CNPG cluster (`postgres` namespace, cluster name `pg-rw`) via a one-off Job.
  - Publish connection credentials into Kubernetes Secrets and wire them into the `ConvexInstance` spec (`backend.db.secretRef`, `backend.s3.secretRef`).
- Tear down bucket and Postgres user/db on instance deletion (best-effort; do not block deletion on cleanup failures).
- Preserve existing API surface, adding the new provisioning lifecycle and configuration knobs.

## Cluster-default wiring (tailored)
- **S3**: StorageClass `rook-ceph-bucket` (provisioner `rook-ceph.ceph.rook.io/bucket`, objectStore `s3` in `rook-ceph` namespace). Bucket name == instance name (e.g., `dating-dev`). Region omitted (single-cluster default). Credentials stored as a Kubernetes Secret.
- **Postgres (CNPG)**: Cluster service `pg-rw` in namespace `postgres`. Use a one-off Job that runs `psql` against `pg-rw.postgres.svc.cluster.local:5432`. Admin secret default: `pg-rw-superuser` in namespace `postgres` (configurable). New user/db name == instance name (sanitized to Postgres rules). Password generated per instance. Connection secret stored in the instance namespace.
- **Namespaces**: Buckets/OBCs and S3/DB consumer secrets live in the `ConvexInstance` namespace. Postgres Jobs run in `postgres` namespace to access the CNPG admin secret by default.

## Scope
- Pre-create and wire S3 + Postgres for every `POST /api/v1/instances` request.
- Best-effort cleanup on `DELETE /api/v1/instances/{ns}/{name}` (and when delete is called after a failed create).
- Expose configuration flags/env vars to override defaults (StorageClass, CNPG host/port/cluster/admin secret, job namespace, secret names/keys, timeouts).
- Emit status/errors clearly to API clients; avoid partial success by ordering operations and performing compensating cleanup when feasible.

## Non-goals
- No direct reconciliation loops beyond request-scoped provisioning/cleanup (no background controller).
- No direct Rook/Ceph or CNPG API calls outside Kubernetes resources (no out-of-band cloud creds).
- No multi-cluster provisioning; one target cluster at a time.

## Lifecycle flows
- **Create (blocking before CR)**:
  1. Validate request + allowed namespace.
  2. Ensure S3 bucket via OBC in instance namespace; wait for `Bound` and capture the generated Secret.
  3. Create Postgres user/db via Job in `postgres` namespace; wait for `Complete`; fail if `Failed`.
  4. Materialize canonical Secrets in the instance namespace:
     - S3 secret with endpoint/access key/secret/bucket.
     - DB secret with URL and discrete fields.
  5. Inject secret refs into the `ConvexInstance` spec (`backend.db.secretRef`, `backend.db.urlKey`, `backend.s3.secretRef`, key names).
  6. Create the `ConvexInstance` CR.
  7. If any step fails before CR creation, clean up what was created (Job, OBC, temp secrets) on a best-effort basis.
- **Delete (best-effort after CR delete)**:
  1. Issue ConvexInstance deletion.
  2. Kick off cleanup:
     - Delete OBC (relies on StorageClass reclaimPolicy=Delete to drop bucket).
     - Launch Postgres cleanup Job (DROP DATABASE/ROLE IF EXISTS).
     - Delete the S3 and DB Secrets created by the API.
  3. Do not block the API response on cleanup; report cleanup errors in response payload/logs.

## S3 provisioning details
- Resource: `ObjectBucketClaim` in the instance namespace, `storageClassName: rook-ceph-bucket`, `bucketName: <instance>`.
- Wait condition: `status.phase == Bound`; timeout configurable.
- Source secret (from OBC): expect keys like `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `BUCKET_NAME`, `BUCKET_HOST`, `BUCKET_PORT`, `BUCKET_REGION` (key names configurable).
- Canonical secret (instance namespace, default name `convex-<instance>-s3`):
  - Keys: `endpoint`, `bucket`, `region`, `accessKeyId`, `secretAccessKey`.
  - `endpoint` built from host/port with `https://` unless port == 80 (configurable).
- ConvexInstance wiring: `spec.backend.s3.enabled=true`, `secretRef=<canonical secret>`, `endpointKey=endpoint`, `accessKeyIdKey=accessKeyId`, `secretAccessKeyKey=secretAccessKey`, `bucketKey=bucket`.

## Postgres provisioning details
- Admin connection:
  - Service: `pg-rw.postgres.svc.cluster.local` (override via config).
  - Admin secret: `pg-rw-superuser` in `postgres` namespace (override).
  - Admin secret expected keys: `username`, `password`, `host`, `port` (host/port fall back to config).
- Create Job (default namespace `postgres`) using `postgres:16-alpine` or CNPG-provided image (configurable):
  - Command runs `psql` with SQL: create role if not exists, set password, create database if not exists, grant privileges, ensure ownership.
  - Env vars carry admin creds, target user/db, password (password generated in API; not stored in Job logs).
  - Backoff limit 0 or 1; ttlSecondsAfterFinished set (configurable).
- Cleanup Job: same pattern with `DROP DATABASE IF EXISTS <db>; DROP ROLE IF EXISTS <user>;`.
- Canonical DB secret (instance namespace, default name `convex-<instance>-db`):
  - Keys: `url`, `username`, `password`, `database`, `host`, `port`, `sslmode` (default `require`, configurable).
  - URL format: `postgres://<user>:<pass>@<host>:<port>/<db>?sslmode=<sslmode>`.
- ConvexInstance wiring: `spec.backend.db.engine=postgres`, `secretRef=<canonical db secret>`, `urlKey=url`.

## Secrets and naming
- Names default to `convex-<instance>-s3` and `convex-<instance>-db` in the instance namespace.
- Bucket name and DB/user name derive from instance name; enforce lowercase, alphanumeric + dash, trimmed to provider limits.
- Password generation: random, strong, length configurable (default 32).
- Kubernetes Secrets (type `Opaque`) are sufficient; no external secret store required.

## Permissions model
- Default: delegate to Kubernetes resources with minimal RBAC; no direct cloud creds in the API binary.
  - API ServiceAccount needs RBAC to create/get/watch/delete OBCs in instance namespaces, create/watch Jobs in `postgres`, and create/delete Secrets in instance namespaces.
  - Postgres Jobs run with a Job-specific ServiceAccount in `postgres` namespace that can read the admin secret and create Pods; no cross-namespace secret reads.
- Alternative (not default): direct S3/CNPG API calls from the API process (reduces K8s RBAC) but requires embedding cloud/DB creds and managing TLS; we will not do this.

## Configuration (env/flags; defaults target the current cluster)
- `S3_STORAGE_CLASS` (default `rook-ceph-bucket`)
- `S3_SECRET_NAME_TEMPLATE` (default `convex-{{name}}-s3`)
- `S3_ENDPOINT_SCHEME` (default `https`)
- `S3_SECRET_KEYS` (mapping for endpoint/accessKeyId/secretAccessKey/bucket/region)
- `CNPG_HOST` (default `pg-rw.postgres.svc.cluster.local`)
- `CNPG_PORT` (default `5432`)
- `CNPG_ADMIN_SECRET` (default `pg-rw-superuser`)
- `CNPG_JOB_NAMESPACE` (default `postgres`)
- `CNPG_JOB_IMAGE` (default `postgres:18-alpine`)
- `CNPG_SSLMODE` (default `require`)
- `DB_SECRET_NAME_TEMPLATE` (default `convex-{{name}}-db`)
- Timeouts: `S3_PROVISION_TIMEOUT`, `CNPG_JOB_TIMEOUT`
- Feature toggles: `ENABLE_S3_PROVISIONING`, `ENABLE_DB_PROVISIONING` (default true) for phased rollout/testing.

## Error handling and idempotency
- Treat existing bucket/OBC or DB/user as non-fatal when matching desired state; fail on conflicting credentials/ownership.
- On create failure:
  - If OBC created, delete it.
  - If Postgres Job created, delete Job and skip DB secret creation; optionally run cleanup Job if DB might exist.
  - Delete any canonical secrets created by the API.
- On delete failure: log and return warning; do not requeue in background (future enhancement).

## Testing and observability
- Unit tests for provisioning orchestrator logic and request mutation (secretRef injection).
- Fake clients for OBC and Job creation paths; table-driven error cases.
- Optional e2e (kind/minikube) gated behind `make e2e` to validate OBC + Job flows.
- Logging: redact passwords; log bucket/db names and statuses only.

## Milestones and tasks
- Milestone 1: Design and config plumbing
  - [x] Add config fields/env parsing for S3 and CNPG defaults/overrides.
  - [x] Document default cluster assumptions in README/docs.
- Milestone 2: S3 provisioning
  - [ ] Implement OBC creator/waiter + canonical S3 secret writer.
  - [ ] Inject `backend.s3` references into `ConvexInstance` spec.
  - [ ] Unit tests for success, timeout, existing bucket, and cleanup on failure.
- Milestone 3: Postgres provisioning
  - [ ] Implement password generation and canonical DB secret writer.
  - [ ] Implement CNPG Job templates (create/drop) and waiter with timeouts.
  - [ ] Inject `backend.db` references into `ConvexInstance` spec.
  - [ ] Unit tests for create/drop, job failure, and cleanup on failure.
- Milestone 4: Lifecycle integration
  - [ ] Wire provisioning into `POST /api/v1/instances` before CR creation with compensating cleanup.
  - [ ] Wire best-effort cleanup into `DELETE /api/v1/instances/{ns}/{name}`.
  - [ ] Handle partial states (e.g., delete when bucket already gone).
- Milestone 5: RBAC and deploy
  - [ ] Update `deploy/` manifests with required RBAC (OBC, Jobs, Secrets across namespaces).
  - [ ] Add service accounts for Postgres Jobs and adjust Helm/Kustomize templates.
- Milestone 6: Docs and ops
  - [ ] Update docs with configuration, defaults, and example curl flows showing provisioned secrets.
  - [ ] Add runbook notes for manual cleanup and troubleshooting (OBC stuck, Job failed).
