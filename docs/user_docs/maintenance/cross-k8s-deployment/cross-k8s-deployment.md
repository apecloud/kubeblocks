---
title: Deploy a Cluster across Multiple Kubernetes Clusters by KubeBlocks
description: How to deploy a Cluster across Multiple Kubernetes Clusters by KubeBlocks
keywords: [user account]
sidebar_position: 1
sidebar_label: Deploy a Cluster across Multiple Kubernetes Clusters by KubeBlocks
---

# Deploy a Cluster across Multiple Kubernetes Clusters by KubeBlocks

KubeBlocks supports managing multiple Kubernetes clusters to provide new options for instance disaster recovery and K8s cluster management. KubeBlocks introduces the control plane and data plane to support cross-K8s management.

* Control plane: An independent K8s cluster in which the KubeBlocks operator runs. Most of the objects defined by KubeBlocks, such as definition, cluster, backup, and ops, are stored in this cluster. Users interact with the API of this cluster to manage multiple cluster instances.
* Data plane: A K8s cluster that is used to run the actual workloads. There can be one or more clusters in the data plane. These clusters host resources such as pods, persistent volume claims (PVC), services, service accounts (SA), config maps (CM), secrets, jobs, etc., related to the instances. But in KubeBlocks v0.9.0, the KubeBlocks operator does not run in the data plane.

In terms of actual physical deployment, the control plane can be deployed in a single availability zone (AZ) for simplicity and flexibility. It can also be deployed in multiple different AZs to provide higher availability guarantees. Alternatively, it can be deployed by reusing a data plane, which offers a lower-cost approach to running the control plane.

## Prepare an environment

Create several K8s clusters and prepare the configuration information for deploying KubeBlocks. This tutorial takes three data plane K8s clusters as an example and their contexts are named as k8s-1, k8s-2, and k8s-3.

* Create K8s clusters: one for the control plane and several for data plane. Make sure the API servers of these data plane K8s clusters can be reached from the control plane, which should include both network connectivity and access configuration.
* Prepare the configuration information for the KubeBlocks operator to access the data plane K8s clusters. Store this information in the control plane cluster as a secret and this secrect should passed when deploying the KubeBlocks operator. The secret key should be "kubeconfig" and its value should follow the standard kubeconfig format. For example,

   ```bash
   apiVersion: v1
   kind: Secret
   metadata:
     namespace: kb-system
     name: <your-secret-name> 
   type: kubernetes.kubeconfig
   stringData:
     kubeconfig: |
       apiVersion: v1
       clusters:
         ...
       contexts:
         ...
       kind: Config
       users:
         ...
   ```

## Deploy cross-K8s clusters

### Deploy the Kubeblocks operator

Install KubeBlocks in the control plane.

1. Run the command below to install KubeBlocks.

   ```bash
   # multiCluster.kubeConfig specifies the secret where the kubeconfig information for the data plane k8s clusters is stored
   # multiCluster.contexts specifies the contexts of the data plane k8s clusters
   kbcli kubeblocks install --version=0.9.0 --set multiCluster.kubeConfig=<secret-name> --set multiCluster.contexts=<contexts>
   ```

2. Validate the installation.

   ```bash
   kbcli kubeblocks status
   ```

### RBAC

When the workload instances are running in the data plane k8s clusters, specific RBAC (Role-Based Access Control) resources are required to perform management actions. Therefore, it is necessary to install the required RBAC resources for KubeBlocks in each data plane cluster separately.

```bash
# 1. Extract the required clusterrole resource from the control plane dump: kubeblocks-cluster-pod-role
kubectl get clusterrole kubeblocks-cluster-pod-role -o yaml > /tmp/kubeblocks-cluster-pod-role.yaml

# 2. Edit the file content to remove unnecessary meta information such as UID and resource version, while retaining other content

# 3.Apply the file to the other data plane clusters
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-1
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-2
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-3
```

### Network

KubeBlocks leverages the abstraction of K8s services to provide internal and external service access. For service abstraction, there is usually a default implementation for accessing k8s within the cluster, while traffic from outside the cluster typically requires users to provide their own solutions. In the context of multi K8s clusters, whether it's replication traffic between instances or client access traffic, it essentially falls under external traffic. Therefore, to ensure the smooth operation of cross-cluster instances, additional network handling is generally required.

Here illustrates a set of optional solutions to describe the entire process. In the actual environment, you can choose the appropriate deployment solution based on your own cluster and network environment.

#### East-West traffic

##### For the cloud environment

The K8s services provided by the cloud providers include both internal and external load balancer services. You can directly build the inter-instance communication based on the LB service in a simply and user-friendly way.

##### For the self-host environment

This tutorial takes Cilium Cluster Mesh as an example. Deploy Cilium as the overlay mode and the cluster configuration for each data plane clusters is as follows:

| Cluster | Context | Name  | ID | CIDR        |
|:-------:|:-------:|:-----:|:--:|:-----------:|
| 1       | k8s-1   | k8s-1 | 1  | 10.1.0.0/16 |
| 2       | k8s-2   | k8s-2 | 2  | 10.2.0.0/16 |
| 3       | k8s-3   | k8s-3 | 3  | 10.3.0.0/16 |

:::note

The CIDR mentioned here refers to the address of the Cilium Overlay network. When configuring it, it should be distinct from the host network address.

:::

***Steps:***

The following steps can be performed separately in each cluster (without the `--context` parameter) or collectively in an environment with the information of three contexts (by specifying the `--context` parameter for each).

1. Install Cilium, specifying the cluster ID/name and the cluster pool pod CIDR. Refer to the Cilium doc for details: [Specify the Cluster Name and ID](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#specify-the-cluster-name-and-id).

   ```bash
   cilium install --set cluster.name=k8s-1 --set cluster.id=1 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.1.0.0/16 —context k8s-1
   cilium install --set cluster.name=k8s-2 --set cluster.id=2 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.2.0.0/16 —context k8s-2
   cilium install --set cluster.name=k8s-3 --set cluster.id=3 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.3.0.0/16 —context k8s-3
   ```

2. Enable Cilium Cluster Mesh and wait for it to be ready. NodePort here is used to provide access to the clustermesh control plane. Refer to [the official doc](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#enable-cluster-mesh) for other optional methods and specific information.

   ```bash
   cilium clustermesh enable --service-type NodePort —context k8s-1
   cilium clustermesh enable --service-type NodePort —context k8s-2
   cilium clustermesh enable --service-type NodePort —context k8s-3
   cilium clustermesh status —wait —context k8s-1
   cilium clustermesh status —wait —context k8s-2
   cilium clustermesh status —wait —context k8s-3
   ```

3. Establish connectivity between the clusters and wait for them to be ready. Refer to [the official doc](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#connect-clusters) for details.

   ```bash
   cilium clustermesh connect --context k8s-1 --destination-context k8s-2
   cilium clustermesh connect --context k8s-1 --destination-context k8s-3
   cilium clustermesh connect --context k8s-2 --destination-context k8s-3
   cilium clustermesh status —wait —context k8s-1
   cilium clustermesh status —wait —context k8s-2
   cilium clustermesh status —wait —context k8s-3
   ```

4. (Optional) Check the status of the tunnels between cluster by using the cilium-dbg tool. Refer to [the official doc](https://docs.cilium.io/en/stable/cmdref/cilium-dbg/) for details.

   ```bash
   cilium-dbg bpf tunnel list
   ```

5. (Optional) Test the cluster connectivity. Refer to [the official doc](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#test-pod-connectivity-between-clusters) for details.

#### South-North traffic

The South-North traffic provides services for clients and requires each Pod in the data plane k8s clusters to have a connection address accessible from outside. This address can be implemented using NodePort, LoadBalancer, or other solutions. This tutorial takes NodePort and LoadBalancer as examples.

If the clients do not have routing capabilities for read and write, in addition to the Pod addresses, read-write separation addresses are also required. This can be achieved using a 7-layer proxy, 4-layer SDN VIP, or pure DNS-based solutions. To simplify the problem, this tutorial assumes that the clients have routing capabilities for read and write and can directly configure the connection addresses for all Pods.

##### NodePort

For each Pod in the data plane, a NodePort service is created. Clients can connect using the host network IP and NodePort.

##### LoadBalancer

Here takes MetalLB as an example for providing LoadBalancer Services.

1. Prepare the LB subnet for the data plane. This subnet needs to be reachable by client routing, and it should be different for each k8s cluster:

   | Cluster | Context | Name  | ID | CIDR        |
   |:-------:|:-------:|:-----:|:--:|:-----------:|
   | 1       | k8s-1   | k8s-1 | 1  | 10.4.0.0/16 |
   | 2       | k8s-2   | k8s-2 | 2  | 10.5.0.0/16 |
   | 3       | k8s-3   | k8s-3 | 3  | 10.6.0.0/16 |

2. Deploy MetalLB in all the data place K8s clusters.

   ```bash
   helm repo add metallb https://metallb.github.io/metallb
   helm install metallb metallb/metallb
   ```

3. Wait for the Pods to be ready.

   ```bash
   kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s
   ```

4. Apply the YAML file below in three data plane K8s clusters. Replace `spec.addresses` as the LB network address of the corresponding clusters.

   ```yaml
   apiVersion: metallb.io/v1beta1
   kind: IPAddressPool
   metadata:
     name: example
     namespace: metallb-system
   spec:
     addresses:
     - x.x.x.x/x
   ---
   apiVersion: metallb.io/v1beta1
   kind: L2Advertisement
   metadata:
     name: empty
     namespace: metallb-system
   ```

5. Create a LoadBalancer Service for each Pod in the data plane K8s clusters and obtain all the VIPs (Virtual IP addresses) so that clients can connect to them.

## Verify cross-K8s clusters

When running multiple cluster instances, the access addresses between the replicas cannot directly use the addresses within the original domain (such as Pod FQDN). It requires explicit creation and configuration of service addresses for cross-cluster communication. Therefore, some adaptation work needs to be done for the addons.

This tutorial takes the community edition of etcd as an example. You can refer to [the etcd addon](https://github.com/apecloud/kubeblocks-addons/blob/release-0.9/addons/etcd/templates/componentdefinition.yaml) for the related adaptation results.

### Create an instance

Since different network configurations have different requirements, the following sections provide examples of creating a cross-cluster etcd instance using both cloud-based and self-hosted approaches.

#### Cloud

This example illustrates creating an etcd cluster on Alibaba Cloud. For the configurations of other cloud providers, you can refer to the official docs.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # optional：You can use this annotation to explicitly specify the cluster where the current instance should be distributed
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi # The smallest size required by the cloud provisioning?
        - name: peer
          serviceType: LoadBalancer
          annotations:
            # If you are running on a mutual access solution based on LoadBalancer services, this annotation key is required.
            apps.kubeblocks.io/multi-cluster-service-placement: unique
            #  The annotation key required by ACK LoadBalancer service
            service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
          podService: true
```

The example below illustrates how to deploy clusters across cloud providers.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # optional：You can use this annotation to explicitly specify the cluster where the current instance should be distributed
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi # The smallest size required by the cloud provisioning?
      services:
        - name: peer
          serviceType: LoadBalancer
          annotations:
            # If you are running on a mutual access solution based on LoadBalancer services, this annotation key is required.
            apps.kubeblocks.io/multi-cluster-service-placement: unique
            # The annotation key required by the ACK LoadBalancer service. Since cross-cloud access is required, this key should be configured as a public network type.
            service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
            # The annotation keys required by the VKE LoadBalancer service. Since cross-cloud access is required, this key should be configured as a public network type.
            service.beta.kubernetes.io/volcengine-loadbalancer-subnet-id: <subnet-id>
            service.beta.kubernetes.io/volcengine-loadbalancer-address-type: "PUBLIC"
          podService: true
```

#### Self-hosted environment

The example below illustrates how to create an instance in the self-hosted environment.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # optional：You can use this annotation to explicitly specify the cluster where the current instance should be distributed
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
      services:
        - name: peer
          serviceType: ClusterIP
          annotations:
            service.cilium.io/global: "true" # cilium clustermesh global service
          podService: true
```
