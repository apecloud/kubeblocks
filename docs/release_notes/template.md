# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our appreciation to all contributors who helped make this release happen.

**Breaking changes**


**Highlights**


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

