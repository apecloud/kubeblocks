---
title: Simulate GCP faults
description: Simulate GCP faults
sidebar_position: 11
sidebar_label: Simulate GCP faults
---

# Simulate GCP faults

By creating a GCPChaos experiment, you can simulate fault scenarios of the specified GCP instance. Currently, GCPChaos supports the following fault types:

* Node Stop: stops the specified GCP instance.
* Node Restart: reboots the specified GCP instance.
* Disk Loss: uninstalls the storage volume from the specified instance.

## Before you start

* By default, the GCP authentication information for local code has been imported. If you have not imported the authentication, follow the steps in [Prerequisite](./prerequisite.md#check-your-permission).

* To connect to the GCP cluster easily, you can create a Kubernetes Secret file in advance to store authentication information. A `Secret` file sample is as follows:
  
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: cloud-key-secret-gcp
    namespace: default
  type: Opaque
  stringData:
    service_account: your-gcp-service-account-base64-encode
  ```
  
  * `name` means the Kubernetes Secret object.
  * `namespace` means the namespace of the Kubernetes Secret object.
  * `service_account` stores the service account key of your GCP cluster. Remember to complete Base64 encoding for your GCP service account key. To learn more about service account key, see [Creating and managing service account keys](https://cloud.google.com/iam/docs/keys-create-delete).

## Simulate fault injections by kbcli

### Stop

The command below injects the `node-stop` fault into the specified GCP instance so that the GCP instance will be unavailable in 3 minutes.

```bash
kbcli fault node stop [node1] [node2] -c=gcp --region=us-central1-c --duration=3m
```

After running the above command, the `node-stop` command creates resources, Secret `cloud-key-secret-gcp` and GCPChaos `node-chaos-w98j5`. You can run `kubectl describe node-chaos-w98j5` to verify whether the `node-stop` fault is injected successfully.

:::caution

When changing the cluster permissions, updating the key, or changing the cluster context, the `cloud-key-secret-gcp` must be deleted, and then the `node-stop` injection creates a new `cloud-key-secret-gcp` according to the new key.

:::

### Restart

The command below injects an `node-restart` fault into the specified GCP instance so that this instance will be restarted.

```bash
kbcli fault node restart [node1] [node2] -c=gcp --region=us-central1-c
```

### Detach volume

The command below injects a `detach-volume` fault into the specified GCP instance so that this instance is detached from the specified storage volume within 3 minutes.

```bash
kbcli fault node detach-volume [node1] -c=gcp --region=us-central1-c --device-name=/dev/sdb
```

## Simulate fault injections by YAML file

### GCP-stop example

1. Write the experiment configuration to the `gcp-stop.yaml` file.

   In the following example, Chaos Mesh injects the `node-stop` fault into the specified GCP instance so that the GCP instance will be unavailable in 3 minutes.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: node-stop
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### GCP-restart example

1. Write the experiment configuration to the `gcp-restart.yaml` file.

   In the following example, Chaos Mesh injects an `node-reset` fault into the specified GCP instance so that this instance will be restarted.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: node-reset
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### GCP-detach-volume example

1. Write the experiment configuration to the `gcp-detach-volume.yaml` file.

   In the following example, Chaos Mesh injects a `disk-loss` fault into the specified GCP instance so that this instance is detached from the specified storage volume within 3 minutes.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: disk-loss
     deviceNames:
     - /dev/sdb
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### Field description

The following table shows the fields in the YAML configuration file.

| Parameter | Type | Description | Default value | Required |
| :--- | :--- | :--- | :--- | :--- |
| action | string | It indicates the specific type of faults. The available fault types include `node-stop`, `node-reset`, and `disk-loss`. | `node-stop` | Yes |
| mode | string | It indicates the mode of the experiment. The mode options include `one` (selecting a Pod at random), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of the eligible Pods), and `random-max-percent` (selecting the maximum percentage of the eligible Pods). | None | Yes |
| value | string | It provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of pods. | None | No |
| secretName | string | It indicates the name of the Kubernetes secret that stores the GCP authentication information. | None | No |
| project | string | It indicates the ID of GCP project. | None | Yes | real-testing-project |
| zone | string | Indicates the region of GCP instance. | None | Yes |
| instance | string | It indicates the name of GCP instance. | None | Yes |
| deviceNames | []string | This is a required field when the `action` is `disk-loss`. This field specifies the machine disk ID. | None | no |
| duration | string | It indicates the duration of the experiment. | None | Yes |
