# Convex Operator API Backend

This service exposes an HTTP API for managing `ConvexInstance` resources through the Kubernetes API. It uses the Convex CRDs as the single source of truth and leaves reconciliation to the Convex operator.

## Deploy in cluster (Kustomize)

1. Build/push an image (or use a prebuilt tag). Override the image with `kustomize edit set image ghcr.io/your-org/convex-operator-api=<your-image>` or by setting the `images` block when invoking Kustomize.
2. (Optional) Create a bearer token secret if you want auth enabled:
   ```sh
   kubectl -n convex-operator-api create secret generic convex-operator-api-auth --from-literal=token=<token>
   ```
3. Apply the base manifests:
   ```sh
   kustomize build deploy | kubectl apply -f -
   ```
   This creates the ServiceAccount/ClusterRole/Deployment/Service in the `convex-operator-api` namespace.
   For Postgres jobs, also apply `deploy/postgres-job-rbac.yaml` to create a service account in the `postgres` namespace with access to the CNPG admin secret:
   ```sh
   kubectl apply -f deploy/postgres-job-rbac.yaml
   ```
4. Optionally expose the API via Gateway API:
   ```sh
   kubectl apply -f deploy/gateway-example.yaml
   ```
   The example binds to a GatewayClass named `nginx` and routes `convex-operator-api.internal.example.com` to the Service.

The Deployment defaults to `MODE=in-cluster` and listens on `:8080`. Set `ALLOWED_NAMESPACES` or other env/flags as needed.

## Run out-of-cluster

For local use or a central ops service:
```sh
go run ./cmd/api --mode=out-of-cluster --kubeconfig=$HOME/.kube/config --kubecontext=dev --listen=:8080
```
Environment equivalents are available (`MODE`, `KUBECONFIG`, `KUBECONTEXT`, `ALLOWED_NAMESPACES`, `CONVEX_API_AUTH_TOKEN`).

## Sample API calls

```sh
TOKEN=<token>
BASE=http://localhost:8080
AUTH="-H \"Authorization: Bearer $TOKEN\""

curl -i $BASE/healthz
curl -i $BASE/readyz
curl -s $AUTH "$BASE/api/v1/instances"
curl -s $AUTH -X POST "$BASE/api/v1/instances" \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"demo","namespace":"default"},"spec":{"environment":"dev","version":"1.0.0","backend":{"db":{"engine":"postgres"}},"networking":{"host":"example.com"}}}'
curl -s $AUTH "$BASE/api/v1/instances/default/demo/status"
curl -s $AUTH "$BASE/api/v1/instances/watch"
```

## Provisioning configuration (v2)

- Enabled by default: `ENABLE_S3_PROVISIONING=true`, `ENABLE_DB_PROVISIONING=true`.
- S3 (Rook-Ceph OBC): `S3_STORAGE_CLASS=rook-ceph-bucket`, `S3_SECRET_NAME_TEMPLATE=convex-{{name}}-s3`, `S3_ENDPOINT_SCHEME=https`, `S3_SECRET_KEYS=endpoint=endpoint,bucket=bucket,region=region,accessKeyId=accessKeyId,secretAccessKey=secretAccessKey`, `S3_PROVISION_TIMEOUT=2m`.
- Postgres (existing CNPG cluster): `CNPG_HOST=pg-rw.postgres.svc.cluster.local`, `CNPG_PORT=5432`, `CNPG_ADMIN_SECRET=pg-rw-superuser`, `CNPG_JOB_NAMESPACE=postgres`, `CNPG_JOB_SERVICE_ACCOUNT=convex-operator-api-db`, `CNPG_JOB_IMAGE=postgres:18-alpine`, `CNPG_SSLMODE=require`, `DB_SECRET_NAME_TEMPLATE=convex-{{name}}-db`, `CNPG_JOB_TIMEOUT=2m`.
- All values can be overridden via flags or environment variables; set the toggles to `false` to skip provisioning when needed.

## Runbook: provisioning diagnostics

- Bucket provisioning stuck (OBC not Bound):
  - Check OBC status: `kubectl -n <ns> get obc <instance> -o yaml`.
  - Inspect provisioner pod logs in `rook-ceph` namespace.
  - Ensure StorageClass `rook-ceph-bucket` exists and objectStore `s3` is healthy.
  - Retry by deleting the OBC; the API will recreate on next POST.
- DB job fails:
  - Inspect Job: `kubectl -n postgres describe job convex-<instance>-db-create`.
  - Check Pod logs: `kubectl -n postgres logs job/convex-<instance>-db-create`.
  - Verify admin secret `pg-rw-superuser` is readable by SA `convex-operator-api-db`.
  - If password drift occurred, delete the canonical DB secret in the instance namespace and rerun POST.
- Cleanup:
  - Delete instance: `curl -s $AUTH -X DELETE "$BASE/api/v1/instances/<ns>/<name>"`.
  - If bucket/DB linger, delete OBC and run drop job manually: `kubectl -n postgres create job --from=job/convex-<name>-db-drop manual-drop`.

## TDD and pipeline expectations

- Always keep `go test ./...`, `go vet ./...`, and `golangci-lint run ./...` green before merging.
- CI builds the container image to ensure it remains buildable.
- Add or update tests first when changing handlers, validation, or kube helpers.
