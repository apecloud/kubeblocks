---
title: Kubernetes and Operator 101
description: things about K8s you need to know 
keywords: [K8s, operator, concept]
sidebar_position: 3
---

# Kubernetes and Operator 101

# K8s

What is Kubernetes? Some say it's a container orchestration system, others describe it as a distributed operating system, while some view it as a multi-cloud PaaS (Platform as a Service) platform, and others consider it a platform for building PaaS solutions.

This article will introduce the key concepts and building blocks within Kubernetes.

## K8s Control Plane

The Kubernetes Control Plane is the brain and heart of Kubernetes. It manages the overall operation of the cluster, including processing API requests, storing configuration data, and ensuring the cluster's desired state. Key components include the API Server (which handles communication), etcd (which stores all cluster data), the Controller Manager (which enforces the desired state), the Scheduler (which assigns workloads to Nodes), and the Cloud Controller Manager (which manages cloud-specific integrations, such as load balancers, storage, and networking). Together, these components orchestrate the deployment, scaling, and management of containers across the cluster.

## Node

Some describe Kubernetes as a distributed operating system, capable of managing many Nodes. A Node is a physical or virtual machine that acts as a worker within the cluster. Each Node runs essential services, including the container runtime (such as Docker or containerd), the kubelet, and the kube-proxy. The kubelet ensures that containers are running as specified in a Pod, the smallest deployable unit in Kubernetes. The kube-proxy handles network routing, maintaining network rules, and enabling communication between Pods and services. Nodes provide the computational resources needed to run containerized applications and are managed by the Kubernetes Master, which distributes tasks, monitors Node health, and maintains the desired state of the cluster.

:::note

In certain contexts, the term "Node" can be confusing when discussing Kubernetes (K8s) alongside databases. In Kubernetes, a "Node" refers to a physical or virtual machine that is part of the Kubernetes cluster and serves as a worker to run containerized applications. However, when a database is running within Kubernetes, the term "Database Node" typically refers to a Pod that hosts a database instance.

In the KubeBlocks documentation, "Node" generally refers to a Database Node. If we are referring to a Kubernetes Node, we will explicitly specify it as a "K8s Node" to avoid any confusion.

:::

## kubelet

The kubelet is the agent that the Kubernetes Control Plane uses to manage each Node in the cluster. It ensures that containers are running in a Pod as defined by the Kubernetes control plane. The kubelet continuously monitors the state of the containers, making sure they are healthy and running as expected. If a container fails, the kubelet attempts to restart it according to the specified policies.

## Pod

In Kubernetes, a Pod is somewhat analogous to a virtual machine but is much more lightweight and specialized. It is the smallest deployable unit in Kubernetes.

It represents one or more containers that are tightly coupled and need to work together, along with shared storage (volumes), network resources, and a specification for how to run the containers. These containers can communicate with each other using localhost and share resources like memory and storage.

Kubernetes dynamically manages Pods, ensuring they are running as specified and automatically restarting or replacing them if they fail. Pods can be distributed across Nodes for redundancy, making them fundamental to deploying and managing containerized applications (including databases) in Kubernetes.

## Storage Class

When creating disks for workloads inside a Pod, such as databases, you may need to specify the type of disk media, whether it's HDD or SSD. In cloud environments, there are often more options available. For example, AWS EBS offers various volume types, such as General Purpose SSD (gp2/gp3), Provisioned IOPS SSD (io1/io2), and Throughput Optimized HDD (st1). In Kubernetes, you can select the desired disk type through a StorageClass.

## PVC

A Persistent Volume Claim (PVC) in Kubernetes is a request for storage by a user. A PVC is essentially a way to ask for storage with specific characteristics, such as storage class, size and access modes (e.g., read-write or read-only). PVCs enable Pods to use storage without needing to know the details of the underlying infrastructure.

In K8s, to use this storage, users create a PVC. When a PVC is created, Kubernetes looks for a StorageClass that matches the request. If a matching StorageClass is found, it automatically provisions the storage according to the defined parameters—whether it's SSD, HDD, EBS or NAS. If a PVC does not specify a StorageClass, Kubernetes will use the default StorageClass (if one is configured) to provision storage.

## CSI

In Kubernetes, various StorageClasses are provided through the Container Storage Interface (CSI), which is responsible for provisioning the underlying storage "disks" used by applications. CSI functions similarly to a "disk driver" in Kubernetes, enabling the platform to adapt to and integrate with a wide range of storage systems, such as local disks, AWS EBS, and Ceph. These StorageClasses, and the associated storage resources, are provisioned by specific CSI drivers that handle the interaction with the underlying storage infrastructure.

CSI is a standard API that enables Kubernetes to interact with various storage systems in a consistent and extensible manner. CSI drivers, created by storage vendors or the Kubernetes community, expose essential storage functions like dynamic provisioning, attaching, mounting, and snapshotting to Kubernetes.

When you define a StorageClass in Kubernetes, it typically specifies a CSI driver as its provisioner. This driver automatically provisions Persistent Volumes (PVs) based on the parameters in the StorageClass and associated Persistent Volume Claims (PVCs), ensuring the appropriate type and configuration of storage—whether SSD, HDD, or otherwise—is provided for your applications.

## PV

In Kubernetes, a Persistent Volume (PV) represents a storage resource that can be backed by various systems like local disks, NFS, or cloud-based storage (e.g., AWS EBS, Google Cloud Persistent Disks), typically managed by different CSI drivers.

A PV has its own lifecycle, independent of the Pod, and is managed by the Kubernetes control plane. It allows data to persist even if the associated Pod is deleted. PVs are bound to Persistent Volume Claims (PVCs), which request specific storage characteristics like size and access modes, ensuring that applications receive the storage they require.

In summary, PV is the actual storage resource, while PVC is a request for storage. Through the StorageClass in the PVC, it can be bound to a PV provisioned by different CSI drivers.

## Service

In Kubernetes, a Service acts as a load balancer. It defines a logical set of Pods and provides a policy for accessing them. Since Pods are ephemeral and can be dynamically created and destroyed, their IP addresses are not stable. A Service resolves this issue by providing a stable network endpoint (a virtual IP address, known as a ClusterIP) that remains constant, allowing other Pods or external clients to communicate with the set of Pods behind the Service without needing to know their specific IP addresses.

Service supports different types: ClusterIP (internal cluster access), NodePort (external access via `<NodeIP>:<NodePort>`), LoadBalancer (exposes the Service externally using a cloud provider’s load balancer), and ExternalName (maps the Service to an external DNS).

## ConfigMap

A ConfigMap is used to store configuration data in key-value pairs, allowing you to decouple configuration from application code. This way, you can manage application settings separately and reuse them across multiple environments. ConfigMaps can be used to inject configuration data into Pods as environment variables, command-line arguments, or configuration files. They provide a flexible and convenient way to manage application configurations without hardcoding values directly into your application container.

## Secret

A Secret is used to store sensitive data such as passwords, tokens, or encryption keys. Secrets allow you to manage confidential information separately from your application code and avoid exposing sensitive data in your container images. Kubernetes Secrets can be injected into Pods as environment variables or mounted as files, ensuring that sensitive information is handled in a secure and controlled manner. 

However, Secrets are not encrypted by default—they are simply base64-encoded, which does not provide real encryption. They should still be used with care, ensuring proper access controls are in place.

## CRD

If you want to manage database objects using Kubernetes, you need to extend the Kubernetes API to describe the database objects you're managing. This is where the CRD (Custom Resource Definition) mechanism comes in, allowing you to define custom resources specific to your use case, such as database clusters or backups, and manage them just like native Kubernetes resources.

## CR

A Custom Resource (CR) is an instance of a Custom Resource Definition (CRD). It represents a specific configuration or object that extends the Kubernetes API. CRs allow you to define and manage custom resources, such as databases or applications, using Kubernetes' native tools. Once a CR is created, Kubernetes controllers or Operators monitor it and perform actions to maintain the desired state.

CRD and CR are the foundation for developing a Kubernetes Operator. CRDs are often used to implement custom controllers or operators, allowing for continuously watches for changes to CRs (representing, for example, database clusters) and automatically performs actions.

## What is Kubernetes Operator?

A Kubernetes Operator is a software, typically composed of one or more controllers,  that automates the management of complex applications by translating changes made to a Custom Resource (CR) into actions on native Kubernetes objects, such as Pods, Services, PVCs, ConfigMaps, and Secrets.

- Input: User modifications to the CR.
- Output: Corresponding changes to underlying Kubernetes resources or interactions with external systems (e.g., writing to a database or calling APIs), depending on the requirements of the managed application.

The Operator continuously watches the state of these Kubernetes objects. When changes occur (e.g., a Pod crashes), the Operator automatically takes corrective actions, like recreating the Pod or adjusting traffic (e.g., updating Service Endpoints).

In essence, a Kubernetes Operator encapsulates complex operational knowledge into software, automating tasks like deployment, scaling, upgrades, and backups, ensuring the application consistently maintains its desired state without manual intervention.

## Helm and Helm Chart

Helm is a popular package manager for Kubernetes that helps manage and deploy applications. It packages all the necessary Kubernetes resources into a single Helm Chart, allowing you to install applications with a single command (helm install). Helm also handles configuration management and updates (helm upgrade), making the entire lifecycle of the application much easier to manage. 
Key components of a Helm Chart:

- Templates: YAML files with placeholders that define Kubernetes resources (like Pods, Services, and ConfigMaps).
- Values.yaml: A file where users specify default values for the templates, allowing easy customization. Helm allows you to take an existing chart and override the default values using values.yaml or command-line flags, enabling you to provide environment-specific configurations without modifying the underlying templates.
- Chart.yaml: Metadata about the chart, including the name, version, and description.

Helm integrates well with CI/CD tools like Jenkins, GitLab CI, and GitHub Actions. It can be used to automate deployments and rollbacks as part of a continuous delivery pipeline, ensuring that applications are consistently deployed across different environments.
