# Component Network

KubeBlocks 1.1 adds `spec.componentSpecs[*].network`, a component-level network configuration API. It lets users describe pod network behavior directly in the `Cluster` spec instead of relying on addon defaults, manual pod patches, or environment-specific workarounds.

The API covers five related settings:

* host networking;
* named host port mappings;
* pod host aliases;
* DNS policy;
* DNS config.

## Why This Helps

Database components often need network behavior that differs from the Kubernetes default. For example:

* a database engine may need stable node ports for external clients;
* a component may need host networking for performance, migration, or compatibility with an existing network design;
* pods may need temporary `/etc/hosts` entries during hybrid migration or failover testing;
* DNS search domains or resolver options may need to be tuned for multi-cluster or cross-domain discovery.

`ComponentNetwork` makes these settings part of the declarative database cluster spec, so operators can review, apply, and audit them with the rest of the workload configuration.

## What It Does

`ComponentNetwork` is available on:

* `Cluster.spec.componentSpecs[*].network`
* `Component.spec.network`

The common shape is:

```yaml
network:
  hostNetwork: false
  hostPorts: []
  hostAliases: []
  dnsPolicy: ClusterFirst
  dnsConfig: {}
```

| Field | Purpose |
| --- | --- |
| `hostNetwork` | Run component pods in the host network namespace. |
| `hostPorts` | Map named container ports to explicit host ports, or request automatic host-port allocation with host networking. |
| `hostAliases` | Add static entries to the pod `/etc/hosts` file. |
| `dnsPolicy` | Set pod DNS policy, such as `ClusterFirst`, `ClusterFirstWithHostNet`, or `None`. |
| `dnsConfig` | Set pod DNS nameservers, search domains, and resolver options. |

## How to Use It

### Enable host networking

Use `hostNetwork: true` when the component must share the node network namespace. The referenced `ComponentDefinition` must declare host-network capability in `spec.hostNetwork`.

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

If `hostNetwork` is enabled and `dnsPolicy` is not set, KubeBlocks defaults the DNS policy to `ClusterFirstWithHostNet`.

### Configure host ports

Use `hostPorts` when a component needs predictable host port values or when host-network ports must be rendered into addon variables.

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

Port resolution rules:

* `hostPorts` entries are matched by **port name**.
* With `hostNetwork: true`, mappings for ports declared in `ComponentDefinition.spec.hostNetwork` are required. Kbagent ports such as `http` and `streaming` are optional; omitted kbagent ports can still be allocated automatically.
* With `hostNetwork: true` and no explicit `hostPorts`, KubeBlocks allocates required ports automatically through the host-port manager.
* With `hostNetwork: false`, explicit `hostPorts` can map runtime container ports to node host ports. Port names must match ports declared in `ComponentDefinition.spec.runtime.containers[*].ports`; unknown runtime port names are ignored.
* The allocated or mapped port value is rendered into vars that use `hostNetworkVarRef`.

KubeBlocks tracks host port allocations in a ConfigMap in the KubeBlocks namespace. The allocatable range is controlled by Helm values `hostPorts.include` and `hostPorts.exclude`; the 1.1 default include range is `55000-59999`.

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

This is useful during migration, hybrid-cloud connectivity testing, or integration with systems that still depend on fixed hostnames.

`hostAliases` is only valid for non-hostNetwork pods.

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

DNS changes can affect database discovery, replication, and client connection behavior. Validate them in staging before applying them to production clusters.

## Verify the Result

Check the generated pods:

```bash
kubectl get pod -n demo
kubectl get pod <pod-name> -n demo -o yaml
```

Inspect these pod fields:

```yaml
spec:
  hostNetwork:
  hostAliases:
  dnsPolicy:
  dnsConfig:
  containers:
    - ports:
```

For host ports, also check pod placement and the host-port allocation ConfigMap:

```bash
kubectl get pod <pod-name> -n demo -o wide
kubectl -n kb-system get configmap
```

For release validation, confirm that host-port allocations are created when the component is reconciled, remain stable across ordinary reconciliations, and are removed after the component or cluster is deleted.

## Notes

* `hostNetwork` requires the referenced `ComponentDefinition` to declare host-network capability. Without it, the setting is ignored.
* Plan host port ranges carefully to avoid conflicts with other workloads on the same nodes.
* Combine explicit host ports with scheduling rules when specific ports must not collide on a node.
* `hostAliases` is not valid for host-network pods.
* DNS changes are workload-sensitive; test database discovery and replication after changing DNS settings.
