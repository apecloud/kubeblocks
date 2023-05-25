# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our appreciation to all contributors who helped make this release happen.

**Breaking changes**
* Reconstructed existing "dbaas.kubeblocks.io" API group to new "apps.kubeblocks.io" API group, affected following APIs:
  - ClusterDefinition
  - ClusterVersion
  - Cluster
  - ConfigConstraint
  - OpsRequest
* Refactored ConfigTemplate related API, affected following APIs:
    - ClusterDefinition
    - ClusterVersion

* Existing APIs will no longer be functional, please make sure you have removed the deprecated APIs and transformed CRDs before upgrade. Please refer to the upgrade notes under this release notes.

**Highlights**
  * Automatic pod container environment variables updates:
    * [NEW] KB_POD_FQDN - KubeBlock Cluster component workload associated headless service name, N/A if workloadType=Stateless.
    * [NEW] KB_POD_IP -  Pod IP address
    * [NEW] KB_POD_IPS - Pod IP addresses
    * [NEW] KB_HOST_IP - Host IP address
    * [DEPRECATED] KB_PODIPS - Pod IP addresses
    * [DEPRECATED] KB_PODIP -  Pod IP address
    * [DEPRECATED] KB_HOSTIP - Host IP address
    * KB_POD_NAME - Pod Name
    * KB_NAMESPACE - Namespace
    * KB_SA_NAME - Service Account Name
    * KB_NODENAME - Node Name
    * KB_CLUSTER_NAME - KubeBlock Cluster API object name
    * KB_COMP_NAME - Running pod's KubeBlock Cluster API object's `.spec.components.name`
    * KB_CLUSTER_COMP_NAME - Running pod's KubeBlock Cluster API object's `<.metadata.name>-<.spec.components.name>`, same name is used for Deployment or StatefulSet workload name, and Service object name
  * New KubeBlocks addon extensions management (an addon components are part of KubeBlocks control plane extensions). Highlights include: 
    * New addons.extensions.kubeblocks.io API that provide running cluster installable check and auto-installation settings.
    * Following addons are provided:
      * Prometheus and Alertmanager
      * AlertManager Webhook Adaptor
      * Grafana
      * Kubeblocks CSI driver
      * S3 CSI driver
      * Snapshot Controller
      * KubeBlocks private network Load Balancer
      * ApeCloud MySQL ClusterDefinition API
      * Community PostgreSQL ClusterDefinition API
      * Community Redis ClusterDefinition API
      * Cluster availability demo application named NyanCat
  * ClusterDefinition API `spec.connectionCredential` add following built-in variables:
    * A random UUID v4 generator `$(UUID)`
    * A random UUID v4 generator with BASE64 encoded `$(UUID_B64)`
    * A random UUID v4 generator in UUID string then BASE64 encoded `$(UUID_STR_B64)`
    * A random UUID v4 generator in HEX representation `$(UUID_HEX)`
    * Service FQDN `$(SVC_FQDN)` placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute
    * Service ports `$(SVC_PORT_<NAME>)` placeholder
    * example usage:
    
  ```yaml
  # ClusterDefinition API
  kind: ClusterDefinition
    metadata:
    name: my-clusterdefinition
  spec:
    connectionCredential:
      username: "admin" 
      "admin-password": "$(RANDOM_PASSWD)"
      endpoint: "http://$(SVC_FQDN):$(SVC_PORT_http)"

    componentsDefs:
      - name: my-comp-type
        service:
          ports:
            - name: http
              port: 8123

  # Cluster API
  kind: Cluster
  metadata:
    name: my-cluster
    namespace: my-ns
  spec:
    clusterDefinitionRef: my-clusterdefinition
    componentSpecs:
      - name: my-comp
        type: my-comp-type

  # output:
  kind: Secret
  metadata:
    name: my-cluster-conn-credential
    namespace: my-ns
    labels:
      "app.kubernetes.io/instance": my-cluster
  stringData:
    username: "admin"
    admin-password: "<some random 8 characters password>"
    endpoint: "http://my-cluster-my-comp.my-ns.svc:8123"
  ```

**Known issues and limitations**
  * Limitations of cluster's horizontal scale operation:
    * Only support VolumeSnapshot API to make a clone of Cluster's PV for syncing data when horizontal scaling.
    * Only 1st pod container and 1st volume mount associated PV will be processed for VolumeSnapshot, do assure that data volume is placed in 1st pod container's 1st volume mount.
    * Unused PVCs will be deleted in 30 minutes after scale in.

If you're new to KubeBlocks, visit the [getting started](https://github.com/apecloud/kubeblocks/blob/v$kubeblocks_version/docs/user_docs/quick_start_guide.md) page and get a quick start with KubeBlocks.

$warnings

See [this](#upgrading-to-kubeblocks-$kubeblocks_version) section to upgrade KubeBlocks to version $kubeblocks_version.

## Acknowledgements

Thanks to everyone who made this release possible!

$kubeblocks_contributors

## What's Changed
$kubeblocks_changes

## Upgrading to KubeBlocks $kubeblocks_version

To upgrade to this release of KubeBlocks, follow the steps here to ensure a smooth upgrade.

Release Notes for `v0.3.0`:
- Rename CRD name `backupjobs.dataprotection.kubeblocks.io` to `backups.dataprotection.kubeblocks.io`
  - upgrade KubeBlocks with the following command:
    ```
    helm upgrade --install kubeblocks kubeblocks/kubeblocks --version 0.3.0
    ```
  - after you upgrade KubeBlocks, check CRD `backupjobs.dataprotection.kubeblocks.io` and delete it
    ```
    kubectl delete crd backupjobs.dataprotection.kubeblocks.io
    ```
- Rename CRD name `appversions.dbaas.kubeblocks.io` to `clusterversions.dbaas.kubeblocks.io`
  - before you upgrade KubeBlocks, please backup your Cluster CR yaml first.
    ```
    kubectl get cluster -oyaml > clusters.yaml
    ```
    then replace all spec.appVersionRef to spec.clusterVersionRef in the clusters.yaml.

    Then, handle OpsRequest CR the same way.
  - after you upgrade KubeBlocks, you can delete the CRD `appversions.dbaas.kubeblocks.io`
    ```
    kubectl delete crd appversions.dbaas.kubeblocks.io
    ```
    the last step, use the above backup of Clusters and OpsRequests to apply them.
    ```
    kubectl apply -f clusters.yaml
      ```
- Rename group name `dbaas.kubeblocks.io` to `apps.kubeblocks.io`
    - upgrade kubeblocks to create new CRDs, after that, you can delete the CRDs with group name`dbaas.kubeblocks.io`

## Breaking Changes

$kubeblocks_breaking_changes
* Refactored the use of labels. Existing clusters or config need to manually update their labels to ensure proper functionality. The following are specific changes:
  - Pods of `statefulset` and `deployment`
    - Replace label name from `app.kubernetes.io/component-name` to `apps.kubeblocks.io/component-name`
    - Replace label name from `app.kubeblocks.io/workload-type` to `apps.kubeblocks.io/workload-type`
    - Add label `app.kubernetes.io/version` with value `Cluster.Spec.ClusterVersionRef`
    - Add label `app.kubernetes.io/component` with value `Cluster.Spec.ComponentSpecs.ComponentDefRef`
  - CR `backuppolicytemplate`
    - Replace label name from `app.kubernetes.io/created-by` to `app.kubernetes.io/managed-by`
  - Configmap hosted by KubeBlocks and named with `*-env` suffix
    - Replace label name from `app.kubernetes.io/config-type` to `apps.kubeblocks.io/config-type`
* With KubeBlocks Helm chart replaced its optional components install using sub-charts dependencies with Addons extensions API, previous version upgrade to this version will uninstall the optional components completely.
