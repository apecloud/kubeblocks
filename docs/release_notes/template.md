# KubeBlocks $kubeblocks_version ($today)

We're happy to announce the release of KubeBlocks $kubeblocks_version! 🚀 🎉 🎈

This release introduces Redis, a key-value database, and MongoDB, a document-based database. It also supports the primary-secondary topology of PostgreSQL, adapts to more public cloud vendors' hosted Kubernetes versions, improves data backup and recovery experiences, and builds basic data migration capability. We noticed that some users may think that K8s reduces database performance. So in this release we include a comparison test result to explain the throughput and RT differences of various MySQL 8.0 deployment forms on AWS.

We would like to extend our appreciation to all contributors who helped make this release happen.

## **Highlights**

- KubeBlocks supports the primary-secondary topology of PostgreSQL
  Users can actively switch the primary-secondary role of the database cluster with kbcli, or passively trigger failover by deleting a specified Kubernetes pod with kubectl. Failover generally completes within 30 seconds when there are no long transactions and large table DDLs.
- KubeBlocks supports Redis v7.0
  Redis is currently the most popular open-source key-value database, supporting data types such as key-value, string, list, set, hash table, and ordered set. It provides extremely fast data read and write operations and is suitable for cache scenarios in e-commerce, social communication, game, and other internet applications. To provide stable, secure, and efficient Redis services to users, KubeBlocks has adopted Redis 7.0 version, which is currently recommended officially, supporting standalone and primary-secondary topologies. Thus, users can perform operations such as creating, deleting, scaling, backing up, restoring, monitoring, alerting, and changing parameters of Redis clusters in development, testing, and production environments.
- KubeBlocks supports MongoDB v5.0
  MongoDB is currently the most popular document-based database, using JSON data types and dynamic schema designs to maintain high flexibility and scalability. KubeBlocks supports the replica set topology of MongoDB v5.0, providing data redundancy and automatic failover capabilities, ensuring data availability and consistency in the event of a node failure. The replica set topology cluster has one primary node (Primary) and several secondary nodes (Secondary), with the primary node handling all write requests and the secondary nodes handling some read requests. If the primary node fails, one of the secondary nodes is elected as the new primary node.
- KubeBlocks supports the private deployment of ChatGPT retrieval plugin
  For users who do not want to expose sensitive information (such as company documents, meeting minutes, emails), OpenAI has open-sourced the ChatGPT retrieval plugin to enhance the ChatGPT experience. As long as users meet OpenAI's requirements, they can run the ChatGPT retrieval plugin through KubeBlocks addon, store the vectorized data of sensitive information in a private database, and enable ChatGPT to have longer memory of the context while ensuring information security.
- KubeBlocks supports one-command launching of playgrounds on Alibaba Cloud, Tencent Cloud, and GCP
  Public cloud vendors' hosted Kubernetes services have significant differences in version, functionality, and integration, so even if the deployment of stateful services is not difficult, but Kubernetes administrators  have to do a lot of extra heavy lifting to run stateful services normally. After supporting AWS, KubeBlocks provides the ability to one-command launch playgrounds on Alibaba Cloud, Tencent Cloud, and GCP. Users only need to set up public cloud AK locally, and then execute the kbcli playground init command, and KubeBlocks will automatically apply for resources and configure permissions in the specified region, making it easy for users to experience complete functionality. After trying KubeBlocks out, you can clean up the playground environment with one command to avoid incurring costs.

## **Breaking changes**

- Breaking changes between v0.5 and v0.4. Uninstall v0.4 (including any older version) before installing v0.5.
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


## **Known issues and limitations**
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
### New Features
#### MySQL
- Support ZEngine storage engine
- Account management supports creating, modifying, and deleting database accounts with different permissions
  PostgreSQL
- Support migration from AWS RDS to KubeBlocks, supporting pre-checks, full migration, and incremental synchronization, verifying the data migration capabilities of CadenceWorkflow and OpenStreetMap
- Support for pgvector extension
- Support for the primary-secondary topology of PostgreSQL
- Automated failover and self-healing
- Support point-in-time recovery
- Account management supports creating, modifying, and deleting database accounts with different permissions

#### Redis
- Full lifecycle management, including creation, deletion, restart, horizontal/vertical scaling
- Support Redis primary-secondary topology
- Automated failover and self-healing
- Support snapshot backup and recovery
- Metric monitoring, including cluster's basic operation status, connection, OS resources, performance, primary-secondary replication status and other metrics
- Alerts including cluster downtime, OS resource, abnormal connection number, primary-secondary replication abnormality, etc.
- Parameter reconfigure
- Account management

#### MongoDB
- Full lifecycle management, including creation, deletion, restart, vertical scaling, and disk expansion
- Endpoint exposes the access addresses of all nodes
- File-based full backup and recovery
- Automated failover and self-healing
- Monitoring, alerting and logs
- Parameter reconfigure
 
$kubeblocks_changes

### Easy of Use
- `kbcli playground` supports one-command launching on running environments of Alibaba Cloud, Tencent Cloud, and GCP to experience complete KubeBlocks functionality
- kbcli supports creating clusters by entering CPU, memory, or class type
- kbcli supports tagging related resources of cluster
- kbcli is compatible with macOS package manager `brew`
- kbcli supports `preflight` command to check whether the environment meets the requirements for installing KubeBlocks
- kbcli adds object storage addon for storing full file backups, logs, and other data
- `kbcli install` runs preflight to check whether the environment meets the requirements, including node taints, storage class, and other check rules
- kbcli addon adds timeout parameter, printing exception information when enable fails
- Addon inherits the affinity and tolerations configuration of KubeBlocks
- `kbcli uninstall` prompts information to delete backup files, printing log information if the deletion fails
- ClusterDefinition API `spec.connectionCredential` add following built-in variables:
    - Headless service FQDN `$(HEADLESS_SVC_FQDN)` placeholder, value pattern - `$(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc`, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute

#### Compatibility
- Compatible with AWS EKS v1.22/v1.23/v1.24/v1.25
- Compatible with Alibaba Cloud ACK v1.22/v1.24
- Compatible with Tencent Cloud TKE standard cluster v1.22/v1.24
- Compatible with GCP GKE standard cluster v1.24/v1.25

#### Stability
- KubeBlocks limits the combination of CPU and memory to avoid unreasonable configurations that reduce resource utilization or system stability

#### Performance
- High-availability MySQL 8.0 with 4C 8GB 500GB, throughput and RT differences of various products on AWS, including ApeCloud MySQL Raft group, AWS RDS operator, Operator for Percona Server for MySQL, Oracle MySQL Operator for Kubernetes

### API changes
- New APIs:
    - backuppolicytemplates.apps.kubeblocks.io

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
- N/A if upgrading from 0.4 or older version.
