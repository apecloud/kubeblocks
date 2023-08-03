---
title: Overview
description: Overview for fault injection
sidebar_position: 1
sidebar_label: Overview
---

# Overview

Fault injection is an important way to verify the stability and reliability of data products. There are mature fault injection products in the Kubernetes ecosystem, such as Chaos Mesh and Litmus. KubeBlocks directly integrates [Chaos Mesh](https://chaos-mesh.org/) as an add-on and performs fault injection through both CLI and YAML files.

KubeBlocks provides two major types of fault injection.

* Faults of basic resource types:
  * PodChaos: Simulates Pod faults, such as Pod restart, Pods failure, and container restart in a specified Pod.
  * NetworkChaos: Simulates network faults, such as network delay, network packet loss, packet disorder, and various network partitions.
  * DNSChaos: Simulates DNS faults, such as random DNS domain name and returning the wrong IP address.
  * HTTPChaos: Simulates HTTP communication faults, such as HTTP communication delays.
  * StressChaos: Simulates CPU or memory stress.
  * IOChaos: Simulates the file I/O fault of a specific application, such as I/O delay, read and write failure.
  * TimeChaos: Simulates abnormal time offset.

* Faults of a platform:
  * AWSChaos: Simulates AWS platform failures, such as AWS instance stop, restart, and detaching volume.
  * GCPChaos: Simulates GCP platform failures, such as GCP instance stop, restart, and detaching volume.
