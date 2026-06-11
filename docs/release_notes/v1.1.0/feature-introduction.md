# KubeBlocks 1.1.0 Feature Introduction

This document groups the selected KubeBlocks 1.1.0 features by user-facing capability. Baseline is the official remote `v1.0.0` tag (`8a57afeb46c90d6687b67ca586d250b6bc2f5677`). Features already available in official 1.0.0 or the 1.0 maintenance branch are intentionally excluded from this list.

## Multi-Cluster Support Through the Instance API

KubeBlocks 1.1.0 uses the experimental `Instance` API as the foundation for multi-cluster database management. A single KubeBlocks control plane can manage database instances distributed across multiple Kubernetes data clusters.

## Rollout for Cluster Instances

This release adds the experimental `Rollout` API and controller for cluster instance rollout workflows. Rollout supports cluster instance replacement, create strategy, sharding rollout, canary-style updates, and richer rollout status tracking.

## ComponentNetwork API

KubeBlocks 1.1.0 adds component-level network configuration through `spec.componentSpecs[*].network`, including host networking, explicit host port mappings, pod host aliases, DNS policy, and DNS config.

## Dynamic InstanceSet Template Adoption

KubeBlocks 1.1.0 supports dynamically assigning existing pods to named instance templates by ordinal. This lets users tune selected instances while preserving pod identity and PVC data.

## Sharding Enhancements

Sharding support now includes lifecycle actions (`shardAdd`, `shardRemove`, `postProvision`, `preTerminate`), shard-specific scale-in via `spec.shardings[*].offline`, and heterogeneous shard groups via `spec.shardings[*].shardTemplates`. This reduces custom scripts for distributed engines and gives operators a consistent API path for shard-level maintenance.

## Volume Sharing Among Instances

The PVC template adds `persistentVolumeClaimName`, which decouples PVC names from pod and template names (final name: `<prefix>-<ordinal>`). Instances keep or share PVCs when moving between instance templates, and pre-provisioned volumes can be re-attached.

## Release Classification

| Capability | Representative PRs | Classification |
| --- | --- | --- |
| Multi-cluster through Instance API | #9697 | v1.1-focused |
| Multi-cluster Service object references | #10126, #10127 | v1.1-focused |
| Multi-cluster external managed template sync | #10143 | v1.1-focused |
| Rollout API and create strategy | #9456, #10010 | v1.1-focused |
| ComponentNetwork host ports | #9892 | v1.1-focused |
| ComponentNetwork host aliases and DNS config | #9415 | v1.1-focused |
| Dynamic InstanceSet Template pod naming rule | #9292 | v1.1-focused |
| Dynamic InstanceSet Template serviceVersion | #9283 | v1.1-focused |
| Dynamic InstanceSet Template compDef | #9469 | v1.1-focused |
| Dynamic InstanceSet Template ordinal adoption | #9475 | v1.1-focused |
| Sharding lifecycle actions | #9830 | v1.1-focused |
| Scale in specified shard | #9417 | v1.1-focused |
| Heterogeneous shard templates | #9491 | v1.1-focused |
| Volume sharing among instances | #9355 | v1.1-focused |
