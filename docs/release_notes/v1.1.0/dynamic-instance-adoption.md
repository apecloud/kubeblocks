# Dynamic InstanceSet Template Adoption

Dynamic Template Adoption is a KubeBlocks cluster management feature that allows you to assign or reassign instance templates to specific Pods dynamically on demand. You can change settings for individual Pods while preserving their identity, data, and network endpoints.

This is useful when a database component starts out uniform, then one or a few replicas need to be treated differently. A hot replica may need more CPU, one pod may need to try a new engine version, a disaster recovery replica may need to run in another zone, or a data-heavy pod may need a larger PVC.

The key point is that adoption changes template ownership, not logical identity. The pod keeps the same name, ordinal, service endpoint, and PVC identity. KubeBlocks may still recreate the pod if the new template changes fields that cannot be updated in place.

## When to Use It

Use dynamic template adoption when only part of a component needs a different configuration:

* increase CPU or memory for one or a few busy pods;
* test a new service version or ComponentDefinition on a few pods before a full rollout;
* place a few replicas in another node pool, availability zone, or capacity class;
* expand storage for pods with skewed data distribution;
* add temporary labels, annotations, environment variables, or debug settings to one pod.

Before 1.1.0, named instance templates had to be planned when the cluster was created. Dynamic adoption lets you add or remove those assignments later, without changing the instance ordinals.

## What Can Be Heterogeneous

A named instance template can override only the fields supported by `spec.componentSpecs[*].instances[*]`. These are the settings you can use when a few pods need to be heterogeneous from the component default:

| Field | What it changes | Typical use |
| --- | --- | --- |
| `serviceVersion` | Service version for these pods | Try a new engine version on a small set of pods |
| `compDef` | ComponentDefinition for these pods | Move pods to a compatible new ComponentDefinition |
| `annotations` | Pod annotations merged into these pods | Add operational metadata or integration hints |
| `labels` | Pod labels merged into these pods | Mark hot, canary, debug, or DR replicas |
| `schedulingPolicy` | Scheduling rules for these pods | Place pods in a specific zone, node pool, or capacity class |
| `resources` | CPU and memory requests or limits for the first container | Give pods more or fewer resources |
| `env` | Environment variables added to or overriding these pods | Enable debug flags or per-instance runtime settings |
| `volumeClaimTemplates` | Storage requirements for these pods | Use larger PVCs for these pods |

The fields `name`, `replicas`, and `ordinals` control template identity and ownership. They are required for adoption, but they do not change the pod configuration by themselves.

Fields that are not listed above remain component-level settings. This includes services, accounts, TLS, configs, pod update policy, and component-level networking.


## How to Use It

### Prerequisite: Enable Flat Ordinals

Set `flatInstanceOrdinal: true` through the cluster API. You can also list the initial default ordinals explicitly. If all replicas start in the default template, the `ordinals` block is mainly there to make the ownership clear.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: demo-cluster
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      flatInstanceOrdinal: true # required
      ordinals:
        discrete: [0, 1, 2]  # optional, default is [0, 1, 2]. it is listed here for clarity.
```

With flat ordinals, pods are named after `$(component)-$(ordinal)` (e.g., foo-0, foo-1) instead of the default `$(component)-$(template)-$(ordinal)` format.

### Adopt Pods Into a Named Template

To adopt existing pods, remove their ordinals from the component-level `ordinals.discrete` and add them to a named instance template.

The following example moves `foo-0` and `foo-2` into the `highperf` template. `foo-1` remains in the default template.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: demo-cluster
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      flatInstanceOrdinal: true
      ordinals:
        discrete: [1]
      instances:
        - name: highperf # highperf adopts foo-0 and foo-2
          replicas: 2
          ordinals:
            discrete: [0, 2]
          resources:
            requests:
              cpu: "4"
              memory: 8Gi
            limits:
              cpu: "4"
              memory: 8Gi
```

Result:

| Pod | Template |
| --- | --- |
| `foo-0` | `highperf` |
| `foo-1` | default |
| `foo-2` | `highperf` |

## Return Pods to the Default Template

To return a pod to the default template, remove its ordinal from the named template and add it back to the component-level `ordinals.discrete`. This is not a common operation and should be done with caution.

The following example moves `foo-0` back to the default template. `foo-2` stays in `highperf`.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: demo-cluster
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      flatInstanceOrdinal: true
      ordinals:
        discrete: [0, 1] # [1] -> [0,1], adopt foo-0 back to the default template
      instances:
        - name: highperf
          replicas: 1
          ordinals:
            discrete: [2] # [0,2] -> [2], remove foo-0 from highperf tempalte
```

Result:

| Pod | Template |
| --- | --- |
| `foo-0` | default |
| `foo-1` | default |
| `foo-2` | `highperf` |

If the named template should no longer own any pods, remove that entry from `instances` and put all ordinals back under the component-level `ordinals.discrete`.

KubeBlocks **requires explicit ordinals** because the intent is otherwise ambiguous. Removing an ordinal from a template could mean "give this same pod back to the default template", or it could mean "scale down this template and create a different pod elsewhere". Listing the target ordinals makes the intent clear.

## Common Use Cases

### Increase Resources for a Hot Pod

Move the busy pod into a higher-resource template:

```yaml
instances:
  - name: highperf
    replicas: 1
    ordinals:
      discrete: [0]
    resources:
      requests:
        cpu: "8"
        memory: 16Gi
      limits:
        cpu: "8"
        memory: 16Gi
```

The pod stays `foo-0` and keeps its PVC. Depending on the component's pod update policy and the exact resource change, KubeBlocks may update the pod in place or recreate it.

### Canary a New Service Version

Move one pod into a canary template:

```yaml
instances:
  - name: canary
    replicas: 1
    ordinals:
      discrete: [2]
    serviceVersion: "x.y.z"
```

After validation, expand the canary template to more ordinals:

```yaml
instances:
  - name: canary
    replicas: 2
    ordinals:
      discrete: [1, 2]
    serviceVersion: "x.y.z"
```

When all pods should use the new version, move the version to the component-level spec, put all ordinals back under the default template, and remove the temporary canary template.

### Geo-Distributed Replicas

Move one replica into a template with a different scheduling policy:

```yaml
instances:
  - name: zone-b
    replicas: 1
    ordinals:
      discrete: [2]
    schedulingPolicy:
      nodeSelector:
        topology.kubernetes.io/zone: us-west-2b
```

This is useful for fault-domain placement or disaster recovery replicas. A scheduling change usually recreates the selected pod so Kubernetes can place it on a matching node.

### Expand Storage for Selected Pods

Use a named template with larger `volumeClaimTemplates` for pods that need more storage:

```yaml
instances:
  - name: large-storage
    replicas: 2
    ordinals:
      discrete: [2, 5]
    volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 500Gi
```

Make sure the StorageClass supports volume expansion. To keep PVC identity stable when a pod moves between templates, use the same volume claim template `name` and the same `persistentVolumeClaimName` prefix in both templates.

### Add Temporary Debug Settings

Move one pod into a debug template when you need extra logs or profiling on a specific replica:

```yaml
instances:
  - name: debug
    replicas: 1
    ordinals:
      discrete: [1]
    env:
      - name: LOG_LEVEL
        value: DEBUG
      - name: ENABLE_PROFILING
        value: "true"
```

Environment variable changes may require pod recreation.

## Limitations and Notes

* `flatInstanceOrdinal: true` is required.
* Ordinals must be unique across the default template and all named templates.
* A pod can move from the default template to a named template, or from a named template back to the default template, but not between two named templates. To move a pod from template `A` to template `B`, first return the ordinal from `A` to the default template, then adopt it into `B`.
* Pod names remain stable, but pods may be recreated when the new template changes fields that cannot be updated in place, such as scheduling or environment variables.
* PVC data is preserved according to the component's PVC retention policy, volume claim template compatibility, and storage behavior.
* If a template changes `serviceVersion`, verify the actual container images instead of relying soly on the `apps.kubeblocks.io/service-version` label.
* When returning a pod to the default template, explicitly specify the ordinals that should stay in the default template.
* For sharded clusters, use `shardTemplates[*].shardIDs` to adopt specific shards. See [Heterogeneous Shards and Shard-Specific Scale-In](./heterogeneous-shards.md).
