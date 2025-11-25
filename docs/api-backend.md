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

## TDD and pipeline expectations

- Always keep `go test ./...`, `go vet ./...`, and `golangci-lint run ./...` green before merging.
- CI builds the container image to ensure it remains buildable.
- Add or update tests first when changing handlers, validation, or kube helpers.
