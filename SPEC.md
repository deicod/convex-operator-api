# Convex Operator API Backend SPEC

## 1. Objectives

- Provide an HTTP API for managing `ConvexInstance` resources in Kubernetes.
- Default mode: run **in-cluster** alongside `github.com/deicod/convex-operator`.
- Optional mode: run **out-of-cluster**, using a kubeconfig.
- Use the Kubernetes API (CRDs + status) as the **only** source of truth; no extra database.
- Implement the backend in Go, using `net/http` with `http.ServeMux` for routing.
- Use `sigs.k8s.io/controller-runtime/pkg/client` (or client-go under the hood) to talk to Kubernetes.
- Reuse the existing Convex CRD types from the `convex-operator` repo.
- Establish a **TDD-friendly workflow** from the start: tests first, then implementation, with static analysis and linting wired in early.

## 2. Non-goals

- Do not introduce a separate database for Convex instances, history, or users.
- Do not implement reconciliation logic in this service (that stays in the operator).
- Do not directly edit underlying StatefulSets, Services, Gateways, etc.; only edit `ConvexInstance` CRs.
- Do not build a multi-cluster control plane beyond “point to one cluster via kubeconfig”.
- Do not ship a full RBAC UI model in v1 (beyond basic namespace scoping).

## 3. High-level architecture

- Binary name (example value): `convex-operator-api`.
- Entry point (example value): `cmd/api/main.go`.
- Suggested packages:
  - `internal/config`: configuration parsing and validation.
  - `internal/kube`: Kubernetes client initialization and helpers.
  - `internal/httpapi`: HTTP handlers, DTOs, request/response model.
  - `internal/auth`: authentication / authorization middleware (can be minimal in v1).

### 3.1 Routing with http.ServeMux

- Use `http.ServeMux` as the router:
  - `mux := http.NewServeMux()`
  - `mux.Handle("/healthz", healthHandler)`
  - `mux.Handle("/readyz", readyHandler)`
  - `mux.Handle("/api/v1/instances", instancesHandler)`
  - `mux.Handle("/api/v1/instances/", instanceDetailHandler)`
- Compose middleware by wrapping `http.Handler` around the mux for logging, auth, etc.

### 3.2 Kubernetes client

- Use controller-runtime `client.Client` over a `*rest.Config`.
- Create exactly one `client.Client` per process and inject it into the handler layer.
- Use the Convex API types from `github.com/deicod/convex-operator/api/v1alpha1` (update module path if needed).

## 4. Configuration and modes

Configuration sources: defaults < environment variables < CLI flags.

Example configuration fields:

- `ListenAddress` (string, example value: `:8080`).
- `Mode` (string): `in-cluster` (default) or `out-of-cluster`.
- `KubeconfigPath` (string, only used when `Mode=out-of-cluster`, example value: `/home/user/.kube/config`).
- `KubeContext` (string, optional; selects context within kubeconfig).
- `AllowedNamespaces` (string slice; comma-separated env/flag, empty meaning all namespaces for internal use).

### 4.1 In-cluster mode

- Use `rest.InClusterConfig()` or `ctrl.GetConfigOrDie()` to obtain configuration.
- The process runs as a Deployment in the cluster with a ServiceAccount.
- RBAC must grant access to the needed CRDs and core resources.

### 4.2 Out-of-cluster mode

- Use `clientcmd.BuildConfigFromFlags("", KubeconfigPath)` and optional `KubeContext`.
- Same `client.Client` type; only initialization differs.
- Used for local development or for running the backend outside the cluster.

## 5. API surface (initial)

Base path (example value): `/api/v1`.

All responses are JSON. Errors include at least `code` and `message` fields.

### 5.1 ConvexInstance CRUD

- `GET /api/v1/instances`
  - Query parameters:
    - `namespace` (optional; default = all allowed namespaces).
  - Behavior:
    - Validate namespace against AllowedNamespaces (if configured).
    - List ConvexInstances with K8s client.
    - Return list with metadata, spec, and status.

- `GET /api/v1/instances/{namespace}/{name}`
  - Behavior:
    - Validate namespace against AllowedNamespaces.
    - Fetch ConvexInstance.
    - Return full spec + status or 404 if not found.

- `POST /api/v1/instances`
  - Request body:
    - `metadata.name` (string).
    - `metadata.namespace` (string).
    - `spec` (ConvexInstanceSpec, as defined in the operator).
  - Behavior:
    - Validate required fields.
    - Validate `environment` (example allowed values: `dev`, `prod`).
    - Validate `maintenance.upgradeStrategy` (example allowed values: `inPlace`, `exportImport`).
    - Enforce AllowedNamespaces.
    - Create ConvexInstance via K8s client.
    - Return created object with HTTP 201.

- `PUT /api/v1/instances/{namespace}/{name}`
  - Full spec replacement:
    - Fetch existing CR.
    - Replace `spec` with request body.
    - Preserve `status` and important labels/annotations unless explicitly overridden by design.
    - Validate spec before update.
    - Update CR and return updated object.

- `PATCH /api/v1/instances/{namespace}/{name}`
  - Partial update:
    - Accept JSON body with fields mapping to `spec`.
    - Apply patch only to `spec`; do not allow `status` changes.
    - Validate resulting spec.
    - Update CR.

- `DELETE /api/v1/instances/{namespace}/{name}`
  - Behavior:
    - Enforce AllowedNamespaces.
    - Delete CR.
    - Return HTTP 202 or 204 when deletion has been initiated.

### 5.2 Actions

- `POST /api/v1/instances/{namespace}/{name}/upgrade`
  - Request body fields:
    - `targetVersion` (string).
    - `strategy` (string, example allowed values: `inPlace`, `exportImport`). 
  - Behavior:
    - Fetch CR.
    - Validate `strategy` value.
    - Update `spec.version` and optionally `spec.maintenance.upgradeStrategy`.
    - Save CR and return updated object or a concise status.

(Additional actions such as `pause`/`resume` can be added later when the CRD supports them.)

### 5.3 Status and diagnostics

- `GET /api/v1/instances/{namespace}/{name}/status`
  - Behavior:
    - Fetch ConvexInstance.
    - Return `status.phase`, `status.conditions`, and `status.endpoints`.
    - Optionally include derived fields such as the number of ready pods (by querying Pods).

- `GET /api/v1/instances/{namespace}/{name}/events`
  - Behavior:
    - Query Events in the namespace filtered by owner reference or labels associated with the ConvexInstance.
    - Return a trimmed event list with fields: type, reason, message, lastTimestamp.

### 5.4 Watch / streaming (optional)

- `GET /api/v1/instances/watch`
  - Recommendation: use Server-Sent Events (SSE) for simplicity.
  - Behavior:
    - Set up a watch or shared informer for ConvexInstances in AllowedNamespaces.
    - For each event (ADDED, MODIFIED, DELETED), write an SSE frame with:
      - event type.
      - instance summary payload (name, namespace, phase, version, etc.).
    - Handle client disconnects cleanly and avoid leaking watchers.

## 6. Security and RBAC

### 6.1 In-cluster RBAC

- ServiceAccount (example value: `convex-operator-api`) in a dedicated namespace.
- Role/ClusterRole must allow:
  - `get`, `list`, `watch`, `create`, `update`, `patch`, `delete` on `convexinstances.convex.icod.de`.
  - `get`, `list`, `watch` on `pods`, `events`, and optional `httproutes.gateway.networking.k8s.io` for diagnostics.
- RoleBinding/ClusterRoleBinding grants the permissions to the API ServiceAccount.

### 6.2 HTTP authentication and authorization

- Initial simple model:
  - Static bearer token (example value provided via env: `CONVEX_API_AUTH_TOKEN`).
  - Middleware checks Authorization header (`Bearer <token>`).
  - Requests without valid token receive HTTP 401.
- Authorization:
  - Enforce AllowedNamespaces for all operations.
  - Long-term design: map user identity to per-namespace permissions, but this can come later.

### 6.3 Hardening

- Set HTTP server timeouts:
  - Read timeout, write timeout, idle timeout.
- Size limits:
  - Limit request body size for safety.
- Logging:
  - Do not log sensitive fields (DB URLs, secrets).
  - Redact or truncate large payloads.

## 7. Deployment and operation

### 7.1 In-cluster deployment

- Build and push image (example value: `ghcr.io/your-org/convex-operator-api:<tag>`).
- Create Deployment:
  - 1–2 replicas.
  - Uses the API image.
  - Env vars and flags set for Mode=`in-cluster`.
- Create Service:
  - ClusterIP exposing the API port.
- Optionally create Gateway + HTTPRoute:
  - Route an internal host (example value: `convex-operator-api.internal.example.com`) to the Service.

### 7.2 Out-of-cluster deployment

- Same binary, run outside the cluster:
  - Pass `--mode=out-of-cluster` and `--kubeconfig=/path/to/kubeconfig`.
  - Optionally `--kubecontext`.
- Useful for local development or a central ops service.

### 7.3 Observability

- `/healthz`:
  - Returns 200 if the process is running.
- `/readyz`:
  - Returns 200 only when the Kubernetes client can talk to the cluster successfully.
- `/metrics` (optional):
  - Prometheus-format metrics (if you add a metrics library).
  - Count and latency per endpoint, error rates, etc.

## 8. Compatibility with convex-operator

- Import `ConvexInstance` types from `github.com/deicod/convex-operator` to avoid duplicating CRD structs.
- Do not write to status directly unless explicitly decided and the CRD status subresource allows it; normally the operator owns status updates.
- Treat the ConvexInstance CR as the declarative contract:
  - This API manipulates the CR spec.
  - The operator reconciles actual resources based on the spec.
  - The UI (or CLI using this API) reads status through the same CR.

---

## Milestones and Tasks

### Milestone 0 – Tooling, TDD workflow, and CI skeleton

Goal: Establish testing and static analysis as first-class citizens before feature work.

- [x] Initialize or update Go module layout for the API backend, ensuring it compiles and integrates cleanly with the existing `convex-operator` repo.
- [x] Add `go test ./...` as the primary test command and ensure it passes with an empty or trivial implementation.
- [x] Integrate `go vet`:
  - [x] Add a `make vet` target or equivalent script.
  - [x] Ensure `go vet ./...` runs cleanly on the existing codebase.
- [x] Integrate `golangci-lint` early:
  - [x] Add a configuration file (example value: `.golangci.yml`) with a reasonable set of linters.
  - [x] Add a `make lint` target that runs `golangci-lint run ./...`.
  - [x] Fix or explicitly suppress any initial lint findings before adding new code.
- [x] Wire basic CI (example values: GitHub Actions, Gitea CI) to run, at minimum:
  - [x] `go test ./...`
  - [x] `go vet ./...`
  - [x] `golangci-lint run ./...`
- [x] Document the TDD workflow for this service:
  - [x] Encourage writing or updating tests before implementing handlers and helpers.
  - [x] Make tests and linting mandatory gates for merges (protected branch rules if available).

### Milestone 1 – Skeleton, config, and Kubernetes client

Goal: A running HTTP server with `http.ServeMux`, config parsing, and a live Kubernetes client in both modes, implemented test-first.

- [x] Write tests for configuration loading:
  - [x] Verify precedence of defaults, env, and flags.
  - [x] Verify error behavior when `Mode=out-of-cluster` and kubeconfig is missing.
- [x] Implement `internal/config`:
  - [x] Define `Config` struct for listen address, mode, kubeconfig path, kube context, and allowed namespaces.
  - [x] Implement parsing from env and flags to match the tests.
- [x] Write tests for Kubernetes client initialization behavior (using stubs/fakes where appropriate):
  - [x] Ensure in-cluster mode selects `InClusterConfig` path.
  - [x] Ensure out-of-cluster mode uses `clientcmd` with the given kubeconfig path.
- [x] Implement `internal/kube` initialization according to tests:
  - [x] In `in-cluster` mode, use `InClusterConfig` / `ctrl.GetConfigOrDie()` and create a controller-runtime client.
  - [x] In `out-of-cluster` mode, use `clientcmd.BuildConfigFromFlags` with the configured kubeconfig path and optional context.
  - [x] Log which mode and context are being used at startup.
- [x] Write tests for basic HTTP routing:
  - [x] Ensure `/healthz` and `/readyz` are wired up and return expected status codes given mock dependencies.
- [x] Implement `cmd/api/main.go`:
  - [x] Initialize config and Kubernetes client.
  - [x] Initialize `http.ServeMux` and register `/healthz` and `/readyz` handlers.
  - [x] Register placeholder handlers for `/api/v1/instances` and `/api/v1/instances/` which can return `501 Not Implemented` initially.
  - [x] Wrap the mux with logging middleware and start the HTTP server.

### Milestone 2 – ConvexInstance CRUD

Goal: Typed CRUD HTTP endpoints backed by the Kubernetes API, built with TDD.

- [x] Write tests for `internal/kube` ConvexInstance helpers:
  - [x] Test ListConvexInstances, GetConvexInstance, CreateConvexInstance, UpdateConvexInstance, DeleteConvexInstance, using a fake client.
- [x] Implement `internal/kube` helpers to satisfy the tests:
  - [x] ListConvexInstances(namespace string) with support for all namespaces when namespace is empty.
  - [x] GetConvexInstance(namespace, name string).
  - [x] CreateConvexInstance(obj *v1alpha1.ConvexInstance).
  - [x] UpdateConvexInstance(obj *v1alpha1.ConvexInstance).
  - [x] DeleteConvexInstance(namespace, name string).
- [x] Define DTOs in `internal/httpapi` for request/response and write tests for JSON marshalling and validation:
  - [x] DTO for create/update requests (metadata + spec).
  - [x] DTO for responses (metadata, spec, status).
- [x] Write handler tests for `GET /api/v1/instances`:
  - [x] Verify namespace filtering and AllowedNamespaces enforcement.
  - [x] Verify correct mapping from Kubernetes errors to HTTP status codes.
- [x] Implement `GET /api/v1/instances` handler to satisfy tests.
- [x] Write handler tests for `GET /api/v1/instances/{namespace}/{name}`:
  - [x] Verify path parsing, allowed namespace checks, 404 behavior, and happy path.
- [x] Implement `GET /api/v1/instances/{namespace}/{name}` handler.
- [x] Write handler tests for `POST /api/v1/instances`:
  - [x] Verify required field validation and enum validation (environment, upgrade strategy).
  - [x] Verify AllowedNamespaces enforcement.
  - [x] Verify that successful creation returns HTTP 201.
- [x] Implement `POST /api/v1/instances` handler according to tests.
- [x] Write handler tests for `PUT /api/v1/instances/{namespace}/{name}` and `PATCH /api/v1/instances/{namespace}/{name}`:
  - [x] Verify that only `spec` is changed.
  - [x] Verify validation and error mapping.
- [x] Implement `PUT` and `PATCH` handlers.
- [x] Write handler tests for `DELETE /api/v1/instances/{namespace}/{name}`:
  - [x] Verify AllowedNamespaces enforcement and deletion behavior.
- [x] Implement `DELETE` handler.
- [x] Ensure all CRUD tests pass and that `go vet` and `golangci-lint` remain clean.

### Milestone 3 – Actions, status, diagnostics, and watch

Goal: Provide upgrade action, richer status, and optional streaming updates, following TDD.

- [ ] Write tests for `POST /api/v1/instances/{namespace}/{name}/upgrade`:
  - [ ] Verify validation of `targetVersion` and `strategy`.
  - [ ] Verify that invalid strategies are rejected with the correct HTTP status.
- [ ] Implement `POST /api/v1/instances/{namespace}/{name}/upgrade` to satisfy tests.
- [ ] Write tests for `GET /api/v1/instances/{namespace}/{name}/status`:
  - [ ] Verify that status fields are correctly returned from the CR.
  - [ ] If computed metrics (pods readiness) are included, verify their computation via fake data.
- [ ] Implement `GET /api/v1/instances/{namespace}/{name}/status` handler.
- [ ] Write tests for `GET /api/v1/instances/{namespace}/{name}/events` (if implemented in v1):
  - [ ] Use fake Events to test filtering and output format.
- [ ] Implement `/events` handler once tests are in place.
- [ ] Design and test SSE or watch behavior for `/api/v1/instances/watch`:
  - [ ] Abstract watcher behind an interface to allow deterministic unit tests.
  - [ ] Test event serialization and connection lifecycle handling.
- [ ] Implement `/watch` handler using SSE and the watcher abstraction.
- [ ] Re-run `go test`, `go vet`, and `golangci-lint` and fix any issues.

### Milestone 4 – Security, auth, and RBAC

Goal: Secure HTTP access and ensure least-privilege cluster permissions.

- [ ] Write tests for bearer token authentication middleware:
  - [ ] Ensure requests without Authorization are rejected.
  - [ ] Ensure requests with incorrect tokens are rejected.
  - [ ] Ensure requests with correct tokens pass through.
- [ ] Implement bearer token auth middleware according to tests.
- [ ] Write tests for AllowedNamespaces enforcement:
  - [ ] Ensure helper correctly denies access to disallowed namespaces.
  - [ ] Ensure handlers use the helper consistently.
- [ ] Implement namespace authorization helper and wire into handlers.
- [ ] Author RBAC manifests for in-cluster Deployment:
  - [ ] Create a ServiceAccount for the API backend.
  - [ ] Define Role/ClusterRole granting only needed verbs on `convexinstances`, `pods`, `events`, and optional Gateway API resources.
  - [ ] Bind the Role/ClusterRole to the ServiceAccount.
- [ ] Configure HTTP server timeouts and limits:
  - [ ] Set ReadTimeout, WriteTimeout, IdleTimeout on `http.Server`.
  - [ ] Set a safe maximum request body size in handlers where appropriate.
- [ ] Review and adjust logging:
  - [ ] Add tests or checks ensuring sensitive fields are not logged.
  - [ ] Ensure error messages are useful but not leaking secrets.
- [ ] Validate security by running the full test and lint suite and, if possible, security scanning tools.

### Milestone 5 – Deployment, integration, and documentation

Goal: Ship manifests and explain how to use the backend in and out of the cluster.

- [ ] Add Kustomize configuration for the API backend:
  - [ ] Deployment YAML using the built image and configuration env/flags.
  - [ ] Service YAML exposing the API on a ClusterIP.
  - [ ] RBAC YAML defining ServiceAccount, Role/ClusterRole, and bindings.
- [ ] Add Gateway API integration examples:
  - [ ] HTTPRoute mapping an internal host (example value: `convex-operator-api.internal.example.com`) to the API Service.
  - [ ] Show how to attach to an existing GatewayClass (for example `nginx` for NGINX Gateway Fabric).
- [ ] Write documentation (for example `docs/api-backend.md`):
  - [ ] Describe the purpose of the API and its relationship to the operator.
  - [ ] Document in-cluster deployment (using Kustomize).
  - [ ] Document out-of-cluster usage with kubeconfig and flags.
  - [ ] Provide sample curl commands for common operations.
  - [ ] Document the TDD and tooling expectations (tests and linting must pass before merging).
- [ ] Ensure CI is enforcing the full pipeline for this service:
  - [ ] `go test ./...`
  - [ ] `go vet ./...`
  - [ ] `golangci-lint run ./...`
  - [ ] Container build and (optionally) push steps to a registry.

This SPEC and task list are ordered to put tooling, static analysis, and TDD at the front so that all subsequent work is built with tests and linting in place from the beginning.
