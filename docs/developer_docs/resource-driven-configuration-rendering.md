# Automatic configuration rendering from Component resources (1.1)

Configuration templates automatically re-render when their Component's CPU,
memory, or volume claim template capacity changes. Addons do not need to add a
`reRenderResourceTypes` entry for these changes. Existing trigger declarations,
custom templates, parameter overrides, and reload/restart policies remain supported.

## Addon example

Declare a storage variable in the ComponentDefinition:

```yaml
spec:
  vars:
    - name: DATA_CAPACITY
      valueFrom:
        resourceVarRef:
          storage:
            name: data
```

Use it in a configuration template, for example to allocate half the requested
volume capacity to a cache:

```gotemplate
cache_capacity_bytes={{ div (int64 $.DATA_CAPACITY) 2 }}
```

The built-in `getComponentPVCSizeByName $.component "data"` function and direct
access to Component resources also participate in automatic re-rendering.
The input is the Component's desired capacity, with the existing variable
resolver's request/limit fallback semantics. It does not wait for PVC status or
filesystem expansion, and it does not change the meaning of OpsRequest success.

A `resourceVarRef.compDef` can reference another Component in the same Cluster.
Changes to that source enqueue resource-var consumers. The existing resolver
continues to handle component matching, optional references, and multiple-object
selection. Required unresolved references retain the previous configuration and
retry; optional references follow their existing rendering semantics.

## Controller behavior

The Component-driven parameter controller computes a versioned fingerprint of:

- The Component's CPU and memory requests/limits.
- Storage requests/limits indexed by volume claim template name.
- Declared resource variable selectors and their resolved resource operands.

It stores the fingerprint under the controller-owned
`ComponentParameter.spec.configItemDetails[].payload.resourceRenderInputs` key.
Changing that payload advances ComponentParameter generation and reuses the 1.1
configuration revision and status machinery. External payload keys are preserved.
Do not edit the managed key manually.

Resource quantities are normalized, so equivalent units and volume-list ordering
do not create new revisions. Component status and unrelated metadata are not
included. A resource change can re-render every configuration template for the
Component; no template-language dependency analysis is performed.

Rendering and explicit parameter merging run in their original order. The existing
configuration controller compares final content with the last applied content.
Unchanged content completes the new revision without running a reload or restart;
metadata can still change to record the processed input. Explicit user parameters
can therefore keep the final configuration unchanged even after a resource change.

Fingerprinting and rendering use consistent Component reads within a reconcile.
If the inputs no longer match the payload being processed, rendering waits for the
Component-driven controller to synchronize the next revision.

On controller upgrade, existing configurations acquire the fingerprint on their
first reconciliation and are rendered once. This also refreshes stale resource-derived
configuration. Subsequent unchanged inputs are no-ops.

## Scope and validation

Automatic watches cover Component resource changes and resource-variable consumers,
including source creation/deletion and matching changes from Cluster or
ComponentDefinition updates. Candidate consumer scans are bounded to the source
Cluster; resolved fingerprints decide whether a configuration actually needs work.
This does not add automatic watches for Secret, ConfigMap, Service, or other
non-resource variable sources, nor for per-instance resource overrides.

Controller integration tests exercise storage variables, the built-in PVC-size
function, consecutive resource revisions, cross-Component changes without changing
the consumer generation, explicit parameter overrides, and unchanged output.
The parameter controller tests use envtest and emulate the Component update normally
propagated by the apps controller. They do not expand a real CSI volume or run a
live database. Existing data-cluster ConfigMap routing and configuration application
policies are reused.
