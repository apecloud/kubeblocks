---
title: 连接到 Kafka 集群 
description: 如何连接到 Kafka 集群
keywords: [kafka, 集群, 连接, 网络]
sidebar_position: 2
sidebar_label: 连接
---
## 概述

在连接 Kafka 集群之前，请检查网络环境，并确认连接场景。一般来说，有四种连接场景：

- 在 Kubernetes 集群内连接到 Kafka 集群。
- 在 Kubernetes 集群外但在同一个 VPC 中连接到 Kafka 集群。
- 在公共互联网连接到 Kafka 集群。

## 在 Kubernetes 集群内连接到 Kafka 集群

使用 ClusterIP 服务：9092 直接连接 Kafka 集群。

***步骤：***

1. 获取 Kafka ClusterIP 服务的地址和端口号。

   ```bash
   kubectl get svc 
   ```

   *示例:*

   ```bash
   NAME                    TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                               AGE
   kubernetes              ClusterIP   10.43.0.1     <none>        443/TCP                               9d
   ivy85-broker-headless   ClusterIP   None          <none>        9092/TCP,9093/TCP,9094/TCP,5556/TCP   7d16h
   ivy85-broker            ClusterIP   10.43.8.124   <none>        9093/TCP,9092/TCP,5556/TCP            7d16h
   ```

2. 使用端口号连接到 Kafka 集群。

   下面展示如何使用官方客户端脚本连接 Kafka 集群。

   a. 启动客户端 Pod。

     ```bash
     kubectl run kafka-producer --restart='Never' --image docker.io/bitnami/kafka:3.3.2-debian-11-r54 --command -- sleep infinity
     kubectl run kafka-consumer --restart='Never' --image docker.io/bitnami/kafka:3.3.2-debian-11-r54 --command -- sleep infinity
     ```

   b. 登录 kafka-producer。

     ```bash
     kubectl exec -ti kafka-producer -- bash
     ```

   c. 创建主题。

     ```bash
     kafka-topics.sh --create --topic quickstart-events --bootstrap-server xxx-broker:9092
     ```

   d. 创建生产者。

     ```bash
     kafka-console-producer.sh --topic quickstart-events --bootstrap-server xxx-broker:9092 
     ```

   e. 输入：Hello, KubeBlocks，然后按 Enter 键。

   f. 打开新的终端窗口并登录到 kafka-consumer。

     ```bash
     kubectl exec -ti kafka-consumer -- bash
     ```

   g. 创建消费者，指定消费主题和从开头开始消费消息。

     ```bash
     kafka-console-consumer.sh --topic quickstart-events --from-beginning --bootstrap-server xxx-broker:9092
     ```

然后将得到输出 'Hello, KubeBlocks'。

## 在 Kubernetes 集群外但在同一个 VPC 中连接到 Kafka 集群

许多使用 AWS EKS 的用户希望能从 EC2 实例访问 Kafka 集群，本节将展示具体操作步骤。

***步骤：***

1. 将 host-network-accessible 的值设置为 true。

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

2. 获取相应的 ELB 地址。

   ```bash
   kubectl get svc 
   ```

   image.png

   请注意：
   - poplar50-broker 是内置的 broker advertised.listeners 的服务名称。
   - a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn 是可以从 Kubernetes 集群外部（但在同一 VPC 内）访问的 ELB 地址。

3. 配置主机名映射。
  
   a. 登录到 EC2 实例。
   b. 检查 ELB 地址的 IP 地址。

     ```bash
     nslookup a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn
     ```

   image.
   c. 配置 /etc/hosts 映射。
  
     ```bash
     vi /etc/hosts
     # at the bottom, add the address.
     52.83.xx.xx poplar50-broker
     ```

4. 使用 ELB 地址进行连接。 

在上例中，ELB 地址为 a0e01377fa33xxx-xxx.cn-northwest-1.elb.amazonaws.com.cn:9092。

## 在公共互联网连接到 Kafka 集群

***步骤：***

1. 将 --publicly-accessible 的值设置为 true。

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
