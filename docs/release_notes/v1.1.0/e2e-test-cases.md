# KubeBlocks 1.1 Key Feature E2E Test Cases

This document defines end-to-end test cases for the key KubeBlocks 1.1 features:

1. Multi-cluster support through the Instance API
2. Experimental Rollout API
3. ComponentNetwork API
4. Dynamic InstanceSet template adoption
5. Sharding lifecycle actions
6. Heterogeneous shards and shard-specific scale-in
7. KubeBlocks upgrade from 1.0.x to 1.1

Each case is written so it can be executed manually first and later converted into automated e2e tests. For release sign-off, each executed case must record the exact KubeBlocks chart/image version, Kubernetes version, cloud/provider or local environment, commands, relevant object YAML or JSON snippets, and the final pass/fail result.

## Shared Test Environment

### Required Tools

* `kubectl`
* `helm`
* `jq`
* `yq`, recommended for YAML snapshots
* `grpcurl`, only for optional config-manager checks
* `kbcli`, if addon installation or database-specific checks use kbcli workflows

### Required Clusters

| Test Area | Cluster Requirement |
| --- | --- |
| Multi-cluster | 1 control Kubernetes cluster and 2 data Kubernetes clusters |
| Other features | 1 Kubernetes cluster with at least 3 schedulable worker nodes |

### Shared Variables

```bash
export KB_NAMESPACE=kb-system
export TEST_NAMESPACE=kb-11-e2e
export CONTROL_CONTEXT=control
export DATA_CONTEXT_A=data-a
export DATA_CONTEXT_B=data-b
```

Create the test namespace:

```bash
kubectl create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
```

For multi-cluster tests, create the namespace in every data cluster:

```bash
kubectl --context ${DATA_CONTEXT_A} create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl --context ${DATA_CONTEXT_A} apply -f -
kubectl --context ${DATA_CONTEXT_B} create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl --context ${DATA_CONTEXT_B} apply -f -
```

### Execution Order

Run the upgrade test (E2E-7) in a clean cluster because it starts from KubeBlocks 1.0.x. Run the feature tests (E2E-1 to E2E-6) against the final KubeBlocks 1.1.0 chart, CRDs, and images.

Recommended order:

1. E2E-7 upgrade validation on a dedicated cluster.
2. E2E-1 multi-cluster validation on one control cluster and two data clusters.
3. E2E-2 to E2E-6 on the primary single-cluster e2e environment.
4. Repeat the high-risk cases (E2E-1, E2E-2, E2E-5, E2E-7) on the release-blocking Kubernetes versions if the release matrix includes more than one version.

### Evidence to Capture

Before each case:

```bash
kubectl version
helm -n ${KB_NAMESPACE} list
kubectl -n ${KB_NAMESPACE} get deploy,pod -o wide
kubectl get crd | grep kubeblocks
```

During and after each case, capture the objects that prove behavior instead of relying only on visual observation:

```bash
kubectl get clusters.apps.kubeblocks.io,components.apps.kubeblocks.io,instancesets.workloads.kubeblocks.io,instances.workloads.kubeblocks.io,rollouts.apps.kubeblocks.io -n ${TEST_NAMESPACE} -o yaml
kubectl get pod,pvc -n ${TEST_NAMESPACE} -o wide
kubectl -n ${KB_NAMESPACE} logs deploy/kubeblocks --tail=300
```

If a case fails, keep the namespace until `kubectl describe` output, controller logs, and the relevant CR YAML have been saved. Do not reuse a failed namespace for another case unless the failure has been triaged and cleanup is complete.

### Shared Cleanup

```bash
kubectl delete namespace ${TEST_NAMESPACE} --ignore-not-found
```

For multi-cluster tests, run cleanup on all data-cluster contexts as well:

```bash
kubectl --context ${DATA_CONTEXT_A} delete namespace ${TEST_NAMESPACE} --ignore-not-found
kubectl --context ${DATA_CONTEXT_B} delete namespace ${TEST_NAMESPACE} --ignore-not-found
```

## E2E-1: Multi-Cluster Support Through the Instance API

### Purpose

Verify that one KubeBlocks control plane can create and manage database instances across multiple Kubernetes data clusters through the Instance API.

### Prerequisites

* KubeBlocks 1.1 is installed in the control cluster with multi-cluster enabled.
* The control cluster has the test addon and `ComponentDefinition`, for example the etcd addon with `componentDef: etcd-3-1.1.0-alpha.0`.
* Data clusters are reachable from the control cluster through kubeconfig contexts.
* Data clusters run KubeBlocks with `autoInstalledAddons=[]`; they only need the `Instance` CRD, required RBAC, storage class, network access, and the controller path that reconciles distributed instances.
* The test `ComponentDefinition` in the control cluster supports `enableInstanceAPI`.

### Test Data

Create a kubeconfig secret in the control cluster:

```bash
kubectl --context ${CONTROL_CONTEXT} -n ${KB_NAMESPACE} create secret generic kubeblocks-multicluster-kubeconfig \
  --from-file=kubeconfig=/path/to/multicluster.kubeconfig
```

Install or upgrade KubeBlocks in the control cluster with:

```yaml
autoInstalledAddons:
  - etcd
multiCluster:
  kubeConfig: kubeblocks-multicluster-kubeconfig
  mountPath: /var/run/secrets/kubeblocks.io/multicluster
  contexts: data-a,data-b
  contextsDisabled: ""
```

Install or upgrade KubeBlocks in each data cluster with addons disabled:

```yaml
autoInstalledAddons: []
```

Create a cluster:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: etcd-mc
  namespace: kb-11-e2e
  annotations:
    apps.kubeblocks.io/multi-cluster-placement: data-a,data-b
spec:
  terminationPolicy: Delete
  componentSpecs:
    - name: etcd
      componentDef: etcd-3-1.1.0-alpha.0
      serviceVersion: "3.6.1"
      enableInstanceAPI: true
      replicas: 2
      resources:
        requests:
          cpu: "500m"
          memory: 512Mi
        limits:
          cpu: "500m"
          memory: 512Mi
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 10Gi
```

### Steps

1. Create the cluster in the control cluster.
2. Wait until the KubeBlocks `Cluster` reaches `Running`.
3. List `Instance` objects in the control cluster.
4. List pods and PVCs in each data cluster.
5. Delete one database pod in data cluster A.
6. Confirm the pod is recreated in data cluster A.
7. Disable data cluster B by updating Helm `multiCluster.contextsDisabled` to `data-b`, then restart the manager.
8. Scale the component from 2 replicas to 3 replicas.
9. Confirm the new instance is not placed into disabled data cluster B.

### Commands

```bash
kubectl --context ${CONTROL_CONTEXT} create -f etcd-mc.yaml
kubectl --context ${CONTROL_CONTEXT} wait --for=jsonpath='{.status.phase}'=Running cluster/etcd-mc -n ${TEST_NAMESPACE} --timeout=20m
kubectl --context ${CONTROL_CONTEXT} get instances.workloads.kubeblocks.io -n ${TEST_NAMESPACE} -o wide
kubectl --context ${CONTROL_CONTEXT} get instances.workloads.kubeblocks.io -n ${TEST_NAMESPACE} -o json | jq '.items[] | {name: .metadata.name, placement: .metadata.annotations["apps.kubeblocks.io/multi-cluster-placement"]}'
kubectl --context ${DATA_CONTEXT_A} get pod,pvc -n ${TEST_NAMESPACE}
kubectl --context ${DATA_CONTEXT_B} get pod,pvc -n ${TEST_NAMESPACE}
```

### Expected Results

* `Cluster/etcd-mc` reaches `Running` in the control cluster.
* Two `Instance` objects exist in the control cluster.
* Each distributed object carries the annotation `apps.kubeblocks.io/multi-cluster-placement: <context>`; instances are assigned round-robin by ordinal across the listed contexts (ordinal 0 → `data-a`, ordinal 1 → `data-b`).
* Runtime pods and PVCs exist in the data clusters, not only in the control cluster.
* Data clusters do not require the etcd addon or etcd `ComponentDefinition`; absence of addon resources in data clusters must not block runtime placement.
* Pod deletion in a data cluster is repaired by KubeBlocks.
* After `data-b` is disabled, new instances are not scheduled to data cluster B.
* Existing instances in `data-b` are not moved merely because the context is disabled; disabling affects new placement.
* Cluster status in the control cluster reflects the aggregate status of instances in data clusters.

### E2E-1B: Placement Validation and Auto-Assignment

1. Create a cluster with the placement annotation but with `enableInstanceAPI` unset (or `false`) on a component. Expected: reconciliation fails with `the multi-cluster object is only supported for components that enable the instance API: <comp>`; the cluster does not provision pods.
2. Create a cluster with an empty placement annotation value (`apps.kubeblocks.io/multi-cluster-placement: ""`) and `enableInstanceAPI: true`. Expected: KubeBlocks auto-assigns contexts and writes them back into the cluster annotation; the number of assigned contexts is bounded by the largest component replica count.

### Failure Checks

* Manager logs must not contain repeated data-cluster authentication or discovery errors.
* No duplicate `Instance` objects should be created for the same ordinal.
* No PVC should be created in the wrong namespace or wrong data cluster.

## E2E-2: Experimental Rollout API

### Purpose

Verify that `Rollout` can update component instances with `inplace`, `replace`, and `create` strategies, report status, preserve the intended `ComponentDefinition`, and roll out sharded clusters.

### Prerequisites

* KubeBlocks 1.1 is installed.
* A test database cluster can be created with at least 2 replicas.
* The selected addon has at least two valid `serviceVersion` values or two compatible `ComponentDefinition` revisions.
* When multiple `ComponentDefinition` objects can match the same service and version, every rollout target must specify `compDef` explicitly.

### Test Data

Base MySQL cluster:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mysql-cluster
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  componentSpecs:
    - name: mysql
      componentDef: mysql-8.0-1.0.3
      serviceVersion: "8.0.35"
      replicas: 2
      resources:
        requests:
          cpu: 500m
          memory: 512Mi
        limits:
          cpu: 500m
          memory: 512Mi
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
```

Create the base cluster and record the baseline:

```bash
kubectl create -f mysql-rollout-base.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Running cluster/mysql-cluster -n ${TEST_NAMESPACE} --timeout=20m
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\n"}'
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o json \
  | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, created: .metadata.creationTimestamp, images: [.spec.containers[] | {name, image}]}'
```

Expected baseline:

* `Cluster.status.phase` is `Running`.
* `Component` is `mysql-8.0-1.0.3` and `serviceVersion` is `8.0.35`.
* Two MySQL pods are `4/4 Running`.

### E2E-2A: In-place Rollout

Create the rollout:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-inplace-8036
  namespace: kb-11-e2e
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      compDef: mysql-8.0-1.0.3
      serviceVersion: "8.0.36"
      strategy:
        inplace: {}
```

Steps:

1. Create `mysql-inplace-8036`.
2. Wait until `Rollout.status.state` is `Succeed`.
3. Compare pod UIDs before and after.
4. Verify the `mysql` and `kbagent` container images are `8.0.36`.
5. Delete the finished rollout before starting the next strategy test.

Commands:

```bash
kubectl create -f mysql-inplace-8036.yaml
kubectl wait rollout/mysql-inplace-8036 -n ${TEST_NAMESPACE} --for=jsonpath='{.status.state}'=Succeed --timeout=20m
kubectl get rollout mysql-inplace-8036 -n ${TEST_NAMESPACE} -o yaml
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\n"}'
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o json \
  | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, restarts: [.status.containerStatuses[] | {name, restartCount}], images: [.spec.containers[] | {name, image}]}'
kubectl delete rollout mysql-inplace-8036 -n ${TEST_NAMESPACE}
```

Expected results:

* `Rollout.status.state` is `Succeed`.
* `spec.componentSpecs[0].componentDef` remains `mysql-8.0-1.0.3`.
* Pod names and UIDs do not change.
* `mysql` and `kbagent` containers are restarted and use the `8.0.36` image.

### E2E-2B: Replace Rollout

Create the rollout:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-replace-8037
  namespace: kb-11-e2e
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      compDef: mysql-8.0-1.0.3
      serviceVersion: "8.0.37"
      strategy:
        replace:
          perInstanceIntervalSeconds: 30
          scaleDownDelaySeconds: 30
```

Steps:

1. Create `mysql-replace-8037`.
2. Watch `Rollout.status.state`, `status.components[*].newReplicas`, and `rolledOutReplicas`.
3. During rollout, verify old and new pods overlap temporarily.
4. Verify the next pod is not updated until at least 30 seconds after the previous new pod becomes Ready. Compare pod `Ready` condition `lastTransitionTime` values from the post-rollout pod snapshot.
5. Wait until rollout reaches `Succeed`.
6. Confirm all running pods use `8.0.37`.
7. Delete the finished rollout before starting the next strategy test.

Commands:

```bash
kubectl create -f mysql-replace-8037.yaml
kubectl get rollout mysql-replace-8037 -n ${TEST_NAMESPACE} -w
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\treplicas="}{.spec.componentSpecs[0].replicas}{"\tinstances="}{range .spec.componentSpecs[0].instances[*]}{.name}:{.compDef}:{.serviceVersion}:{.replicas}{" "}{end}{"\n"}'
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o wide
kubectl wait rollout/mysql-replace-8037 -n ${TEST_NAMESPACE} --for=jsonpath='{.status.state}'=Succeed --timeout=25m
kubectl get rollout mysql-replace-8037 -n ${TEST_NAMESPACE} -o yaml
kubectl delete rollout mysql-replace-8037 -n ${TEST_NAMESPACE}
```

Expected results:

* The cluster is labeled `apps.kubeblocks.io/rollout-name: mysql-replace-8037` while the rollout is active.
* The component spec temporarily contains a rollout-managed instance template (name derived from the rollout UID, annotated `apps.kubeblocks.io/instance-template-created-by-rollout`), and `flatInstanceOrdinal` is enabled.
* New pods carry the pod label `apps.kubeblocks.io/instance-template: <rollout-template-name>`; old pods do not.
* Replacement proceeds one instance at a time: scale up one new instance, wait for the component to become Running, wait the configured `perInstanceIntervalSeconds` (`30s` in this test), then proceed to the next instance.
* `Rollout.status.state` moves from `Pending` to `Rolling` to `Succeed`.
* `status.components[*].newReplicas` and `rolledOutReplicas` advance to the original replica count; scaled-down instances are recorded in `status.components[*].scaleDownInstances`.
* On success, stable pods are newly created pods using the target image.
* The cluster returns to `Running`.
* `componentDef` remains `mysql-8.0-1.0.3`; it must not drift to another matching definition.
* No PVC is accidentally deleted unless the workload policy requires it.

### E2E-2C: Create/Canary Rollout

Create the rollout:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-create-8038
  namespace: kb-11-e2e
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      compDef: mysql-8.0-1.0.3
      serviceVersion: "8.0.38"
      replicas: 1
      strategy:
        create:
          canary: true
          promotion:
            auto: true
            delaySeconds: 0
            scaleDownDelaySeconds: 0
      instanceMeta:
        canary:
          labels:
            traffic: canary
          annotations:
            rollout.apps.kubeblocks.io/version: "8.0.38"
```

Steps:

1. Create `mysql-create-8038`.
2. Verify one canary instance is created.
3. Confirm canary labels and annotations are present on the canary pod or instance.
4. Wait for automatic promotion.
5. Confirm the rollout reaches `Succeed`.
6. Confirm the cluster returns to `Running`.
7. Record the final component default version and named instance templates.

Commands:

```bash
kubectl create -f mysql-create-8038.yaml
kubectl get rollout mysql-create-8038 -n ${TEST_NAMESPACE} -w
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\treplicas="}{.spec.componentSpecs[0].replicas}{"\tinstances="}{range .spec.componentSpecs[0].instances[*]}{.name}:{.compDef}:{.serviceVersion}:{.replicas}:{.canary}{" "}{end}{"\n"}'
kubectl wait rollout/mysql-create-8038 -n ${TEST_NAMESPACE} --for=jsonpath='{.status.state}'=Succeed --timeout=25m
kubectl wait cluster/mysql-cluster -n ${TEST_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=15m
kubectl get rollout mysql-create-8038 -n ${TEST_NAMESPACE} -o yaml
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o json \
  | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, template: .metadata.labels["apps.kubeblocks.io/instance-template"], serviceVersionLabel: .metadata.labels["apps.kubeblocks.io/service-version"], images: [.spec.containers[] | {name, image}]}'
kubectl delete rollout mysql-create-8038 -n ${TEST_NAMESPACE}
```

Expected results:

* Exactly one canary instance is created before promotion, identified by the pod label `apps.kubeblocks.io/instance-template: <rollout-template-name>`.
* Canary metadata from `instanceMeta.canary` (labels/annotations) is applied to the canary pod.
* The rollout stays in `Rolling` while the canary template's `canary` flag is set; after `delaySeconds`, promotion clears the flag and scales down old instances according to `scaleDownDelaySeconds`.
* When `replicas` is smaller than the stable replica count, success can leave the component intentionally heterogeneous: the component-level `serviceVersion` remains on the previous stable version, and the promoted rollout-managed instance template keeps the new version for only the promoted replicas.
* `status.components[*].canaryReplicas` and `rolledOutReplicas` reflect progress.
* `componentDef` remains `mysql-8.0-1.0.3`; it must not drift to another matching definition.

Variation, manual promotion: create the same rollout with `promotion.auto: false`. Expected: the canary is created and the rollout stays in `Rolling` indefinitely; after validating the canary, edit the rollout to set `promotion.auto: true` and verify promotion completes as above.

Constraint check: a rollout with `promotion.condition` set must fail with a "not supported" error (`promotion.condition` is not supported in 1.1).

### E2E-2D: Sharding Rollout

Create a sharded cluster with two shards, then create:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: redis-sharding-rollout
  namespace: kb-11-e2e
spec:
  clusterName: redis-sharding
  shardings:
    - name: shard
      compDef: redis
      serviceVersion: "7.2.5"
      strategy:
        replace:
          perInstanceIntervalSeconds: 30
          scaleDownDelaySeconds: 30
```

Expected results:

* `status.shardings` is populated.
* Each shard is rolled out.
* Shard instances are rolled one at a time with at least 30 seconds between successive instance updates.
* No shard remains in stale `Updating` or `Failed` state.
* The sharded cluster returns to `Running`.

### E2E-2E: Concurrency and Abort Semantics

1. While a rollout is `Rolling`, create a second rollout against the same cluster. Expected: the second rollout enters `Error` with message `the cluster mysql-cluster is already bound to rollout <active-rollout>`; the first rollout is unaffected.
2. Delete a rollout while it is `Rolling`. Expected: the `apps.kubeblocks.io/rollout-name` label is removed from the cluster; changes already applied to the cluster spec are **not** reverted (document the resulting spec); the cluster eventually converges to `Running`.
3. After a rollout reaches `Succeed`, delete the finished rollout and then create another rollout targeting a new or previous version. Expected: the new rollout is accepted because the binding is released.

### Failure Checks

* A rollout that defines no strategy, more than one strategy, or neither `serviceVersion` nor `compDef` for a target must fail with a clear error.
* A `replace`/`inplace` rollout with `replicas` set to less than the full replica count must fail (partial rollout is not supported).
* A rollout with an invalid target component or sharding should enter `Error` with a clear message.
* A rollout that omits `compDef` in an addon with several compatible definitions should be treated as unsafe for release validation, even if it is accepted by the API. Record the selected `spec.componentSpecs[*].componentDef` before and after the rollout.
* Manager restart during rollout should not lose rollout progress (progress is derived from the cluster spec and `status.components`).

## E2E-3: ComponentNetwork API

### Purpose

Verify component-level network settings: host networking, explicit host ports, host aliases, DNS policy, and DNS config.

### Prerequisites

* At least 3 schedulable nodes.
* A ComponentDefinition that declares host-network capability for hostNetwork subcases.
* KubeBlocks host port include/exclude ranges are configured or defaults are acceptable.

### E2E-3A: HostNetwork With Auto Host Port Allocation

Create:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-auto
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 2
      network:
        hostNetwork: true
```

Expected results:

* Pods have `spec.hostNetwork: true`.
* Pod DNS policy defaults to `ClusterFirstWithHostNet` unless explicitly set.
* Required host-network ports are allocated from the configured include/exclude ranges (default `55000-59999`); container ports in the pod spec are rewritten to the allocated values.
* Allocations are recorded in the host-port ConfigMap in the KubeBlocks namespace, keyed `<cluster>-<comp>-<container>-<portName>`.
* Two replicas do not collide on the same node and host port.
* Deleting the cluster removes its entries from the host-port ConfigMap.

Negative check: apply the same spec with a ComponentDefinition that does **not** declare `spec.hostNetwork` capability. Expected: the `network.hostNetwork` setting has no effect (pods run with normal pod networking).

### E2E-3B: Explicit Host Ports

Create:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-ports
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 1
      network:
        hostNetwork: true
        hostPorts:
          - name: client
            port: 31001
          - name: metrics
            port: 31002
```

Steps:

1. Apply the cluster.
2. Confirm pod reaches `Running`.
3. Inspect pod container ports.
4. Create another cluster on the same node with the same host port.

Expected results:

* First cluster uses the requested host ports. Explicit `hostPorts` are matched by port **name** against the ComponentDefinition container ports.
* The kbagent ports (`http`, `streaming`) need not be listed; they fall back to automatic allocation.
* If a non-kbagent host-network port name has no `hostPorts` entry, reconciliation fails with `no available port`.
* Conflicting host port allocation is rejected, rescheduled, or reported with a clear error.
* Deleting the first cluster releases the port for later use.

Runtime-port check: set `hostPorts` with `hostNetwork: false` and a port name declared in `ComponentDefinition.spec.runtime.containers[*].ports`. Expected: the pod keeps pod networking, but the matching runtime container port gets the requested `hostPort`; unknown runtime port names are ignored.

### E2E-3C: Host Aliases and DNS Config

Create:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-dns
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 1
      network:
        hostAliases:
          - ip: "10.10.0.12"
            hostnames:
              - legacy-db.internal
        dnsPolicy: None
        dnsConfig:
          nameservers:
            - 10.96.0.10
          searches:
            - kb-11-e2e.svc.cluster.local
            - svc.cluster.local
          options:
            - name: ndots
              value: "2"
```

Expected results:

* Pod spec contains the configured `hostAliases`.
* Pod spec contains `dnsPolicy: None`.
* Pod spec contains the configured `dnsConfig`.
* Inside the pod, `legacy-db.internal` resolves to `10.10.0.12`.

### Commands

```bash
kubectl get pod -n ${TEST_NAMESPACE} -o wide
kubectl get pod <pod-name> -n ${TEST_NAMESPACE} -o json | jq '.spec.hostNetwork,.spec.hostAliases,.spec.dnsPolicy,.spec.dnsConfig'
kubectl get pod <pod-name> -n ${TEST_NAMESPACE} -o json | jq '.spec.containers[].ports'
kubectl exec -n ${TEST_NAMESPACE} <pod-name> -- getent hosts legacy-db.internal
```

## E2E-4: Dynamic InstanceSet Template Adoption

### Purpose

Verify that existing pods can be dynamically adopted by named instance templates while preserving pod identity and PVC data.

### Prerequisites

* A component that supports multiple replicas.
* StorageClass supports the test storage behavior.
* `flatInstanceOrdinal: true` is set.

### Base Cluster

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: adopt-demo
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: cluster
  componentSpecs:
    - name: foo
      componentDef: foo
      replicas: 3
      flatInstanceOrdinal: true
      ordinals:
        discrete: [0, 1, 2]
      resources:
        requests:
          cpu: "500m"
          memory: 512Mi
        limits:
          cpu: "500m"
          memory: 512Mi
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 10Gi
```

### E2E-4A: Adopt One Pod Into a High-Resource Template

Patch the cluster:

```yaml
spec:
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
          resources:
            requests:
              cpu: "2"
              memory: 2Gi
            limits:
              cpu: "2"
              memory: 2Gi
```

Steps:

1. Apply the base cluster and wait for `Running`.
2. Record pod names, pod UIDs, PVC names, and PVC UIDs.
3. Patch the cluster to add `instances[0].name=highperf` with ordinal `2`.
4. Wait for reconciliation.
5. Inspect pod `foo-2`.

Commands:

```bash
kubectl apply -f adopt-demo.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Running cluster/adopt-demo -n ${TEST_NAMESPACE} --timeout=20m
kubectl get pod,pvc -n ${TEST_NAMESPACE} -o json | jq '.items[] | {kind: .kind, name: .metadata.name, uid: .metadata.uid}'
kubectl patch cluster adopt-demo -n ${TEST_NAMESPACE} --type merge --patch-file adopt-highperf-patch.yaml
kubectl get instanceset -n ${TEST_NAMESPACE} -o json | jq '.items[] | {name: .metadata.name, assignedOrdinals: .status.assignedOrdinals}'
kubectl get pod foo-2 -n ${TEST_NAMESPACE} -o json | jq '{name: .metadata.name, uid: .metadata.uid, template: .metadata.labels["apps.kubeblocks.io/instance-template"], resources: .spec.containers[].resources}'
```

Expected results:

* Pod name remains `foo-2`.
* `foo-2` is associated with the `highperf` template (pod label `apps.kubeblocks.io/instance-template: highperf`).
* `InstanceSet.status.assignedOrdinals` records ordinal `2` under `highperf` and ordinals `0,1` under the default template; assignments are sticky across reconciliations.
* PVC name and UID for ordinal `2` are preserved.
* New resource requests/limits are applied to `foo-2`.
* `foo-0` and `foo-1` remain on the default template.

### E2E-4B: Return the Pod to the Default Template

Patch the cluster so all ordinals are back in the default template and remove `instances`.

Expected results:

* `foo-2` returns to the default template.
* Pod name remains `foo-2`.
* PVC data remains attached to ordinal `2`.
* No extra pods or PVCs remain.

### E2E-4C: Dynamically Change a MySQL Hot Instance Template

This case validates the same behavior on a real addon and can be run after E2E-2C, where a `create` rollout leaves a promoted MySQL instance template.

Steps:

1. Pick one named MySQL instance template, for example the template created by a successful `create` rollout.
2. Record the pod UID, service-version label, container images, and container resource requests.
3. Replace the `Cluster` object and add a hot-instance override to that template:

```yaml
instances:
  - name: <rollout-template-name>
    compDef: mysql-8.0-1.0.3
    serviceVersion: "8.0.38"
    replicas: 1
    labels:
      workload-tier: hot
    resources:
      requests:
        cpu: 750m
        memory: 768Mi
      limits:
        cpu: 750m
        memory: 768Mi
```

Commands:

```bash
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o yaml > /tmp/mysql-hot-before.yaml
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o json \
  | jq '(.spec.componentSpecs[0].instances[] | select(.name=="<rollout-template-name>").labels) = {"workload-tier":"hot"}
        | (.spec.componentSpecs[0].instances[] | select(.name=="<rollout-template-name>").resources) = {"requests":{"cpu":"750m","memory":"768Mi"},"limits":{"cpu":"750m","memory":"768Mi"}}' \
  | kubectl replace -f -
kubectl wait cluster/mysql-cluster -n ${TEST_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=15m
kubectl get pod -n ${TEST_NAMESPACE} -l workload-tier=hot --show-labels -o wide
kubectl get pod -n ${TEST_NAMESPACE} -l workload-tier=hot -o json \
  | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, serviceVersionLabel: .metadata.labels["apps.kubeblocks.io/service-version"], template: .metadata.labels["workloads.kubeblocks.io/template-name"], images: [.spec.containers[] | {name, image, resources}]}'
```

Expected results:

* The selected pod carries `workload-tier=hot`.
* The first workload container uses the hot resource requests and limits.
* The pod may be recreated because resource changes affect the pod template; record old and new UIDs.
* The pod image should match the instance template's `serviceVersion`.
* Also verify the pod's `apps.kubeblocks.io/service-version` label. A mismatch between the label and the actual image should be recorded as a release issue.

### E2E-4D: Adopt a Shard by Shard ID

For a sharded cluster, discover an existing shard ID and adopt only that shard:

Shard components are named `<cluster>-<sharding>-<id>`; the shard ID is the trailing 3-character segment:

```bash
export TARGET_SHARD_ID=$(
  kubectl get components -n ${TEST_NAMESPACE} \
    -l apps.kubeblocks.io/sharding-name=shard \
    -o jsonpath='{.items[0].metadata.name}' \
    | awk -F- '{print $NF}'
)

test -n "${TARGET_SHARD_ID}"
```

Patch the cluster with the selected shard ID:

```yaml
spec:
  shardings:
    - name: shard
      shards: 3
      template:
        componentDef: foo
        replicas: 1
      shardTemplates:
        - name: highperf
          shards: 1
          shardIDs:
            - ${TARGET_SHARD_ID}
          replicas: 2
          resources:
            requests:
              cpu: "2"
              memory: 2Gi
            limits:
              cpu: "2"
              memory: 2Gi
```

Expected results:

* Only the selected shard is adopted by `highperf` and gains the label `apps.kubeblocks.io/shard-template: highperf`.
* Other shards remain on the default sharding template.
* Selected shard keeps its shard identity (same component name and shard ID).
* The selected shard's component name and PVC ownership remain stable.

### Commands

```bash
kubectl get pod,pvc -n ${TEST_NAMESPACE}
kubectl get instanceset -n ${TEST_NAMESPACE} -o yaml
kubectl get cluster adopt-demo -n ${TEST_NAMESPACE} -o yaml
```

### Failure Checks

* Declaring the same ordinal in two templates must fail validation with `duplicate ordinal(<n>)`.
* A template whose `ordinals` provide fewer available ordinals than `replicas` must fail with `template(<name>) has available ordinals less than replicas`.
* If templates without explicit `ordinals` cannot obtain enough free ordinals (e.g. ordinals are being released), the InstanceSet emits a Warning event `OrdinalsNotEnough` and converges once ordinals free up.
* Scale-in without explicit ordinals removes the highest ordinals first.

## E2E-5: Sharding Lifecycle Actions

### Purpose

Verify that `postProvision`, `preTerminate`, `shardAdd`, and `shardRemove` actions run at the correct points in the sharding lifecycle.

### Prerequisites

* A sharding-capable ComponentDefinition.
* An action image that can write observable markers to logs or an external endpoint.
* The test action commands are idempotent.

### Test ShardingDefinition

Use an action that writes the action name and shard env variable to stdout:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ShardingDefinition
metadata:
  name: e2e-sharding
spec:
  template:
    compDef: foo
  lifecycleActions:
    postProvision:
      targetShardSelector: Any
      exec:
        command:
          - /bin/sh
          - -c
          - echo "E2E_POST_PROVISION"
    preTerminate:
      targetShardSelector: All
      exec:
        command:
          - /bin/sh
          - -c
          - echo "E2E_PRE_TERMINATE"
    shardAdd:
      targetShardSelector: Any
      exec:
        command:
          - /bin/sh
          - -c
          - echo "E2E_SHARD_ADD ${KB_ADD_SHARD_NAME}"
    shardRemove:
      targetShardSelector: Any
      exec:
        command:
          - /bin/sh
          - -c
          - echo "E2E_SHARD_REMOVE ${KB_REMOVE_SHARD_NAME}"
```

### Test Cluster

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: shard-life
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: sharding
  shardings:
    - name: shard
      shardingDef: e2e-sharding
      shards: 2
      template:
        componentDef: foo
        replicas: 1
```

### E2E-5A: postProvision Runs After Initial Sharding Creation

Steps:

1. Apply the `ShardingDefinition`.
2. Apply the sharded cluster.
3. Wait for the cluster to become `Running`.
4. Inspect cluster status and action logs.

Expected results:

* `postProvision` runs once.
* `status.shardings[*].postProvision.phase` reaches `Succeeded`.
* The action log contains `E2E_POST_PROVISION`.

### E2E-5B: shardAdd Runs When Shard Count Increases

Steps:

1. Patch `spec.shardings[0].shards` from `2` to `3`.
2. Wait for the new shard component to be created and ready.
3. Inspect action logs and cluster status.

Commands:

```bash
kubectl patch cluster shard-life -n ${TEST_NAMESPACE} --type merge -p '{"spec":{"shardings":[{"name":"shard","shards":3}]}}'
kubectl get components -n ${TEST_NAMESPACE} -l apps.kubeblocks.io/sharding-name=shard -w
kubectl get components -n ${TEST_NAMESPACE} -l apps.kubeblocks.io/sharding-name=shard -o json | jq '.items[] | {name: .metadata.name, add: .metadata.annotations["kubeblocks.io/sharding-add-shard"]}'
kubectl get cluster shard-life -n ${TEST_NAMESPACE} -o yaml
```

Expected results:

* Exactly one new shard component is created, named `<cluster>-<sharding>-<id>` with a generated 3-character shard ID.
* The new shard component initially carries the annotation `kubeblocks.io/sharding-add-shard: <timestamp>`; the annotation is removed once `shardAdd` succeeds.
* `shardAdd` runs for the new shard with `KB_ADD_SHARD_NAME` set to the new component name.
* The action log contains `E2E_SHARD_ADD <new-shard-name>`.
* Cluster returns to `Running`.

### E2E-5C: shardRemove Runs Before Shard Deletion

Steps:

1. Record existing shard component names.
2. Patch `spec.shardings[0].shards` from `3` to `2`.
3. Watch action logs and component deletion.

Expected results:

* `shardRemove` runs before the selected shard component is deleted.
* The action log contains `E2E_SHARD_REMOVE <removed-shard-name>`.
* Remaining shards continue running.
* Cluster returns to `Running`.

### E2E-5D: preTerminate Runs Before Cluster Deletion

Steps:

1. Delete the sharded cluster.
2. Watch action logs and final resource cleanup.

Expected results:

* `preTerminate` runs before sharding resources are removed.
* The action log contains `E2E_PRE_TERMINATE`.
* Components, pods, and PVCs are cleaned according to `terminationPolicy` and PVC retention policy.

### Failure Checks

* If `postProvision` or `preTerminate` exits non-zero, `status.shardings[*].<action>.phase` reports `Failed` with the error message; the controller retries.
* A failed `shardRemove` blocks deletion of that shard component; the shard stays until the action succeeds.
* If a shard is removed before its `shardAdd` ever completed (annotation still present), `shardRemove` is skipped for it.
* If `postProvision` did not succeed, `preTerminate` is recorded as `Skipped` with message `the PostProvision action is not succeeded`.
* Actions not defined in the `ShardingDefinition` are recorded as `Skipped` (for `postProvision`/`preTerminate`) rather than failing.
* Manager restart during action execution should not run a completed action repeatedly.

## E2E-6: Heterogeneous Shards and Shard-Specific Scale-In

### Purpose

Verify that `spec.shardings[*].shardTemplates` creates shard groups with distinct configurations, and `spec.shardings[*].offline` removes exactly the named shard.

### Prerequisites

* A sharding-capable ComponentDefinition.
* Reuse the `ShardingDefinition` from E2E-5 so `shardRemove` evidence is also captured.

### Test Cluster

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: shard-hetero
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  clusterDef: foo
  topology: sharding
  shardings:
    - name: shard
      shardingDef: e2e-sharding
      shards: 3
      template:
        componentDef: foo
        replicas: 1
        resources:
          requests: { cpu: 500m, memory: 512Mi }
          limits: { cpu: 500m, memory: 512Mi }
      shardTemplates:
        - name: hot
          shards: 1
          replicas: 1
          resources:
            requests: { cpu: "1", memory: 1Gi }
            limits: { cpu: "1", memory: 1Gi }
```

### E2E-6A: Shard Template Creates a Heterogeneous Shard Group

Steps:

1. Apply the cluster.
2. Wait for `Running`.
3. List shard components and inspect their pod resources.

Expected results:

* Three shards exist in total: one from the `hot` template, two from the base template (`shards` minus the sum of `shardTemplates[*].shards`).
* The `hot` shard component carries the label `apps.kubeblocks.io/shard-template: hot`; base-template shards do not carry the label.
* Exactly one shard's pods request `cpu: 1, memory: 1Gi`; the other shards keep base-template resources.
* `status.shardings` reports the expected shard counts.

### E2E-6B: Shard Template Version Canary

Steps:

1. Patch the `hot` shard template with a different `serviceVersion` (or `compDef`).
2. Wait for reconciliation.

Expected results:

* Only the pods of the `hot` shard group are updated to the new version.
* Other shards show no pod restarts (compare pod UIDs before and after).

### E2E-6C: Scale In a Specific Shard via offline

Steps:

1. Record all shard component names; choose one base-template shard `<victim>`.
2. Patch the sharding: set `shards: 2` and `offline: ["<victim>"]`.
3. Watch component deletion and action logs.

Commands:

```bash
kubectl get components -n ${TEST_NAMESPACE} -l apps.kubeblocks.io/sharding-name=shard -o wide
kubectl get pod -n ${TEST_NAMESPACE} -o json | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, component: .metadata.labels["apps.kubeblocks.io/component-name"]}'
kubectl patch cluster shard-hetero -n ${TEST_NAMESPACE} --type merge --patch-file shard-offline-patch.yaml
kubectl get components -n ${TEST_NAMESPACE} -l apps.kubeblocks.io/sharding-name=shard -w
```

Expected results:

* Exactly `<victim>` is removed; the other shards keep running with unchanged pod UIDs.
* `shardRemove` runs for `<victim>` before deletion (`E2E_SHARD_REMOVE <victim>` in the action log).
* Cluster returns to `Running` with two shards.

### E2E-6D: Offline With Unchanged Shard Count Creates a Replacement

Steps:

1. Keep `shards: 3` and add one shard's full component name to `offline`.
2. Wait for reconciliation.

Expected results:

* The offline shard is removed (after `shardRemove`), and a new shard with a fresh ID is created to keep three shards.
* The freed shard ID is not reused.

### Failure Checks

* `offline` entries must be full component names (`<cluster>-<sharding>-<id>`); naming a non-existent shard is a no-op, not a deletion of an arbitrary shard.
* Decreasing `shards` without `offline` should still work; KubeBlocks removes the shards with the highest names, proving `offline` is optional.
* A sharding where the sum of `shardTemplates[*].shards` exceeds `shards` must fail with `the sum of shards in shard templates is greater than the total shards`.
* Duplicate shard template names or duplicate `shardIDs` must be rejected.

## E2E-7: KubeBlocks Upgrade From 1.0.x to 1.1

### Purpose

Verify that upgrading the KubeBlocks operator from the latest 1.0.x GA release to 1.1.0 is non-disruptive for existing database clusters, installs the new CRDs, and enables 1.1 features after the upgrade.

### Prerequisites

* A clean Kubernetes cluster with enough CPU, memory, and storage for the selected workload set.
* The source chart and CRD release asset are available, for example `v1.0.3-beta.9`.
* The target chart, images, and `kubeblocks_crds.yaml` release asset are available, for example `v1.1.0-beta.6`.
* Snapshot CRDs and KubeBlocks CRDs can be installed before the Helm release.
* The test cluster can pull the selected registry images. For kind-based validation, make sure the node container does not inherit an invalid local proxy such as `127.0.0.1:<port>`.

### E2E-7A: Operator Upgrade Is Non-Disruptive

Steps:

1. Install Snapshot CRDs and source-version KubeBlocks CRDs before installing the Helm release:

```bash
kubectl get crd volumesnapshots.snapshot.storage.k8s.io || kubectl create -f deploy/helm/crds/snapshot/
kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v<source-version>/kubeblocks_crds.yaml
kubectl get crd clusters.apps.kubeblocks.io rollouts.apps.kubeblocks.io instances.workloads.kubeblocks.io instancesets.workloads.kubeblocks.io volumesnapshots.snapshot.storage.k8s.io
```

2. Install KubeBlocks source version with Helm, skipping CRDs because they were installed explicitly:

```bash
helm install kubeblocks kubeblocks/kubeblocks \
  --version <source-version> \
  -n ${KB_NAMESPACE} \
  --create-namespace \
  --skip-crds \
  --set-json autoInstalledAddons='[]'
kubectl -n ${KB_NAMESPACE} rollout status deploy/kubeblocks --timeout=10m
kubectl -n ${KB_NAMESPACE} rollout status deploy/kubeblocks-dataprotection --timeout=10m
```

3. Install the MySQL addon and create a representative workload under the source version:
   * MySQL component `mysql-8.0-1.0.3`, service version `8.0.35`, 2 replicas;
   * a replication cluster (3 replicas, with PVCs);
   * a sharded cluster (2 shards);
   * a cluster with a configured backup schedule, if data protection is in scope.
4. Record baseline state: pod names and UIDs, PVC names and UIDs, cluster/component status, Helm release list, and CRD list.
5. Write a marker row/file into each database.
6. Upgrade CRDs, then the operator. Use `kubectl create` for CRDs that do not exist in the source version and `kubectl replace` for existing CRDs; do not use `kubectl apply` for CRD upgrade validation:

```bash
kubectl get crd rollouts.apps.kubeblocks.io || kubectl create -f deploy/helm/crds/apps.kubeblocks.io_rollouts.yaml
kubectl get crd instances.workloads.kubeblocks.io || kubectl create -f deploy/helm/crds/workloads.kubeblocks.io_instances.yaml
kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v<target-version>/kubeblocks_crds.yaml
helm -n ${KB_NAMESPACE} upgrade kubeblocks kubeblocks/kubeblocks \
  --version <target-version> \
  --skip-crds \
  --reuse-values \
  --set-json autoInstalledAddons='[]'
```

7. Wait for the KubeBlocks manager pods to roll to the target image and become Ready.
8. Compare pod/PVC UIDs and cluster status against the baseline; read the marker data back.
9. Run a post-upgrade Rollout API smoke test against the existing MySQL cluster: create a rollout with `compDef: mysql-8.0-1.0.3`, `serviceVersion: "8.0.36"`, and `strategy.inplace: {}`. Verify it reaches `Succeed` and the component remains on the intended `compDef`.

Baseline and comparison commands:

```bash
kubectl get clusters -A -o yaml > /tmp/kb11-before-clusters.yaml
kubectl get components -A -o yaml > /tmp/kb11-before-components.yaml
kubectl get pod -A -l app.kubernetes.io/managed-by=kubeblocks -o json | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' > /tmp/kb11-before-pods.json
kubectl get pvc -A -o json | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' > /tmp/kb11-before-pvcs.json

kubectl -n ${KB_NAMESPACE} rollout status deploy/kubeblocks --timeout=10m
kubectl get pod -A -l app.kubernetes.io/managed-by=kubeblocks -o json | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' > /tmp/kb11-after-pods.json
kubectl get pvc -A -o json | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' > /tmp/kb11-after-pvcs.json
diff -u /tmp/kb11-before-pods.json /tmp/kb11-after-pods.json
diff -u /tmp/kb11-before-pvcs.json /tmp/kb11-after-pvcs.json
```

Expected results:

* All existing clusters stay `Running` throughout the upgrade; no database pod is restarted or recreated (pod UIDs unchanged).
* PVCs are untouched (UIDs unchanged) and the marker data is readable after the upgrade.
* New CRDs are installed: `rollouts.apps.kubeblocks.io`, `instances.workloads.kubeblocks.io`.
* No CRDs are removed; existing CRs (including `apps.kubeblocks.io/v1alpha1` resources) are still readable.
* The manager logs show no conversion or schema errors after the upgrade.

### E2E-7B: 1.1 Features Work on Upgraded Clusters

Steps (after E2E-7A, on the same installation):

1. Run a day-2 operation on a pre-upgrade cluster (restart or vertical scaling via OpsRequest) and confirm it completes.
2. Apply an `inplace` or `replace` `Rollout` against the pre-upgrade replication cluster and confirm it reaches `Succeed` (see E2E-2).
3. Patch the pre-upgrade sharded cluster to add a shard and confirm normal shard provisioning.
4. Create a new cluster that uses a 1.1-only field (for example `network.hostNetwork` or `shardTemplates`) and confirm it provisions correctly.

Expected results:

* Day-2 operations on pre-upgrade clusters behave exactly as before the upgrade.
* The `Rollout` API is usable against clusters created under 1.0.x.
* New 1.1 API fields are accepted and reconciled on the upgraded control plane.

### E2E-7C: Helm Values Compatibility

Steps:

1. Before the upgrade, note any custom Helm values in use; include `hostPorts` if host networking is used.
2. Upgrade with the same values file.
3. Inspect the rendered configuration after upgrade.

Expected results:

* The upgrade succeeds with a 1.0.x values file; the removed value `enabledAlphaFeatureGates.recoverVolumeExpansionFailure` is ignored (warn if set) and must be removed from values going forward.
* The default host port include range changes to `55000-59999`; host-port allocations made under 1.0.x are preserved in the host-port ConfigMap, while new allocations come from the new range. If the old range is required, setting `hostPorts.include` explicitly restores it.
* `clusterDefaultResources.zero`, `dataProtection.zeroResourceForUnset`, and `operations.zeroResourceForUnset` default to the 1.0-compatible behavior.

### Failure Checks

* `helm upgrade` without first updating CRDs should fail clearly or leave the new CRDs missing; document the error mode (1.1 fields/objects are rejected until `kubeblocks_crds.yaml` is applied).
* Downgrade path: rolling back the chart to 1.0.x after creating 1.1-only objects (`Rollout`, clusters using 1.1 fields) is not supported and must be documented; existing clusters without 1.1 fields must still reconcile after a rollback.
* The upgrade must not delete or orphan any `Cluster`, `Component`, `InstanceSet`, backup, or restore object.

## Release Exit Criteria

The KubeBlocks 1.1 release is not ready until:

* all seven feature areas pass on the primary Kubernetes version;
* the 1.0.x → 1.1 upgrade tests prove the operator upgrade is non-disruptive, new CRDs are installed, and 1.1 features work on upgraded clusters;
* multi-cluster tests pass with two real data clusters;
* rollout tests cover inplace, replace, create/canary, and sharding targets;
* ComponentNetwork tests cover host ports, host aliases, and DNS config;
* dynamic adoption tests prove pod/PVC identity is preserved;
* sharding lifecycle tests prove add/remove hooks run with the expected shard variables;
* heterogeneous shard tests prove per-group configuration and shard-specific scale-in via `offline`;
* no P0/P1 bugs remain open for these feature areas.
