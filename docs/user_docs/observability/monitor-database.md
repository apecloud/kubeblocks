---
title: Monitor database
description: How to monitor your database
keywords: [monitor database, monitor a cluster, monitor]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Monitor a database

This tutorial demonstrates how to configure the monitoring function for a PostgreSQL cluster, using Prometheus and Grafana.

## Step 1. Install the Prometheus Operator and Grafana

Install the Promethus Operator and Grafana to monitor the performance of a database. Skip this step if a Prometheus Operator is already installed in your environment.

1. Create a new namespace for Prometheus Operator.

   ```bash
   kubectl create namespace monitoring
   ```

2. Add the Prometheus Operator Helm repository.

   ```bash
   helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
   ```

3. Install the Prometheus Operator.

   ```bash
   helm install prometheus-operator prometheus-community/kube-prometheus-stack --namespace monitoring
   ```

4. Verify the deployment of the Prometheus Operator. Make sure all pods are in the Ready state.

   ```bash
   kubectl get pods -n monitoring
   ```

5. Access the Prometheus and Grafana dashboards.

   1. Check the service endpoints of Prometheus and Grafana.

     ```bash
     kubectl get svc -n monitoring
     ```

   2. Use port forwarding to access the Prometheus dashboard locally.

     ```bash
     kubectl port-forward svc/prometheus-operator-kube-p-prometheus -n monitoring 9090:9090
     ```

     You can also access the Prometheus dashboard by opening "http://localhost:9090" in your browser.

   3. Retrieve the Grafana's login credential from the secret.

     ```bash
     kubectl get secrets prometheus-operator-grafana -n monitoring -o yaml
     ```  

   4. Use port forwarding to access the Grafana dashboard locally.

     ```bash
     kubectl port-forward svc/prometheus-operator-grafana -n monitoring 3000:80
     ```

     You can also access the Grafana dashboard by opening "http://localhost:3000" in your browser.

6. Configure the selectors for PodMonitor and ServiceMonitor to match your monitoring requirements.

   Prometheus Operator uses Prometheus CRD to set up a Prometheus instance and to customize configurations of replicas, PVCs, etc.

   To update the configuration on PodMonitor and ServiceMonitor, modify the Prometheus CR according to your needs:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: Prometheus
   metadata:
   spec:
     podMonitorNamespaceSelector: {} # Namespaces to match for PodMonitors discovery
     #  PodMonitors to be selected for target discovery. An empty label selector
     #  matches all objects.
     podMonitorSelector:
       matchLabels:
         release: prometheus # Make sure your PodMonitor CR labels matches the selector
     serviceMonitorNamespaceSelector: {} # Namespaces to match for ServiceMonitors discovery
     # ServiceMonitors to be selected for target discovery. An empty label selector
     # matches all objects.
     serviceMonitorSelector:
       matchLabels:
         release: prometheus # Make sure your ServiceMonitor CR labels matches the selector
   ```

## Step 2. Monitor a database cluster

This section demonstrates how to use Prometheus and Grafana for monitoring a database cluster.

### Enable the monitoring function for a database cluster

#### For a new cluster

Create a new cluster with the following command, ensuring the monitoring exporter is enabled.

:::note

Make sure `spec.componentSpecs.disableExporter` is set to `false` when creating a cluster.

:::

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-12.14.0
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: postgresql
    componentDefRef: postgresql
    enabledLogs:
    - running
    disableExporter: true # Set to `false` to enable exporter
    replicas: 2
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
EOF
```

#### For an existing cluster

If a cluster already exists, you can run the command below to verify whether the monitoring exporter is enabled.

```bash
kubectl get cluster mycluster -o yaml
```

View the output.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
...
spec:
   ...
   componentSpecs:
   ...
      disableExporter: false
```

Setting `disableExporter: false` or leaving this field unset enables the monitoring exporter, which is the prerequisite of the monitoring function. If the output shows `disableExporter: true`, you need to change it to `false` to enable the exporter.

Note that updating `disableExporter` will restart all pods in the cluster.

<Tabs>

<TabItem value="kubectl patch" label="kubectl patch" default>

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

You can also edit the `cluster.yaml` to enable/disable the monitoring function.

```bash
kubectl edit cluster mycluster -n demo
```

Edit the value of `disableExporter`.

```yaml
...
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: true # Set to `false` to enable exporter
...
```

</TabItem>

</Tabs>

When the cluster is running, each Pod should have a sidecar container, named `exporter` running the postgres-exporter.

### Create PodMonitor

1. Query `scrapePath` and `scrapePort`.

   Retrieve the `scrapePath` and `scrapePort` from the Pod's exporter container.

   ```bash
   kubectl get po mycluster-postgresql-0 -oyaml | yq '.spec.containers[] | select(.name=="exporter") | .ports '
   ```

   <details>

   <summary>Expected Output</summary>

   ```bash
   - containerPort: 9187
     name: http-metrics
     protocol: TCP
   ```

   </details>

2. Create `PodMonitor`.

   Apply the `PodMonitor` file to monitor the cluster. You can also find the latest example YAML file in the [KubeBlocks Addons repo](https://github.com/apecloud/kubeblocks-addons/blob/main/examples/postgresql/pod-monitor.yml).

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: pg-cluster-pod-monitor
     namespace: monitoring # Note: this is namespace for prometheus operator
     labels:               # This is labels set in `prometheus.spec.podMonitorSelector`
       release: prometheus
   spec:
     jobLabel: kubeblocks-service
     # Define the labels which are transferred from the
     # associated Kubernetes `Pod` object onto the ingested metrics
     # set the labels w.r.t your own needs
     podTargetLabels:
     - app.kubernetes.io/instance
     - app.kubernetes.io/managed-by
     - apps.kubeblocks.io/component-name
     - apps.kubeblocks.io/pod-name
     podMetricsEndpoints:
       - path: /metrics
         port: http-metrics
         scheme: http
     namespaceSelector:
       matchNames:
         - default
     selector:
       matchLabels:
         app.kubernetes.io/instance: pg-cluster
         apps.kubeblocks.io/component-name: postgresql
   EOF
   ```

3. Access the Grafana dashboard.

   Log in to the Grafana dashboard and import the dashboard.

   There is a pre-configured dashboard for PostgreSQL under the `APPS / PostgreSQL` folder in the Grafana dashboard. And more dashboards can be found in the [Grafana dashboard store](https://grafana.com/grafana/dashboards/).

::::note

Make sure the labels (such as the values of path and port in endpoint) are set correctly in the `PodMonitor` file to match your dashboard.

:::
