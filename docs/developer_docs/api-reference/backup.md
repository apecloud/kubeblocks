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
</li></ul>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet
</h3>
<div>
<p>ActionSet is the Schema for the actionsets API</p><br />
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
<p>backupType specifies the backup type, supported values: Full, Continuous.<br />Full means full backup.<br />Incremental means back up data that have changed since the last backup (full or incremental).<br />Differential means back up data that have changed since the last full backup.<br />Continuous will back up the transaction log continuously, the PITR (Point in Time Recovery).<br />can be performed based on the continuous backup and full backup.</p><br />
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
<p>List of environment variables to set in the container.</p><br />
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
<p>List of sources to populate environment variables in the container.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated.</p><br />
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
<p>backup specifies the backup action.</p><br />
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
<p>restore specifies the restore action.</p><br />
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
<p>Backup is the Schema for the backups API.</p><br />
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
<p>Which backupPolicy is applied to perform this backup.</p><br />
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
<p>backupMethod specifies the backup method name that is defined in backupPolicy.</p><br />
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
<p>deletionPolicy determines whether the backup contents stored in backup repository<br />should be deleted when the backup custom resource is deleted.<br />Supported values are &ldquo;Retain&rdquo; and &ldquo;Delete&rdquo;.<br />&ldquo;Retain&rdquo; means that the backup can not be deleted and remains in &lsquo;Deleting&rsquo; phase.<br />&ldquo;Delete&rdquo; means that the backup content and its physical snapshot on backup repository are deleted.</p><br />
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
<p>retentionPeriod determines a duration up to which the backup should be kept.<br />Controller will remove all backups that are older than the RetentionPeriod.<br />For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.<br />Sample duration format:<br />- years: 	2y<br />- months: 	6mo<br />- days: 		30d<br />- hours: 	12h<br />- minutes: 	30m<br />You can also combine the above durations. For example: 30d12h30m.<br />If not set, the backup will be kept forever.</p><br />
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
<p>parentBackupName determines the parent backup name for incremental or<br />differential backup.</p><br />
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
<p>BackupPolicy is the Schema for the backuppolicies API.</p><br />
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
<p>backupRepoName is the name of BackupRepo and the backup data will be<br />stored in this repository. If not set, will be stored in the default<br />backup repository.</p><br />
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
<p>pathPrefix is the directory inside the backup repository to store the backup content.<br />It is a relative to the path of the backup repository.</p><br />
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
<p>Specifies the number of retries before marking the backup failed.</p><br />
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
<p>target specifies the target information to back up.</p><br />
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
<p>backupMethods defines the backup methods.</p><br />
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
<p>useKopia specifies whether backup data should be stored in a Kopia repository.<br />Data within the Kopia repository is both compressed and encrypted. Furthermore,<br />data deduplication is implemented across various backups of the same cluster.<br />This approach significantly reduces the actual storage usage, particularly for<br />clusters with a low update frequency.<br />NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition,<br />otherwise the backup will not be processed.</p><br />
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
<p>BackupRepo is the Schema for the backuprepos API</p><br />
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
<p>The storage provider used by this backup repo.</p><br />
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
<p>Specifies the access method of the backup repo.</p><br />
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
<p>The requested capacity for the PVC created by this backup repo.</p><br />
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
<p>The reclaim policy for the PV created by this backup repo.</p><br />
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
<p>Non-secret configurations for the storage provider.</p><br />
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
<p>A secret that contains the credentials needed by the storage provider.</p><br />
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
<p>BackupSchedule is the Schema for the backupschedules API.</p><br />
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
<p>Which backupPolicy is applied to perform this backup.</p><br />
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the<br />backup workload if it misses scheduled time for any reason.</p><br />
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
<p>schedules defines the list of backup schedules.</p><br />
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
<p>Restore is the Schema for the restores API</p><br />
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
<p>backup to be restored. The restore behavior based on the backup type:<br />1. Full: will be restored the full backup directly.<br />2. Incremental: will be restored sequentially from the most recent full backup of this incremental backup.<br />3. Differential: will be restored sequentially from the parent backup of the differential backup.<br />4. Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.</p><br />
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
<p>restoreTime is the point in time for restoring.</p><br />
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
<p>restore the specified resources of kubernetes.</p><br />
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
<p>configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims<br />that need to be restored and scheduling strategy of temporary recovery pod.</p><br />
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
<p>service account name which needs for recovery pod.</p><br />
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
<p>configuration for the action of &ldquo;postReady&rdquo; phase.</p><br />
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
<p>list of environment variables to set in the container for restore and will be<br />merged with the env of Backup and ActionSet.<br />The priority of merging is as follows: Restore env &gt; Backup env &gt; ActionSet env.</p><br />
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
<p>specified the required resources of restore job&rsquo;s container.</p><br />
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
<p>Specifies the number of retries before marking the restore failed.</p><br />
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.AccessMethod">AccessMethod
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepoSpec">BackupRepoSpec</a>)
</p>
<div>
<p>AccessMethod is an enum type that defines the access method of the backup repo.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Mount&#34;</p></td>
<td><p>AccessMethodMount means that the storage is mounted locally,<br />so that remote files can be accessed just like a local file.</p><br /></td>
</tr><tr><td><p>&#34;Tool&#34;</p></td>
<td><p>AccessMethodTool means to access the storage with a command-line tool,<br />which helps to transfer files between the storage and local.</p><br /></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionErrorMode">ActionErrorMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ExecActionSpec">ExecActionSpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionSpec">JobActionSpec</a>)
</p>
<div>
<p>ActionErrorMode defines how should treat an error from an action.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Continue&#34;</p></td>
<td><p>ActionErrorModeContinue means that an error from an action is acceptable.</p><br /></td>
</tr><tr><td><p>&#34;Fail&#34;</p></td>
<td><p>ActionErrorModeFail means that an error from an action is problematic.</p><br /></td>
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
<td><p>ActionPhaseCompleted means the action has run successfully without errors.</p><br /></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>ActionPhaseFailed means the action ran but encountered an error that</p><br /></td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td><p>ActionPhaseNew means the action has been created but not yet processed by<br />the BackupController.</p><br /></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>ActionPhaseRunning means the action is currently executing.</p><br /></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ActionSetSpec">ActionSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSet">ActionSet</a>)
</p>
<div>
<p>ActionSetSpec defines the desired state of ActionSet</p><br />
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
<p>backupType specifies the backup type, supported values: Full, Continuous.<br />Full means full backup.<br />Incremental means back up data that have changed since the last backup (full or incremental).<br />Differential means back up data that have changed since the last full backup.<br />Continuous will back up the transaction log continuously, the PITR (Point in Time Recovery).<br />can be performed based on the continuous backup and full backup.</p><br />
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
<p>List of environment variables to set in the container.</p><br />
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
<p>List of sources to populate environment variables in the container.<br />The keys defined within a source must be a C_IDENTIFIER. All invalid keys<br />will be reported as an event when the container is starting. When a key exists in multiple<br />sources, the value associated with the last source will take precedence.<br />Values defined by an Env with a duplicate key will take precedence.<br />Cannot be updated.</p><br />
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
<p>backup specifies the backup action.</p><br />
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
<p>restore specifies the restore action.</p><br />
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
<p>ActionSetStatus defines the observed state of ActionSet</p><br />
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
<p>phase - in list of [Available,Unavailable]</p><br />
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
<p>A human-readable message indicating details about why the ActionSet is in this phase.</p><br />
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
<p>generation number</p><br />
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
<p>ActionSpec defines an action that should be executed. Only one of the fields may be set.</p><br />
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
<p>exec specifies the action should be executed by the pod exec API in a container.</p><br />
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
<p>job specifies the action should be executed by a Kubernetes Job.</p><br />
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
<p>name is the name of the action.</p><br />
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
<p>phase is the current state of the action.</p><br />
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
<p>startTimestamp records the time an action was started.</p><br />
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
<p>completionTimestamp records the time an action was completed.</p><br />
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
<p>failureReason is an error that caused the backup to fail.</p><br />
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
<p>actionType is the type of the action.</p><br />
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
<p>availableReplicas available replicas for statefulSet action.</p><br />
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
<p>objectRef is the object reference for the action.</p><br />
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
<p>totalSize is the total size of backed up data size.<br />A string with capacity units in the format of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p><br />
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
<p>timeRange records the time range of backed up data, for PITR, this is the<br />time range of recoverable data.</p><br />
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
<p>volumeSnapshots records the volume snapshot status for the action.</p><br />
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
<p>backupData specifies the backup data action.</p><br />
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
<p>preBackup specifies a hook that should be executed before the backup.</p><br />
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
<p>postBackup specifies a hook that should be executed after the backup.</p><br />
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
<p>preDelete defines that custom deletion action which can be executed before executing the built-in deletion action.<br />note that preDelete action job will ignore the env/envFrom.</p><br />
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
<p>BackupDataActionSpec defines how to back up data.</p><br />
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
<p>syncProgress specifies whether to sync the backup progress and its interval seconds.</p><br />
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
<p>BackupDeletionPolicy describes a policy for end-of-life maintenance of backup content.</p><br />
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
<p>BackupMethod defines the backup method.</p><br />
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
<p>the name of backup method.</p><br />
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
<p>snapshotVolumes specifies whether to take snapshots of persistent volumes.<br />if true, the BackupScript is not required, the controller will use the CSI<br />volume snapshotter to create the snapshot.</p><br />
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
<p>actionSetName refers to the ActionSet object that defines the backup actions.<br />For volume snapshot backup, the actionSet is not required, the controller<br />will use the CSI volume snapshotter to create the snapshot.</p><br />
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
<p>targetVolumes specifies which volumes from the target should be mounted in<br />the backup workload.</p><br />
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
<p>env specifies the environment variables for the backup workload.</p><br />
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
<p>runtimeSettings specifies runtime settings for the backup workload container.</p><br />
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
<p>target specifies the target information to back up, it will override the global target policy.</p><br />
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
<p>BackupPhase is a string representation of the lifecycle phase of a Backup.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Completed&#34;</p></td>
<td><p>BackupPhaseCompleted means the backup has run successfully without errors.</p><br /></td>
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td><p>BackupPhaseDeleting means the backup and all its associated data are being deleted.</p><br /></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupPhaseFailed means the backup ran but encountered an error that<br />prevented it from completing successfully.</p><br /></td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td><p>BackupPhaseNew means the backup has been created but not yet processed by<br />the BackupController.</p><br /></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>BackupPhaseRunning means the backup is currently executing.</p><br /></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyPhase">BackupPolicyPhase
(<code>string</code> alias)</h3>
<div>
<p>BackupPolicyPhase defines phases for BackupPolicy.</p><br />
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
<p>BackupPolicySpec defines the desired state of BackupPolicy</p><br />
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
<p>backupRepoName is the name of BackupRepo and the backup data will be<br />stored in this repository. If not set, will be stored in the default<br />backup repository.</p><br />
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
<p>pathPrefix is the directory inside the backup repository to store the backup content.<br />It is a relative to the path of the backup repository.</p><br />
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
<p>Specifies the number of retries before marking the backup failed.</p><br />
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
<p>target specifies the target information to back up.</p><br />
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
<p>backupMethods defines the backup methods.</p><br />
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
<p>useKopia specifies whether backup data should be stored in a Kopia repository.<br />Data within the Kopia repository is both compressed and encrypted. Furthermore,<br />data deduplication is implemented across various backups of the same cluster.<br />This approach significantly reduces the actual storage usage, particularly for<br />clusters with a low update frequency.<br />NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition,<br />otherwise the backup will not be processed.</p><br />
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
<p>BackupPolicyStatus defines the observed state of BackupPolicy</p><br />
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
<p>phase - in list of [Available,Unavailable]</p><br />
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
<p>A human-readable message indicating details about why the BackupPolicy is<br />in this phase.</p><br />
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
<p>observedGeneration is the most recent generation observed for this<br />BackupPolicy. It refers to the BackupPolicy&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
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
<p>BackupRef describes the backup name and namespace.</p><br />
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
<p>backup name</p><br />
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
<p>backup namespace</p><br />
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
<p>BackupRepoPhase defines phases for BackupRepo CR.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PreChecking&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupRepoSpec">BackupRepoSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo</a>)
</p>
<div>
<p>BackupRepoSpec defines the desired state of BackupRepo</p><br />
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
<p>The storage provider used by this backup repo.</p><br />
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
<p>Specifies the access method of the backup repo.</p><br />
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
<p>The requested capacity for the PVC created by this backup repo.</p><br />
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
<p>The reclaim policy for the PV created by this backup repo.</p><br />
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
<p>Non-secret configurations for the storage provider.</p><br />
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
<p>A secret that contains the credentials needed by the storage provider.</p><br />
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
<p>BackupRepoStatus defines the observed state of BackupRepo</p><br />
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
<p>Backup repo reconciliation phases. Valid values are PreChecking, Failed, Ready, Deleting.</p><br />
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
<p>conditions describes the current state of the repo.</p><br />
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
<p>observedGeneration is the latest generation observed by the controller.</p><br />
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
<p>generatedCSIDriverSecret references the generated secret used by the CSI driver.</p><br />
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
<p>generatedStorageClassName indicates the generated storage class name.</p><br />
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
<p>backupPVCName is the name of the PVC used to store backup data.</p><br />
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
<p>toolConfigSecretName is the name of the secret containing the configuration for the access tool.</p><br />
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
<p>isDefault indicates whether this backup repo is the default one.</p><br />
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
<p>BackupSchedulePhase defines the phase of BackupSchedule</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td><p>BackupSchedulePhaseAvailable means the backup schedule is available.</p><br /></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupSchedulePhaseFailed means the backup schedule is failed.</p><br /></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupScheduleSpec">BackupScheduleSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSchedule">BackupSchedule</a>)
</p>
<div>
<p>BackupScheduleSpec defines the desired state of BackupSchedule.</p><br />
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
<p>Which backupPolicy is applied to perform this backup.</p><br />
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the<br />backup workload if it misses scheduled time for any reason.</p><br />
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
<p>schedules defines the list of backup schedules.</p><br />
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
<p>BackupScheduleStatus defines the observed state of BackupSchedule.</p><br />
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
<p>phase describes the phase of the BackupSchedule.</p><br />
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
<p>observedGeneration is the most recent generation observed for this<br />BackupSchedule. It refers to the BackupSchedule&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
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
<p>failureReason is an error that caused the backup to fail.</p><br />
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
<p>schedules describes the status of each schedule.</p><br />
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
<p>BackupSpec defines the desired state of Backup.</p><br />
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
<p>Which backupPolicy is applied to perform this backup.</p><br />
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
<p>backupMethod specifies the backup method name that is defined in backupPolicy.</p><br />
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
<p>deletionPolicy determines whether the backup contents stored in backup repository<br />should be deleted when the backup custom resource is deleted.<br />Supported values are &ldquo;Retain&rdquo; and &ldquo;Delete&rdquo;.<br />&ldquo;Retain&rdquo; means that the backup can not be deleted and remains in &lsquo;Deleting&rsquo; phase.<br />&ldquo;Delete&rdquo; means that the backup content and its physical snapshot on backup repository are deleted.</p><br />
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
<p>retentionPeriod determines a duration up to which the backup should be kept.<br />Controller will remove all backups that are older than the RetentionPeriod.<br />For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.<br />Sample duration format:<br />- years: 	2y<br />- months: 	6mo<br />- days: 		30d<br />- hours: 	12h<br />- minutes: 	30m<br />You can also combine the above durations. For example: 30d12h30m.<br />If not set, the backup will be kept forever.</p><br />
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
<p>parentBackupName determines the parent backup name for incremental or<br />differential backup.</p><br />
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
<p>BackupStatus defines the observed state of Backup.</p><br />
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
<p>formatVersion is the backup format version, including major, minor and patch version.</p><br />
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
<p>phase is the current state of the Backup.</p><br />
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
<p>expiration is when this backup is eligible for garbage collection.<br />&lsquo;null&rsquo; means the Backup will NOT be cleaned except delete manual.</p><br />
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
<p>startTimestamp records the time a backup was started.<br />The server&rsquo;s time is used for StartTimestamp.</p><br />
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
<p>completionTimestamp records the time a backup was completed.<br />Completion time is recorded even on failed backups.<br />The server&rsquo;s time is used for CompletionTimestamp.</p><br />
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
<p>The duration time of backup execution.<br />When converted to a string, the format is &ldquo;1h2m0.5s&rdquo;.</p><br />
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
<p>totalSize is the total size of backed up data size.<br />A string with capacity units in the format of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.<br />If no capacity unit is specified, it is assumed to be in bytes.</p><br />
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
<p>failureReason is an error that caused the backup to fail.</p><br />
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
<p>backupRepoName is the name of the backup repository.</p><br />
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
<p>path is the directory inside the backup repository where the backup data is stored.<br />It is an absolute path in the backup repository.</p><br />
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
<p>kopiaRepoPath records the path of the Kopia repository.</p><br />
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
<p>persistentVolumeClaimName is the name of the persistent volume claim that<br />is used to store the backup data.</p><br />
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
<p>timeRange records the time range of backed up data, for PITR, this is the<br />time range of recoverable data.</p><br />
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
<p>target records the target information for this backup.</p><br />
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
<p>backupMethod records the backup method information for this backup.<br />Refer to BackupMethod for more details.</p><br />
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
<p>actions records the actions information for this backup.</p><br />
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
<p>volumeSnapshots records the volume snapshot status for the action.</p><br />
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
<p>extra records the extra info for the backup.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
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
<p>podSelector is used to find the target pod. The volumes of the target pod<br />will be backed up.</p><br />
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
<p>connectionCredential specifies the connection credential to connect to the<br />target database cluster.</p><br />
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
<p>resources specifies the kubernetes resources to back up.</p><br />
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
<p>serviceAccountName specifies the service account to run the backup workload.</p><br />
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
<p>BackupTimeRange records the time range of backed up data, for PITR, this is the<br />time range of recoverable data.</p><br />
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
<p>time zone, only support zone offset, value range: &ldquo;-12:59 ~ +13:00&rdquo;</p><br />
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
<p>start records the start time of backup(Coordinated Universal Time, UTC).</p><br />
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
<p>end records the end time of backup(Coordinated Universal Time, UTC).</p><br />
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
<p>BackupType the backup type.</p><br />
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
<p>BaseJobActionSpec is an action that creates a Kubernetes Job to execute a command.</p><br />
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
<p>image specifies the image of backup container.</p><br />
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
<p>command specifies the commands to back up the volume data.</p><br />
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
<p>ConnectionCredential specifies the connection credential to connect to the<br />target database cluster.</p><br />
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
<p>secretName refers to the Secret object that contains the connection credential.</p><br />
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
<p>usernameKey specifies the map key of the user in the connection credential secret.</p><br />
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
<p>passwordKey specifies the map key of the password in the connection credential secret.<br />This password will be saved in the backup annotation for full backup.<br />You can use the environment variable DP_ENCRYPTION_KEY to specify encryption key.</p><br />
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
<p>hostKey specifies the map key of the host in the connection credential secret.</p><br />
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
<p>portKey specifies the map key of the port in the connection credential secret.</p><br />
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
<p>execActionTarget defines the pods that need to be executed for the exec action.<br />will execute on all pods that meet the conditions.</p><br />
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
<p>ExecActionSpec is an action that uses the pod exec API to execute a command in a container<br />in a pod.</p><br />
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
<p>container is the container in the pod where the command should be executed.<br />If not specified, the pod&rsquo;s first container is used.</p><br />
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
<p>Command is the command and arguments to execute.</p><br />
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
<p>OnError specifies how should behave if it encounters an error executing this action.</p><br />
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
<p>Timeout defines the maximum amount of time should wait for the hook to complete before<br />considering the execution a failure.</p><br />
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
<p>kubectl exec in all selected pods.</p><br />
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
<p>select the specified resource for recovery by label.</p><br />
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
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.JobActionTarget">
JobActionTarget
</a>
</em>
</td>
<td>
<p>jobActionTarget defines the pod that need to be executed for the job action.<br /> will select a pod that meets the conditions to execute.</p><br />
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
<p>JobActionSpec is an action that creates a Kubernetes Job to execute a command.</p><br />
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
<p>runOnTargetPodNode specifies whether to run the job workload on the<br />target pod node. If backup container should mount the target pod&rsquo;s<br />volumes, this field should be set to true. otherwise the target pod&rsquo;s<br />volumes will be ignored.</p><br />
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
<p>OnError specifies how should behave if it encounters an error executing<br />this action.</p><br />
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>select one of the pods which selected by labels to build the job spec, such as mount required volumes and inject built-in env of the selected pod.</p><br />
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
<p>volumeMounts defines which volumes of the selected pod need to be mounted on the restoring pod.</p><br />
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
<p>KubeResources defines the kubernetes resources to back up.</p><br />
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
<p>selector is a metav1.LabelSelector to filter the target kubernetes resources<br />that need to be backed up.<br />If not set, will do not back up any kubernetes resources.</p><br />
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
<p>included is a slice of namespaced-scoped resource type names to include in<br />the kubernetes resources.<br />The default value is &ldquo;*&rdquo;, which means all resource types will be included.</p><br />
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
<p>excluded is a slice of namespaced-scoped resource type names to exclude in<br />the kubernetes resources.<br />The default value is empty.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetStatus">ActionSetStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">BackupPolicyStatus</a>)
</p>
<div>
<p>Phase defines the BackupPolicy and ActionSet CR .status.phase</p><br />
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
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>)
</p>
<div>
<p>PodSelectionStrategy specifies the strategy to select when multiple pods are<br />selected for backup target</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;All&#34;</p></td>
<td><p>PodSelectionStrategyAll selects all pods that match the labelsSelector.<br />TODO: support PodSelectionStrategyAll</p><br /></td>
</tr><tr><td><p>&#34;Any&#34;</p></td>
<td><p>PodSelectionStrategyAny selects any one pod that match the labelsSelector.</p><br /></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PodSelector">PodSelector
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">BackupTarget</a>)
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
<p>labelsSelector is the label selector to filter the target pods.</p><br />
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
<p>strategy specifies the strategy to select the target pod when multiple pods<br />are selected.<br />Valid values are:<br />- Any: select any one pod that match the labelsSelector.<br />- All: select all pods that match the labelsSelector.</p><br />
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
<code>dataSourceRef</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeConfig">
VolumeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>dataSourceRef describes the configuration when using <code>persistentVolumeClaim.spec.dataSourceRef</code> method for restoring.<br />it describes the source volume of the backup targetVolumes and how to mount path in the restoring container.</p><br />
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
<p>volumeClaims defines the persistent Volume claims that need to be restored and mount them together into the restore job.<br />these persistent Volume claims will be created if not exist.</p><br />
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
<p>volumeClaimsTemplate defines a template to build persistent Volume claims that need to be restored.<br />these claims will be created in an orderly manner based on the number of replicas or reused if already exist.</p><br />
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
<p>VolumeClaimRestorePolicy defines restore policy for persistent volume claim.<br />Supported policies are as follows:<br />1. Parallel: parallel recovery of persistent volume claim.<br />2. Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.</p><br />
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
<p>scheduling spec for restoring pod.</p><br />
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
<p>number of seconds after the container has started before probe is initiated.</p><br />
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
<p>number of seconds after which the probe times out.<br />defaults to 30 second, minimum value is 1.</p><br />
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
<p>how often (in seconds) to perform the probe.<br />defaults to 5 second, minimum value is 1.</p><br />
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
<p>exec specifies the action to take.</p><br />
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
<p>refer to container image.</p><br />
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
<p>refer to container command.</p><br />
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
<p>configuration for job action.</p><br />
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
<p>configuration for exec action.</p><br />
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
<p>credential template used for creating a connection credential</p><br />
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
<p>periodic probe of the service readiness.<br />controller will perform postReadyHooks of BackupScript.spec.restore after the service readiness when readinessProbe is configured.</p><br />
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
<p>RestoreActionSpec defines how to restore data.</p><br />
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
<p>prepareData specifies the action to prepare data.</p><br />
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
<p>postReady specifies the action to execute after the data is ready.</p><br />
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
<p>RestoreActionStatus the status of restore action.</p><br />
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
<p>will restore the specified resources</p><br />
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
<p>RestorePhase The current phase. Valid values are Running, Completed, Failed, AsDataSource.</p><br />
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
<p>RestoreSpec defines the desired state of Restore</p><br />
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
<p>backup to be restored. The restore behavior based on the backup type:<br />1. Full: will be restored the full backup directly.<br />2. Incremental: will be restored sequentially from the most recent full backup of this incremental backup.<br />3. Differential: will be restored sequentially from the parent backup of the differential backup.<br />4. Continuous: will find the most recent full backup at this time point and the continuous backups after it to restore.</p><br />
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
<p>restoreTime is the point in time for restoring.</p><br />
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
<p>restore the specified resources of kubernetes.</p><br />
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
<p>configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims<br />that need to be restored and scheduling strategy of temporary recovery pod.</p><br />
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
<p>service account name which needs for recovery pod.</p><br />
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
<p>configuration for the action of &ldquo;postReady&rdquo; phase.</p><br />
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
<p>list of environment variables to set in the container for restore and will be<br />merged with the env of Backup and ActionSet.<br />The priority of merging is as follows: Restore env &gt; Backup env &gt; ActionSet env.</p><br />
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
<p>specified the required resources of restore job&rsquo;s container.</p><br />
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
<p>Specifies the number of retries before marking the restore failed.</p><br />
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
<p>RestoreStatus defines the observed state of Restore</p><br />
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
<p>Date/time when the restore started being processed.</p><br />
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
<p>Date/time when the restore finished being processed.</p><br />
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
<p>The duration time of restore execution.<br />When converted to a string, the form is &ldquo;1h2m0.5s&rdquo;.</p><br />
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
<p>recorded all restore actions performed.</p><br />
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
<p>describe current state of restore API Resource, like warning.</p><br />
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
<p>name describes the name of the recovery action based on the current backup.</p><br />
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
<p>which backup&rsquo;s restore action belongs to.</p><br />
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
<p>the execution object of the restore action.</p><br />
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
<p>message is a human readable message indicating details about the object condition.</p><br />
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
<p>the status of this action.</p><br />
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
<p>startTime is the start time for the restore job.</p><br />
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
<p>endTime is the completion time for the restore job.</p><br />
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
<p>record the actions for prepareData phase.</p><br />
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
<p>record the actions for postReady phase.</p><br />
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
<p>Standard object&rsquo;s metadata.<br />More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata</a></p><br />
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
<p>volumeClaimSpec defines the desired characteristics of a persistent volume claim.</p><br />
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
<p>describing the source volume of the backup targetVolumes and how to mount path in the restoring container.</p><br />
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
<p>templates is a list of volume claims.</p><br />
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
<p>the replicas of persistent volume claim which need to be created and restored.<br />the format of created claim name is &ldquo;<template-name>-<index>&rdquo;.</p><br />
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
<p>the starting index for the created persistent volume claim by according to template.<br />minimum is 0.</p><br />
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
<p>RetentionPeriod represents a duration in the format &ldquo;1y2mo3w4d5h6m&rdquo;, where<br />y=year, mo=month, w=week, d=day, h=hour, m=minute.</p><br />
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RuntimeSettings">RuntimeSettings
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>)
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
<p>resources specifies the resource required by container.<br />More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/">https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/</a></p><br />
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
<p>SchedulePhase defines the phase of schedule</p><br />
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
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupScheduleSpec">BackupScheduleSpec</a>)
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
<p>enabled specifies whether the backup schedule is enabled or not.</p><br />
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
<p>backupMethod specifies the backup method name that is defined in backupPolicy.</p><br />
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
<p>the cron expression for schedule, the timezone is in UTC.<br />see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p><br />
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
<p>retentionPeriod determines a duration up to which the backup should be kept.<br />controller will remove all backups that are older than the RetentionPeriod.<br />For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.<br />Sample duration format:<br />- years: 	2y<br />- months: 	6mo<br />- days: 		30d<br />- hours: 	12h<br />- minutes: 	30m<br />You can also combine the above durations. For example: 30d12h30m</p><br />
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
<p>ScheduleStatus defines the status of each schedule.</p><br />
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
<p>phase describes the phase of the schedule.</p><br />
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
<p>failureReason is an error that caused the backup to fail.</p><br />
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
<p>lastScheduleTime records the last time the backup was scheduled.</p><br />
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
<p>lastSuccessfulTime records the last time the backup was successfully completed.</p><br />
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
<p>the restoring pod&rsquo;s tolerations.</p><br />
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
<p>nodeSelector is a selector which must be true for the pod to fit on a node.<br />Selector which must match a node&rsquo;s labels for the pod to be scheduled on that node.<br />More info: <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p><br />
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
<p>nodeName is a request to schedule this pod onto a specific node. If it is non-empty,<br />the scheduler simply schedules this pod onto that node, assuming that it fits resource<br />requirements.</p><br />
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
<p>affinity is a group of affinity scheduling rules.<br />refer to <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p><br />
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
<p>topologySpreadConstraints describes how a group of pods ought to spread across topology<br />domains. Scheduler will schedule pods in a way which abides by the constraints.<br />refer to <a href="https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/">https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/</a></p><br />
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
<p>If specified, the pod will be dispatched by specified scheduler.<br />If not specified, the pod will be dispatched by default scheduler.</p><br />
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
<p>enabled specifies whether to sync the backup progress. If enabled,<br />a sidecar container will be created to sync the backup progress to the<br />Backup CR status.</p><br />
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
<p>intervalSeconds specifies the interval seconds to sync the backup progress.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.TargetVolumeInfo">TargetVolumeInfo
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>)
</p>
<div>
<p>TargetVolumeInfo specifies the volumes and their mounts of the targeted application<br />that should be mounted in backup workload.</p><br />
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
<p>Volumes indicates the list of volumes of targeted application that should<br />be mounted on the backup job.</p><br />
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
<p>volumeMounts specifies the mount for the volumes specified in <code>Volumes</code> section.</p><br />
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
<p>VolumeClaimRestorePolicy defines restore policy for persistent volume claim.<br />Supported policies are as follows:<br />1. Parallel: parallel recovery of persistent volume claim.<br />2. Serial: restore the persistent volume claim in sequence, and wait until the<br />previous persistent volume claim is restored before restoring a new one.</p><br />
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
<p>volumeSource describes the volume will be restored from the specified volume of the backup targetVolumes.<br />required if the backup uses volume snapshot.</p><br />
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
<p>mountPath path within the restoring container at which the volume should be mounted.</p><br />
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
<p>name is the name of the volume snapshot.</p><br />
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
<p>contentName is the name of the volume snapshot content.</p><br />
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
<p>volumeName is the name of the volume.</p><br />
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
<p>size is the size of the volume snapshot.</p><br />
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
