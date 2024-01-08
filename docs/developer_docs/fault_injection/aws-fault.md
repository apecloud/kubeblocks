---
title: Simulate AWS faults
description: Simulate AWS faults
sidebar_position: 10
sidebar_label: Simulate AWS faults
---

# Simulate AWS faults

AWSChaos simulates fault scenarios on the specified AWS instance. Currently, AWSChaos supports the following fault types:

* EC2 Stop: stops the specified instance.
* EC2 Restart: restarts the specified instance.
* Detach Volume: uninstalls the storage volume from the specified instance.

## Before you start

* By default, the AWS authentication information for local code has been imported. If you have not imported the authentication, follow the steps in [Prerequisite](./prerequisite.md#check-your-permission).

* To connect to the AWS cluster easily, you can create a Kubernetes Secret file in advance to store authentication information. A `Secret` file sample is as follows:

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: cloud-key-secret-aws
      namespace: default
    type: Opaque
    stringData:
      aws_access_key_id: your-aws-access-key-id
      aws_secret_access_key: your-aws-secret-access-key
    ```

  * `name` means the Kubernetes Secret object.
  * `namespace` means the namespace of the Kubernetes Secret object.
  * `aws_access_key_id` stores the ID of the access key to the AWS cluster.
  * `aws_secret_access_key` stores the secret access key to the AWS cluster.

## Simulate fault injections by kbcli

### Stop

The command below injects an `instance-stop` fault into the specified EC2 instance so that this instance will be unavailable in 3 minutes.

```bash
kbcli fault node stop [node1] -c=aws --region=cn-northwest-1 --duration=3m
```

### Restart

The command below injects an `instance-restart` fault into the specified EC2 instance so that this instance will be restarted.

```bash
kbcli fault node restart [node1] -c=aws --region=cn-northwest-1 --duration=3m
```

### Detach volume

The command below injects a `detach-volume` fault into the specified EC2 instance so that this instance is detached from the specified storage volume within 1 minute.

```bash
kbcli fault node detach-volume [node1] -c=aws --region=cn-northwest-1 --duration=1m --volume-id=vol-xxx --device-name=/dev/xvdaa
```

You can also add multiple nodes and their volumes. For example,

```bash
kbcli fault node detach-volume [node1] [node2] -c=aws --region=cn-northwest-1 --duration=1m --volume-id=vol-xxx,vol-xxx --device-name=/dev/sda,/dev/sdb
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-aws-chaos/#create-experiments-using-the-yaml-file) for details.

### AWS-stop example

1. Write the experiment configuration to the `aws-stop.yaml` file.

   In the following example, Chaos Mesh injects an `ec2-stop` fault into the specified EC2 instance so that this instance will be unavailable in 3 minutes.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: ec2-stop
     awsRegion: cn-northwest-1
     duration: 3m
     ec2Instance: i-037b1f38debb59bd7
     secretName: cloud-key-secret-aws
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-stop.yaml
   ```

### AWS-restart example

1. Write the experiment configuration to the `aws-restart.yaml` file.

   In the following example, Chaos Mesh injects an `ec2-restart` fault into the specified EC2 instance so that this instance will be restarted.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: ec2-restart
     awsRegion: cn-northwest-1
     duration: 3m
     ec2Instance: i-037b1f38debb59bd7
     secretName: cloud-key-secret-aws
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-restart.yaml
   ```

### AWS-detach-volume example

1. Write the experiment configuration to the `aws-detach-volume.yaml` file.

   In the following example, Chaos Mesh injects a `detach-volume` fault into the two specified EC2 instance so that these two instances are detached from their own storage volume within 1 minute.

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: detach-volume
     awsRegion: cn-northwest-1
     deviceName: /dev/xvda
     duration: 1m
     ec2Instance: i-0e368667e544fa955
     secretName: cloud-key-secret-aws
     volumeID: vol-01b3d68c074cd93a9
   status:
     experiment: {}
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: detach-volume
     awsRegion: cn-northwest-1
     deviceName: /dev/xvdaa
     duration: 1m
     ec2Instance: i-01da8eef32743b5de
     secretName: cloud-key-secret-aws
     volumeID: vol-0f1ecf66cb8d0328e
   ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### Field description

The fields in the YAML configuration file are described in the following table:

| Parameter | Type | Description | Default value | Required |
| :--- | :--- | :--- | :--- | :--- |
| action | string | It indicates the specific type of faults. Only `ec2-stop`, `ec2-restart`, and `detach-volume` are supported. | `ec2-stop` | Yes |
| mode | string | It specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes |
| value | string | It provides parameters for the `mode` configuration, depending on `mode`.For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No |
| secretName | string | It specifies the name of the Kubernetes Secret that stores the AWS authentication information. | None | No |
| awsRegion | string | It specifies the AWS region. | None | Yes | us-east-2 |
| ec2Instance | string | It specifies the ID of the EC2 instance. | None | Yes |
| volumeID | string | This is a required field when the `action` is `detach-volume`. This field specifies the EBS volume ID. | None | No |
| deviceName | string | This is a required field when the `action` is `detach-volume`. This field specifies the machine name. | None | No |
| duration | string | It specifies the duration of the experiment. | None | Yes |
