---
title: Connect to a Kafka cluster 
description: Guide for cluster creation for kafka
keywords: [kafka, cluster, connect, network]
sidebar_position: 2
sidebar_label: Connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Overview

Before you connect to the Kafka cluster, you must check your network environment, and from which network you would like to connect to the cluster.
There are three scenarios of connecting.

* Connect to the cluster within the same Kubernetes cluster.
* Connect to a kafka cluster from outside of the Kubernetes cluster but in the same VPC.
* Connect to a kafka cluster from public internet.

## Connect to a kafka cluster within the Kubernetes cluster

Within the same Kubernetes cluster, you can directly access the Kafka cluster with clusterIp service:9092

***Steps:***

1. Get the address of the Kafka ClusterIP service port No..

   ```bash
   kubectl get svc 
   ```

   *Example:*

   ```bash
   NAME                    TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                               AGE
   kubernetes              ClusterIP   10.43.0.1     <none>        443/TCP                               9d
   ivy85-broker-headless   ClusterIP   None          <none>        9092/TCP,9093/TCP,9094/TCP,5556/TCP   7d16h
   ivy85-broker            ClusterIP   10.43.8.124   <none>        9093/TCP,9092/TCP,5556/TCP            7d16h
   ```

2. Connect to the kafka cluster with the port No..

   Below is an example of connecting with the official client script.

   a. Start client pod

     ```bash
     kubectl run kafka-producer --restart='Never' --image docker.io/bitnami/kafka:3.3.2-debian-11-r54 --command -- sleep infinity
     kubectl run kafka-consumer --restart='Never' --image docker.io/bitnami/kafka:3.3.2-debian-11-r54 --command -- sleep infinity
     ```

   b. Login to kafka-producer

     ```bash
     kubectl exec -ti kafka-producer -- bash
     ```

   c. Create topic

     ```bash
     kafka-topics.sh --create --topic quickstart-events --bootstrap-server xxx-broker:9092
     ```

   d. Create producer

     ```bash
     kafka-console-producer.sh --topic quickstart-events --bootstrap-server xxx-broker:9092 
     ```

   e. Enter："Hello, KubeBlocks" and press Enter.

   f. Start a new terminal session and login to kafka-consumer.

     ```bash
     kubectl exec -ti kafka-consumer -- bash
     ```

   g. Create consumer and specify consuming topic, and consuming message from the beginning.

     ```bash
     kafka-console-consumer.sh --topic quickstart-events --from-beginning --bootstrap-server xxx-broker:9092
     ```

And you get the output 'Hello, KubeBlocks'.

## Connect to a Kafka cluster from outside of the Kubernetes cluster but in the same VPC

If you use AWS EKS, you may want to access to the Kafka cluster from EC2 instance. This section shows how to perform the connection.

***Steps:***

1. Set the value of host-network-accessible as true.

    <Tabs>
    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster create kafka --host-network-accessible=true
    ```

    </TabItem>
    <TabItem value="kubectl" label="kubectl" >

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: kafka
      namespace: default
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: kafka-server
        monitor: false
        name: broker
        replicas: 1
        resources:
          limits:
            cpu: "1"
            memory: 1Gi
          requests:
            cpu: "1"
            memory: 1Gi
        serviceAccountName: kb-sa-kafka
        services:
        - annotations: 
            service.beta.kubernetes.io/aws-load-balancer-type: nlb
            service.beta.kubernetes.io/aws-load-balancer-internal: "true"
          name: vpc
          serviceType: LoadBalancer
        tls: false
      terminationPolicy: Delete
    EOF
    ```

    </TabItem>

    </Tabs>

2. Get the corresponding ELB address.

   ```bash
   kubectl get svc 
   ```

   image.png

   Information to be noticed:

   * poplar50-broker: broker build-in advertised.listeners, service name
   * a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn: The ELB address which can be accessed from outside of the Kubernetes cluster within the same VPC.  

3. Configure hostname mapping.
  
   a. Login to the EC2 instance.
   b. Check ELB address IP address.

     ```bash
     nslookup a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn
     ```

   image.
   c. Configure /etc/hosts mapping.
  
     ```bash
     vi /etc/hosts
     # at the bottom, add the address.
     52.83.xx.xx poplar50-broker
     ```

4. Use ELB address to connect. In the above example, it is a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn:9092

## Connect to a Kafka cluster from public internet

***Steps:***

1. Set the --publicly-accessible value as true when creating cluster.

    <Tabs>
    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster create kafka --publicly-accessible=true
    ```

    </TabItem>

    <TabItem value="kubectl" label="kubectl" >

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: kafka
      namespace: default
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: kafka-server
        monitor: false
        name: broker
        replicas: 1
        resources:
          limits:
            cpu: "1"
            memory: 1Gi
          requests:
            cpu: "1"
            memory: 1Gi
        serviceAccountName: kb-sa-kafka
        services:
        - annotations: 
            service.beta.kubernetes.io/aws-load-balancer-type: nlb
            service.beta.kubernetes.io/aws-load-balancer-internal: "false"
          name: vpc
          serviceType: LoadBalancer
        tls: false
      terminationPolicy: Delete
    EOF
    ```

    </TabItem>

    </Tabs>
