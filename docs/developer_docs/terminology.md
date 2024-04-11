---
title: Terminology
description: Terms you should know of KubeBlocks
keywords: [terminology]
sidebar_position: 2
sidebar_label: Terminology
---
# Terminology

##### Addon

An addon is an efficient and open extension mechanism. With the KubeBlocks addon, developers can quickly add a new database engine to KubeBlocks and obtain specific foundational management functionalities of that database engine, including but not limited to lifecycle management, data backup and recovery, metrics and log collection, etc.
##### ActionSet

An ActionSet declares a set of commands to perform backup and restore operations using specific tools, such as commands to backup MySQL using xtrabackup, as well as commands to restore data from the backup.

##### BackupPolicy

A BackupPolicy represents a backup strategy for a Cluster, including details such as the backup repository (BackupRepo), backup targets, and backup methods. Multiple backup methods can be defined within a backup policy, with each method referring to a corresponding ActionSet. When creating a backup, the backup policy and backup method can be specified for the backup process.

##### BackupRepo

BackupRepo is the storage repository for backup data. Its principle involves using a CSI driver to upload backup data to various storage systems, such as object storage systems like S3, GCS, as well as storage servers like FTP, NFS, and others.

##### BackupSchedule

BackupSchedule declares the configuration for automatic backups in a Cluster, including backup frequency, retention period, backup policy, and backup method. The BackupSchedule Controller creates a CronJob to automatically backup the Cluster based on the configuration specified in the Custom Resource (CR).

##### Cluster 

Cluster is composed by [components](#component-is-the-fundamental-assembly-component-used-to-build-a-data-storage-and-processing-system-a-component-utilizes-a-statefulset-either-native-to-kubernetes-or-specified-by-the-customer-such-as-openkruise-to-manage-one-to-multiple-pods).

##### Component

A component is the fundamental assembly component used to build a data storage and processing system. A Component utilizes a StatefulSet (either native to Kubernetes or specified by the customer, such as OpenKruise) to manage one to multiple Pods.

##### ComponentRef

ComponentRef is used to select the component and its fields to be referenced.

##### ConfigConstraint

KubeBlocks abstracts engine configuration files into ConfigConstraints to better support configuration changes. The abstracted information within ConfigConstraints includes the following content:
 - the format of the configuration file;
 - the dynamic and static parameters and the immutable parameters;
 - the dynamically changing parameters;
 - the parameter parity rules.

##### CRD (Custom Resource Definition)

CRD (Custom Resource Definition) extends the Kubernetes API, empowering developers to introduce new data types and objects known as custom resources.

##### Operator

Operator, a type of custom resource, automates tasks typically performed by human operators when managing one or more applications or services. By ensuring that a resource's defined state consistently aligns with its observed state, an operator supports Kubernetes in its management responsibilities.

##### OpsDefinition

Ops is short for "Operations," representing database maintenance operations. It defines the operations tasks related to database management, specifying which operations are supported by the cluster and components.

##### OpsRequest

An OpsRequest represents a single operation request.

##### RBAC (Role-Based Access Control)

RBAC (Role-Based Access Control), also known as role-based security, is a methodology employed in computer systems security to limit access to a system's network and resources exclusively to authorized users. Kubernetes features a built-in API for managing roles within namespaces and clusters, enabling their association with specific resources and individuals.

##### RSM （ReplicatedStateMachines）

ReplicatedStateMachines is a workload that manages native Kubernetes objects such as StatefulSet and Pods.

##### ServiceDescriptor

The ServiceDescriptor is a Custom Resource (CR) object used to describe API objects that reference storage services. It allows users to abstract a service provided either by Kubernetes or non-Kubernetes environments, making it available for referencing by other Cluster objects within KubeBlocks. The "ServiceDescriptor" can be used to address issues such as service dependencies, component dependencies, and component sharing within KubeBlocks.
