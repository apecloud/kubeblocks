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
<a href="#dataprotection.kubeblocks.io/v1alpha1.Backup">Backup</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupRepo">BackupRepo</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJob">RestoreJob</a>
</li></ul>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Backup">Backup
</h3>
<div>
<p>Backup is the Schema for the backups API (defined by User).</p><br />
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
<p>Which backupPolicy is applied to perform this backup</p><br />
</td>
</tr>
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
<p>Backup Type. datafile or logfile or snapshot. If not set, datafile is the default type.</p><br />
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
<p>if backupType is incremental, parentBackupName is required.</p><br />
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
<p>BackupPolicy is the Schema for the backuppolicies API (defined by User)</p><br />
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
<code>retention</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionSpec">
RetentionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>retention describe how long the Backup should be retained. if not set, will be retained forever.</p><br />
</td>
</tr>
<tr>
<td>
<code>schedule</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Schedule">
Schedule
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>snapshot</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">
SnapshotPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for snapshot backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>datafile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">
CommonBackupPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for datafile backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>logfile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">
CommonBackupPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for logfile backup.</p><br />
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool
</h3>
<div>
<p>BackupTool is the Schema for the backuptools API (defined by provider)</p><br />
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
<td><code>BackupTool</code></td>
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
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">
BackupToolSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<p>Backup tool Container image name.</p><br />
</td>
</tr>
<tr>
<td>
<code>deployKind</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.DeployKind">
DeployKind
</a>
</em>
</td>
<td>
<p>which kind for run a backup tool, supported values: job, statefulSet.</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>the type of backup tool, file or pitr</p><br />
</td>
</tr>
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
<p>Compute Resources required by this container.<br />Cannot be updated.</p><br />
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
<code>backupCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Array of command that apps can do database backup.<br />from invoke args<br />the order of commands follows the order of array.</p><br />
</td>
</tr>
<tr>
<td>
<code>incrementalBackupCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Array of command that apps can do database incremental backup.<br />like xtrabackup, that can performs an incremental backup file.</p><br />
</td>
</tr>
<tr>
<td>
<code>physical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PhysicalConfig">
PhysicalConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support physical restore, in this case, restore must be RESTART database.</p><br />
</td>
</tr>
<tr>
<td>
<code>logical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.LogicalConfig">
LogicalConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support logical restore, in this case, restore NOT RESTART database.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolStatus">
BackupToolStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreJob">RestoreJob
</h3>
<div>
<p>RestoreJob is the Schema for the restorejobs API (defined by User)</p><br />
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
<td><code>RestoreJob</code></td>
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
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobSpec">
RestoreJobSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>backupJobName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specified one backupJob to restore.</p><br />
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetCluster">
TargetCluster
</a>
</em>
</td>
<td>
<p>the target database workload to restore</p><br />
</td>
</tr>
<tr>
<td>
<code>targetVolumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>array of restore volumes .</p><br />
</td>
</tr>
<tr>
<td>
<code>targetVolumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>array of restore volume mounts .</p><br />
</td>
</tr>
<tr>
<td>
<code>onFailAttempted</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>count of backup stop retries on fail.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobStatus">
RestoreJobStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupLogStatus">BackupLogStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ManifestsStatus">ManifestsStatus</a>)
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
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>startTime records the start time of data logging.</p><br />
</td>
</tr>
<tr>
<td>
<code>stopTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>stopTime records the stop time of data logging.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod
(<code>string</code> alias)</h3>
<div>
<p>BackupMethod the backup method</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;backupTool&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;snapshot&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPhase">BackupPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatus">BackupStatus</a>)
</p>
<div>
<p>BackupPhase The current phase. Valid values are New, InProgress, Completed, Failed.</p><br />
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
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;InProgress&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyHook">BackupPolicyHook
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy</a>)
</p>
<div>
<p>BackupPolicyHook defines for the database execute commands before and after backup.</p><br />
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
<code>preCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>pre backup to perform commands</p><br />
</td>
</tr>
<tr>
<td>
<code>postCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>post backup to perform commands</p><br />
</td>
</tr>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>exec command with image</p><br />
</td>
</tr>
<tr>
<td>
<code>containerName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>which container can exec command</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyPhase">BackupPolicyPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyStatus">BackupPolicyStatus</a>)
</p>
<div>
<p>BackupPolicyPhase defines phases for BackupPolicy CR.</p><br />
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicySecret">BackupPolicySecret
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.TargetCluster">TargetCluster</a>)
</p>
<div>
<p>BackupPolicySecret defines for the target database secret that backup tool can connect.</p><br />
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
<p>the secret name</p><br />
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
<p>usernameKey the map key of the user in the connection credential secret</p><br />
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
<p>passwordKey the map key of the password in the connection credential secret</p><br />
</td>
</tr>
</tbody>
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
<code>retention</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionSpec">
RetentionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>retention describe how long the Backup should be retained. if not set, will be retained forever.</p><br />
</td>
</tr>
<tr>
<td>
<code>schedule</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Schedule">
Schedule
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>snapshot</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">
SnapshotPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for snapshot backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>datafile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">
CommonBackupPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for datafile backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>logfile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">
CommonBackupPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the policy for logfile backup.</p><br />
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
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>observedGeneration is the most recent generation observed for this<br />BackupPolicy. It corresponds to the Cluster&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyPhase">
BackupPolicyPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup policy phase valid value: Available, Failed.</p><br />
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
<p>the reason if backup policy check failed.</p><br />
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
<p>information when was the last time the job was successfully scheduled.</p><br />
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
<p>information when was the last time the job successfully completed.</p><br />
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupSnapshotStatus">BackupSnapshotStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ManifestsStatus">ManifestsStatus</a>)
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
<code>volumeSnapshotName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeSnapshotName records the volumeSnapshot name.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeSnapshotContentName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeSnapshotContentName specifies the name of a pre-existing VolumeSnapshotContent<br />object representing an existing volume snapshot.<br />This field should be set if the snapshot already exists and only needs a representation in Kubernetes.<br />This field is immutable.</p><br />
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
<p>Which backupPolicy is applied to perform this backup</p><br />
</td>
</tr>
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
<p>Backup Type. datafile or logfile or snapshot. If not set, datafile is the default type.</p><br />
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
<p>if backupType is incremental, parentBackupName is required.</p><br />
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
<code>phase</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPhase">
BackupPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>Records parentBackupName if backupType is incremental.</p><br />
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
<p>The date and time when the Backup is eligible for garbage collection.<br />&lsquo;null&rsquo; means the Backup is NOT be cleaned except delete manual.</p><br />
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
<p>Date/time when the backup started being processed.</p><br />
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
<p>Date/time when the backup finished being processed.</p><br />
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
<p>The duration time of backup execution.<br />When converted to a string, the form is &ldquo;1h2m0.5s&rdquo;.</p><br />
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
<p>Backup total size.<br />A string with capacity units in the form of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p><br />
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
<p>The reason for a backup failure.</p><br />
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
<p>remoteVolume saves the backup data.</p><br />
</td>
</tr>
<tr>
<td>
<code>logFilePersistentVolumeClaimName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>logFilePersistentVolumeClaimName saves the logfile backup data.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupToolName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>backupToolName references the backup tool name.</p><br />
</td>
</tr>
<tr>
<td>
<code>sourceCluster</code><br/>
<em>
string
</em>
</td>
<td>
<p>sourceCluster records the source cluster information for this backup.</p><br />
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
<p>availableReplicas available replicas for statefulSet which created by backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>manifests</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.ManifestsStatus">
ManifestsStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>manifests determines the backup metadata info.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupStatusUpdate">BackupStatusUpdate
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy</a>)
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
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>specify the json path of backup object for patch.<br />example: manifests.backupLog &ndash; means patch the backup json path of status.manifests.backupLog.</p><br />
</td>
</tr>
<tr>
<td>
<code>containerName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>which container name that kubectl can execute.</p><br />
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>the shell Script commands to collect backup status metadata.<br />The script must exist in the container of ContainerName and the output format must be set to JSON.<br />Note that outputting to stderr may cause the result format to not be in JSON.</p><br />
</td>
</tr>
<tr>
<td>
<code>useTargetPodServiceAccount</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>useTargetPodServiceAccount defines whether this job requires the service account of the backup target pod.<br />if true, will use the service account of the backup target pod. otherwise, will use the system service account.</p><br />
</td>
</tr>
<tr>
<td>
<code>updateStage</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusUpdateStage">
BackupStatusUpdateStage
</a>
</em>
</td>
<td>
<p>when to update the backup status, pre: before backup, post: after backup</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupStatusUpdateStage">BackupStatusUpdateStage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusUpdate">BackupStatusUpdate</a>)
</p>
<div>
<p>BackupStatusUpdateStage defines the stage of backup status update.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;post&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;pre&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupToolManifestsStatus">BackupToolManifestsStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.ManifestsStatus">ManifestsStatus</a>)
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
<code>filePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>filePath records the file path of backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>logFilePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>logFilePath records the log file path of backup.</p><br />
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
<p>volumeName records volume name of backup data pvc.</p><br />
</td>
</tr>
<tr>
<td>
<code>uploadTotalSize</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup upload total size.<br />A string with capacity units in the form of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p><br />
</td>
</tr>
<tr>
<td>
<code>checksum</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>checksum of backup file, generated by md5 or sha1 or sha256.</p><br />
</td>
</tr>
<tr>
<td>
<code>checkpoint</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup checkpoint, for incremental backup.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">BackupToolRestoreCommand
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.LogicalConfig">LogicalConfig</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.PhysicalConfig">PhysicalConfig</a>)
</p>
<div>
<p>BackupToolRestoreCommand defines the restore commands of BackupTool</p><br />
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
<code>restoreCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Array of command that apps can perform database restore.<br />like xtrabackup, that can performs restore mysql from files.</p><br />
</td>
</tr>
<tr>
<td>
<code>incrementalRestoreCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Array of incremental restore commands.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">BackupToolSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool</a>)
</p>
<div>
<p>BackupToolSpec defines the desired state of BackupTool</p><br />
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
<p>Backup tool Container image name.</p><br />
</td>
</tr>
<tr>
<td>
<code>deployKind</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.DeployKind">
DeployKind
</a>
</em>
</td>
<td>
<p>which kind for run a backup tool, supported values: job, statefulSet.</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<p>the type of backup tool, file or pitr</p><br />
</td>
</tr>
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
<p>Compute Resources required by this container.<br />Cannot be updated.</p><br />
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
<code>backupCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Array of command that apps can do database backup.<br />from invoke args<br />the order of commands follows the order of array.</p><br />
</td>
</tr>
<tr>
<td>
<code>incrementalBackupCommands</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Array of command that apps can do database incremental backup.<br />like xtrabackup, that can performs an incremental backup file.</p><br />
</td>
</tr>
<tr>
<td>
<code>physical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PhysicalConfig">
PhysicalConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support physical restore, in this case, restore must be RESTART database.</p><br />
</td>
</tr>
<tr>
<td>
<code>logical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.LogicalConfig">
LogicalConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support logical restore, in this case, restore NOT RESTART database.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupToolStatus">BackupToolStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool</a>)
</p>
<div>
<p>BackupToolStatus defines the observed state of BackupTool</p><br />
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupType">BackupType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec</a>)
</p>
<div>
<p>BackupType the backup type, marked backup set is datafile or logfile or snapshot.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;datafile&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;logfile&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;snapshot&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BaseBackupType">BaseBackupType
(<code>string</code> alias)</h3>
<div>
<p>BaseBackupType the base backup type.</p><br />
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">CommonBackupPolicy</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy</a>)
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
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetCluster">
TargetCluster
</a>
</em>
</td>
<td>
<p>target database cluster for backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupsHistoryLimit</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>the number of automatic backups to retain. Value must be non-negative integer.<br />0 means NO limit on the number of backups.</p><br />
</td>
</tr>
<tr>
<td>
<code>onFailAttempted</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>count of backup stop retries on fail.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupStatusUpdates</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupStatusUpdate">
[]BackupStatusUpdate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>define how to update metadata for backup status.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">CommonBackupPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>)
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
<code>BasePolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BasePolicy">
BasePolicy
</a>
</em>
</td>
<td>
<p>
(Members of <code>BasePolicy</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeClaim</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeClaim">
PersistentVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>refer to PersistentVolumeClaim and the backup data will be stored in the corresponding persistent volume.</p><br />
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
<p>refer to BackupRepo and the backup data will be stored in the corresponding repo.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupToolName</code><br/>
<em>
string
</em>
</td>
<td>
<p>which backup tool to perform database backup, only support one tool.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.CreatePVCPolicy">CreatePVCPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeClaim">PersistentVolumeClaim</a>)
</p>
<div>
<p>CreatePVCPolicy the policy how to create the PersistentVolumeClaim for backup.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;IfNotPresent&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Never&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.DeployKind">DeployKind
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">BackupToolSpec</a>)
</p>
<div>
<p>DeployKind which kind for run a backup tool.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;job&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;statefulSet&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.LogicalConfig">LogicalConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">BackupToolSpec</a>)
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
<code>BackupToolRestoreCommand</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<p>
(Members of <code>BackupToolRestoreCommand</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>podScope</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PodRestoreScope">
PodRestoreScope
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>podScope defines the pod scope for restore from backup, supported values:<br />- &lsquo;All&rsquo; will exec the restore command on all pods.<br />- &lsquo;ReadWrite&rsquo; will pick a ReadWrite pod to exec the restore command.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.ManifestsStatus">ManifestsStatus
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
<code>backupLog</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupLogStatus">
BackupLogStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backupLog records startTime and stopTime of data logging.</p><br />
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>target records the target cluster metadata string, which is in JSON format.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupSnapshot</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSnapshotStatus">
BackupSnapshotStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>snapshot records the volume snapshot metadata.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupTool</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolManifestsStatus">
BackupToolManifestsStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backupTool records information about backup files generated by the backup tool.</p><br />
</td>
</tr>
<tr>
<td>
<code>userContext</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>userContext stores some loosely structured and extensible information.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeClaim">PersistentVolumeClaim
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.CommonBackupPolicy">CommonBackupPolicy</a>)
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
<p>the name of PersistentVolumeClaim to store backup data.</p><br />
</td>
</tr>
<tr>
<td>
<code>storageClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>storageClassName is the name of the StorageClass required by the claim.</p><br />
</td>
</tr>
<tr>
<td>
<code>initCapacity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>initCapacity represents the init storage size of the PersistentVolumeClaim which should be created if not exist.<br />and the default value is 100Gi if it is empty.</p><br />
</td>
</tr>
<tr>
<td>
<code>createPolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.CreatePVCPolicy">
CreatePVCPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>createPolicy defines the policy for creating the PersistentVolumeClaim, enum values:<br />- Never: do nothing if the PersistentVolumeClaim not exists.<br />- IfNotPresent: create the PersistentVolumeClaim if not present and the accessModes only contains &lsquo;ReadWriteMany&rsquo;.</p><br />
</td>
</tr>
<tr>
<td>
<code>persistentVolumeConfigMap</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeConfigMap">
PersistentVolumeConfigMap
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>persistentVolumeConfigMap references the configmap which contains a persistentVolume template.<br />key must be &ldquo;persistentVolume&rdquo; and value is the &ldquo;PersistentVolume&rdquo; struct.<br />support the following built-in Objects:<br />- $(GENERATE_NAME): generate a specific format &ldquo;<code>PVC NAME</code>-<code>PVC NAMESPACE</code>&rdquo;.<br />if the PersistentVolumeClaim not exists and CreatePolicy is &ldquo;IfNotPresent&rdquo;, the controller<br />will create it by this template. this is a mutually exclusive setting with &ldquo;storageClassName&rdquo;.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeConfigMap">PersistentVolumeConfigMap
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.PersistentVolumeClaim">PersistentVolumeClaim</a>)
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
<p>the name of the persistentVolume ConfigMap.</p><br />
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
<p>the namespace of the persistentVolume ConfigMap.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PhysicalConfig">PhysicalConfig
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">BackupToolSpec</a>)
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
<code>BackupToolRestoreCommand</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<p>
(Members of <code>BackupToolRestoreCommand</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>relyOnLogfile</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>relyOnLogfile defines whether the current recovery relies on log files</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.PodRestoreScope">PodRestoreScope
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.LogicalConfig">LogicalConfig</a>)
</p>
<div>
<p>PodRestoreScope defines the scope pod for restore from backup.</p><br />
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreJobPhase">RestoreJobPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobStatus">RestoreJobStatus</a>)
</p>
<div>
<p>RestoreJobPhase The current phase. Valid values are New, InProgressPhy, InProgressLogic, Completed, Failed.</p><br />
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
</tr><tr><td><p>&#34;InProgressLogic&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;InProgressPhy&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreJobSpec">RestoreJobSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJob">RestoreJob</a>)
</p>
<div>
<p>RestoreJobSpec defines the desired state of RestoreJob</p><br />
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
<code>backupJobName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specified one backupJob to restore.</p><br />
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.TargetCluster">
TargetCluster
</a>
</em>
</td>
<td>
<p>the target database workload to restore</p><br />
</td>
</tr>
<tr>
<td>
<code>targetVolumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>array of restore volumes .</p><br />
</td>
</tr>
<tr>
<td>
<code>targetVolumeMounts</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>array of restore volume mounts .</p><br />
</td>
</tr>
<tr>
<td>
<code>onFailAttempted</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>count of backup stop retries on fail.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreJobStatus">RestoreJobStatus
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJob">RestoreJob</a>)
</p>
<div>
<p>RestoreJobStatus defines the observed state of RestoreJob</p><br />
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
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobPhase">
RestoreJobPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>The date and time when the Backup is eligible for garbage collection.<br />&lsquo;null&rsquo; means the Backup is NOT be cleaned except delete manual.</p><br />
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
<p>Date/time when the backup started being processed.</p><br />
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
<p>Date/time when the backup finished being processed.</p><br />
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
<p>Job failed reason.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RetentionSpec">RetentionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>)
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
<code>ttl</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ttl is a time string ending with the &rsquo;d&rsquo;|&rsquo;D&rsquo;|&lsquo;h&rsquo;|&lsquo;H&rsquo; character to describe how long<br />the Backup should be retained. if not set, will be retained forever.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Schedule">Schedule
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>)
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
<code>startingDeadlineMinutes</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>startingDeadlineMinutes defines the deadline in minutes for starting the backup job<br />if it misses scheduled time for any reason.</p><br />
</td>
</tr>
<tr>
<td>
<code>snapshot</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for snapshot backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>datafile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for datafile backup.</p><br />
</td>
</tr>
<tr>
<td>
<code>logfile</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for logfile backup.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">SchedulePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.Schedule">Schedule</a>)
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
<code>cronExpression</code><br/>
<em>
string
</em>
</td>
<td>
<p>the cron expression for schedule, the timezone is in UTC. see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p><br />
</td>
</tr>
<tr>
<td>
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
<p>enable or disable the schedule.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySpec">BackupPolicySpec</a>)
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
<code>BasePolicy</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BasePolicy">
BasePolicy
</a>
</em>
</td>
<td>
<p>
(Members of <code>BasePolicy</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>hooks</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicyHook">
BackupPolicyHook
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>execute hook commands for backup.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.TargetCluster">TargetCluster
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy</a>, <a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobSpec">RestoreJobSpec</a>)
</p>
<div>
<p>TargetCluster TODO (dsj): target cluster need redefined from Cluster API</p><br />
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
<code>labelsSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>labelsSelector is used to find matching pods.<br />Pods that match this label selector are counted to determine the number of pods<br />in their corresponding topology domain.</p><br />
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicySecret">
BackupPolicySecret
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secret is used to connect to the target database cluster.<br />If not set, secret will be inherited from backup policy template.<br />if still not set, the controller will check if any system account for dataprotection has been created.</p><br />
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
