# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our thanks to all the new and existing contributors who helped make this release happen.

**Highlights**

  * Limitations of cluster's horizontal scale operation:
    * Only support VolumeSnapshot API to make a clone of Cluster's PV needs sync data when horizontal scaling.
    * Only 1st pod container and 1st volume mount associated PV will be processed for VolumeSnapshot, do assure that data volume is placed in 1st pod container's 1st volume mount.
    * Unused PVCs will be deleted 30 minutes after scale in.


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

Release Notes for `v0.3.0`:
- Rename CRD name `backupjobs.dataprotection.kubeblocks.io` 
to `backups.dataprotection.kubeblocks.io`
  - upgrade kubeblocks following commands:
      ```
      helm upgrade --install kubeblocks kubeblocks/kubeblocks --version 0.3.0
      ```
  - after you upgrade kubeblocks, check CRD `backupjobs.dataprotection.kubeblocks.io` and delete it
    ```
    kubectl delete crd backupjobs.dataprotection.kubeblocks.io
    ```
  

## Breaking Changes

$kubeblocks_breaking_changes
