# Component Controllers

`controllers/apps/component/` reconciles `Component` resources into concrete Kubernetes workloads and supporting objects. It uses the transformer DAG pipeline pattern: a plan builder chains transformers that each describe one scenario's desired object changes.

## Layout

- `component_controller.go`, `component_plan_builder.go`: reconciler entry and plan builder that assembles the transformer chain.
- `transformer_component_*.go`: 18 transformers, each handling one aspect: init, deletion, meta, workload, service, RBAC, account, account-provision, TLS, template, vars, validation, hostnetwork, hostport, monitor, notifier, post-provision, pre-terminate, status.
- `scheme.go`: scheme registration via `model.AddScheme()`.
- `utils.go`, `suite_test.go`: helpers and Ginkgo test suite.

## Editing Rules

- Follow the transformer chain pattern: add behavior as a new `transformer_component_*.go` implementing `graph.Transformer` rather than inlining logic in the reconciler.
- Transformer order matters — the plan builder chains them sequentially and `graph.ErrPrematureStop` short-circuits the remaining chain. Document ordering assumptions in comments.
- Transformers declare intent through `graphCli.Update/Create/Delete(dag, ...)` — do not call `client.Create/Patch` directly inside transformers.
- One transformer per scenario; do not combine unrelated concerns in a single transformer.
- Preserve `TransformContext` type assertions (`transCtx, _ := ctx.(*ComponentTransformContext)`) — extend the context struct rather than passing ad-hoc parameters.
- Keep generated object names, labels, and annotations stable — tests compare full object graphs and are sensitive to drift.
- File naming: `transformer_component_{aspect}.go` + `transformer_component_{aspect}_test.go`.

## Testing

- Each transformer should have a paired `_test.go` covering its specific scenario.
- The `suite_test.go` sets up envtest; transformer tests use mock GraphClient to verify DAG mutations.
