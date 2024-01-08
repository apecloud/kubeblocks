---
title: Introduction
description: Introduction of fault injection
sidebar_position: 1
sidebar_label: Introduction
---

# Introduction

Fault injection is an important way to verify the stability and reliability of data products. There are mature fault injection products in the Kubernetes ecosystem, such as Chaos Mesh and Litmus. KubeBlocks directly integrates [Chaos Mesh](https://chaos-mesh.org/) as an add-on to perform fault injection through both CLI and YAML files.

KubeBlocks provides two major types of fault injection.

* Faults of basic resource types:
  * [PodChaos](./pod-faults.md): simulates Pod faults, such as Pod restart, Pods failure, and container restart in a specified Pod.
  * [NetworkChaos](./network-faults.md): simulates network faults, such as network delay, network packet loss, packet disorder, and various network partitions.
  * [DNSChaos](./dns-faults.md): simulates DNS faults, such as random DNS domain names and returning the wrong IP address.
  * [HTTPChaos](./http-fault.md): simulates HTTP communication faults, such as HTTP communication delays.
  * [StressChaos](./stress-fault.md): simulates CPU or memory stress.
  * [IOChaos](./io-faults.md): simulates the file I/O fault of a specific application, such as I/O delay, read and write failure.
  * [TimeChaos](./time-fault.md): simulates abnormal time offset.

* Faults of a platform:
  * [AWSChaos](./aws-fault.md): simulates AWS platform failures, such as AWS instance stop, restart, and detaching volume.
  * [GCPChaos](./gcp-fault.md): simulates GCP platform failures, such as GCP instance stop, restart, and detaching volume.
