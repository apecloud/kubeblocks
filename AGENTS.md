# KubeBlocks

KubeBlocks is a Kubernetes database control plane. The repo is organized around CRD APIs, controller-runtime reconcilers, shared controller/model packages, and generated deployment manifests.

## Repository Map

- `apis/`: API type definitions, conversion/webhook code, and generated deepcopy code.
- `controllers/`: reconcilers that watch KubeBlocks and Kubernetes resources.
- `pkg/`: reusable libraries used by controllers and binaries; keep this layer independent from `controllers/`.
- `cmd/`: binaries for `manager`, `dataprotection`, and `kbagent`.
- `config/`: kustomize bases and generated CRDs/RBAC/webhooks.
- `deploy/helm/`: Helm chart output.
- `openspec/`: OpenSpec project configuration.
- `.codex/skills/`, `.claude/skills/`, `.opencode/skills/`: matching OpenSpec agent skills. Keep behavior changes synchronized across the three agent surfaces.
- `.claude/commands/opsx/` and `.opencode/commands/`: command wrappers for the OpenSpec skills.
- `.github/PULL_REQUEST_TEMPLATE.md`: PR checklist used by the branch workflow; keep it aligned with repository policy checks.

## Editing Rules

- After changing API types under `apis/`, run `make generate` for deepcopy updates and `make manifests` for generated CRDs/RBAC/webhooks.
- Do not edit `zz_generated.deepcopy.go`, `pkg/client/`, or `config/crd/bases/*.yaml` by hand unless the task is explicitly about generated output. Regenerate them from source changes.
- Keep `pkg/` free of imports from `controllers/`; shared logic belongs in `pkg/`, while reconciliation wiring belongs in `controllers/`.
- Prefer the controller-runtime client and local helper packages over direct `client-go` usage.
- Status changes normally use `Status().Patch(..., client.MergeFrom(old.DeepCopy()))` or the existing package helper. Preserve conflict-safe patch patterns when editing reconcilers.
- Preserve license headers and run formatting before handing off Go changes.

## Commands

```bash
make generate          # Generate deepcopy code.
make manifests         # Generate CRDs, RBAC, and webhook manifests.
make client-sdk-gen    # Generate typed clients under pkg/client/.
make build-checks      # fmt, vet, goimports, and lint-fast.
make test-fast         # Unit tests using envtest.
make manager           # Build the manager binary.
make dataprotection    # Build the dataprotection binary.
make kbagent           # Build the kbagent binary.
make install           # Install CRDs into the current kubeconfig cluster.
make deploy            # Deploy the operator to the current kubeconfig cluster.
```

## Testing

- Put focused Go tests next to the package under test.
- Controller and API tests use Ginkgo/Gomega and envtest patterns already present in the repo.
- For API changes, include validation/default/conversion coverage when behavior changes, not only compile-time checks.

## OpenSpec Agent Assets

- The OpenSpec skill set is duplicated for Codex, Claude, and OpenCode. When changing a skill workflow, update all copies intentionally.
- Keep OpenCode command files under `.opencode/commands/`; `.opencode/command/` is the old singular path.
- `openspec/config.yaml` is the source of OpenSpec project identity and capability metadata; do not duplicate that data in individual command wrappers.
