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
<li>
<a href="#storage.kubeblocks.io%2fv1alpha1">storage.kubeblocks.io/v1alpha1</a>
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
<p>backupRepoName is the name of BackupRepo and the backup data will be
stored in this repository. If not set, will be stored in the default
backup repository.</p>
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
<p>pathPrefix is the directory inside the backup repository to store the backup content.
It is a relative to the path of the backup repository.</p>
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
<p>Specifies the number of retries before marking the backup failed.</p>
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
<p>target specifies the target information to back up.</p>
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
<p>backupMethods defines the backup methods.</p>
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
<p>useKopia specifies whether backup data should be stored in a Kopia repository.</p>
<p>Data within the Kopia repository is both compressed and encrypted. Furthermore,
data deduplication is implemented across various backups of the same cluster.
This approach significantly reduces the actual storage usage, particularly for
clusters with a low update frequency.</p>
<p>NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition,
otherwise the backup will not be processed.</p>
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
<p>Which backupPolicy is applied to perform this backup.</p>
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the
backup workload if it misses scheduled time for any reason.</p>
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
<p>schedules defines the list of backup schedules.</p>
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
<p>backup to be restored. The restore behavior based on the backup type:</p>
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
<p>restoreTime is the point in time for restoring.</p>
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
<p>restore the specified resources of kubernetes.</p>
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
<p>configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims
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
<p>service account name which needs for recovery pod.</p>
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
<p>configuration for the action of &ldquo;postReady&rdquo; phase.</p>
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
<p>list of environment variables to set in the container for restore and will be
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
<p>specified the required resources of restore job&rsquo;s container.</p>
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
<p>name is the name of the action.</p>
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
<p>phase is the current state of the action.</p>
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
<p>startTimestamp records the time an action was started.</p>
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
<p>completionTimestamp records the time an action was completed.</p>
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
<p>failureReason is an error that caused the backup to fail.</p>
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
<p>actionType is the type of the action.</p>
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
<p>availableReplicas available replicas for statefulSet action.</p>
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
<p>objectRef is the object reference for the action.</p>
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
<p>totalSize is the total size of backed up data size.
A string with capacity units in the format of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p>
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
<p>timeRange records the time range of backed up data, for PITR, this is the
time range of recoverable data.</p>
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
<p>volumeSnapshots records the volume snapshot status for the action.</p>
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
<p>BackupDeletionPolicy describes a policy for end-of-life maintenance of backup content.</p>
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
<p>the name of backup method.</p>
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
<p>snapshotVolumes specifies whether to take snapshots of persistent volumes.
if true, the BackupScript is not required, the controller will use the CSI
volume snapshotter to create the snapshot.</p>
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
<p>actionSetName refers to the ActionSet object that defines the backup actions.
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
<p>targetVolumes specifies which volumes from the target should be mounted in
the backup workload.</p>
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
<p>env specifies the environment variables for the backup workload.</p>
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
<p>runtimeSettings specifies runtime settings for the backup workload container.</p>
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
<p>target specifies the target information to back up, it will override the global target policy.</p>
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
<p>BackupPhase is a string representation of the lifecycle phase of a Backup.</p>
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
<p>backupRepoName is the name of BackupRepo and the backup data will be
stored in this repository. If not set, will be stored in the default
backup repository.</p>
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
<p>pathPrefix is the directory inside the backup repository to store the backup content.
It is a relative to the path of the backup repository.</p>
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
<p>Specifies the number of retries before marking the backup failed.</p>
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
<p>target specifies the target information to back up.</p>
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
<p>backupMethods defines the backup methods.</p>
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
<p>useKopia specifies whether backup data should be stored in a Kopia repository.</p>
<p>Data within the Kopia repository is both compressed and encrypted. Furthermore,
data deduplication is implemented across various backups of the same cluster.
This approach significantly reduces the actual storage usage, particularly for
clusters with a low update frequency.</p>
<p>NOTE: This feature should NOT be enabled when using KubeBlocks Community Edition,
otherwise the backup will not be processed.</p>
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
<p>phase - in list of [Available,Unavailable]</p>
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
<p>A human-readable message indicating details about why the BackupPolicy is
in this phase.</p>
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
<p>observedGeneration is the most recent generation observed for this
BackupPolicy. It refers to the BackupPolicy&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>BackupRef describes the backup name and namespace.</p>
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
<p>backup name</p>
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
<p>backup namespace</p>
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
<td><p>BackupSchedulePhaseAvailable means the backup schedule is available.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>BackupSchedulePhaseFailed means the backup schedule is failed.</p>
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
<p>Which backupPolicy is applied to perform this backup.</p>
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the
backup workload if it misses scheduled time for any reason.</p>
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
<p>schedules defines the list of backup schedules.</p>
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
<p>phase describes the phase of the BackupSchedule.</p>
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
<p>observedGeneration is the most recent generation observed for this
BackupSchedule. It refers to the BackupSchedule&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>failureReason is an error that caused the backup to fail.</p>
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
<p>schedules describes the status of each schedule.</p>
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
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTarget">
BackupTarget
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
<p>podSelector is used to find the target pod. The volumes of the target pod
will be backed up.</p>
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
<p>connectionCredential specifies the connection credential to connect to the
target database cluster.</p>
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
<p>resources specifies the kubernetes resources to back up.</p>
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
<p>serviceAccountName specifies the service account to run the backup workload.</p>
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
<p>time zone, only support zone offset, value range: &ldquo;-12:59 ~ +13:00&rdquo;</p>
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
<p>start records the start time of backup(Coordinated Universal Time, UTC).</p>
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
<p>end records the end time of backup(Coordinated Universal Time, UTC).</p>
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
<p>secretName refers to the Secret object that contains the connection credential.</p>
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
<p>usernameKey specifies the map key of the user in the connection credential secret.</p>
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
<p>passwordKey specifies the map key of the password in the connection credential secret.
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
<p>hostKey specifies the map key of the host in the connection credential secret.</p>
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
<p>portKey specifies the map key of the port in the connection credential secret.</p>
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
<p>execActionTarget defines the pods that need to be executed for the exec action.
will execute on all pods that meet the conditions.</p>
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
<p>kubectl exec in all selected pods.</p>
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
<p>select the specified resource for recovery by label.</p>
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
<p>jobActionTarget defines the pod that need to be executed for the job action.
will select a pod that meets the conditions to execute.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>select one of the pods which selected by labels to build the job spec, such as mount required volumes and inject built-in env of the selected pod.</p>
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
<p>volumeMounts defines which volumes of the selected pod need to be mounted on the restoring pod.</p>
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
<p>selector is a metav1.LabelSelector to filter the target kubernetes resources
that need to be backed up.
If not set, will do not back up any kubernetes resources.</p>
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
The default value is &ldquo;*&rdquo;, which means all resource types will be included.</p>
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ActionSetStatus">ActionSetStatus</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">BackupPolicyStatus</a>)
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
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>)
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
<p>labelsSelector is the label selector to filter the target pods.</p>
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
<p>strategy specifies the strategy to select the target pod when multiple pods
are selected.
Valid values are:</p>
<ul>
<li>Any: select any one pod that match the labelsSelector.</li>
<li>All: select all pods that match the labelsSelector.</li>
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
<code>dataSourceRef</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.VolumeConfig">
VolumeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>dataSourceRef describes the configuration when using <code>persistentVolumeClaim.spec.dataSourceRef</code> method for restoring.
it describes the source volume of the backup targetVolumes and how to mount path in the restoring container.</p>
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
<p>volumeClaims defines the persistent Volume claims that need to be restored and mount them together into the restore job.
these persistent Volume claims will be created if not exist.</p>
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
<p>volumeClaimsTemplate defines a template to build persistent Volume claims that need to be restored.
these claims will be created in an orderly manner based on the number of replicas or reused if already exist.</p>
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
<p>VolumeClaimRestorePolicy defines restore policy for persistent volume claim.
Supported policies are as follows:</p>
<ol>
<li>Parallel: parallel recovery of persistent volume claim.</li>
<li>Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.</li>
</ol>
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
<p>scheduling spec for restoring pod.</p>
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
<p>number of seconds after the container has started before probe is initiated.</p>
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
<p>number of seconds after which the probe times out.
defaults to 30 second, minimum value is 1.</p>
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
<p>how often (in seconds) to perform the probe.
defaults to 5 second, minimum value is 1.</p>
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
<p>exec specifies the action to take.</p>
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
<p>refer to container image.</p>
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
<p>refer to container command.</p>
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
<p>configuration for job action.</p>
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
<p>configuration for exec action.</p>
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
<p>credential template used for creating a connection credential</p>
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
<p>periodic probe of the service readiness.
controller will perform postReadyHooks of BackupScript.spec.restore after the service readiness when readinessProbe is configured.</p>
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
<p>will restore the specified resources</p>
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
<p>backup to be restored. The restore behavior based on the backup type:</p>
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
<p>restoreTime is the point in time for restoring.</p>
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
<p>restore the specified resources of kubernetes.</p>
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
<p>configuration for the action of &ldquo;prepareData&rdquo; phase, including the persistent volume claims
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
<p>service account name which needs for recovery pod.</p>
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
<p>configuration for the action of &ldquo;postReady&rdquo; phase.</p>
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
<p>list of environment variables to set in the container for restore and will be
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
<p>specified the required resources of restore job&rsquo;s container.</p>
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
<p>Date/time when the restore started being processed.</p>
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
<p>Date/time when the restore finished being processed.</p>
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
<p>The duration time of restore execution.
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
<p>recorded all restore actions performed.</p>
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
<p>describe current state of restore API Resource, like warning.</p>
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
<p>name describes the name of the recovery action based on the current backup.</p>
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
<p>which backup&rsquo;s restore action belongs to.</p>
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
<p>the execution object of the restore action.</p>
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
<p>message is a human-readable message indicating details about the object condition.</p>
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
<p>the status of this action.</p>
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
<p>startTime is the start time for the restore job.</p>
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
<p>endTime is the completion time for the restore job.</p>
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
<p>record the actions for prepareData phase.</p>
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
<p>record the actions for postReady phase.</p>
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
<p>Standard object&rsquo;s metadata.
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
<p>volumeClaimSpec defines the desired characteristics of a persistent volume claim.</p>
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
<p>describing the source volume of the backup targetVolumes and how to mount path in the restoring container.</p>
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
<p>templates is a list of volume claims.</p>
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
<p>the replicas of persistent volume claim which need to be created and restored.
the format of created claim name is <code>$(template-name)-$(index)</code>.</p>
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
<p>the starting index for the created persistent volume claim by according to template.
minimum is 0.</p>
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
<p>resources specifies the resource required by container.
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
<p>SchedulePhase defines the phase of schedule</p>
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
<p>enabled specifies whether the backup schedule is enabled or not.</p>
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
<p>backupMethod specifies the backup method name that is defined in backupPolicy.</p>
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
<p>the cron expression for schedule, the timezone is in UTC.
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
<p>retentionPeriod determines a duration up to which the backup should be kept.
controller will remove all backups that are older than the RetentionPeriod.
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
<p>ScheduleStatus defines the status of each schedule.</p>
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
<p>phase describes the phase of the schedule.</p>
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
<p>failureReason is an error that caused the backup to fail.</p>
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
<p>lastScheduleTime records the last time the backup was scheduled.</p>
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
<p>lastSuccessfulTime records the last time the backup was successfully completed.</p>
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
<p>the restoring pod&rsquo;s tolerations.</p>
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
<p>nodeSelector is a selector which must be true for the pod to fit on a node.
Selector which must match a node&rsquo;s labels for the pod to be scheduled on that node.
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
<p>nodeName is a request to schedule this pod onto a specific node. If it is non-empty,
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
<p>affinity is a group of affinity scheduling rules.
refer to <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p>
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
<p>topologySpreadConstraints describes how a group of pods ought to spread across topology
domains. Scheduler will schedule pods in a way which abides by the constraints.
refer to <a href="https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/">https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/</a></p>
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
<p>If specified, the pod will be dispatched by specified scheduler.
If not specified, the pod will be dispatched by default scheduler.</p>
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.TargetVolumeInfo">TargetVolumeInfo
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>)
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
<p>Volumes indicates the list of volumes of targeted application that should
be mounted on the backup job.</p>
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
<p>volumeMounts specifies the mount for the volumes specified in <code>Volumes</code> section.</p>
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
<p>volumeSource describes the volume will be restored from the specified volume of the backup targetVolumes.
required if the backup uses volume snapshot.</p>
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
<p>mountPath path within the restoring container at which the volume should be mounted.</p>
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
<p>name is the name of the volume snapshot.</p>
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
<p>contentName is the name of the volume snapshot content.</p>
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
<p>volumeName is the name of the volume.</p>
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
<p>size is the size of the volume snapshot.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="storage.kubeblocks.io/v1alpha1">storage.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#storage.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>
</li></ul>
<h3 id="storage.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider
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
<code>storage.kubeblocks.io/v1alpha1</code>
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
<a href="#storage.kubeblocks.io/v1alpha1.StorageProviderSpec">
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
<a href="#storage.kubeblocks.io/v1alpha1.ParametersSchema">
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
<a href="#storage.kubeblocks.io/v1alpha1.StorageProviderStatus">
StorageProviderStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="storage.kubeblocks.io/v1alpha1.ParametersSchema">ParametersSchema
</h3>
<p>
(<em>Appears on:</em><a href="#storage.kubeblocks.io/v1alpha1.StorageProviderSpec">StorageProviderSpec</a>)
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
<h3 id="storage.kubeblocks.io/v1alpha1.StorageProviderPhase">StorageProviderPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#storage.kubeblocks.io/v1alpha1.StorageProviderStatus">StorageProviderStatus</a>)
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
<h3 id="storage.kubeblocks.io/v1alpha1.StorageProviderSpec">StorageProviderSpec
</h3>
<p>
(<em>Appears on:</em><a href="#storage.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>)
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
<a href="#storage.kubeblocks.io/v1alpha1.ParametersSchema">
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
<h3 id="storage.kubeblocks.io/v1alpha1.StorageProviderStatus">StorageProviderStatus
</h3>
<p>
(<em>Appears on:</em><a href="#storage.kubeblocks.io/v1alpha1.StorageProvider">StorageProvider</a>)
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
<a href="#storage.kubeblocks.io/v1alpha1.StorageProviderPhase">
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
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
