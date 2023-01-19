# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our appreciation to all contributors who helped make this release happen.

**Highlights**
  * ClusterDefinition API `spec.conectionCredential` add following placeholder name:
    * Service FQDN `$(SVC_FQDN)` placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.components[].service` attribute
    * Service ports `$(SVC_PORT_<NAME>)` placeholder
    * example usage:
    
  ```yaml
  # ClusterDefinition API
  kind: ClusterDefinition
  spec:
    connectionCredential:
      username: "admin" 
      "admin-password": "$(RANDOM_PASSWD)"
      endpoint: "http://$(SVC_FQDN):$(SVC_PORT_http)"

    components:
      - typeName: my-comp-type
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
    components:
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

**Known Issues**
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
## Breaking Changes

$kubeblocks_breaking_changes