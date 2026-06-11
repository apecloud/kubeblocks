# Dynamic InstanceSet Template Adoption

KubeBlocks 1.1 supports dynamic adoption of existing instances by named instance templates. This lets you move specific pods from the default component template into a specialized template, or move them back, by assigning their ordinals.

## Why This Helps

Real database clusters do not always stay homogeneous. A user may start with three identical replicas, then later discover that one replica needs more CPU, another should run in a different zone, or one pod should test a new engine version before the full cluster is upgraded.

Before dynamic template adoption, users had to plan these templates ahead of time. If requirements changed later, changing a pod's template was difficult because pod identity, PVC identity, and network identity all matter for stateful workloads.

Dynamic template adoption helps users:

* tune resources for one or a few hot instances;
* test new versions on selected instances;
* move specific replicas to different nodes or zones;
* expand storage for only the instances that need it;
* preserve pod identity and PVC data while changing the template assignment.

## What It Does

A KubeBlocks component has a default template and optional named templates in `spec.componentSpecs[*].instances`.

With dynamic adoption:

* each pod is identified by its ordinal;
* `flatInstanceOrdinal: true` keeps pod names independent of template names;
* `ordinals.discrete` on a named template tells KubeBlocks which existing pods the template should adopt;
* pods not assigned to a named template remain in the default template.

For example, with pods `foo-0`, `foo-1`, and `foo-2`, you can assign only `foo-2` to a `highperf` template while `foo-0` and `foo-1` continue using the default template.

## Prerequisite: Enable Flat Ordinals

Dynamic adoption requires flat instance ordinals. Set `flatInstanceOrdinal: true` on the component.

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
        discrete: [0, 1, 2]
```

With flat ordinals, pod names use:

```text
$(component)-$(ordinal)
```

For example:

| Pod | Template |
| --- | --- |
| `foo-0` | default |
| `foo-1` | default |
| `foo-2` | default |

## How to Use It

### Adopt selected pods into a named template

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
        - name: highperf
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

### Return a pod to the default template

To move `foo-0` back to the default template, remove ordinal `0` from the named template and include it in the component-level ordinals.

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
        discrete: [0, 1]
      instances:
        - name: highperf
          replicas: 1
          ordinals:
            discrete: [2]
```

Result:

| Pod | Template |
| --- | --- |
| `foo-0` | default |
| `foo-1` | default |
| `foo-2` | `highperf` |

## Common Use Cases

### Increase resources for one hot pod

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

This keeps `foo-0` as `foo-0`, while applying the higher resource template to that pod.

### Test a new service version on one pod

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

When all pods should use the new version, move the version to the component-level spec and remove the temporary template.

### Move one replica to another zone

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

This is useful for fault-domain placement or disaster recovery replicas.

### Expand storage for one instance

```yaml
instances:
  - name: large-storage
    replicas: 1
    ordinals:
      discrete: [0]
    volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 500Gi
```

Use this when data distribution is skewed and only one instance needs more storage.

## Verify the Result

Check pods:

```bash
kubectl get pod -n demo
```

Check the generated InstanceSet:

```bash
kubectl get instanceset -n demo
kubectl get instanceset <name> -n demo -o yaml
```

Look for:

* stable pod names;
* expected ordinals in template status;
* expected resource, scheduling, image, or storage changes on adopted pods.

## Notes

* `flatInstanceOrdinal: true` is required.
* Ordinals must be unique across the default template and all named templates.
* Pod names remain stable, but pods may be recreated when the new template changes immutable pod fields.
* PVC data is preserved according to the component's PVC retention policy and storage behavior.
* To keep PVC identity stable when an instance moves between templates, set the same `persistentVolumeClaimName` prefix in both templates. See [Volume Sharing Among Instances](./volume-sharing.md).
* For sharded clusters, use `shardTemplates[*].shardIDs` to adopt specific shards. See [Heterogeneous Shards and Shard-Specific Scale-In](./heterogeneous-shards.md).
