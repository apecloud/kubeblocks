---
title: Overview
description: Overview for fault injection
sidebar_position: 1
sidebar_label: Overview
---

# Overview

Fault injection is an important way to verify the stability and reliability of data products. Currently, there are mature fault injection products in the Kubernetes ecosystem, such as Chaos Mesh and Litmus. KubeBlocks directly integrates [Chaos Mesh](https://chaos-mesh.org/) as an add-on and performs fault injection through CLI.

KubeBlocks provides two major types of fault injection.

- Faults of basic resource types:
  - PodChaos: Simulate Pod faults, such as Pod restarting, Pods being continuously unavailable, and faults of some containers in a specific Pod.
  - NetworkChaos: Simulate network faults, such as network delay, network packet loss, packet disorder, and various network partitions.
  - DNSChaos: Simulate DNS faults, such as DNS domain name resolution failure and returning the wrong IP address.
  - HTTPChaos: Simulate HTTP communication faults, such as HTTP communication delays.
  - StressChaos: Simulate CPU or memory hogging scenarios.
  - IOChaos: Simulate the file I/O fault of a specific application, such as I/O delay, read and write failure.
  - TimeChaos: Simulates abnormal time jumps.
- Faults of a platform:
  - AWSChaos: Simulates AWS platform failures, such as AWS node restarting.
  - GCPChaos: Simulates GCP platform failures, such as GCP node restarting.
