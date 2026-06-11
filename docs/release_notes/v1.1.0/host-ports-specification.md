# ComponentNetwork API

KubeBlocks 1.1 adds `spec.componentSpecs[*].network`, a component-level network configuration API. It lets users configure host networking, explicit host port mappings, pod host aliases, DNS policy, and DNS config directly in the `Cluster` spec.

## Why This Helps

Some database deployments need predictable network behavior. For example:

* a database engine may need stable host ports for external clients;
* a component may need host networking for performance or legacy network integration;
* pods may need custom `/etc/hosts` entries for migration, hybrid deployment, or testing;
* DNS options may need to be tuned for cross-domain or multi-cluster name resolution.

Before this API, users often had to rely on component definition defaults, manual patches, or environment-specific workarounds. `ComponentNetwork` puts these settings in the user-facing cluster spec.

## What It Does

`ComponentNetwork` is available on both `Cluster.spec.componentSpecs[*].network` and `Component.spec.network`.

```yaml
network:
  hostNetwork: false
  hostAliases: []
  dnsPolicy: ClusterFirst
  dnsConfig: {}
  hostPorts: []
```

The fields are:

| Field | Purpose |
| --- | --- |
| `hostNetwork` | Run component pods in the host network namespace. |
| `hostPorts` | Map named container ports to explicit host ports. |
| `hostAliases` | Add entries to the pod `/etc/hosts` file. |
| `dnsPolicy` | Set pod DNS policy, such as `ClusterFirst` or `ClusterFirstWithHostNet`. |
| `dnsConfig` | Set pod DNS nameservers, searches, and resolver options. |

## How to Use It

### Use host networking with automatic port allocation

If a `ComponentDefinition` declares host-network capability, users can enable host networking from the cluster spec.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: foo
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      network:
        hostNetwork: true
```

When `hostNetwork: true` and `hostPorts` is omitted, KubeBlocks allocates required host ports automatically through the host-port manager.

### Use host networking with explicit host ports

Use explicit host ports when the database or external clients require predictable ports.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: foo
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      network:
        hostNetwork: true
        hostPorts:
          - name: client
            port: 31001
          - name: metrics
            port: 31002
```

When `hostNetwork` is enabled:

* mappings for ports declared in `ComponentDefinition.spec.hostNetwork` are required;
* kbagent ports such as `http` and `streaming` are optional;
* omitted kbagent ports can still be allocated automatically.

### How port allocation works

KubeBlocks tracks host port allocations in a ConfigMap in the KubeBlocks namespace (name from the `HOST_PORT_CM_NAME` setting). The allocatable range is configured by Helm values `hostPorts.include` / `hostPorts.exclude`; the default dedicated range is `55000-59999`, chosen to avoid the node ephemeral port range.

Port resolution rules:

* `hostPorts` entries are matched by **port name** against the container ports declared in the `ComponentDefinition`.
* Explicit `hostPorts` take effect only when `hostNetwork: true`. Without host networking, the entries are not used for port mapping.
* When `hostPorts` is set, every host-network port declared by the `ComponentDefinition` must have a mapping; only the kbagent ports (`http`, `streaming`) may be omitted and fall back to automatic allocation.
* The allocated or mapped port value is what gets rendered into vars that use `hostNetworkVarRef`, so other components and clients see the real port.

### Add host aliases

Use `hostAliases` to add static host-to-IP mappings to component pods.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: foo
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      network:
        hostAliases:
          - ip: "10.10.0.12"
            hostnames:
              - old-primary.example.internal
              - legacy-db.internal
```

This is useful during migration, hybrid-cloud connectivity testing, or integration with services that still depend on fixed hostnames.

### Configure DNS

Use `dnsPolicy` and `dnsConfig` when pods need custom name resolution behavior.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: foo
  namespace: demo
spec:
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      network:
        dnsPolicy: None
        dnsConfig:
          nameservers:
            - 10.96.0.10
          searches:
            - demo.svc.cluster.local
            - svc.cluster.local
          options:
            - name: ndots
              value: "2"
```

If `hostNetwork` is enabled and `dnsPolicy` is not set, KubeBlocks defaults the DNS policy to `ClusterFirstWithHostNet`.

## Verify the Result

Check generated pods:

```bash
kubectl get pod -n demo
kubectl get pod <pod-name> -n demo -o yaml
```

Inspect these fields:

```yaml
spec:
  hostNetwork: true
  hostAliases:
  dnsPolicy:
  dnsConfig:
  containers:
    - ports:
```

For host ports, also check the node:

```bash
kubectl get pod <pod-name> -n demo -o wide
```

## Notes

* `hostNetwork` requires the referenced `ComponentDefinition` to declare host-network capability (`spec.hostNetwork`). Without it, the setting is ignored.
* Explicit `hostPorts` only apply together with `hostNetwork: true`.
* Plan host port ranges carefully to avoid conflicts with other workloads on the same nodes.
* Combine host ports with scheduling rules when specific ports must not collide on a node.
* `hostAliases` is only valid for non-hostNetwork pods.
* DNS changes can affect database discovery and replication. Validate in staging before production rollout.
