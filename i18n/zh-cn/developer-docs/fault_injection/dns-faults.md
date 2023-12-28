---
title: Simulate DNS faults
description: Simulate DNS faults
sidebar_position: 5
sidebar_label: Simulate DNS faults
---

# Simulate DNS faults

DNSChaos is used to simulate wrong DNS responses. For example, DNSChaos can return an error or return a random IP address when receiving a DNS request.

## Deploy Chaos DNS server

Check whether Chaos DNS Server is deployed by running the following command:

```bash
kubectl get pods -n chaos-mesh -l app.kubernetes.io/component=chaos-dns-server
```

Make sure that the Pod status is `Running`.

## Notes

1. Currently, DNSChaos only supports record types `A` and `AAAA`.

2. The chaos DNS service runs CoreDNS with the [k8s_dns_chaos](https://github.com/chaos-mesh/k8s_dns_chaos) plugin. If the CoreDNS service in your Kubernetes cluster contains some special configurations, you can edit configMap `dns-server-config` to make the configuration of the chaos DNS service consistent with that of the K8s CoreDNS service using the following command:

    ```bash
    kubectl edit configmap dns-server-config -n chaos-mesh
    ```

## Simulate fault injections by kbcli

DNS faults can be simulated as `random` and `error`. You can select one type for DNS fault injection.

`--pattern` selects a domain template that matches faults and it is required. Placeholder `?` and wildcard `*` are supported.

### DNS random

Run the command below to inject DNS faults into all Pods in the default namespace, which means a random IP address will be returned when a DNS request is sent to the specified domains.

```bash
kbcli fault network dns random --patterns=google.com --duration=1m
```

### DNS error

Run the command below to inject DNS faults into all Pods in the default namespace, which means an error will be returned when a DNS request is sent to the specified domains.

```bash
kbcli fault network dns error --patterns=google.com --duration=1m
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can view the YAML file by adding `--dry-run` at the end of the above kbcli commands. Meanwhile, you can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-dns-chaos-on-kubernetes/#create-experiments-using-the-yaml-file) for details.

### DNS-random example

1. Write the experiment configuration to the `dns-random.yaml` file.

    In the following example, Chaos Mesh injects DNS faults into all Pods in the default namespace, which means an IP address will be returned when a DNS request is sent to the specified domains.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: DNSChaos
    metadata:
      creationTimestamp: null
      generateName: dns-chaos-
      namespace: default
    spec:
      action: random
      duration: 1m
      mode: all
      patterns:
      - google.com
      selector:
        namespaces:
        - default
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./dns-random.yaml
   ```

### DNS-error example

1. Write the experiment configuration to the `dns-error.yaml` file.

    In the following example, Chaos Mesh injects DNS faults into all Pods in the default namespace, which means an error will be returned when a DNS request is sent to the specified domains.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: DNSChaos
    metadata:
      creationTimestamp: null
      generateName: dns-chaos-
      namespace: default
    spec:
      action: error
      duration: 1m
      mode: all
      patterns:
      - google.com
      selector:
        namespaces:
        - default
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-partition.yaml
   ```

### Field description

| Parameter | Type | Description | Default value | Required | Example |
| :-- | :-- | :-- | :-- | :-- | :-- |
| `action` | string | Defines the behavior of DNS fault. Optional values: `random` or `error`. When the value is `random`, DNS service returns a random IP address; when the value is `error`, DNS service returns an error. | None | Yes | `random` or `error` |
| `patterns` | String array | Selects a domain template that matches faults. Placeholder `?` and wildcard `*` are supported.  | [] | No | `google.com`, `chaos-mesh.org`, `github.com` |
| `mode` | string | Specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes | `one` |
| `value` | string | Provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No | `1` |
| `selector` | struct | Specifies the target Pod. | None | Yes |  |
