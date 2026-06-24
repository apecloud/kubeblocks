# Component Network

## Overview

KubeBlocks 1.1.0 introduces `spec.componentSpecs[*].network` for component-level pod network settings. Instead of relying on addon defaults, annotations, or manual pod patches, you can describe the network behavior directly in the `Cluster` spec.

Use this when a database component needs something beyond the Kubernetes default network setup. Typical examples include enabling host networking for a supported engine, pinning a runtime port to a fixed host port, adding temporary host aliases during migration, or customizing DNS for cross-domain service discovery.

For users, the supported entry point is:

* `Cluster.spec.componentSpecs[*].network`

KubeBlocks reconciles the managed `Component` object from the `Cluster`. Do not edit `Component.spec.network` directly. `Component` is controlled by KubeBlocks and should be used for observation, not as the source of user configuration.

## What You Can Configure

| Field | What it controls | When to use it |
| --- | --- | --- |
| `hostNetwork` | Whether component pods use the node network namespace | Use host networking for engines that require direct host network access or host-network port rendering |
| `hostPorts` | Mapping from named container ports to node host ports | Pin a runtime port to a known host port, or provide explicit ports for host-network mode |
| `hostAliases` | Static entries in the pod `/etc/hosts` file | Bridge temporary hostname dependencies during migration, failover testing, or hybrid connectivity checks |
| `dnsPolicy` | Pod DNS policy | Use `None`, `ClusterFirst`, or `ClusterFirstWithHostNet` when the default resolver behavior is not correct |
| `dnsConfig` | Pod DNS nameservers, search domains, and resolver options | Tune search domains, `ndots`, or nameservers for database discovery |

## Host Networking vs. Pod Networking

Kubernetes normally runs database workloads on the pod network. Each pod gets its own IP address, traffic is routed by the cluster CNI, and clients usually connect through Kubernetes Services or DNS names. This is the default and recommended choice for most KubeBlocks clusters.

Host networking works differently. A pod that uses host networking shares the node's network namespace, binds ports directly on the node, and is reached through the node network path. This is useful when an engine or deployment environment needs node-level addresses, host-network port rendering, or direct integration with an existing network design.

The tradeoff is operational complexity. Host networking removes one layer of network isolation and turns port conflicts into a node-level concern. It can also affect scheduling, because two pods that need the same host port cannot run on the same node. DNS behavior is slightly different as well: when host networking is enabled and no DNS policy is specified, KubeBlocks uses `ClusterFirstWithHostNet`.

In practice, start with pod networking. Use host networking only when the addon supports it and there is a concrete requirement that Services, LoadBalancers, NodePorts, or normal `hostPorts` cannot satisfy.

## Practical Network Choices

Most clusters should stay on the default Kubernetes pod network. It is easier to schedule, avoids node-level port conflicts, and works well with Services and Kubernetes DNS.

Use the following guidance when choosing a configuration:

| Scenario | Recommended configuration | Notes |
| --- | --- | --- |
| The component only needs normal in-cluster traffic | Leave `network` unset | KubeBlocks uses the default pod network and Kubernetes DNS. |
| The addon supports host networking and the engine needs node-level network access | Set `network.hostNetwork: true` | Check `ComponentDefinition.spec.hostNetwork` first. For new manifests, prefer the API field over annotations. |
| You need one runtime port to be reachable through a fixed node port | Set `network.hostPorts` with the runtime port name | The name must exist in `ComponentDefinition.spec.runtime.containers[*].ports`; unknown names are ignored. |
| A host-network addon needs predictable port values | Set `network.hostNetwork: true` and list required ports in `hostPorts` | Get the required port names from `ComponentDefinition.spec.hostNetwork.containerPorts`. If fixed values are not required, omit `hostPorts` and let KubeBlocks allocate them. |
| The pod needs to resolve an old or external hostname temporarily | Use `hostAliases` | Keep the mapping small and temporary. Prefer DNS or Kubernetes Services for long-term service discovery. |
| The component must use custom resolvers, search domains, or resolver options | Set `dnsPolicy` and `dnsConfig` | Use `dnsPolicy: None` only when you provide the full resolver configuration in `dnsConfig`. |

## Enable Host Networking

Host networking only works for addons whose `ComponentDefinition` declares host-network support. Check the `ComponentDefinition` before enabling it:

```bash
kubectl get cmpd <cmpd-name> -o yaml | yq '.spec.hostNetwork'
```

If `spec.hostNetwork` or `spec.hostNetwork.containerPorts` is empty, the addon does not support host networking. Setting `network.hostNetwork: true` in the `Cluster` spec will not force it on; KubeBlocks keeps the pod on normal pod networking.

For a supported addon, the recommended 1.1 API is `network.hostNetwork: true`:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-mongodb
  namespace: demo
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostNetwork: true
# other component spec fields are omitted for brevity
```

When host networking is enabled and `dnsPolicy` is not set, KubeBlocks uses `ClusterFirstWithHostNet` for the pod DNS policy.

In host-network mode:

* Allocated ports are recorded in a ConfigMap in the KubeBlocks namespace.
* The default automatic allocation range in 1.1 is `55000-59999`, controlled by Helm values `hostPorts.include` and `hostPorts.exclude` when deploying KubeBlocks.

## Configure Host Ports

Use `hostPorts` when a component needs a stable node port for a named container port.

For normal pod networking, host ports are matched by port name. The port name must match a port declared in `ComponentDefinition.spec.runtime.containers[*].ports`. Unknown runtime port names are ignored.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-mongodb-hostport
  namespace: demo
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostPorts:
          - name: mongodb  # port name in the ComponentDefinition's runtime.containers[*].ports
            port: 31110
# other component spec fields are omitted for brevity
```

With `hostNetwork: true`, `hostPorts` is also used to provide explicit host-network port values:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-mongodb-hostport
  namespace: demo
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostNetwork: true
        hostPorts:
          - name: mongodb
            port: 31110
          - name: ha
            port: 31000
# other component spec fields are omitted for brevity
```

To sum up, the host-network port mapping behavior is as follows:

* When HostNetwork is Enabled
  1. **If `hostPorts` is empty**: All ports are automatically allocated by the host-port manager.
  2. **If `hostPorts` is specified**:
    - Mappings for all ports defined in `cmpd.spec.hostNetwork` are MANDATORY
* When HostNetwork is Disabled
  - Host port mappings are optional (per user request)
  - Mappings are restricted to ports defined in `cmpd.spec.runtime.containers.ports`
  - Any specified container ports not present in the runtime definition will be ignored


## Add Host Aliases

Use `hostAliases` when a pod needs a small number of fixed hostname-to-IP entries in `/etc/hosts`. This is most useful for temporary compatibility cases, such as migrating from an old database endpoint, testing hybrid-cloud connectivity, or keeping a legacy hostname working while DNS records are being moved.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-dns
  namespace: demo
spec:
  componentSpecs:
    - name: mysql
      componentDef: mysql-8.0
      serviceVersion: "8.0.30"
      replicas: 1
      network:
        hostAliases:
          - ip: "10.10.0.12"
            hostnames:
              - legacy-db.internal
# other component spec fields are omitted for brevity
```

Keep this list small and explicit. Kubernetes applies `hostAliases` when the pod is created, so it is best for stable or short-lived mappings. For normal service discovery, prefer Kubernetes Services or DNS records that can be updated without changing the pod spec.

`hostAliases` is only valid for pods that are not using host networking.

## Configure DNS

Most components should use the Kubernetes default DNS behavior. Configure `dnsPolicy` and `dnsConfig` only when the default `ClusterFirst` policy cannot provide the resolver behavior the pod needs, such as custom nameservers, additional search domains, or resolver options like `ndots`.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-dns
  namespace: demo
spec:
  componentSpecs:
    - name: mysql
      componentDef: mysql-8.0
      serviceVersion: "8.0.30"
      replicas: 1
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
# other component spec fields are omitted for brevity
```

When `dnsPolicy: None` is used, the pod depends on the values in `dnsConfig`; make sure nameservers, search domains, and options are all set intentionally. DNS changes can affect database discovery, replication, backup agents, and client connection paths, so validate the resolver behavior before applying it to production clusters.

## Notes

* `network.hostNetwork: true` requires host-network capability in the referenced `ComponentDefinition`; otherwise the pod stays on normal pod networking.
* `hostPorts` entries are matched by port name declared by the `ComponentDefinition`; unknown names are ignored.
* DNS settings are workload-sensitive. Test database discovery and replication after changing them.
* KubeBlocks keeps compatibility with the earlier annotation-based form. Existing manifests can continue to use `kubeblocks.io/host-network: <component-name>` to enable host networking.
