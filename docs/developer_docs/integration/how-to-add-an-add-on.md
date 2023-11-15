---
title: Add an add-on
description: Add an add-on to KubeBlocks
keywords: [add-on, add an add-on]
sidebar_position: 2
sidebar_label: Add an add-on
---

# Add an add-on to KubeBlocks

This tutorial explains how to integrate an add-on to KubeBlocks, and takes Oracle MySQL as an example. You can also find the [PR here](https://github.com/apecloud/learn-kubeblocks-addon).

There are altogether 3 steps to integrate an add-on:

1. Design cluster blueprint.
2. Prepare cluster templates.
3. Add an `addon.yaml` file.

## Step 1. Design a blueprint for cluster

Before getting started, make sure to design your cluster blueprint. Think about what you want your cluster to look like. For example:

- What components it has
- What format each component takes
  - stateful/stateless
  - Standalone/Replication/RaftGroup

In this tutorial you will learn how to deploy a cluster with one Stateful component which has only one node. The design configuration of the cluster is shown in the following table.

Cluster Format: Deploying a MySQL 8.0 Standalone.

:paperclip: Table 1. Blueprint for Oracle MySQL Cluster

| Term              | Settings                                                                                                     |
|-------------------|--------------------------------------------------------------------------------------------------------------|
| CLusterDefinition | Startup Scripts: Default Configuration Files: Default Service Port: 3306 Number of Components: 1, i.e. MySQL |
| ClusterVersion    | Image: docker.io/mysql:8.0.34                                                                                |
| Cluster.yaml      | Specified by the user during creation                                                                        |

## Step 2. Prepare cluster templates

### 2.1 Create a Helm chart

Opt 1.`helm create oracle-mysql`

Opt 2. Directly create `mkdir oracle-mysql`

It should contain the following information:

```bash
> tree oracle-mysql
.
├── Chart.yaml        #  A YAML file containing information about the chart
├── templates         # A directory of templates that, when combined with values, will generate valid Kubernetes manifest files.
│   ├── NOTES.txt     # OPTIONAL: A plain text file containing short usage notes
│   ├── _helpers.tpl  # A place to put template helpers that you can re-use throughout the chart
│   ├── clusterdefinition.yaml  
│   └── clusterversion.yaml
└── values.yaml       # The default configuration values for this chart

2 directories, 6 files
```

There are two YAML files under `templates`, `clusterDefinition.yaml` and `clusterVersion.yaml`, which is about the component topology and version.

- `clusterDefinition.yaml`

  This YAML file is very long, and each field is explained as follows.

  - `ConnectionCredential`

    ```yaml
      connectionCredential:
        username: root
        password: "$(RANDOM_PASSWD)"
        endpoint: "$(SVC_FQDN):$(SVC_PORT_mysql)"
        host: "$(SVC_FQDN)"
        port: "$(SVC_PORT_mysql)"
    ```

    It generates a secret, whose naming convention is `{clusterName}-conn-credential`.

    The field contains general information such as username, password, endpoint and port, and will be used when other services access the cluster (The secret is created before other resources, which can be used elsewhere).

    `$(RANDOM_PASSWD)` will be replaced with a random password when created.

    `$(SVC_PORT_mysql)` specifies the port number to be exposed by selecting the port name. Here the port name is mysql.

    For more information, please refer to KubeBlocks Environment Variables (this doc is under developing and will be published soon).

  - `ComponentDefs`

      ```yaml
        componentDefs:
          - name: mysql-compdef
            characterType: mysql
            workloadType: Stateful
            service:
              ports:
                - name: mysql
                  port: 3306
                  targetPort: mysql
            podSpec:
              containers:
              ...
      ```

    `componentDefs` (Component Definitions) defines the basic information required for each component, including startup scripts, configurations and ports.

    Since there is only one MySQL component, you can just name it `mysql-compdef`, which stands for a component definition for MySQL.

  - `name` [Required]

    It is the name of the component. As there are no specific criteria, you can just choose a distinguishable and expressive one.

    Remember the equation in the previous article?

    $$Cluster = ClusterDefinition.yaml \Join ClusterVersion.yaml \Join ...$$

    `name` here is the join key.

    Remember the `name`; it will be useful.

  - `characterType` [Optional]

    `characterType` is a string type used to identify the engine. For example, `mysql`, `postgresql` and `redis` are several predefined engine types used for database connection. When operating a database, it helps to quickly recognize the engine type and find the matching operation command.

    It can be an arbitrary string, or a unique name as you define. The fact is that people seldom have engine-related operations in the early stage, so you can just leave it blank.

  - `workloadType` [Required]

    It is the type of workload. Kubernetes is equipped with several basic workload types, such as Deployment and StatefulSet.

    On top of that, KubeBlocks makes abstractions and provides more choices, such as:

    - Stateless, meaning it has no stateful services
    - Stateful, meaning it has stateful services
    - Consensus, meaning it has stateful services with self-election capabilities and roles.

    A more in-depth introduction to workloads will be presented later (including design, implementation, usage, etc.).

    For a MySQL Standalone, `Stateful` will do.

  - `service` [Optional]

    ```yaml
          service:
            ports:
              - name: mysql  #The port name is mysql, so connectionCredential will look for it to find the corresponding port
                port: 3306
                targetPort: mysql
    ```

    It defines how to create a service for a component and which ports to expose.

    Remember that in the `ConnectionCredential` section, it is mentioned that a cluster will expose ports and endpoints?

    You can invoke `$(SVC_PORT_mysql)$` to select a port, where `mysql` is the `service.ports[0].name` here.

:::note

If the `connectionCredential` is filled with a port name, make sure the port name appears here.

:::

  - `podSpec`

    The definition of podSpec is the same as that of the Kubernetes.

    ```yaml
          podSpec:
            containers:
              - name: mysql-container
                imagePullPolicy: IfNotPresent
                volumeMounts:
                  - mountPath: /var/lib/mysql
                    name: data
                ports:
                  - containerPort: 3306
                    name: mysql
                env:
                  - name: MYSQL_ROOT_HOST
                    value: {{ .Values.auth.rootHost | default "%" | quote }}
                  - name: MYSQL_ROOT_USER
                    valueFrom:
                      secretKeyRef:
                        name: $(CONN_CREDENTIAL_SECRET_NAME)
                        key: username
                  - name: MYSQL_ROOT_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: $(CONN_CREDENTIAL_SECRET_NAME)
                        key: password
    ```

    As is shown above, a pod is defined with a single container named `mysql-container`, along with other essential information, such as environment variables and ports.

    Yet here is something worth noting: `$(CONN_CREDENTIAL_SECRET_NAME)`.

    The username and password are obtained as pod environment variables from the secret in `$(CONN_CREDENTIAL_SECRET_NAME)`.

    This is a placeholder for ConnectionCredential Secret mentioned earlier.

- ClusterVersion

   All version-related information is configured in `ClusterVersion.yaml`.

   Now you can add the required image information for each container needed for each component.

   ```yaml
     clusterDefinitionRef: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
         - name: mysql-container
           image: {{ .Values.image.registry | default "docker.io" }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}
           imagePullPolicy: {{ default .Values.image.pullPolicy "IfNotPresent" }}
   ```

   Remember the ComponentDef Name used in ClusterDefinition? Yes, `mysql-compdef`, fill in the image information here.

:::note

Now you've finished with ClusterDefinition and ClusterVersion, try to do a quick test by installing them locally.

:::

### 2.2 Install Helm chart

Install Helm.

```bash
helm install oracle-mysql ./oracle-mysql
```

After successful installation, you can see the following information:

```yaml
NAME: oracle-mysql
LAST DEPLOYED: Wed Aug  2 20:50:33 2023
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

### 2.3 Create a cluster

Create a MySQL cluster with `kbcli cluster create`.

```bash
kbcli cluster create mycluster --cluster-definition oracle-mysql
>
Info: --cluster-version is not specified, ClusterVersion oracle-mysql-8.0.34 is applied by default
Cluster mycluster created
```

You can specify the name of ClusterDefinition by using `--cluster-definition`.

:::note

If only one ClusterVersion object is associated with this ClusterDefinition, kbcli will use it when creating the cluster.

However, if there are multiple ClusterVersion objects associated, you will need to explicitly specify which one to use.

:::

After the creating, you can:

**A. Check cluster status**

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     oracle-mysql         oracle-mysql-8.0.34   Delete               Running   Aug 02,2023 20:52 UTC+0800
   ```

**B. Connect to the cluster**

   ```bash
   kbcli cluster connect mycluster
   >
   Connect to instance mycluster-mysql-compdef-0
   mysql: [Warning] Using a password on the command line interface can be insecure.
   Welcome to the MySQL monitor.  Commands end with ; or \g.
   Your MySQL connection id is 8
   Server version: 8.0.34 MySQL Community Server - GPL

   Copyright (c) 2000, 2023, Oracle and/or its affiliates.

   Oracle is a registered trademark of Oracle Corporation and/or its
   affiliates. Other names may be trademarks of their respective
   owners.

   Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

   mysql>
   ```

**C. Scale up a cluster**

   ```bash
   kbcli cluster vscale mycluster --components mysql-compdef --cpu='2' --memory=2Gi
   ```

**D. Stop a cluster**

   Stopping the cluster releases all computing resources.

   ```bash
   kbcli cluster stop mycluster
   ```

## Step 3. Add an addon.yaml file

This is the last step to integrate an add-on to KubeBlocks. After creating this addon.yaml file, this add-on is in the KubeBlocks add-on family. Please refer to `tutorial-1-create-an-addon/oracle-mysql-addon.yaml`.

```bash
apiVersion: extensions.kubeblocks.io/v1alpha1
kind: Addon
metadata:
  name: tutorial-mysql
spec:
  description: 'MySQL is a widely used, open-source....'
  type: Helm
  helm:                                     
    chartsImage: registry-of-your-helm-chart
  installable:
    autoInstall: false
    
  defaultInstallValues:
    - enabled: true
```

And then configure your Helm chart remote repository address with `chartsImage`.

## Step 4. (Optional) Publish to Kubeblocks community

You can contribute the Helm chart to the [KubeBlocks add-ons](https://github.com/apecloud/kubeblocks-addons) and `addon.yaml` to the [KubeBlocks](https://github.com/apecloud/kubeblocks). The `addon.yaml` can be found in the `kubeblocks/deploy/helm/templates/addons` directory.

## Appendix

### A.1 How to configure multiple versions for the same engine?

To support multiple versions is one of the common problems in the daily production environment. And the problem can be solved by **associating multiple ClusterVersions with the same ClusterDefinition**.

Take MySQL as an example.

1. Modify `ClusterVersion.yaml` file to support multiple versions.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: ClusterVersion
   metadata:
     name: oracle-mysql-8.0.32
   spec:
     clusterDefinitionRef: oracle-mysql   ## Associate the same clusterdefinition: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
           - name: mysql-container
             image: <image-of-mysql-8.0.32> ## The mirror address is 8.0.32
   ---
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: ClusterVersion
   metadata:
     name: oracle-mysql-8.0.18
   spec:
     clusterDefinitionRef: oracle-mysql  ## Associate the same clusterdefinition: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
           - name: mysql-container
             image: <image-of-mysql-8.0.18> ## The mirror address is 8.0.18
   ```

2. Specify the version information when creating a cluster.

   - Create a cluster with version 8.0.32

     ```bash
     kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.32
     ```

   - Create a cluster with version 8.0.18

     ```bash
     kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.18
     ```

   It allows you to quickly configure multiple versions for your engine.

### A.2 What if kbcli cannot meet your needs?

While kbcli provides a convenient and generic way to create clusters, it may not meet the specific needs of every engine, especially when a cluster contains multiple components and needs to be used according to different requirements.

In that case, try to use a Helm chart to render the cluster, or create it through a cluster.yaml file.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: default
spec:
  clusterDefinitionRef: oracle-mysql        # Specify ClusterDefinition
  clusterVersionRef: oracle-mysql-8.0.32    # Specify ClusterVersion
  componentSpecs:                           # List required components
  - componentDefRef: mysql-compdef          # The type of the first component: mysql-compdef
    name: mysql-comp                        # The name of the first component: mysql-comp
    replicas: 1 
    resources:                              # Specify CPU and memory size
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    volumeClaimTemplates:                   # Set the PVC information, where the name must correspond to that of the Component Def.
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
```
