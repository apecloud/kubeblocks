# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our thanks to all the new and existing contributors who helped make this release happen.

**Highlights**
  * Limitations of cluster's horizontal scale operation:
    * Only support VolumeSnapshot API to make a clone of Cluster's PV needs sync data when horizontal scaling.
    * Only 1st pod container and 1st volume mount associated PV will be processed for VolumeSnapshot, do assure that data volume is placed in 1st pod container's 1st volume mount.
    * Unused PVCs will be deleted 30 minutes after scale in.
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

If you're new to KubeBlocks, visit the [getting started](https://kubeblocks.io) page and
familiarize yourself with KubeBlocks.

$warnings

See [this](#upgrading-to-kubeblocks-$kubeblocks_version) section on upgrading KubeBlocks to version $kubeblocks_version.

## Acknowledgements

Thanks to everyone who made this release possible!

$kubeblocks_contributors

## What's Changed
$kubeblocks_changes

## Upgrading to KubeBlocks $kubeblocks_version

To upgrade to this release of KubeBlocks, follow the steps here to ensure a smooth upgrade.

TODO: add upgrade steps

## Breaking Changes

$kubeblocks_breaking_changes
