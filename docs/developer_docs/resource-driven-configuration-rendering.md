# Automatic resource-triggered configuration rendering (1.1)

Resource changes automatically trigger configuration rendering without a
`reRenderResourceTypes` declaration. The supported inputs are the existing
trigger scope plus the Component's desired storage capacity:

| Existing trigger | Automatically tracked input | Scope |
| --- | --- | --- |
| vscale | Component resources (requests and limits) | Current Component |
| hscale | Component replicas | Current Component |
| tls | Component TLS configuration | Current Component |
| shardingHScale | Shard count and sharding template replicas | Current Component's sharding |
| Storage (no new enum) | Volume claim template storage requests and limits | Current Component |

Existing declarations remain valid API fields. They are no longer required to
enable these triggers. All configuration items receive these fixed inputs;
there is no analysis of which template expressions use them.

## Addon example

Declare a variable in the ComponentDefinition:

```yaml
spec:
  vars:
    - name: DATA_CAPACITY
      valueFrom:
        resourceVarRef:
          storage:
            name: data
```

Use it in a configuration template:

```gotemplate
cache_capacity_bytes={{ div (int64 $.DATA_CAPACITY) 2 }}
```

The existing built-in `getComponentPVCSizeByName $.component "data"` works too.
Both keep their existing request/limit semantics. Input comes from Component
spec, not PVC status; rendering does not wait for CSI or filesystem expansion.

## Flow

1. Component changes enqueue the existing Component-driven parameter controller.
   A small Cluster watch also enqueues that Cluster's sharding members when
   Cluster spec changes, because shard count may change without modifying an
   existing member's spec.
2. `UpdateConfigPayload` records the fixed fields in existing payload keys:
   `componentResource`, `replicas`, `tls` and `sharding`.
   The new internal `volumeClaimTemplates` key maps volume names to storage
   request/limit bytes. Equivalent storage units and volume ordering are stable.
3. The existing merge/update path compares the generated ComponentParameter
   with the persisted object. Changed payload advances its generation; unchanged
   inputs do not create a new revision.
4. The original parameter controller renders templates, merges explicit user
   parameters, validates configuration, and publishes the ConfigMap.
5. The original reconfigure controller compares final configuration content.
   Changed content follows existing reload/restart policy; unchanged content
   finishes without applying it again.

Rendering, variable resolution, parameter precedence, revision processing and
application policies are unchanged. Existing configurations may acquire missing
payload fields and render once after controller upgrade.

Cross-Component references continue to resolve through the original renderer
when rendering occurs. This change does not introduce subscriptions to their
sources, a dependency graph, fingerprints, snapshots or new retry states.
Per-instance resources and Secret/ConfigMap/Service dependencies are outside
this trigger scope.

## Validation

Unit tests cover the old trigger types, storage changes/removal/normalization,
and sharding watch scope. Envtest covers storage vars and the built-in size
function, consecutive expansions, explicit parameter overrides, and Cluster-only
shard count changes. An unchanged-output test checks that reconfigure skips apply.
The suite emulates Component updates; it does not expand real CSI volumes or run
a live database.
