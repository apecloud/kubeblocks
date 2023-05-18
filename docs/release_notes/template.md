# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

We would like to extend our appreciation to all contributors who helped make this release happen.

**Breaking changes**

- Breaking changes between v0.5 and v0.4. Uninstall v0.4 before installing v0.5.
    - Move the backupPolicyTemplate API from dataprotection group to apps group.
      Before installing v0.5, please ensure that the resources have been cleaned up:
       ```
         kubectl delete backuppolicytemplates.dataprotection.kubeblocks.io --all
         kubectl delete backuppolicies.dataprotection.kubeblocks.io --all
       ```
    - redefines the phase of cluster and component.
      Before installing v0.5, please ensure that the resources have been cleaned up:
       ```
         kubectl delete clusters.apps.kubeblocks.io --all
         kubectl delete opsrequets.apps.kubeblocks.io --all
       ```
- `addons.extensions.kubeblocks.io` API deleted `spec.helm.valuesMapping.jsonMap.additionalProperties`, `spec.helm.valuesMapping.valueMap.additionalProperties`, `spec.helm.valuesMapping.extras.jsonMap.additionalProperties` and `spec.helm.valuesMapping.extras.valueMap.additionalProperties` attributes that was introduced by CRD generator, all existing Addons API YAML shouldn't have referenced these attributes.


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

### Easy of Use
* ClusterDefinition API `spec.connectionCredential` add following built-in variables:
    * Headless service FQDN `$(HEADLESS_SVC_FQDN)` placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute

#### Compatibility
- Pass the AWS EKS v1.22 / v1.23 / v1.24 / v1.25 compatibility test.

### API changes
- New APIs:
    - backuppolicytemplates.apps.kubeblocks.io
    - componentclassdefinitions.apps.kubeblocks.io
    - componentresourceconstraints.apps.kubeblocks.io

- Deleted APIs:
    - backuppolicytemplates.dataprotection.kubeblocks.io

- New API attributes:
    - clusterdefinitions.apps.kubeblocks.io API
        - spec.type
        - spec.componentDefs.customLabelSpecs
    - clusterversions.apps.kubeblocks.io API
        - spec.componentVersions.clientImage (EXPERIMENTAL)
    - clusters.apps.kubeblocks.io API
        - spec.componentSpecs.classDefRef
        - spec.componentSpecs.serviceAccountName
    - configconstraints.apps.kubeblocks.io API
        - spec.reloadOptions.shellTrigger.namespace
        - spec.reloadOptions.shellTrigger.scriptConfigMapRef
        - spec.reloadOptions.tplScriptTrigger.sync
        - spec.selector
    - opsrequests.apps.kubeblocks.io API
        - spec.restoreFrom
        - spec.verticalScaling.class
        - status.reconfiguringStatus.configurationStatus.updatePolicy
    - backuppolicies.dataprotection.kubeblocks.io API
        - spec.full
        - spec.logfile
        - spec.retention
    - backups.dataprotection.kubeblocks.io
        - status.manifests
    - backuptools.dataprotection.kubeblocks.io
        - spec.type

- Renamed API attributes:
    - clusterdefinitions.apps.kubeblocks.io API
        - spec.componentDefs.horizontalScalePolicy.backupTemplateSelector -> spec.componentDefs.horizontalScalePolicy.backupPolicyTemplateName
        - spec.componentDefs.probe.roleChangedProbe -> spec.componentDefs.probe.roleProbe
    - backuppolicies.dataprotection.kubeblocks.io API
        - spec.full
    - restorejobs.dataprotection.kubeblocks.io API
        - spec.target.secret.passwordKeyword -> spec.target.secret.passwordKey
        - spec.target.secret.userKeyword -> spec.target.secret.usernameKey
    - addons.extensions.kubeblocks.io API
        - spec.helm.installValues.secretsRefs -> spec.helm.installValues.secretRefs

- Deleted API attributes:
    - opsrequests.apps.kubeblocks.io API
        - status.observedGeneration
    - backuppolicies.dataprotection.kubeblocks.io API
        - spec.backupPolicyTemplateName
        - spec.backupToolName
        - spec.backupType
        - spec.backupsHistoryLimit
        - spec.hooks
        - spec.incremental
    - backups.dataprotection.kubeblocks.io API
        - spec.ttl
        - status.CheckPoint
        - status.checkSum
    - addons.extensions.kubeblocks.io API
        - spec.helm.valuesMapping.jsonMap.additionalProperties
        - spec.helm.valuesMapping.valueMap.additionalProperties
        - spec.helm.valuesMapping.extras.jsonMap.additionalProperties
        - spec.helm.valuesMapping.extras.valueMap.additionalProperties

- Updates API Status info:
    - clusters.apps.kubeblocks.io API
        - status.components.phase valid values are Running, Stopped, Failed, Abnormal, Creating, Updating; REMOVED phases are SpecUpdating, Deleting, Deleted, VolumeExpanding, Reconfiguring, HorizontalScaling, VerticalScaling, VersionUpgrading, Rebooting, Stopping, Starting.
        - status.phase valid values are Running, Stopped, Failed, Abnormal, Creating, Updating; REMOVED phases are ConditionsError, SpecUpdating, Deleting, Deleted, VolumeExpanding, Reconfiguring, HorizontalScaling, VerticalScaling, VersionUpgrading, Rebooting, Stopping, Starting.
    - opsrequests.apps.kubeblocks.io API
        - status.components.phase valid values are Running, Stopped, Failed, Abnormal, Creating, Updating; REMOVED phases are SpecUpdating, Deleting, Deleted, VolumeExpanding, Reconfiguring, HorizontalScaling, VerticalScaling, VersionUpgrading, Rebooting, Stopping, Starting, Exposing.
        - status.phase added 'Creating' phase.

## Upgrading to KubeBlocks $kubeblocks_version

