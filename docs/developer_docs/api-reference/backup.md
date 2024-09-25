---
title: Backup API Reference
description: Backup API Reference
keywords: [backup, api]
sidebar_position: 2
sidebar_label: Backup
---
<br />
<p>Packages:</p>
<ul>
<li>
<a href="#dataprotection.kubeblocks.io%2fv1alpha1">dataprotection.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="dataprotection.kubeblocks.io/v1alpha1">dataprotection.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Backup">Backup</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSchedule">BackupSchedule</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Restore">Restore</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>
</li></ul>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet
</h3>
<div>
<p>ActionSet is the Schema for the actionsets API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ActionSet</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">
ActionSetSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backupType</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupType">
BackupType
</a>
</em>
</td>
<td>
<p>Specifies the backup type. Supported values include:</p>
<ul>
<li><code>Full</code> for a full backup.</li>
<li><code>Incremental</code> back up data that have changed since the last backup (either full or incremental).</li>
<li><code>Differential</code> back up data that has changed since the last full backup.</li>
<li><code>Continuous</code> back up transaction logs continuously, such as MySQL binlog, PostgreSQL WAL, etc.</li>
</ul>
<p>Continuous backup is essential for implementing Point-in-Time Recovery (PITR).</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of environment variables to be set in the container.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of sources to populate environment variables in the container.
The keys within a source must be a C_IDENTIFIER. Any invalid keys will be
reported as an event when the container starts. If a key exists in multiple
sources, the value from the last source will take precedence. Any values
defined by an Env with a duplicate key will take precedence.</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">
BackupActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the backup action.</p>
</td>
</tr>
<tr>
<td>
<code>restore</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreActionSpec">
RestoreActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the restore action.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetStatus">
ActionSetStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Backup">Backup
</h3>
<div>
<p>Backup is the Schema for the backups API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Backup</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSpec">
BackupSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backupPolicyName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup policy to be applied for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethod</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup method name that is defined in the backup policy.</p>
</td>
</tr>
<tr>
<td>
<code>deletionPolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupDeletionPolicy">
BackupDeletionPolicy
</a>
</em>
</td>
<td>
<p>Determines whether the backup contents stored in the backup repository
should be deleted when the backup custom resource(CR) is deleted.
Supported values are <code>Retain</code> and <code>Delete</code>.</p>
<ul>
<li><code>Retain</code> means that the backup content and its physical snapshot on backup repository are kept.</li>
<li><code>Delete</code> means that the backup content and its physical snapshot on backup repository are deleted.</li>
</ul>
<p>the backup CR but retaining the backup contents in backup repository.
The current implementation only prevent accidental deletion of backup data.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionPeriod">
RetentionPeriod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines a duration up to which the backup should be kept.
Controller will remove all backups that are older than the RetentionPeriod.
If not set, the backup will be kept forever.
For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 	2y</li>
<li>months: 	6mo</li>
<li>days: 		30d</li>
<li>hours: 	12h</li>
<li>minutes: 	30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m.</p>
</td>
</tr>
<tr>
<td>
<code>parentBackupName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines the parent backup name for incremental or differential backup.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">
BackupStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy
</h3>
<div>
<p>BackupPolicy is the Schema for the backuppolicies API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>BackupPolicy</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">
BackupPolicySpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backupRepoName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of BackupRepo where the backup data will be stored.
If not set, data will be stored in the default backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>pathPrefix</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the directory inside the backup repository to store the backup.
This path is relative to the path of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of retries before marking the backup as failed.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
BackupTarget
</a>
</em>
</td>
<td>
<p>Specifies the target information to back up, such as the target pod, the
cluster connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>targets</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
[]BackupTarget
</a>
</em>
</td>
<td>
<p>Specifies multiple target information for backup operations. This includes details
such as the target pod and cluster connection credentials. All specified targets
will be backed up collectively.
optional</p>
</td>
</tr>
<tr>
<td>
<code>backupMethods</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">
[]BackupMethod
</a>
</em>
</td>
<td>
<p>Defines the backup methods.</p>
</td>
</tr>
<tr>
<td>
<code>useKopia</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether backup data should be stored in a Kopia repository.</p>
<p>Data within the Kopia repository is both compressed and encrypted. Furthermore,
data deduplication is implemented across various backups of the same cluster.
This approach significantly reduces the actual storage usage, particularly
for clusters with a low update frequency.</p>
<p>NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition, otherwise the backup will not be processed.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.EncryptionConfig">
EncryptionConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters for encrypting backup data.
Encryption will be disabled if the field is not set.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">
BackupPolicyStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo
</h3>
<div>
<p>BackupRepo is a repository for storing backup data.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>BackupRepo</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoSpec">
BackupRepoSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>storageProviderRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the <code>StorageProvider</code> used by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>accessMethod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.AccessMethod">
AccessMethod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the access method of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>volumeCapacity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the capacity of the PVC created by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>pvReclaimPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumereclaimpolicy-v1-core">
Kubernetes core/v1.PersistentVolumeReclaimPolicy
</a>
</em>
</td>
<td>
<p>Specifies reclaim policy of the PV created by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>config</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores the non-secret configuration parameters for the <code>StorageProvider</code>.</p>
</td>
</tr>
<tr>
<td>
<code>credential</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>References to the secret that holds the credentials for the <code>StorageProvider</code>.</p>
</td>
</tr>
<tr>
<td>
<code>pathPrefix</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the prefix of the path for storing backup data.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoStatus">
BackupRepoStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupSchedule">BackupSchedule
</h3>
<div>
<p>BackupSchedule is the Schema for the backupschedules API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>BackupSchedule</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleSpec">
BackupScheduleSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backupPolicyName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backupPolicy to be applied for the <code>schedules</code>.</p>
</td>
</tr>
<tr>
<td>
<code>startingDeadlineMinutes</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the deadline in minutes for starting the backup workload if it
misses its scheduled time for any reason.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
[]SchedulePolicy
</a>
</em>
</td>
<td>
<p>Defines the list of backup schedules.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleStatus">
BackupScheduleStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Restore">Restore
</h3>
<div>
<p>Restore is the Schema for the restores API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Restore</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">
RestoreSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRef">
BackupRef
</a>
</em>
</td>
<td>
<p>Specifies the backup to be restored. The restore behavior is based on the backup type:</p>
<ol>
<li>Full: will be restored the full backup directly.</li>
<li>Incremental: will be restored sequentially from the most recent full backup of this incremental backup.</li>
<li>Differential: will be restored sequentially from the parent backup of the differential backup.</li>
<li>Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.</li>
</ol>
</td>
</tr>
<tr>
<td>
<code>restoreTime</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the point in time for restoring.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreKubeResources">
RestoreKubeResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Restores the specified resources of Kubernetes.</p>
</td>
</tr>
<tr>
<td>
<code>prepareDataConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">
PrepareDataConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims
that need to be restored and scheduling strategy of temporary recovery pod.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the service account name needed for recovery pod.</p>
</td>
</tr>
<tr>
<td>
<code>readyConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">
ReadyConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the action of &ldquo;postReady&rdquo; phase.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the container for restore. These will be
merged with the env of Backup and ActionSet.</p>
<p>The priority of merging is as follows: <code>Restore env &gt; Backup env &gt; ActionSet env</code>.</p>
</td>
</tr>
<tr>
<td>
<code>containerResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the required resources of restore job&rsquo;s container.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of retries before marking the restore failed.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatus">
RestoreStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider
</h3>
<div>
<p>StorageProvider comprises specifications that provide guidance on accessing remote storage.
Currently the supported access methods are via a dedicated CSI driver or the <code>datasafed</code> tool.
In case of CSI driver, the specification expounds on provisioning PVCs for that driver.
As for the <code>datasafed</code> tool, the specification provides insights on generating the necessary
configuration file.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br/>
string</td>
<td>
<code>dataprotection.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>StorageProvider</code></td>
</tr>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProviderSpec">
StorageProviderSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>csiDriverName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the CSI driver used to access remote storage.
This field can be empty, it indicates that the storage is not accessible via CSI.</p>
</td>
</tr>
<tr>
<td>
<code>csiDriverSecretTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template that used to render and generate <code>k8s.io/api/core/v1.Secret</code>
resources for a specific CSI driver.
For example, <code>accessKey</code> and <code>secretKey</code> needed by CSI-S3 are stored in this
<code>Secret</code> resource.</p>
</td>
</tr>
<tr>
<td>
<code>storageClassTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template utilized to render and generate <code>kubernetes.storage.k8s.io.v1.StorageClass</code>
resources. The `StorageClass&rsquo; created by this template is aimed at using the CSI driver.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaimTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template that renders and generates <code>k8s.io/api/core/v1.PersistentVolumeClaim</code>
resources. This PVC can reference the <code>StorageClass</code> created from <code>storageClassTemplate</code>,
allowing Pods to access remote storage by mounting the PVC.</p>
</td>
</tr>
<tr>
<td>
<code>datasafedConfigTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template used to render and generate <code>k8s.io/api/core/v1.Secret</code>.
This <code>Secret</code> involves the configuration details required by the <code>datasafed</code> tool
to access remote storage. For example, the <code>Secret</code> should contain <code>endpoint</code>,
<code>bucket</code>, &lsquo;region&rsquo;, &lsquo;accessKey&rsquo;, &lsquo;secretKey&rsquo;, or something else for S3 storage.
This field can be empty, it means this kind of storage is not accessible via
the <code>datasafed</code> tool.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the parameters required for storage.
The parameters defined here can be referenced in the above templates,
and <code>kbcli</code> uses this definition for dynamic command-line parameter parsing.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProviderStatus">
StorageProviderStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.AccessMethod">AccessMethod
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoSpec">BackupRepoSpec</a>)
</p>
<div>
<p>AccessMethod represents an enumeration type that outlines
how the <code>BackupRepo</code> can be accessed.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Mount&#34;</p></td>
<td><p>AccessMethodMount suggests that the storage is mounted locally
which allows for remote files to be accessed akin to local ones.</p>
</td>
</tr><tr><td><p>&#34;Tool&#34;</p></td>
<td><p>AccessMethodTool indicates the utilization of a command-line
tool for accessing the storage.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionErrorMode">ActionErrorMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ExecActionSpec">ExecActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">JobActionSpec</a>)
</p>
<div>
<p>ActionErrorMode defines how to handle an error from an action.
Currently, only the Fail mode is supported, but the Continue mode will be supported in the future.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Continue&#34;</p></td>
<td><p>ActionErrorModeContinue signifies that an error from an action is acceptable and can be ignored.</p>
</td>
</tr><tr><td><p>&#34;Fail&#34;</p></td>
<td><p>ActionErrorModeFail signifies that an error from an action is problematic and should be treated as a failure.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionPhase">ActionPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionStatus">ActionStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Completed&#34;</p></td>
<td><p>ActionPhaseCompleted means the action has run successfully without errors.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>ActionPhaseFailed means the action ran but encountered an error that</p>
</td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td><p>ActionPhaseNew means the action has been created but not yet processed by
the BackupController.</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>ActionPhaseRunning means the action is currently executing.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">ActionSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet</a>)
</p>
<div>
<p>ActionSetSpec defines the desired state of ActionSet</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backupType</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupType">
BackupType
</a>
</em>
</td>
<td>
<p>Specifies the backup type. Supported values include:</p>
<ul>
<li><code>Full</code> for a full backup.</li>
<li><code>Incremental</code> back up data that have changed since the last backup (either full or incremental).</li>
<li><code>Differential</code> back up data that has changed since the last full backup.</li>
<li><code>Continuous</code> back up transaction logs continuously, such as MySQL binlog, PostgreSQL WAL, etc.</li>
</ul>
<p>Continuous backup is essential for implementing Point-in-Time Recovery (PITR).</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of environment variables to be set in the container.</p>
</td>
</tr>
<tr>
<td>
<code>envFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envfromsource-v1-core">
[]Kubernetes core/v1.EnvFromSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of sources to populate environment variables in the container.
The keys within a source must be a C_IDENTIFIER. Any invalid keys will be
reported as an event when the container starts. If a key exists in multiple
sources, the value from the last source will take precedence. Any values
defined by an Env with a duplicate key will take precedence.</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">
BackupActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the backup action.</p>
</td>
</tr>
<tr>
<td>
<code>restore</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreActionSpec">
RestoreActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the restore action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSetStatus">ActionSetStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet</a>)
</p>
<div>
<p>ActionSetStatus defines the observed state of ActionSet</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Phase">
Phase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the phase of the ActionSet. This can be either &lsquo;Available&rsquo; or &lsquo;Unavailable&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provides a human-readable explanation detailing the reason for the current phase of the ActionSet.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the generation number that has been observed by the controller.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSpec">ActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">BackupActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreActionSpec">RestoreActionSpec</a>)
</p>
<div>
<p>ActionSpec defines an action that should be executed. Only one of the fields may be set.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ExecActionSpec">
ExecActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies that the action should be executed using the pod&rsquo;s exec API within a container.</p>
</td>
</tr>
<tr>
<td>
<code>job</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">
JobActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies that the action should be executed by a Kubernetes Job.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionStatus">ActionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the action.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Records the target pod name which has been backed up.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionPhase">
ActionPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The current phase of the action.</p>
</td>
</tr>
<tr>
<td>
<code>startTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time an action was started.</p>
</td>
</tr>
<tr>
<td>
<code>completionTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time an action was completed.</p>
</td>
</tr>
<tr>
<td>
<code>failureReason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>An error that caused the action to fail.</p>
</td>
</tr>
<tr>
<td>
<code>actionType</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionType">
ActionType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The type of the action.</p>
</td>
</tr>
<tr>
<td>
<code>availableReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Available replicas for statefulSet action.</p>
</td>
</tr>
<tr>
<td>
<code>objectRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The object reference for the action.</p>
</td>
</tr>
<tr>
<td>
<code>totalSize</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The total size of backed up data size.
A string with capacity units in the format of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.
If no capacity unit is specified, it is assumed to be in bytes.</p>
</td>
</tr>
<tr>
<td>
<code>timeRange</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTimeRange">
BackupTimeRange
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time range of backed up data, for PITR, this is the time
range of recoverable data.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSnapshots</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeSnapshotStatus">
[]VolumeSnapshotStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the volume snapshot status for the action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionType">ActionType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionStatus">ActionStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Job&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;StatefulSet&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">BackupActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">ActionSetSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backupData</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupDataActionSpec">
BackupDataActionSpec
</a>
</em>
</td>
<td>
<p>Represents the action to be performed for backing up data.</p>
</td>
</tr>
<tr>
<td>
<code>preBackup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSpec">
[]ActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents a set of actions that should be executed before the backup process begins.</p>
</td>
</tr>
<tr>
<td>
<code>postBackup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSpec">
[]ActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents a set of actions that should be executed after the backup process has completed.</p>
</td>
</tr>
<tr>
<td>
<code>preDelete</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BaseJobActionSpec">
BaseJobActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents a custom deletion action that can be executed before the built-in deletion action.
Note: The preDelete action job will ignore the env/envFrom.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupDataActionSpec">BackupDataActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">BackupActionSpec</a>)
</p>
<div>
<p>BackupDataActionSpec defines how to back up data.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>JobActionSpec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">
JobActionSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>JobActionSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>syncProgress</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SyncProgress">
SyncProgress
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines if the backup progress should be synchronized and the interval
for synchronization in seconds.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupDeletionPolicy">BackupDeletionPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec</a>)
</p>
<div>
<p>BackupDeletionPolicy describes the policy for end-of-life maintenance of backup content.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Delete&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Retain&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
<p>BackupMethod defines the backup method.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of backup method.</p>
</td>
</tr>
<tr>
<td>
<code>snapshotVolumes</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether to take snapshots of persistent volumes. If true,
the ActionSetName is not required, the controller will use the CSI volume
snapshotter to create the snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>actionSetName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to the ActionSet object that defines the backup actions.
For volume snapshot backup, the actionSet is not required, the controller
will use the CSI volume snapshotter to create the snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>targetVolumes</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetVolumeInfo">
TargetVolumeInfo
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies which volumes from the target should be mounted in the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the environment variables for the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeSettings</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RuntimeSettings">
RuntimeSettings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies runtime settings for the backup workload container.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
BackupTarget
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the target information to back up, it will override the target in backup policy.</p>
</td>
</tr>
<tr>
<td>
<code>targets</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
[]BackupTarget
</a>
</em>
</td>
<td>
<p>Specifies multiple target information for backup operations. This includes details
such as the target pod and cluster connection credentials. All specified targets
will be backed up collectively.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">BackupMethodTPL
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of backup method.</p>
</td>
</tr>
<tr>
<td>
<code>snapshotVolumes</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether to take snapshots of persistent volumes. If true,
the ActionSetName is not required, the controller will use the CSI volume
snapshotter to create the snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>actionSetName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to the ActionSet object that defines the backup actions.
For volume snapshot backup, the actionSet is not required, the controller
will use the CSI volume snapshotter to create the snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>targetVolumes</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetVolumeInfo">
TargetVolumeInfo
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies which volumes from the target should be mounted in the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.EnvVar">
[]EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the environment variables for the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeSettings</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RuntimeSettings">
RuntimeSettings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies runtime settings for the backup workload container.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetInstance">
TargetInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>If set, specifies the method for selecting the replica to be backed up using the criteria defined here.
If this field is not set, the selection method specified in <code>backupPolicy.target</code> is used.</p>
<p>This field provides a way to override the global <code>backupPolicy.target</code> setting for specific BackupMethod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPhase">BackupPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
<p>BackupPhase describes the lifecycle phase of a Backup.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Completed&#34;</p></td>
<td><p>BackupPhaseCompleted means the backup has run successfully without errors.</p>
</td>
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td><p>BackupPhaseDeleting means the backup and all its associated data are being deleted.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupPhaseFailed means the backup ran but encountered an error that
prevented it from completing successfully.</p>
</td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td><p>BackupPhaseNew means the backup has been created but not yet processed by
the BackupController.</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>BackupPhaseRunning means the backup is currently executing.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyPhase">BackupPolicyPhase
(<code>string</code> alias)</h3>
<div>
<p>BackupPolicyPhase defines phases for BackupPolicy.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
</p>
<div>
<p>BackupPolicySpec defines the desired state of BackupPolicy</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backupRepoName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of BackupRepo where the backup data will be stored.
If not set, data will be stored in the default backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>pathPrefix</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the directory inside the backup repository to store the backup.
This path is relative to the path of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of retries before marking the backup as failed.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
BackupTarget
</a>
</em>
</td>
<td>
<p>Specifies the target information to back up, such as the target pod, the
cluster connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>targets</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
[]BackupTarget
</a>
</em>
</td>
<td>
<p>Specifies multiple target information for backup operations. This includes details
such as the target pod and cluster connection credentials. All specified targets
will be backed up collectively.
optional</p>
</td>
</tr>
<tr>
<td>
<code>backupMethods</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">
[]BackupMethod
</a>
</em>
</td>
<td>
<p>Defines the backup methods.</p>
</td>
</tr>
<tr>
<td>
<code>useKopia</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether backup data should be stored in a Kopia repository.</p>
<p>Data within the Kopia repository is both compressed and encrypted. Furthermore,
data deduplication is implemented across various backups of the same cluster.
This approach significantly reduces the actual storage usage, particularly
for clusters with a low update frequency.</p>
<p>NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition, otherwise the backup will not be processed.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.EncryptionConfig">
EncryptionConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters for encrypting backup data.
Encryption will be disabled if the field is not set.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">BackupPolicyStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
</p>
<div>
<p>BackupPolicyStatus defines the observed state of BackupPolicy</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Phase">
Phase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Phase - in list of [Available,Unavailable]</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A human-readable message indicating details about why the BackupPolicy
is in this phase.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this BackupPolicy.
It refers to the BackupPolicy&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate
</h3>
<div>
<p>BackupPolicyTemplate should be provided by addon developers.
It is responsible for generating BackupPolicies for the addon that requires backup operations,
also determining the suitable backup methods and strategies.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<p>The metadata for the BackupPolicyTemplate object, including name, namespace, labels, and annotations.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">
BackupPolicyTemplateSpec
</a>
</em>
</td>
<td>
<p>Defines the desired state of the BackupPolicyTemplate.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>serviceKind</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the type of well-known service protocol that the BackupPolicyTemplate provides, and it is optional.
Some examples of well-known service protocols include:</p>
<ul>
<li>&ldquo;MySQL&rdquo;: Indicates that the Component provides a MySQL database service.</li>
<li>&ldquo;PostgreSQL&rdquo;: Indicates that the Component offers a PostgreSQL database service.</li>
<li>&ldquo;Redis&rdquo;: Signifies that the Component functions as a Redis key-value store.</li>
<li>&ldquo;ETCD&rdquo;: Denotes that the Component serves as an ETCD distributed key-value store</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetInstance">
TargetInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the selection criteria of instance to be backed up, and the connection credential to be used
during the backup process.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
[]SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the execution plans for backup tasks, specifying when and how backups should occur,
and the retention period of backup files.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethods</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">
[]BackupMethodTPL
</a>
</em>
</td>
<td>
<p>Defines an array of BackupMethods to be used.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum number of retry attempts for a backup before it is considered a failure.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateStatus">
BackupPolicyTemplateStatus
</a>
</em>
</td>
<td>
<p>Populated by the system, it represents the current information about the BackupPolicyTemplate.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>)
</p>
<div>
<p>BackupPolicyTemplateSpec contains the settings in a BackupPolicyTemplate.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>serviceKind</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the type of well-known service protocol that the BackupPolicyTemplate provides, and it is optional.
Some examples of well-known service protocols include:</p>
<ul>
<li>&ldquo;MySQL&rdquo;: Indicates that the Component provides a MySQL database service.</li>
<li>&ldquo;PostgreSQL&rdquo;: Indicates that the Component offers a PostgreSQL database service.</li>
<li>&ldquo;Redis&rdquo;: Signifies that the Component functions as a Redis key-value store.</li>
<li>&ldquo;ETCD&rdquo;: Denotes that the Component serves as an ETCD distributed key-value store</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetInstance">
TargetInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the selection criteria of instance to be backed up, and the connection credential to be used
during the backup process.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
[]SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the execution plans for backup tasks, specifying when and how backups should occur,
and the retention period of backup files.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethods</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">
[]BackupMethodTPL
</a>
</em>
</td>
<td>
<p>Defines an array of BackupMethods to be used.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum number of retry attempts for a backup before it is considered a failure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateStatus">BackupPolicyTemplateStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>)
</p>
<div>
<p>BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the most recent generation observed for this BackupPolicyTemplate.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Phase">
Phase
</a>
</em>
</td>
<td>
<p>Specifies the current phase of the BackupPolicyTemplate. Valid values are <code>empty</code>, <code>Available</code>, <code>Unavailable</code>.
When <code>Available</code>, the BackupPolicyTemplate is ready and can be referenced by related objects.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provides additional information about the current phase.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRef">BackupRef
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec</a>)
</p>
<div>
<p>BackupRef describes the backup info.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup name.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup namespace.</p>
</td>
</tr>
<tr>
<td>
<code>sourceTargetName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the source target for restoration, identified by its name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRepoPhase">BackupRepoPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoStatus">BackupRepoStatus</a>)
</p>
<div>
<p>BackupRepoPhase denotes different stages for the <code>BackupRepo</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Deleting&#34;</p></td>
<td><p>BackupRepoDeleting indicates the backup repository is being deleted.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupRepoFailed indicates the pre-check has been failed.</p>
</td>
</tr><tr><td><p>&#34;PreChecking&#34;</p></td>
<td><p>BackupRepoPreChecking indicates the backup repository is being pre-checked.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td><p>BackupRepoReady indicates the backup repository is ready for use.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRepoSpec">BackupRepoSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo</a>)
</p>
<div>
<p>BackupRepoSpec defines the desired state of <code>BackupRepo</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>storageProviderRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the <code>StorageProvider</code> used by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>accessMethod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.AccessMethod">
AccessMethod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the access method of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>volumeCapacity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the capacity of the PVC created by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>pvReclaimPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumereclaimpolicy-v1-core">
Kubernetes core/v1.PersistentVolumeReclaimPolicy
</a>
</em>
</td>
<td>
<p>Specifies reclaim policy of the PV created by this backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>config</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Stores the non-secret configuration parameters for the <code>StorageProvider</code>.</p>
</td>
</tr>
<tr>
<td>
<code>credential</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>References to the secret that holds the credentials for the <code>StorageProvider</code>.</p>
</td>
</tr>
<tr>
<td>
<code>pathPrefix</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the prefix of the path for storing backup data.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRepoStatus">BackupRepoStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo</a>)
</p>
<div>
<p>BackupRepoStatus defines the observed state of <code>BackupRepo</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoPhase">
BackupRepoPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the current phase of reconciliation for the backup repository.
Permissible values are PreChecking, Failed, Ready, Deleting.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provides a detailed description of the current state of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the latest generation of the resource that the controller has observed.</p>
</td>
</tr>
<tr>
<td>
<code>generatedCSIDriverSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to the generated secret for the <code>StorageProvider</code>.</p>
</td>
</tr>
<tr>
<td>
<code>generatedStorageClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the name of the generated storage class.</p>
</td>
</tr>
<tr>
<td>
<code>backupPVCName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the name of the PVC that stores backup data.</p>
</td>
</tr>
<tr>
<td>
<code>toolConfigSecretName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the name of the secret that contains the configuration for the tool.</p>
</td>
</tr>
<tr>
<td>
<code>isDefault</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates if this backup repository is the default one.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupSchedulePhase">BackupSchedulePhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleStatus">BackupScheduleStatus</a>)
</p>
<div>
<p>BackupSchedulePhase defines the phase of BackupSchedule</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td><p>BackupSchedulePhaseAvailable indicates the backup schedule is available.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupSchedulePhaseFailed indicates the backup schedule has failed.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupScheduleSpec">BackupScheduleSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSchedule">BackupSchedule</a>)
</p>
<div>
<p>BackupScheduleSpec defines the desired state of BackupSchedule.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backupPolicyName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backupPolicy to be applied for the <code>schedules</code>.</p>
</td>
</tr>
<tr>
<td>
<code>startingDeadlineMinutes</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the deadline in minutes for starting the backup workload if it
misses its scheduled time for any reason.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
[]SchedulePolicy
</a>
</em>
</td>
<td>
<p>Defines the list of backup schedules.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupScheduleStatus">BackupScheduleStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSchedule">BackupSchedule</a>)
</p>
<div>
<p>BackupScheduleStatus defines the observed state of BackupSchedule.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSchedulePhase">
BackupSchedulePhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the phase of the BackupSchedule.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the most recent generation observed for this BackupSchedule.
It refers to the BackupSchedule&rsquo;s generation, which is updated on mutation
by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>failureReason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents an error that caused the backup to fail.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ScheduleStatus">
map[string]github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.ScheduleStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the status of each schedule.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.Backup">Backup</a>)
</p>
<div>
<p>BackupSpec defines the desired state of Backup.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backupPolicyName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup policy to be applied for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethod</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup method name that is defined in the backup policy.</p>
</td>
</tr>
<tr>
<td>
<code>deletionPolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupDeletionPolicy">
BackupDeletionPolicy
</a>
</em>
</td>
<td>
<p>Determines whether the backup contents stored in the backup repository
should be deleted when the backup custom resource(CR) is deleted.
Supported values are <code>Retain</code> and <code>Delete</code>.</p>
<ul>
<li><code>Retain</code> means that the backup content and its physical snapshot on backup repository are kept.</li>
<li><code>Delete</code> means that the backup content and its physical snapshot on backup repository are deleted.</li>
</ul>
<p>the backup CR but retaining the backup contents in backup repository.
The current implementation only prevent accidental deletion of backup data.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionPeriod">
RetentionPeriod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines a duration up to which the backup should be kept.
Controller will remove all backups that are older than the RetentionPeriod.
If not set, the backup will be kept forever.
For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 	2y</li>
<li>months: 	6mo</li>
<li>days: 		30d</li>
<li>hours: 	12h</li>
<li>minutes: 	30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m.</p>
</td>
</tr>
<tr>
<td>
<code>parentBackupName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines the parent backup name for incremental or differential backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.Backup">Backup</a>)
</p>
<div>
<p>BackupStatus defines the observed state of Backup.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>formatVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the backup format version, which includes major, minor, and patch versions.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPhase">
BackupPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the current state of the backup operation.</p>
</td>
</tr>
<tr>
<td>
<code>expiration</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates when this backup becomes eligible for garbage collection.
A &lsquo;null&rsquo; value implies that the backup will not be cleaned up unless manually deleted.</p>
</td>
</tr>
<tr>
<td>
<code>startTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time when the backup operation was started.
The server&rsquo;s time is used for this timestamp.</p>
</td>
</tr>
<tr>
<td>
<code>completionTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time when the backup operation was completed.
This timestamp is recorded even if the backup operation fails.
The server&rsquo;s time is used for this timestamp.</p>
</td>
</tr>
<tr>
<td>
<code>duration</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the duration of the backup operation.
When converted to a string, the format is &ldquo;1h2m0.5s&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>totalSize</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the total size of the data backed up.
The size is represented as a string with capacity units in the format of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.
If no capacity unit is specified, it is assumed to be in bytes.</p>
</td>
</tr>
<tr>
<td>
<code>failureReason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Any error that caused the backup operation to fail.</p>
</td>
</tr>
<tr>
<td>
<code>backupRepoName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The directory within the backup repository where the backup data is stored.
This is an absolute path within the backup repository.</p>
</td>
</tr>
<tr>
<td>
<code>kopiaRepoPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the path of the Kopia repository.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaimName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the name of the persistent volume claim used to store the backup data.</p>
</td>
</tr>
<tr>
<td>
<code>timeRange</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTimeRange">
BackupTimeRange
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time range of the data backed up. For Point-in-Time Recovery (PITR),
this is the time range of recoverable data.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusTarget">
BackupStatusTarget
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the target information for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>targets</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusTarget">
[]BackupStatusTarget
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the targets information for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">
BackupMethod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the backup method information for this backup.
Refer to BackupMethod for more details.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.EncryptionConfig">
EncryptionConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the encryption config for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionStatus">
[]ActionStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the actions status for this backup.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSnapshots</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeSnapshotStatus">
[]VolumeSnapshotStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the volume snapshot status for the action.</p>
</td>
</tr>
<tr>
<td>
<code>extras</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records any additional information for the backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupStatusTarget">BackupStatusTarget
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>BackupTarget</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
BackupTarget
</a>
</em>
</td>
<td>
<p>
(Members of <code>BackupTarget</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>selectedTargetPods</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Records the selected pods by the target info during backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusTarget">BackupStatusTarget</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a mandatory and unique identifier for each target when using the &ldquo;targets&rdquo; field.
The backup data for the current target is stored in a uniquely named subdirectory.</p>
</td>
</tr>
<tr>
<td>
<code>podSelector</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelector">
PodSelector
</a>
</em>
</td>
<td>
<p>Used to find the target pod. The volumes of the target pod will be backed up.</p>
</td>
</tr>
<tr>
<td>
<code>connectionCredential</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ConnectionCredential">
ConnectionCredential
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the connection credential to connect to the target database cluster.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.KubeResources">
KubeResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the kubernetes resources to back up.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the service account to run the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>containerPort</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ContainerPort">
ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the container port in the target pod.
If not specified, the first container and its first port will be used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupTimeRange">BackupTimeRange
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionStatus">ActionStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
<p>BackupTimeRange records the time range of backed up data, for PITR, this is the
time range of recoverable data.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>timeZone</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>time zone, supports only zone offset, with a value range of &ldquo;-12:59 ~ +13:00&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>start</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the start time of the backup, in Coordinated Universal Time (UTC).</p>
</td>
</tr>
<tr>
<td>
<code>end</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the end time of the backup, in Coordinated Universal Time (UTC).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupType">BackupType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">ActionSetSpec</a>)
</p>
<div>
<p>BackupType the backup type.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Continuous&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Differential&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Full&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Incremental&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BaseJobActionSpec">BaseJobActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupActionSpec">BackupActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">JobActionSpec</a>)
</p>
<div>
<p>BaseJobActionSpec is an action that creates a Kubernetes Job to execute a command.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the image of the backup container.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Defines the commands to back up the volume data.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ConnectionCredential">ConnectionCredential
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">ReadyConfig</a>)
</p>
<div>
<p>ConnectionCredential specifies the connection credential to connect to the
target database cluster.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Refers to the Secret object that contains the connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>usernameKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the map key of the user in the connection credential secret.</p>
</td>
</tr>
<tr>
<td>
<code>passwordKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the map key of the password in the connection credential secret.
This password will be saved in the backup annotation for full backup.
You can use the environment variable DP_ENCRYPTION_KEY to specify encryption key.</p>
</td>
</tr>
<tr>
<td>
<code>hostKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the map key of the host in the connection credential secret.</p>
</td>
</tr>
<tr>
<td>
<code>portKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the map key of the port in the connection credential secret.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ContainerPort">ContainerPort
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>containerName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of container with the port.</p>
</td>
</tr>
<tr>
<td>
<code>portName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the port name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.DataRestorePolicy">DataRestorePolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RequiredPolicyForAllPodSelection">RequiredPolicyForAllPodSelection</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;OneToMany&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;OneToOne&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.EncryptionConfig">EncryptionConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
<p>EncryptionConfig defines the parameters for encrypting backup data.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>algorithm</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the encryption algorithm. Currently supported algorithms are:</p>
<ul>
<li>AES-128-CFB</li>
<li>AES-192-CFB</li>
<li>AES-256-CFB</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>passPhraseSecretKeyRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<p>Selects the key of a secret in the current namespace, the value of the secret
is used as the encryption key.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.EnvVar">EnvVar
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">BackupMethodTPL</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the environment variable key.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the environment variable value.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ValueFrom">
ValueFrom
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the source used to determine the value of the environment variable.
Cannot be used if value is not empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ExecAction">ExecAction
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">ReadyConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ExecActionTarget">
ExecActionTarget
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the pods that need to be executed for the exec action.
Execution will occur on all pods that meet the conditions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ExecActionSpec">ExecActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSpec">ActionSpec</a>)
</p>
<div>
<p>ExecActionSpec is an action that uses the pod exec API to execute a command in a container
in a pod.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the container within the pod where the command should be executed.
If not specified, the first container in the pod is used by default.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Defines the command and arguments to be executed.</p>
</td>
</tr>
<tr>
<td>
<code>onError</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionErrorMode">
ActionErrorMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates how to behave if an error is encountered during the execution of this action.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum duration to wait for the hook to complete before
considering the execution a failure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ExecActionTarget">ExecActionTarget
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ExecAction">ExecAction</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>podSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>Executes kubectl in all selected pods.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.IncludeResource">IncludeResource
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreKubeResources">RestoreKubeResources</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>groupResource</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labelSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects the specified resource for recovery by label.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.JobAction">JobAction
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">ReadyConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>requiredPolicyForAllPodSelection</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RequiredPolicyForAllPodSelection">
RequiredPolicyForAllPodSelection
</a>
</em>
</td>
<td>
<p>Specifies the restore policy, which is required when the pod selection strategy for the source target is &lsquo;All&rsquo;.
This field is ignored if the pod selection strategy is &lsquo;Any&rsquo;.
optional</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionTarget">
JobActionTarget
</a>
</em>
</td>
<td>
<p>Defines the pods that needs to be executed for the job action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">JobActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSpec">ActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupDataActionSpec">BackupDataActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreActionSpec">RestoreActionSpec</a>)
</p>
<div>
<p>JobActionSpec is an action that creates a Kubernetes Job to execute a command.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>BaseJobActionSpec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BaseJobActionSpec">
BaseJobActionSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>BaseJobActionSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>runOnTargetPodNode</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines whether to run the job workload on the target pod node.
If the backup container needs to mount the target pod&rsquo;s volumes, this field
should be set to true. Otherwise, the target pod&rsquo;s volumes will be ignored.</p>
</td>
</tr>
<tr>
<td>
<code>onError</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionErrorMode">
ActionErrorMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates how to behave if an error is encountered during the execution of this action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.JobActionTarget">JobActionTarget
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.JobAction">JobAction</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>podSelector</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelector">
PodSelector
</a>
</em>
</td>
<td>
<p>Selects one of the pods, identified by labels, to build the job spec.
This includes mounting required volumes and injecting built-in environment variables of the selected pod.</p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines which volumes of the selected pod need to be mounted on the restoring pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.KubeResources">KubeResources
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget</a>)
</p>
<div>
<p>KubeResources defines the kubernetes resources to back up.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>selector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>A metav1.LabelSelector to filter the target kubernetes resources that need
to be backed up. If not set, will do not back up any kubernetes resources.</p>
</td>
</tr>
<tr>
<td>
<code>included</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>included is a slice of namespaced-scoped resource type names to include in
the kubernetes resources.
The default value is empty.</p>
</td>
</tr>
<tr>
<td>
<code>excluded</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>excluded is a slice of namespaced-scoped resource type names to exclude in
the kubernetes resources.
The default value is empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ParametersSchema">ParametersSchema
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProviderSpec">StorageProviderSpec</a>)
</p>
<div>
<p>ParametersSchema describes the parameters needed for a certain storage.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>openAPIV3Schema</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jsonschemaprops-v1-apiextensions-k8s-io">
Kubernetes api extensions v1.JSONSchemaProps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the parameters in OpenAPI V3.</p>
</td>
</tr>
<tr>
<td>
<code>credentialFields</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines which parameters are credential fields, which need to be handled specifically.
For instance, these should be stored in a <code>Secret</code> instead of a <code>ConfigMap</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetStatus">ActionSetStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">BackupPolicyStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateStatus">BackupPolicyTemplateStatus</a>)
</p>
<div>
<p>Phase defines the BackupPolicy and ActionSet CR .status.phase</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Unavailable&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PodSelectionStrategy">PodSelectionStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance</a>)
</p>
<div>
<p>PodSelectionStrategy specifies the strategy to select when multiple pods are
selected for backup target</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;All&#34;</p></td>
<td><p>PodSelectionStrategyAll selects all pods that match the labelsSelector.</p>
</td>
</tr><tr><td><p>&#34;Any&#34;</p></td>
<td><p>PodSelectionStrategyAny selects any one pod that match the labelsSelector.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PodSelector">PodSelector
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionTarget">JobActionTarget</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>LabelSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>
(Members of <code>LabelSelector</code> are embedded into this type.)
</p>
<p>labelsSelector is the label selector to filter the target pods.</p>
</td>
</tr>
<tr>
<td>
<code>fallbackLabelSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>fallbackLabelSelector is used to filter available pods when the labelSelector fails.
This only takes effect when the <code>strategy</code> field below is set to <code>Any</code>.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelectionStrategy">
PodSelectionStrategy
</a>
</em>
</td>
<td>
<p>Specifies the strategy to select the target pod when multiple pods are selected.
Valid values are:</p>
<ul>
<li><code>Any</code>: select any one pod that match the labelsSelector.</li>
<li><code>All</code>: select all pods that match the labelsSelector. The backup data for the current pod
will be stored in a subdirectory named after the pod.</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>requiredPolicyForAllPodSelection</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RequiredPolicyForAllPodSelection">
RequiredPolicyForAllPodSelection
</a>
</em>
</td>
<td>
<p>Specifies the restore policy, which is required when the pod selection strategy for the source target is &lsquo;All&rsquo;.
This field is ignored if the pod selection strategy is &lsquo;Any&rsquo;.
optional</p>
</td>
</tr>
<tr>
<td>
<code>dataSourceRef</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeConfig">
VolumeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration when using <code>persistentVolumeClaim.spec.dataSourceRef</code> method for restoring.
Describes the source volume of the backup targetVolumes and the mount path in the restoring container.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaims</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaim">
[]RestoreVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the persistent Volume claims that need to be restored and mounted together into the restore job.
These persistent Volume claims will be created if they do not exist.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimsTemplate</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaimsTemplate">
RestoreVolumeClaimsTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines a template to build persistent Volume claims that need to be restored.
These claims will be created in an orderly manner based on the number of replicas or reused if they already exist.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimRestorePolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeClaimRestorePolicy">
VolumeClaimRestorePolicy
</a>
</em>
</td>
<td>
<p>Defines restore policy for persistent volume claim.
Supported policies are as follows:</p>
<ul>
<li><code>Parallel</code>: parallel recovery of persistent volume claim.</li>
<li><code>Serial</code>: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>schedulingSpec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulingSpec">
SchedulingSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling spec for the restoring pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ReadinessProbe">ReadinessProbe
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">ReadyConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>initialDelaySeconds</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of seconds after the container has started before the probe is initiated.</p>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of seconds after which the probe times out.
The default value is 30 seconds, and the minimum value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies how often (in seconds) to perform the probe.
The default value is 5 seconds, and the minimum value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ReadinessProbeExecAction">
ReadinessProbeExecAction
</a>
</em>
</td>
<td>
<p>Specifies the action to take.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ReadinessProbeExecAction">ReadinessProbeExecAction
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ReadinessProbe">ReadinessProbe</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Refers to the container image.</p>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Refers to the container command.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">ReadyConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>jobAction</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobAction">
JobAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration for a job action.</p>
</td>
</tr>
<tr>
<td>
<code>execAction</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ExecAction">
ExecAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration for an exec action.</p>
</td>
</tr>
<tr>
<td>
<code>connectionCredential</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ConnectionCredential">
ConnectionCredential
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the credential template used to create a connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ReadinessProbe">
ReadinessProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines a periodic probe of the service readiness.
The controller will perform postReadyHooks of BackupScript.spec.restore
after the service readiness when readinessProbe is configured.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RequiredPolicyForAllPodSelection">RequiredPolicyForAllPodSelection
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.JobAction">JobAction</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dataRestorePolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.DataRestorePolicy">
DataRestorePolicy
</a>
</em>
</td>
<td>
<p>Specifies the data restore policy. Options include:
- OneToMany: Enables restoration of all volumes from a single data copy of the original target instance.
The &lsquo;sourceOfOneToMany&rsquo; field must be set when using this policy.
- OneToOne: Restricts data restoration such that each data piece can only be restored to a single target instance.
This is the default policy. When the number of target instances specified for restoration surpasses the count of original backup target instances.</p>
</td>
</tr>
<tr>
<td>
<code>sourceOfOneToMany</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SourceOfOneToMany">
SourceOfOneToMany
</a>
</em>
</td>
<td>
<p>Specifies the name of the source target pod. This field is mandatory when the DataRestorePolicy is configured to &lsquo;OneToMany&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreActionSpec">RestoreActionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">ActionSetSpec</a>)
</p>
<div>
<p>RestoreActionSpec defines how to restore data.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prepareData</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">
JobActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the action required to prepare data for restoration.</p>
</td>
</tr>
<tr>
<td>
<code>postReady</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSpec">
[]ActionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the actions that should be executed after the data has been prepared and is ready for restoration.</p>
</td>
</tr>
<tr>
<td>
<code>baseBackupRequired</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines if a base backup is required during restoration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreActionStatus">RestoreActionStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatusAction">RestoreStatusAction</a>)
</p>
<div>
<p>RestoreActionStatus the status of restore action.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Completed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Processing&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreKubeResources">RestoreKubeResources
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>included</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.IncludeResource">
[]IncludeResource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Restores the specified resources.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestorePhase">RestorePhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatus">RestoreStatus</a>)
</p>
<div>
<p>RestorePhase The current phase. Valid values are Running, Completed, Failed, AsDataSource.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;AsDataSource&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Completed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.Restore">Restore</a>)
</p>
<div>
<p>RestoreSpec defines the desired state of Restore</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRef">
BackupRef
</a>
</em>
</td>
<td>
<p>Specifies the backup to be restored. The restore behavior is based on the backup type:</p>
<ol>
<li>Full: will be restored the full backup directly.</li>
<li>Incremental: will be restored sequentially from the most recent full backup of this incremental backup.</li>
<li>Differential: will be restored sequentially from the parent backup of the differential backup.</li>
<li>Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.</li>
</ol>
</td>
</tr>
<tr>
<td>
<code>restoreTime</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the point in time for restoring.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreKubeResources">
RestoreKubeResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Restores the specified resources of Kubernetes.</p>
</td>
</tr>
<tr>
<td>
<code>prepareDataConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">
PrepareDataConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims
that need to be restored and scheduling strategy of temporary recovery pod.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the service account name needed for recovery pod.</p>
</td>
</tr>
<tr>
<td>
<code>readyConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ReadyConfig">
ReadyConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the action of &ldquo;postReady&rdquo; phase.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of environment variables to set in the container for restore. These will be
merged with the env of Backup and ActionSet.</p>
<p>The priority of merging is as follows: <code>Restore env &gt; Backup env &gt; ActionSet env</code>.</p>
</td>
</tr>
<tr>
<td>
<code>containerResources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the required resources of restore job&rsquo;s container.</p>
</td>
</tr>
<tr>
<td>
<code>backoffLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of retries before marking the restore failed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreStage">RestoreStage
(<code>string</code> alias)</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;postReady&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;prepareData&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreStatus">RestoreStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.Restore">Restore</a>)
</p>
<div>
<p>RestoreStatus defines the observed state of Restore</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestorePhase">
RestorePhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the current phase of the restore.</p>
</td>
</tr>
<tr>
<td>
<code>startTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the date/time when the restore started being processed.</p>
</td>
</tr>
<tr>
<td>
<code>completionTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the date/time when the restore finished being processed.</p>
</td>
</tr>
<tr>
<td>
<code>duration</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the duration of the restore execution.
When converted to a string, the form is &ldquo;1h2m0.5s&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatusActions">
RestoreStatusActions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records all restore actions performed.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the current state of the restore API Resource, like warning.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreStatusAction">RestoreStatusAction
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatusActions">RestoreStatusActions</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Describes the name of the restore action based on the current backup.</p>
</td>
</tr>
<tr>
<td>
<code>backupName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Describes which backup&rsquo;s restore action belongs to.</p>
</td>
</tr>
<tr>
<td>
<code>objectKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>Describes the execution object of the restore action.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provides a human-readable message indicating details about the object condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreActionStatus">
RestoreActionStatus
</a>
</em>
</td>
<td>
<p>The status of this action.</p>
</td>
</tr>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The start time of the restore job.</p>
</td>
</tr>
<tr>
<td>
<code>endTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The completion time of the restore job.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreStatusActions">RestoreStatusActions
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatus">RestoreStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prepareData</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatusAction">
[]RestoreStatusAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the actions for the prepareData phase.</p>
</td>
</tr>
<tr>
<td>
<code>postReady</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreStatusAction">
[]RestoreStatusAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the actions for the postReady phase.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaim">RestoreVolumeClaim
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaimsTemplate">RestoreVolumeClaimsTemplate</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<p>Specifies the standard metadata for the object.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata</a></p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>volumeClaimSpec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumeclaimspec-v1-core">
Kubernetes core/v1.PersistentVolumeClaimSpec
</a>
</em>
</td>
<td>
<p>Defines the desired characteristics of a persistent volume claim.</p>
</td>
</tr>
<tr>
<td>
<code>VolumeConfig</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeConfig">
VolumeConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>VolumeConfig</code> are embedded into this type.)
</p>
<p>Describes the source volume of the backup target volumes and the mount path in the restoring container.
At least one must exist for volumeSource and mountPath.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaimsTemplate">RestoreVolumeClaimsTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>templates</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaim">
[]RestoreVolumeClaim
</a>
</em>
</td>
<td>
<p>Contains a list of volume claims.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Specifies the replicas of persistent volume claim that need to be created and restored.
The format of the created claim name is <code>$(template-name)-$(index)</code>.</p>
</td>
</tr>
<tr>
<td>
<code>startingIndex</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Specifies the starting index for the created persistent volume claim according to the template.
The minimum value is 0.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RetentionPeriod">RetentionPeriod
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">SchedulePolicy</a>)
</p>
<div>
<p>RetentionPeriod represents a duration in the format &ldquo;1y2mo3w4d5h6m&rdquo;, where
y=year, mo=month, w=week, d=day, h=hour, m=minute.</p>
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RuntimeSettings">RuntimeSettings
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">BackupMethodTPL</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the resource required by container.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/">https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SchedulePhase">SchedulePhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ScheduleStatus">ScheduleStatus</a>)
</p>
<div>
<p>SchedulePhase represents the phase of a schedule.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">SchedulePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleSpec">BackupScheduleSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether the backup schedule is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>backupMethod</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the backup method name that is defined in backupPolicy.</p>
</td>
</tr>
<tr>
<td>
<code>cronExpression</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the cron expression for the schedule. The timezone is in UTC.
see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionPeriod">
RetentionPeriod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines the duration for which the backup should be kept.
KubeBlocks will remove all backups that are older than the RetentionPeriod.
For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 	2y</li>
<li>months: 	6mo</li>
<li>days: 		30d</li>
<li>hours: 	12h</li>
<li>minutes: 	30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ScheduleStatus">ScheduleStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleStatus">BackupScheduleStatus</a>)
</p>
<div>
<p>ScheduleStatus represents the status of each schedule.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePhase">
SchedulePhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the phase of the schedule.</p>
</td>
</tr>
<tr>
<td>
<code>failureReason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents an error that caused the backup to fail.</p>
</td>
</tr>
<tr>
<td>
<code>lastScheduleTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the last time the backup was scheduled.</p>
</td>
</tr>
<tr>
<td>
<code>lastSuccessfulTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the last time the backup was successfully completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SchedulingSpec">SchedulingSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tolerations</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tolerations for the restoring pod.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines a selector which must be true for the pod to fit on a node.
The selector must match a node&rsquo;s labels for the pod to be scheduled on that node.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a request to schedule this pod onto a specific node. If it is non-empty,
the scheduler simply schedules this pod onto that node, assuming that it fits resource
requirements.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Contains a group of affinity scheduling rules.
Refer to <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes how a group of pods ought to spread across topology
domains. The scheduler will schedule pods in a way which abides by the constraints.
Refer to <a href="https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/">https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/</a></p>
</td>
</tr>
<tr>
<td>
<code>schedulerName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduler to dispatch the pod.
If not specified, the pod will be dispatched by the default scheduler.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SourceOfOneToMany">SourceOfOneToMany
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RequiredPolicyForAllPodSelection">RequiredPolicyForAllPodSelection</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>targetPodName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the source target pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.StorageProviderPhase">StorageProviderPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProviderStatus">StorageProviderStatus</a>)
</p>
<div>
<p>StorageProviderPhase defines phases of a <code>StorageProvider</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;NotReady&#34;</p></td>
<td><p>StorageProviderNotReady indicates that the <code>StorageProvider</code> is not ready,
usually because the specified CSI driver is not yet installed.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td><p>StorageProviderReady indicates that the <code>StorageProvider</code> is ready for use.</p>
</td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.StorageProviderSpec">StorageProviderSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>)
</p>
<div>
<p>StorageProviderSpec defines the desired state of <code>StorageProvider</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>csiDriverName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the CSI driver used to access remote storage.
This field can be empty, it indicates that the storage is not accessible via CSI.</p>
</td>
</tr>
<tr>
<td>
<code>csiDriverSecretTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template that used to render and generate <code>k8s.io/api/core/v1.Secret</code>
resources for a specific CSI driver.
For example, <code>accessKey</code> and <code>secretKey</code> needed by CSI-S3 are stored in this
<code>Secret</code> resource.</p>
</td>
</tr>
<tr>
<td>
<code>storageClassTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template utilized to render and generate <code>kubernetes.storage.k8s.io.v1.StorageClass</code>
resources. The `StorageClass&rsquo; created by this template is aimed at using the CSI driver.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaimTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template that renders and generates <code>k8s.io/api/core/v1.PersistentVolumeClaim</code>
resources. This PVC can reference the <code>StorageClass</code> created from <code>storageClassTemplate</code>,
allowing Pods to access remote storage by mounting the PVC.</p>
</td>
</tr>
<tr>
<td>
<code>datasafedConfigTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A Go template used to render and generate <code>k8s.io/api/core/v1.Secret</code>.
This <code>Secret</code> involves the configuration details required by the <code>datasafed</code> tool
to access remote storage. For example, the <code>Secret</code> should contain <code>endpoint</code>,
<code>bucket</code>, &lsquo;region&rsquo;, &lsquo;accessKey&rsquo;, &lsquo;secretKey&rsquo;, or something else for S3 storage.
This field can be empty, it means this kind of storage is not accessible via
the <code>datasafed</code> tool.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the parameters required for storage.
The parameters defined here can be referenced in the above templates,
and <code>kbcli</code> uses this definition for dynamic command-line parameter parsing.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.StorageProviderStatus">StorageProviderStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>)
</p>
<div>
<p>StorageProviderStatus defines the observed state of <code>StorageProvider</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.StorageProviderPhase">
StorageProviderPhase
</a>
</em>
</td>
<td>
<p>The phase of the <code>StorageProvider</code>. Valid phases are <code>NotReady</code> and <code>Ready</code>.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the current state of the <code>StorageProvider</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SyncProgress">SyncProgress
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupDataActionSpec">BackupDataActionSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines if the backup progress should be synchronized. If set to true,
a sidecar container will be instantiated to synchronize the backup progress with the
Backup Custom Resource (CR) status.</p>
</td>
</tr>
<tr>
<td>
<code>intervalSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the interval in seconds for synchronizing the backup progress.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">BackupMethodTPL</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>role</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the role to select one or more replicas for backup.</p>
<ul>
<li>If no replica with the specified role exists, the backup task will fail.
Special case: If there is only one replica in the cluster, it will be used for backup,
even if its role differs from the specified one.
For example, if you specify backing up on a secondary replica, but the cluster is single-node
with only one primary replica, the primary will be used for backup.
Future versions will address this special case using role priorities.</li>
<li>If multiple replicas satisfy the specified role, the choice (<code>Any</code> or <code>All</code>) will be made according to
the <code>strategy</code> field below.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>fallbackRole</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the fallback role to select one replica for backup, this only takes effect when the
<code>strategy</code> field below is set to <code>Any</code>.</p>
</td>
</tr>
<tr>
<td>
<code>account</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If <code>backupPolicy.componentDefs</code> is set, this field is required to specify the system account name.
This account must match one listed in <code>componentDefinition.spec.systemAccounts[*].name</code>.
The corresponding secret created by this account is used to connect to the database.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelectionStrategy">
PodSelectionStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the PodSelectionStrategy to use when multiple pods are
selected for the backup target.
Valid values are:</p>
<ul>
<li>Any: Selects any one pod that matches the labelsSelector.</li>
<li>All: Selects all pods that match the labelsSelector.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>containerPort</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ContainerPort">
ContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the container port in the target pod.
If not specified, the first container and its first port will be used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.TargetVolumeInfo">TargetVolumeInfo
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethodTPL">BackupMethodTPL</a>)
</p>
<div>
<p>TargetVolumeInfo specifies the volumes and their mounts of the targeted application
that should be mounted in backup workload.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>volumes</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the list of volumes of targeted application that should be mounted
on the backup workload.</p>
</td>
</tr>
<tr>
<td>
<code>volumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the mount for the volumes specified in <code>volumes</code> section.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ValueFrom">ValueFrom
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.EnvVar">EnvVar</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>versionMapping</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VersionMapping">
[]VersionMapping
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determine the appropriate version of the backup tool image from service version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.VersionMapping">VersionMapping
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ValueFrom">ValueFrom</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>serviceVersions</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Represents an array of the service version that can be mapped to the appropriate value.
Each name in the list can represent an exact name, a name prefix, or a regular expression pattern.</p>
<p>For example:</p>
<ul>
<li>&ldquo;8.0.33&rdquo;: Matches the exact name &ldquo;8.0.33&rdquo;</li>
<li>&ldquo;8.0&rdquo;: Matches all names starting with &ldquo;8.0&rdquo;</li>
<li>&rdquo;^8.0.\d&#123;1,2&#125;$&ldquo;: Matches all names starting with &ldquo;8.0.&rdquo; followed by one or two digits.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>mappedValue</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a mapping value based on service version.
Typically used to set up the tools image required for backup operations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.VolumeClaimRestorePolicy">VolumeClaimRestorePolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>)
</p>
<div>
<p>VolumeClaimRestorePolicy defines restore policy for persistent volume claim.
Supported policies are as follows:</p>
<ol>
<li>Parallel: parallel recovery of persistent volume claim.</li>
<li>Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.</li>
</ol>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Parallel&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Serial&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.VolumeConfig">VolumeConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PrepareDataConfig">PrepareDataConfig</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreVolumeClaim">RestoreVolumeClaim</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>volumeSource</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the volume that will be restored from the specified volume of the backup targetVolumes.
This is required if the backup uses a volume snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>mountPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the path within the restoring container at which the volume should be mounted.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.VolumeSnapshotStatus">VolumeSnapshotStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionStatus">ActionStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the volume snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>contentName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the volume snapshot content.</p>
</td>
</tr>
<tr>
<td>
<code>volumeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the volume.</p>
</td>
</tr>
<tr>
<td>
<code>size</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The size of the volume snapshot.</p>
</td>
</tr>
<tr>
<td>
<code>targetName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Associates this volumeSnapshot with its corresponding target.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>