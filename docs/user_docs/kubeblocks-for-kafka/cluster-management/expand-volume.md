---
title: Expand volume
description: How to expand the volume of a kafka cluster
keywords: [kafka, expand volume, volume expansion]
sidebar_position: 4
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list kafka  
```

## Option 1. Use kbcli

Use `kbcli cluster volume-expand` command, configure the resources required and enter the cluster name again to expand the volume.

```bash
kbcli cluster volume-expand --storage=30G --components=kafka --volume-claim-templates=data kafka
```

- `--components` describes the component name for volume expansion.
- `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
- `--storage` describes the volume storage size.

## Option 2. Change the YAML file of the cluster

Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: kafka
  namespace: default
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  componentSpecs:
  - name: kafka 
    componentDefRef: kafka
    replicas: 1
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi # Change the volume storage size.
  terminationPolicy: Halt
```
