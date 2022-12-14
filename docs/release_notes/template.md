# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our thanks to all the new and existing contributors who helped make this release happen.

**Highlights**

* Automatic pod container environment variables:
  * KB_POD_NAME - Pod Name
  * KB_NAMESPACE - Namespace
  * KB_SA_NAME - Service Account Name
  * KB_NODENAME - Node Name
  * KB_HOSTIP - Host IP address
  * KB_PODIP -  Pod IP address
  * KB_PODIPS - Pod IP addresses
  * KB_CLUSTER_NAME - KubeBlock Cluster API object name
  * KB_COMP_NAME - Running pod's KubeBlock Cluster API object's `.spec.components.name`
  * KB_CLUSTER_COMP_NAME - Running pod's KubeBlock Cluster API object's `<.metadata.name>-<.spec.components..name>`, same name is used for Deployment or StatefulSet workload name, and Service object name
* ClusterDefinition API support following placeholder name:
  * under `.spec.connectionCredential`:
    * random 8 characters `$(RANDOM_PASSWD)` placeholder, 
    * self reference map object `$(CONN_CREDENTIAL)[.<map key>])`
    * example usage:
  
```yaml
spec:
  connectionCredential:
    username: "admin-password" 
    password: "$(RANDOM_PASSWD)"
    "$(CONN_CREDENTIAL).username": "$(CONN_CREDENTIAL).password"

# output:
spec:
  connectionCredential:
    username: "admin-password" 
    password: "<some random 8 characters password>"
    "admin-password": "<value of above password>"
```

  * Connection credential secret name place holder `$(CONN_CREDENTIAL_SECRET_NAME)`

  * Limitations of cluster's horizontal scale operation:
    * Only support VolumeSnapshot API to make a clone of Cluster's PV needs sync data when horizontal scaling.
    * Only 1st pod container and 1st volume mount associated PV will be processed for VolumeSnapshot, do assure that data volume is placed in 1st pod container's 1st volume mount.


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
