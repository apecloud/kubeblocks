# Component Network

## Overview

Most KubeBlocks components work well with the default Kubernetes pod network. Pods get their own IPs, clients connect through Services, and Kubernetes DNS handles in-cluster discovery.

Some database deployments need more control. An engine may require host-network addresses, an integration may need a fixed host port, a migration may temporarily depend on legacy hostnames, or a component may need custom DNS resolver settings. Before KubeBlocks 1.1.0, these cases often depended on addon defaults, annotations, or manual pod-level workarounds.

KubeBlocks 1.1.0 adds `spec.componentSpecs[*].network` so users can describe component-level network behavior directly in the `Cluster` spec. KubeBlocks then reconciles the managed `Component` from that cluster-level intent.

Configure network settings from:

* `Cluster.spec.componentSpecs[*].network`

Do not edit `Component.spec.network` directly. `Component` is managed by KubeBlocks and should be used for observation, not as the user-facing configuration entry point.

## When to Use It

Use component network settings only when the default pod network does not match the workload requirement.

| Situation | Recommended setting | Why |
| --- | --- | --- |
| The component only needs normal in-cluster traffic | Leave `network` unset | KubeBlocks uses pod networking and Kubernetes DNS |
| The addon supports host networking and the engine needs node-level addresses | `hostNetwork: true` | Pods share the node network namespace |
| A named container port must be exposed on a fixed node port | `hostPorts` | KubeBlocks maps the named runtime port to the requested host port |
| A migration or hybrid setup needs temporary hostname mapping | `hostAliases` | KubeBlocks adds static entries to the pod `/etc/hosts` file |
| The pod needs custom resolvers or search domains | `dnsPolicy` and `dnsConfig` | The pod uses the resolver behavior you specify |

Start with the default pod network unless you have a concrete requirement for one of these settings. Host networking and host ports introduce node-level port conflicts and scheduling constraints, so they should be used deliberately.

## What You Can Configure

| Field | What it controls | Typical use |
| --- | --- | --- |
| `hostNetwork` | Whether component pods use the node network namespace | Engines or environments that require node-level network access |
| `hostPorts` | Mapping from named container ports to node host ports | Fixed host ports or explicit host-network port values |
| `hostAliases` | Static entries in `/etc/hosts` | Temporary compatibility with legacy or external hostnames |
| `dnsPolicy` | Pod DNS policy | `None`, `ClusterFirst`, or `ClusterFirstWithHostNet` behavior |
| `dnsConfig` | Nameservers, search domains, and resolver options | Custom resolver settings such as `ndots` |

## Enable Host Networking

Host networking only works for addons whose `ComponentDefinition` declares host-network support. Check the `ComponentDefinition` before enabling it:

```bash
kubectl get cmpd <cmpd-name> -o yaml | yq '.spec.hostNetwork'
```

If `spec.hostNetwork` or `spec.hostNetwork.containerPorts` is empty, the addon does not support host networking. Setting `network.hostNetwork: true` in the `Cluster` spec will not force host networking on; KubeBlocks keeps the pod on normal pod networking.

For a supported addon, use the 1.1 API field:

```yaml
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostNetwork: true
```

When host networking is enabled and `dnsPolicy` is not set, KubeBlocks uses `ClusterFirstWithHostNet`.

Existing manifests can still use the earlier annotation form:

```yaml
metadata:
  annotations:
    kubeblocks.io/host-network: mongodb
```

For new manifests, prefer `spec.componentSpecs[*].network.hostNetwork` because it keeps the setting with the rest of the component spec.

## Configure Host Ports

Use `hostPorts` when a component needs a stable node port for a named container port.

For normal pod networking, the `name` must match a port declared in `ComponentDefinition.spec.runtime.containers[*].ports`. Unknown runtime port names are ignored.

```yaml
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostPorts:
          - name: mongodb
            port: 31110
```

With `hostNetwork: true`, `hostPorts` provides explicit host-network port values:

```yaml
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
```

Host port behavior depends on whether host networking is enabled:

| Mode | If `hostPorts` is empty | If `hostPorts` is set |
| --- | --- | --- |
| `hostNetwork: true` | KubeBlocks automatically allocates all ports required by `ComponentDefinition.spec.hostNetwork` | You must provide mappings for all ports defined by `ComponentDefinition.spec.hostNetwork` |
| Host networking disabled | No host ports are allocated unless requested | Mappings are allowed only for names declared in `ComponentDefinition.spec.runtime.containers[*].ports`; unknown names are ignored |

Allocated host-network ports are recorded in a ConfigMap in the KubeBlocks namespace. The default automatic allocation range in 1.1.0 is `55000-59999`, controlled by Helm values `hostPorts.include` and `hostPorts.exclude`.

## Add Host Aliases

Use `hostAliases` when a pod needs a small number of fixed hostname-to-IP entries in `/etc/hosts`.

This is mainly for temporary compatibility cases: migration from an old database endpoint, hybrid connectivity testing, or keeping a legacy hostname working while DNS records are being moved.

```yaml
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      network:
        hostAliases:
          - ip: "10.10.0.12"
            hostnames:
              - legacy-db.internal
```

Keep this list small and explicit. Kubernetes applies `hostAliases` when the pod is created, so it works best for stable or short-lived mappings. For normal service discovery, prefer Kubernetes Services or DNS records that can be updated without changing the pod spec.

`hostAliases` is only valid for pods that are not using host networking.

## Configure DNS

Most components should use the Kubernetes default DNS behavior. Configure `dnsPolicy` and `dnsConfig` only when the default resolver behavior is not correct for the workload.

Use this for custom nameservers, additional search domains, or resolver options such as `ndots`:

```yaml
spec:
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
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
```

When `dnsPolicy: None` is used, the pod depends on the values in `dnsConfig`; make sure nameservers, search domains, and options are all set intentionally. DNS changes can affect database discovery, replication, backup agents, and client connection paths, so validate resolver behavior before applying it to production clusters.

## Limitations and Notes

* `network.hostNetwork: true` requires host-network capability in the referenced `ComponentDefinition`; otherwise the pod stays on normal pod networking.
* Host networking can create node-level port conflicts and scheduling constraints.
* `hostPorts` entries are matched by port names declared by the `ComponentDefinition`.
* `hostAliases` should be temporary and small. Prefer Services or DNS for long-term discovery.
* DNS settings are workload-sensitive. Test database discovery and replication after changing them.
