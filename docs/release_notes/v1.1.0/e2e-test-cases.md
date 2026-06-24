# KubeBlocks 1.1 Key Feature E2E Test Cases

This document defines end-to-end test cases for the key KubeBlocks 1.1 features:

1. Experimental Rollout API
2. ComponentNetwork API
3. Dynamic InstanceSet template adoption
4. Sharding lifecycle actions
5. Heterogeneous shards and shard-specific scale-in
6. KubeBlocks upgrade from 1.0.x to 1.1

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
| All features | 1 Kubernetes cluster with at least 1 schedulable node |

### Shared Variables

```bash
export KB_NAMESPACE=kb-system
export TEST_NAMESPACE=kb-11-e2e
```

Create the test namespace:

```bash
kubectl create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
```

For script-based validation, prefetch the release CRDs and KubeBlocks Helm charts before running setup or feature cases, so later install/upgrade steps use local files:

```bash
bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-cache
```

### Execution Order

Run the upgrade test (E2E-6) in a clean cluster because it starts from KubeBlocks 1.0.x. Run the feature tests (E2E-1 to E2E-5) against the final KubeBlocks 1.1.0 chart, CRDs, and images.

Recommended order:

1. E2E-6 upgrade validation on a dedicated cluster.
2. E2E-1 to E2E-5 on the primary single-cluster e2e environment.
3. Repeat the high-risk cases (E2E-1, E2E-4, E2E-6) on the release-blocking Kubernetes versions if the release matrix includes more than one version.

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

## E2E-1: Experimental Rollout API

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
      componentDef: mysql-8.0
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
                storage: 10Gi
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
* `Component` is of prefix `mysql-8.0` and `serviceVersion` is `8.0.35`.
* Two MySQL pods are `4/4 Running`.

### E2E-1A: In-place Rollout

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
      compDef: mysql-8.0
      serviceVersion: "8.0.36"
      strategy:
        inplace: {}
```

Steps:

1. Create `mysql-inplace-8036`.
2. Wait until `Rollout.status.state` is `Succeed`.
3. Compare pod UIDs before and after.
4. Verify the `mysql` and `kbagent` container images are `8.0.36`.
5. Keep the finished rollout for status and event inspection.

Commands:

```bash
kubectl create -f mysql-inplace-8036.yaml
kubectl wait rollout/mysql-inplace-8036 -n ${TEST_NAMESPACE} --for=jsonpath='{.status.state}'=Succeed --timeout=20m
kubectl get rollout mysql-inplace-8036 -n ${TEST_NAMESPACE} -o yaml
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\n"}'
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o json \
  | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, restarts: [.status.containerStatuses[] | {name, restartCount}], images: [.spec.containers[] | {name, image}]}'
```

Expected results:

* `Rollout.status.state` is `Succeed`.
* `spec.componentSpecs[0].componentDef` remains of prefix `mysql-8.0`.
* Pod names and UIDs do not change.
* `mysql` and `kbagent` containers are restarted and use the `8.0.36` image.

### E2E-1B: Replace Rollout

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
      compDef: mysql-8.0
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
7. Keep the finished rollout for status and event inspection.

Commands:

```bash
kubectl create -f mysql-replace-8037.yaml
kubectl get rollout mysql-replace-8037 -n ${TEST_NAMESPACE} -w
kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\treplicas="}{.spec.componentSpecs[0].replicas}{"\tinstances="}{range .spec.componentSpecs[0].instances[*]}{.name}:{.compDef}:{.serviceVersion}:{.replicas}{" "}{end}{"\n"}'
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o wide
kubectl wait rollout/mysql-replace-8037 -n ${TEST_NAMESPACE} --for=jsonpath='{.status.state}'=Succeed --timeout=25m
kubectl get rollout mysql-replace-8037 -n ${TEST_NAMESPACE} -o yaml
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
* `componentDef` remains of prefix `mysql-8.0`; it must not drift to another matching definition.
* No PVC is accidentally deleted unless the workload policy requires it.

### E2E-1C: Create/Canary Rollout

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
      compDef: mysql-8.0
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
```

Expected results:

* Exactly one canary instance is created before promotion, identified by the pod label `apps.kubeblocks.io/instance-template: <rollout-template-name>`.
* Canary metadata from `instanceMeta.canary` (labels/annotations) is applied to the canary pod.
* The rollout stays in `Rolling` while the canary template's `canary` flag is set; after `delaySeconds`, promotion clears the flag and scales down old instances according to `scaleDownDelaySeconds`.
* When `replicas` is smaller than the stable replica count, success can leave the component intentionally heterogeneous: the component-level `serviceVersion` remains on the previous stable version, and the promoted rollout-managed instance template keeps the new version for only the promoted replicas.
* `status.components[*].canaryReplicas` and `rolledOutReplicas` reflect progress.
* `componentDef` remains of prefix `mysql-8.0`; it must not drift to another matching definition.

Variation, manual promotion: create the same rollout with `promotion.auto: false`. Expected: the canary is created and the rollout stays in `Rolling` indefinitely; after validating the canary, edit the rollout to set `promotion.auto: true` and verify promotion completes as above.

Constraint check: a rollout with `promotion.condition` set must fail with a "not supported" error (`promotion.condition` is not supported in 1.1).

### E2E-1D: Sharding Rollout

Create a sharded cluster with two shards, then create:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mongodb-sharding-rollout
  namespace: kb-11-e2e
spec:
  clusterName: mongo-sharding
  shardings:
    - name: shard
      compDef: mongo-shard
      serviceVersion: "6.0.27"
      strategy:
        replace:
          perInstanceIntervalSeconds: 30
          scaleDownDelaySeconds: 30
```

Expected results:

* `status.shardings` is populated and reports `state: Succeed`, the source `serviceVersion`, and `replicas == newReplicas == rolledOutReplicas`.
* Each shard is rolled out from the base MongoDB 6.0 patch version, for example `6.0.20`, to another supported 6.0 patch version, for example `6.0.27`.
* Shard instances are rolled one at a time with at least 30 seconds between successive instance updates.
* Shard pod UIDs change after replace rollout, and all shard pods carry the target service version.
* Marker data seeded through `mongos` before rollout is still readable with the same counts and checksums after rollout.
* No shard remains in stale `Updating` or `Failed` state.
* The sharded cluster returns to `Running`.
* Keep the completed `mongodb-sharding-rollout` CR for trace inspection; delete it manually before rerunning the same case.

### E2E-1E: Concurrency and Abort Semantics

1. While a rollout is `Rolling`, create a second rollout against the same cluster. Expected: the second rollout enters `Error` with message `the cluster mysql-cluster is already bound to rollout <active-rollout>`; the first rollout is unaffected.
2. Delete a rollout while it is `Rolling`. Expected: the `apps.kubeblocks.io/rollout-name` label is removed from the cluster; changes already applied to the cluster spec are **not** reverted (document the resulting spec); the cluster eventually converges to `Running`.
3. After inspecting a finished rollout, delete it manually and then create another rollout targeting a new or previous version. Expected: the new rollout is accepted because the binding is released.

### Failure Checks

* A rollout that defines no strategy, more than one strategy, or neither `serviceVersion` nor `compDef` for a target must fail with a clear error.
* A `replace`/`inplace` rollout with `replicas` set to less than the full replica count must fail (partial rollout is not supported).
* A rollout with an invalid target component or sharding should enter `Error` with a clear message.
* A rollout that omits `compDef` in an addon with several compatible definitions should be treated as unsafe for release validation, even if it is accepted by the API. Record the selected `spec.componentSpecs[*].componentDef` before and after the rollout.
* Manager restart during rollout should not lose rollout progress (progress is derived from the cluster spec and `status.components`).

## E2E-2: ComponentNetwork API

### Purpose

Verify component-level network settings: host networking, explicit host ports, host aliases, DNS policy, and DNS config.

### Prerequisites

* At least 3 schedulable nodes.
* A ComponentDefinition that declares host-network capability for hostNetwork subcases.
* KubeBlocks host port include/exclude ranges are configured or defaults are acceptable.

### E2E-2A: HostNetwork Enabled By Annotation

Create:

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: network-mongodb-hostnet-annotation
  namespace: kb-11-e2e
spec:
  terminationPolicy: Delete
  componentSpecs:
    - name: mongodb
      componentDef: mongodb-1.0
      serviceVersion: "6.0.27"
      replicas: 1
      annotations:
        kubeblocks.io/host-network: mongodb
```

Expected results:

* The single MongoDB pod has `spec.hostNetwork: true`.
* Pod DNS policy defaults to `ClusterFirstWithHostNet` unless explicitly set.
* Required host-network ports are allocated from the configured include/exclude ranges (default `55000-59999`); container ports in the pod spec are rewritten to the allocated values.
* Allocations are recorded in the host-port ConfigMap in the KubeBlocks namespace, keyed `<cluster>-<comp>-<container>-<portName>`.

### E2E-2B: HostNetwork Enabled By ComponentNetwork API

Create a single-node MongoDB cluster with the same `componentDef: mongodb-1.0`, but enable host networking through `spec.componentSpecs[*].network.hostNetwork: true` instead of annotation.

Expected results:

* The single MongoDB pod has `spec.hostNetwork: true`.
* Pod DNS policy defaults to `ClusterFirstWithHostNet` unless explicitly set.
* Host-network ports are allocated and recorded in the host-port ConfigMap.

### E2E-2C: Component Without HostNetwork Capability Ignores Network API

Create a single-node MySQL cluster with `spec.componentSpecs[*].network.hostNetwork: true`.

Expected results:

* The MySQL cluster reaches `Running`.
* The MySQL pod keeps normal pod networking (`spec.hostNetwork` is absent or `false`) because the referenced MySQL `ComponentDefinition` does not declare host-network capability.

### E2E-2D: Explicit Host Ports

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

### E2E-2E: Host Aliases and DNS Config

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

## E2E-3: Dynamic InstanceSet Template Adoption

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

### E2E-3A: Adopt One Pod Into a High-Resource Template

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

### E2E-3B: Return the Pod to the Default Template

Patch the cluster so all ordinals are back in the default template and remove `instances`.

Expected results:

* `foo-2` returns to the default template.
* Pod name remains `foo-2`.
* PVC data remains attached to ordinal `2`.
* No extra pods or PVCs remain.

### E2E-3C: Adopt and Return a MySQL Instance Template

This case validates the same dynamic template behavior on a real MySQL addon cluster. It can run after E2E-1C, where the `create` rollout leaves the MySQL cluster with both named instance templates and default-template pods.

Steps:

1. Record the MySQL pod snapshot and choose one pod whose instance-template label is empty.
2. Parse the pod ordinal from its name.
3. Replace the `Cluster` object and add a named instance template that adopts that ordinal from the default template:

```yaml
instances:
  - name: kb11-dynamic-adopt
    compDef: mysql-8.0
    serviceVersion: "8.0.38"
    replicas: 1
    ordinals:
      discrete:
        - <default-pod-ordinal>
    labels:
      workload-tier: dynamic-adopt
```

4. Wait for the cluster to become `Running` and verify the same pod name is now associated with `kb11-dynamic-adopt`.
5. Replace the `Cluster` object again and remove `kb11-dynamic-adopt` from `spec.componentSpecs[*].instances`.
6. Wait for the cluster to become `Running` and verify the same pod name is back on the default template.

Commands:

```bash
kubectl get pod -n ${TEST_NAMESPACE} -l app.kubernetes.io/instance=mysql-cluster -o json \
  | jq '[.items[] | {name: .metadata.name, uid: .metadata.uid, template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"] // "")}] | sort_by(.name)'

export TARGET_POD=<default-template-pod-name>
export TARGET_ORDINAL=${TARGET_POD##*-}

kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o json \
  | jq --argjson ordinal "${TARGET_ORDINAL}" '
      (.spec.componentSpecs[] | select(.name == "mysql")) |= (
        .flatInstanceOrdinal = true
        | .componentDef as $componentDef
        | .serviceVersion as $serviceVersion
        | .instances = ((.instances // []) | map(select(.name != "kb11-dynamic-adopt")) + [{
            "name": "kb11-dynamic-adopt",
            "compDef": $componentDef,
            "serviceVersion": $serviceVersion,
            "replicas": 1,
            "ordinals": {"discrete": [$ordinal]},
            "labels": {"workload-tier": "dynamic-adopt"}
          }])
      )' \
  | kubectl replace -f -

kubectl wait cluster/mysql-cluster -n ${TEST_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=15m
kubectl get pod ${TARGET_POD} -n ${TEST_NAMESPACE} -o json \
  | jq '{name: .metadata.name, uid: .metadata.uid, template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"] // ""), workloadTier: .metadata.labels["workload-tier"]}'

kubectl get cluster mysql-cluster -n ${TEST_NAMESPACE} -o json \
  | jq '
      (.spec.componentSpecs[] | select(.name == "mysql")) |= (
        .instances = ((.instances // []) | map(select(.name != "kb11-dynamic-adopt")))
      )' \
  | kubectl replace -f -

kubectl wait cluster/mysql-cluster -n ${TEST_NAMESPACE} --for=jsonpath='{.status.phase}'=Running --timeout=15m
kubectl get pod ${TARGET_POD} -n ${TEST_NAMESPACE} -o json \
  | jq '{name: .metadata.name, uid: .metadata.uid, template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"] // "")}'
```

Expected results:

* A named template can adopt one existing MySQL pod ordinal from the default template.
* After adoption, the selected pod name is unchanged and the pod template label resolves to `kb11-dynamic-adopt`.
* The selected pod carries `workload-tier=dynamic-adopt` while it is adopted by the named template.
* After removing `kb11-dynamic-adopt`, the same pod name returns to the default template and its template label is empty.
* No extra MySQL pods or PVCs remain after the giveback step.

### E2E-3D: Adopt a Shard by Shard ID

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

## E2E-4: MongoDB Sharding Scale

### Purpose

Verify that a MongoDB sharding cluster can be created and reconciled through shard scale-out and scale-in.

Lifecycle action coverage for `postProvision`, `preTerminate`, `shardAdd`, and `shardRemove` is a TODO and is intentionally not part of the current executable release test.

### Prerequisites

* MongoDB addon is installed.
* A MongoDB sharding-capable ComponentDefinition matching prefix `mongo-` exists.

### Test Cluster

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mongo-sharding
  namespace: kb-11-e2e
spec:
  clusterDef: mongodb
  terminationPolicy: Delete
  topology: sharding
  componentSpecs:
    - name: mongos
      replicas: 1
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: "6.0.20"
    - name: config-server
      replicas: 1
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: "6.0.20"
      systemAccounts:
        - disabled: false
          name: root
          passwordConfig:
            length: 16
            letterCase: MixedCases
            numDigits: 8
            numSymbols: 0
            seed: mongo
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
  shardings:
    - name: shard
      shards: 3
      template:
        name: shard
        replicas: 1
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 500m
            memory: 512Mi
        serviceVersion: "6.0.20"
        volumeClaimTemplates:
          - name: data
            spec:
              accessModes:
                - ReadWriteOnce
              resources:
                requests:
                  storage: 10Gi
```

### E2E-4A: Create MongoDB Sharding Cluster

Steps:

1. Apply the sharded cluster.
2. Wait for the cluster to become `Running`.
3. Inspect cluster status and generated shard components.
4. Discover the current `Running` shard components. Through `mongos`, create one stable marker database per running shard, bind each database's primary shard to one current shard, and insert a marker document. Keep the initial shard name only as placement evidence because shard names can change after scale-out or scale-in.

Expected results:

* `mongos`, `config-server`, and three shard components are created.
* All shard components become available.
* Cluster returns to `Running`.
* Each initial shard gets one marker database at seed time, and all marker documents are readable through `mongos`.

### E2E-4B: Scale Out Shards

Steps:

1. Patch `spec.shardings[0].shards` from `3` to `4`.
2. Wait until `spec.shardings[0].shards` is `4`, exactly four shard components exist, and all four shard components are `Running`.
3. Inspect component count and cluster status.
4. Re-read all marker documents inserted before scale-out.

Commands:

```bash
kubectl patch cluster mongo-sharding -n ${TEST_NAMESPACE} --type merge -p '{"spec":{"shardings":[{"name":"shard","shards":4}]}}'
kubectl get cluster mongo-sharding -n ${TEST_NAMESPACE} -o jsonpath='{.spec.shardings[0].shards}{"\n"}'
kubectl get components -n ${TEST_NAMESPACE} -l apps.kubeblocks.io/sharding-name=shard -w
kubectl get cluster mongo-sharding -n ${TEST_NAMESPACE} -o yaml
```

Expected results:

* Exactly one new shard component is created, named `<cluster>-<sharding>-<id>` with a generated 3-character shard ID.
* `spec.shardings[0].shards` is `4` after the scale-out patch.
* Four shard components are running.
* Cluster returns to `Running`.
* All marker databases and marker documents inserted before scale-out are still readable through `mongos`; the test does not require shard names to remain unchanged.

### E2E-4C: Offline With Unchanged Shard Count Creates a Replacement

Steps:

1. After scaling `mongo-sharding` out to four shards, generate a shard offline readiness report from `mongos`.
2. Select only a shard that is not draining and has no chunks, jumbo chunks, or primary databases (`dbsToMove`).
3. Keep `shards: 4` and add the selected shard's full component name to `offline`.
4. Wait for reconciliation.
5. Verify marker data through `mongos`.

Expected results:

* If no shard is ready for offline, the test fails before patching the Cluster and saves a report listing blockers such as chunks, jumbo chunks, draining state, or databases that require `movePrimary`/`dropDatabase`.
* The offline shard is removed, and a new shard with a fresh ID is created to keep four shards.
* The freed shard ID is not reused.
* The other shards keep running with unchanged pod UIDs.
* All marker databases and marker documents inserted before scale-out are still readable through `mongos`.

### E2E-4D: Scale In a Specific Shard via offline

Steps:

1. Generate a shard offline readiness report from `mongos`.
2. Select only a shard that is not draining and has no chunks, jumbo chunks, or primary databases (`dbsToMove`).
3. Patch the sharding: set `shards: 3` and `offline: ["<victim>"]`.
4. Watch component deletion.
5. Verify marker data through `mongos`.

Expected results:

* If no shard is ready for offline, the test fails before patching the Cluster and saves a report listing why each candidate cannot be removed.
* Exactly `<victim>` is removed; the other shards keep running with unchanged pod UIDs.
* Cluster returns to `Running` with three shards.
* All marker databases and marker documents inserted before scale-out are still readable through `mongos`.

### E2E-4 Failure Checks

* `offline` entries must be full component names (`<cluster>-<sharding>-<id>`); naming a non-existent shard is a no-op, not a deletion of an arbitrary shard.
* Offline precondition checks must report blockers before patching the Cluster. Blockers include chunks still on the shard, jumbo chunks, an already-draining shard, or primary databases that require `movePrimary` or `dropDatabase`.
* Decreasing `shards` without `offline` should still work; KubeBlocks removes the shards with the highest names, proving `offline` is optional.

### TODO: Sharding Lifecycle Actions

Add separate coverage for `postProvision`, `preTerminate`, `shardAdd`, and `shardRemove` after the lifecycle test design and action implementation are finalized.

## E2E-5: Heterogeneous Shards

### Purpose

Verify that `spec.shardings[*].shardTemplates` creates shard groups with distinct configurations. Offline shard removal is covered by the regular `mongo-sharding` scale tests so it is not mixed with heterogeneous shard coverage.

### Prerequisites

* A sharding-capable ComponentDefinition.
* MongoDB sharding cluster creation works without a custom `ShardingDefinition`.

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

### E2E-5A: Shard Template Creates a Heterogeneous Shard Group

Steps:

1. Apply the cluster.
2. Wait for `Running`.
3. List shard components and inspect their pod resources.

Expected results:

* Three shards exist in total: one from the `hot` template, two from the base template (`shards` minus the sum of `shardTemplates[*].shards`).
* The `hot` shard component carries the label `apps.kubeblocks.io/shard-template: hot`; base-template shards do not carry the label.
* Exactly one shard's pods request `cpu: 1, memory: 1Gi`; the other shards keep base-template resources.
* `status.shardings` reports the expected shard counts.

### E2E-5B: Shard Template Version Canary

Steps:

1. Patch the `hot` shard template with a different `serviceVersion` (or `compDef`). The script defaults to `MONGO_SHARD_VERSION_BASE=6.0.20` and `MONGO_SHARD_VERSION_TARGET=6.0.27`; valid sharding rollout versions are `6.0.20`, `6.0.21`, `6.0.22`, and `6.0.27`.
2. Wait for reconciliation.

Expected results:

* Only the pods of the `hot` shard group are updated to the new version.
* The `hot` shard component reports `spec.serviceVersion` equal to the target version.
* Other shards show no pod restarts (compare pod UIDs before and after).

### Failure Checks

* A sharding where the sum of `shardTemplates[*].shards` exceeds `shards` must fail with `the sum of shards in shard templates is greater than the total shards`.
* Duplicate shard template names or duplicate `shardIDs` must be rejected.

## E2E-6: KubeBlocks Upgrade From 1.0.x to 1.1

### Purpose

Verify that upgrading the KubeBlocks operator from the latest 1.0.x GA release to 1.1.0 is non-disruptive for existing database clusters, installs the new CRDs, and enables 1.1 features after the upgrade.

### Prerequisites

* A clean Kubernetes cluster with enough CPU, memory, and storage for the selected workload set.
* The source chart and CRD release asset are available, for example `v1.0.3-beta.9`.
* The target chart, images, and `kubeblocks_crds.yaml` release asset are available, for example `v1.1.0-beta.6`.
* Snapshot CRDs and KubeBlocks CRDs can be installed before the Helm release.
* The test cluster can pull the selected registry images. For kind-based validation, create the kind cluster with proxy variables disabled and `NO_PROXY/no_proxy` set for local Kubernetes addresses, then restore the original proxy variables after kind creation.

### E2E-6A: Operator Upgrade Is Non-Disruptive

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
   * MySQL component definition prefix `mysql-8.0`, service version `8.0.35`, 2 replicas;
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
9. Run a post-upgrade Rollout API smoke test against the existing MySQL cluster: create a rollout with `compDef: mysql-8.0`, `serviceVersion: "8.0.36"`, and `strategy.inplace: {}`. Verify it reaches `Succeed` and the component remains on the intended `compDef` prefix.

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

### E2E-6B: 1.1 Features Work on Upgraded Clusters

Steps (after E2E-6A, on the same installation):

1. Run a day-2 operation on a pre-upgrade cluster (restart or vertical scaling via OpsRequest) and confirm it completes.
2. Apply an `inplace` or `replace` `Rollout` against the pre-upgrade replication cluster and confirm it reaches `Succeed` (see E2E-1).
3. Patch the pre-upgrade sharded cluster to add a shard and confirm normal shard provisioning.
4. Create a new cluster that uses a 1.1-only field (for example `network.hostNetwork` or `shardTemplates`) and confirm it provisions correctly.

Expected results:

* Day-2 operations on pre-upgrade clusters behave exactly as before the upgrade.
* The `Rollout` API is usable against clusters created under 1.0.x.
* New 1.1 API fields are accepted and reconciled on the upgraded control plane.

### E2E-6C: Helm Values Compatibility

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

* all six feature areas pass on the primary Kubernetes version;
* the 1.0.x → 1.1 upgrade tests prove the operator upgrade is non-disruptive, new CRDs are installed, and 1.1 features work on upgraded clusters;
* rollout tests cover inplace, replace, create/canary, and sharding targets;
* ComponentNetwork tests cover host ports, host aliases, and DNS config;
* dynamic adoption tests prove pod/PVC identity is preserved;
* sharding lifecycle tests prove add/remove hooks run with the expected shard variables;
* heterogeneous shard tests prove per-group configuration without mixing in offline removal;
* no P0/P1 bugs remain open for these feature areas.
