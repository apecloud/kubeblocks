---
title: Simulate DNS faults
description: Simulate DNS faults
sidebar_position: 5
sidebar_label: Simulate DNS faults
---

# Simulate DNS faults

DNSChaos is used to simulate wrong DNS responses. For example, DNSChaos can return an error or return a random IP address when receiving a DNS request.

## Deploy Chaos DNS server

Check if Chaos DNS Server is deployed by running the following command:

```bash
kubectl get pods -n chaos-mesh -l app.kubernetes.io/component=chaos-dns-server
```

Make sure that the Pod status is `Running`.

## Notes

1. Currently, DNSChaos only supports record types `A` and `AAAA`.

2. The chaos DNS service runs CoreDNS with the k8s_dns_chaos plugin. If the CoreDNS service in your Kubernetes cluster contains some special configurations, you can edit configMap `dns-server-config` to make the configuration of the chaos DNS service consistent with that of the K8s CoreDNS service using the following command:

```bash
kubectl edit configmap dns-server-config -n chaos-mesh
```

## Simulate fault injections by kbcli

DNS faults can be simulated as `random` and `error`. You can define oen type for DNS fault injection.

`--pattern` selects a domain template that matches faults. Placeholder `?` and wildcard `*` are supported.

```bash
kbcli fault network dns random --patterns=google.com --duration=1m
```

```bash
kbcli fault network dns error --patterns=google.com --duration=1m
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-network-chaos-on-kubernetes/#create-experiments-using-the-yaml-files) for details.
