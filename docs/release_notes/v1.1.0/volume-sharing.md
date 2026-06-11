# Volume Sharing Among Instances

KubeBlocks 1.1 adds `persistentVolumeClaimName` to volume claim templates in the `Cluster` and `Component` APIs. It decouples PVC names from pod and template names, so PVCs can be kept, reused, and shared when instances move between templates.

## Why This Helps

Before 1.1, the PVC name of an instance was derived from the volume claim template name and the pod name. This coupling caused practical problems:

* when an instance was adopted by a different instance template (see [Dynamic InstanceSet Template Adoption](./dynamic-instance-adoption.md)), its PVC name no longer matched the new naming scheme;
* different instance templates could not point at the same set of volumes;
* migration scenarios that wanted to re-attach existing volumes to new instances required manual PVC surgery.

With `persistentVolumeClaimName`, the PVC identity is declared explicitly, so volumes survive template changes and can be shared across templates that agree on the same prefix.

## What It Does

`persistentVolumeClaimName` sets the *prefix* of the PVC name. For each replica, the final PVC name is:

```text
<persistentVolumeClaimName>-<ordinal>
```

instead of the default `<templateName>-<podName>`. Because the name no longer embeds the pod or template name, two templates that declare the same prefix resolve to the same PVCs for the same ordinals.

The field is available at:

* `Cluster.spec.componentSpecs[*].volumeClaimTemplates[*].persistentVolumeClaimName`
* `Cluster.spec.componentSpecs[*].instances[*].volumeClaimTemplates[*].persistentVolumeClaimName`
* `Component.spec.volumeClaimTemplates[*].persistentVolumeClaimName`

## How to Use It

### Keep PVC identity stable across template adoption

Declare the same PVC name prefix in the default template and the named template. When ordinal `2` moves from the default template into `highperf`, it keeps `mydata-2` and its data.

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
      volumeClaimTemplates:
        - name: data
          persistentVolumeClaimName: mydata
          spec:
            accessModes: [ReadWriteOnce]
            resources:
              requests:
                storage: 20Gi
      instances:
        - name: highperf
          replicas: 1
          ordinals:
            discrete: [2]
          resources:
            requests: { cpu: "4", memory: 8Gi }
            limits: { cpu: "4", memory: 8Gi }
          volumeClaimTemplates:
            - name: data
              persistentVolumeClaimName: mydata
              spec:
                accessModes: [ReadWriteOnce]
                resources:
                  requests:
                    storage: 20Gi
```

PVCs are `mydata-0`, `mydata-1`, and `mydata-2`, regardless of which template owns each ordinal.

### Re-attach pre-provisioned volumes

If PVCs named `mydata-0..2` already exist (for example created from volume snapshots or carried over from a previous cluster), a new cluster that declares `persistentVolumeClaimName: mydata` adopts them instead of provisioning new PVCs, as long as the spec is compatible.

## Verify the Result

```bash
kubectl get pvc -n demo
kubectl get pod <pod-name> -n demo -o jsonpath='{.spec.volumes}'
```

Look for:

* PVC names following `<prefix>-<ordinal>`;
* the same PVC staying bound to the same ordinal after template adoption;
* no orphaned or duplicated PVCs after spec changes.

## Notes

* Use this together with `flatInstanceOrdinal: true` so ordinals are stable across templates.
* All templates that share a prefix must declare compatible PVC specs (storage class, access modes, size).
* Sharing one PVC across *concurrently running* instances additionally requires a `ReadWriteMany`-capable storage class; for ordinary engines the feature is mainly about identity preservation and re-attachment.
* PVC deletion still follows the component's `persistentVolumeClaimRetentionPolicy`.
