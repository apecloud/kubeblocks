<p>Packages:</p>
<ul>
<li>
<a href="#dataprotection.kubeblocks.io%2fv1alpha1">dataprotection.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="dataprotection.kubeblocks.io/v1alpha1">dataprotection.kubeblocks.io/v1alpha1</h2>
Resource Types:
<ul><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.Backup">Backup</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool</a>
</li><li>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJob">RestoreJob</a>
</li></ul>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.Backup">Backup
</h3>
<div>
<p>Backup is the Schema for the backups API (defined by User)</p>
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
<p>which backupPolicy to perform this backup</p>
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
<p>Backup Type. datafile or logfile or snapshot. if unset, default is datafile.</p>
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
<p>if backupType is incremental, parentBackupName is required.</p>
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
<p>BackupPolicy is the Schema for the backuppolicies API (defined by User)</p>
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
<p>retention describe how long the Backup should be retained. if not set, will be retained forever.</p>
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
<p>schedule policy for backup.</p>
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
<p>the policy for snapshot backup.</p>
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
<p>the policy for datafile backup.</p>
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
<p>the policy for logfile backup.</p>
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
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupTool">BackupTool
</h3>
<div>
<p>BackupTool is the Schema for the backuptools API (defined by provider)</p>
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
<p>Backup tool Container image name.</p>
</td>
</tr>
<tr>
<td>
<code>deployKind</code><br/>
<em>
string
</em>
</td>
<td>
<p>which kind for run a backup tool.</p>
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
<p>the type of backup tool, file or pitr</p>
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
<p>Compute Resources required by this container.
Cannot be updated.</p>
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
<p>List of environment variables to set in the container.</p>
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
<p>List of sources to populate environment variables in the container.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the container is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
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
<p>Array of command that apps can do database backup.
from invoke args
the order of commands follows the order of array.</p>
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
<p>Array of command that apps can do database incremental backup.
like xtrabackup, that can performs an incremental backup file.</p>
</td>
</tr>
<tr>
<td>
<code>physical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<p>backup tool can support physical restore, in this case, restore must be RESTART database.</p>
</td>
</tr>
<tr>
<td>
<code>logical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support logical restore, in this case, restore NOT RESTART database.</p>
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
<p>RestoreJob is the Schema for the restorejobs API (defined by User)</p>
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
<p>Specified one backupJob to restore.</p>
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
<p>the target database workload to restore</p>
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
<p>array of restore volumes .</p>
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
<p>array of restore volume mounts .</p>
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
<p>count of backup stop retries on fail.</p>
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
<p>startTime record start time of data logging</p>
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
<p>stopTime record start time of data logging</p>
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
<p>BackupPhase The current phase. Valid values are New, InProgress, Completed, Failed.</p>
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
</tr><tr><td><p>&#34;InProgress&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;New&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupPolicyHook">BackupPolicyHook
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy</a>)
</p>
<div>
<p>BackupPolicyHook defines for the database execute commands before and after backup.</p>
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
<p>pre backup to perform commands</p>
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
<p>post backup to perform commands</p>
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
<p>exec command with image</p>
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
<p>which container can exec command</p>
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
<p>BackupPolicyPhase defines phases for BackupPolicy CR.</p>
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
<p>BackupPolicySecret defines for the target database secret that backup tool can connect.</p>
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
<p>the secret name</p>
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
<p>usernameKey the map key of the user in the connection credential secret</p>
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
<p>passwordKey the map key of the password in the connection credential secret</p>
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
<code>retention</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.RetentionSpec">
RetentionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>retention describe how long the Backup should be retained. if not set, will be retained forever.</p>
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
<p>schedule policy for backup.</p>
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
<p>the policy for snapshot backup.</p>
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
<p>the policy for datafile backup.</p>
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
<p>the policy for logfile backup.</p>
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
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>observedGeneration is the most recent generation observed for this
BackupPolicy. It corresponds to the Cluster&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>backup policy phase valid value: Available, Failed.</p>
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
<p>the reason if backup policy check failed.</p>
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
<p>information when was the last time the job was successfully scheduled.</p>
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
<p>information when was the last time the job successfully completed.</p>
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
<p>volumeSnapshotName record the volumeSnapshot name</p>
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
<p>volumeSnapshotContentName specifies the name of a pre-existing VolumeSnapshotContent
object representing an existing volume snapshot.
This field should be set if the snapshot already exists and only needs a representation in Kubernetes.
This field is immutable.</p>
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
<p>BackupSpec defines the desired state of Backup</p>
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
<p>which backupPolicy to perform this backup</p>
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
<p>Backup Type. datafile or logfile or snapshot. if unset, default is datafile.</p>
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
<p>if backupType is incremental, parentBackupName is required.</p>
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
<p>BackupStatus defines the observed state of Backup</p>
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
<p>record parentBackupName if backupType is incremental.</p>
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
<p>The date and time when the Backup is eligible for garbage collection.
&lsquo;null&rsquo; means the Backup is NOT be cleaned except delete manual.</p>
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
<p>Date/time when the backup started being processed.</p>
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
<p>Date/time when the backup finished being processed.</p>
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
<p>The duration time of backup execution.
When converted to a string, the form is &ldquo;1h2m0.5s&rdquo;.</p>
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
<p>backup total size
string with capacity units in the form of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p>
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
<p>the reason if backup failed.</p>
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
<p>remoteVolume saves the backup data.</p>
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
<p>backupToolName referenced backup tool name.</p>
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
<p>manifests determines the backup metadata info</p>
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
<p>specify the json path of backup object for patch.
example: manifests.backupLog &ndash; means patch the backup json path of status.manifests.backupLog.</p>
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
<p>which container name that kubectl can execute.</p>
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
<p>the shell Script commands to collect backup status metadata.
The script must exist in the container of ContainerName and the output format must be set to JSON.
Note that outputting to stderr may cause the result format to not be in JSON.</p>
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
<em>(Optional)</em>
<p>when to update the backup status, pre: before backup, post: after backup</p>
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
<p>BackupStatusUpdateStage defines the stage of backup status update.</p>
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
<p>filePath records the file path of backup.</p>
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
<p>backup upload total size
string with capacity units in the form of &ldquo;1Gi&rdquo;, &ldquo;1Mi&rdquo;, &ldquo;1Ki&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>checkSum</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>checksum of backup file, generated by md5 or sha1 or sha256</p>
</td>
</tr>
<tr>
<td>
<code>CheckPoint</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup check point, for incremental backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">BackupToolRestoreCommand
</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolSpec">BackupToolSpec</a>)
</p>
<div>
<p>BackupToolRestoreCommand defines the restore commands of BackupTool</p>
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
<p>Array of command that apps can perform database restore.
like xtrabackup, that can performs restore mysql from files.</p>
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
<p>Array of incremental restore commands.</p>
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
<p>BackupToolSpec defines the desired state of BackupTool</p>
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
<p>Backup tool Container image name.</p>
</td>
</tr>
<tr>
<td>
<code>deployKind</code><br/>
<em>
string
</em>
</td>
<td>
<p>which kind for run a backup tool.</p>
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
<p>the type of backup tool, file or pitr</p>
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
<p>Compute Resources required by this container.
Cannot be updated.</p>
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
<p>List of environment variables to set in the container.</p>
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
<p>List of sources to populate environment variables in the container.
The keys defined within a source must be a C_IDENTIFIER. All invalid keys
will be reported as an event when the container is starting. When a key exists in multiple
sources, the value associated with the last source will take precedence.
Values defined by an Env with a duplicate key will take precedence.
Cannot be updated.</p>
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
<p>Array of command that apps can do database backup.
from invoke args
the order of commands follows the order of array.</p>
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
<p>Array of command that apps can do database incremental backup.
like xtrabackup, that can performs an incremental backup file.</p>
</td>
</tr>
<tr>
<td>
<code>physical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<p>backup tool can support physical restore, in this case, restore must be RESTART database.</p>
</td>
</tr>
<tr>
<td>
<code>logical</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.BackupToolRestoreCommand">
BackupToolRestoreCommand
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup tool can support logical restore, in this case, restore NOT RESTART database.</p>
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
<p>BackupToolStatus defines the observed state of BackupTool</p>
</div>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.BackupType">BackupType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec</a>)
</p>
<div>
<p>BackupType the backup type, marked backup set is datafile or logfile or snapshot.</p>
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
<p>BaseBackupType the base backup type.</p>
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
<p>target database cluster for backup.</p>
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
<p>the number of automatic backups to retain. Value must be non-negative integer.
0 means NO limit on the number of backups.</p>
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
<p>count of backup stop retries on fail.</p>
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
<p>define how to update metadata for backup status.</p>
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
<p>refer to PersistentVolumeClaim and the backup data will be stored in the corresponding persistent volume.</p>
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
<p>which backup tool to perform database backup, only support one tool.</p>
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
<p>CreatePVCPolicy the policy how to create the PersistentVolumeClaim for backup.</p>
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
<p>backupLog records startTime and stopTime of data logging</p>
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
<p>target records the target cluster metadata string, which are in JSON format.</p>
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
<p>snapshot records the volume snapshot metadata</p>
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
<p>backupTool records information about backup files generated by the backup tool.</p>
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
<p>userContext stores some loosely structured and extensible information.</p>
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
<p>the name of the PersistentVolumeClaim.</p>
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
<p>storageClassName is the name of the StorageClass required by the claim.</p>
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
<p>initCapacity represents the init storage size of the PersistentVolumeClaim which should be created if not exist.
and the default value is 100Gi if it is empty.</p>
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
<p>createPolicy defines the policy for creating the PersistentVolumeClaim, enum values:
- Never: do nothing if the PersistentVolumeClaim not exists.
- IfNotPresent: create the PersistentVolumeClaim if not present and the accessModes only contains &lsquo;ReadWriteMany&rsquo;.</p>
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
<p>persistentVolumeConfigMap references the configmap which contains a persistentVolume template.
key must be &ldquo;persistentVolume&rdquo; and value is the &ldquo;PersistentVolume&rdquo; struct.
support the following built-in Objects:
- $(GENERATE_NAME): generate a specific format &ldquo;pvcName-pvcNamespace&rdquo;.
if the PersistentVolumeClaim not exists and CreatePolicy is &ldquo;IfNotPresent&rdquo;, the controller
will create it by this template. this is a mutually exclusive setting with &ldquo;storageClassName&rdquo;.</p>
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
<p>the name of the persistentVolume ConfigMap.</p>
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
<p>the namespace of the persistentVolume ConfigMap.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="dataprotection.kubeblocks.io/v1alpha1.RestoreJobPhase">RestoreJobPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#dataprotection.kubeblocks.io/v1alpha1.RestoreJobStatus">RestoreJobStatus</a>)
</p>
<div>
<p>RestoreJobPhase The current phase. Valid values are New, InProgressPhy, InProgressLogic, Completed, Failed.</p>
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
<p>RestoreJobSpec defines the desired state of RestoreJob</p>
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
<p>Specified one backupJob to restore.</p>
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
<p>the target database workload to restore</p>
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
<p>array of restore volumes .</p>
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
<p>array of restore volume mounts .</p>
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
<p>count of backup stop retries on fail.</p>
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
<p>RestoreJobStatus defines the observed state of RestoreJob</p>
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
<p>The date and time when the Backup is eligible for garbage collection.
&lsquo;null&rsquo; means the Backup is NOT be cleaned except delete manual.</p>
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
<p>Date/time when the backup started being processed.</p>
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
<p>Date/time when the backup finished being processed.</p>
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
<p>Job failed reason.</p>
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
<p>ttl is a time string ending with the &rsquo;d&rsquo;|&rsquo;D&rsquo;|&lsquo;h&rsquo;|&lsquo;H&rsquo; character to describe how long
the Backup should be retained. if not set, will be retained forever.</p>
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
<code>snapshot</code><br/>
<em>
<a href="#dataprotection.kubeblocks.io/v1alpha1.SchedulePolicy">
SchedulePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>schedule policy for snapshot backup.</p>
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
<p>schedule policy for datafile backup.</p>
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
<p>schedule policy for logfile backup.</p>
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
<p>the cron expression for schedule, the timezone is in UTC. see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
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
<p>enable or disable the schedule.</p>
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
<p>execute hook commands for backup.</p>
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
<p>TargetCluster TODO (dsj): target cluster need redefined from Cluster API</p>
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
<p>labelsSelector is used to find matching pods.
Pods that match this label selector are counted to determine the number of pods
in their corresponding topology domain.</p>
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
<p>secret is used to connect to the target database cluster.
If not set, secret will be inherited from backup policy template.
if still not set, the controller will check if any system account for dataprotection has been created.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
