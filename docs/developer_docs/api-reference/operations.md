---
title: Operations API Reference
description: Operations API Reference
keywords: [operations, api]
sidebar_position: 2
sidebar_label: Operations
---
<br />
<p>Packages:</p>
<ul>
<li>
<a href="#operations.kubeblocks.io%2fv1alpha1">operations.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="operations.kubeblocks.io/v1alpha1">operations.kubeblocks.io/v1alpha1</h2>
Resource Types:
<ul><li>
<a href="#operations.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>
</li><li>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>
</li></ul>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition
</h3>
<div>
<p>OpsDefinition is the Schema for the OpsDefinitions API.</p>
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
<code>operations.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>OpsDefinition</code></td>
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
<a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">
OpsDefinitionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tbody>
<tr>
<td>
<code>preConditions</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the actions for the operation.
if set, it will check the condition before the Component runs this operation.
Example:</p>
<pre><code class="language-yaml"> preConditions:
 - rule:
     expression: '&#123;&#123; eq .component.status.phase &quot;Running&quot; &#125;&#125;'
     message: Component is not in Running status.
</code></pre>
</td>
</tr>
<tr>
<td>
<code>podInfoExtractors</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PodInfoExtractor">
[]PodInfoExtractor
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of PodInfoExtractor, each designed to select a specific Pod and extract selected runtime info
from its PodSpec.
The extracted information, such as environment variables, volumes and tolerations, are then injected into
Jobs or Pods that execute the OpsActions defined in <code>actions</code>.</p>
</td>
</tr>
<tr>
<td>
<code>componentInfos</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentInfo">
[]ComponentInfo
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of ComponentDefinition for Components associated with this OpsDefinition.
It also includes connection credentials (address and account) for each Component.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the schema for validating the data types and value ranges of parameters in OpsActions before their usage.</p>
</td>
</tr>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsAction">
[]OpsAction
</a>
</em>
</td>
<td>
<p>Specifies a list of OpsAction where each customized action is executed sequentially.</p>
</td>
</tr>
</tbody>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionStatus">
OpsDefinitionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest
</h3>
<div>
<p>OpsRequest is the Schema for the opsrequests API</p>
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
<code>operations.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>OpsRequest</code></td>
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
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestSpec">
OpsRequestSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tbody>
<tr>
<td>
<code>clusterName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the Cluster resource that this operation is targeting.</p>
</td>
</tr>
<tr>
<td>
<code>cancel</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the current operation should be canceled and terminated gracefully if it&rsquo;s in the
&ldquo;Pending&rdquo;, &ldquo;Creating&rdquo;, or &ldquo;Running&rdquo; state.</p>
<p>This field applies only to &ldquo;VerticalScaling&rdquo; and &ldquo;HorizontalScaling&rdquo; opsRequests.</p>
<p>Note: Setting <code>cancel</code> to true is irreversible; further modifications to this field are ineffective.</p>
</td>
</tr>
<tr>
<td>
<code>force</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Instructs the system to bypass pre-checks (including cluster state checks and customized pre-conditions hooks)
and immediately execute the opsRequest, except for the opsRequest of &lsquo;Start&rsquo; type, which will still undergo
pre-checks even if <code>force</code> is true.</p>
<p>This is useful for concurrent execution of &lsquo;VerticalScaling&rsquo; and &lsquo;HorizontalScaling&rsquo; opsRequests.
By setting <code>force</code> to true, you can bypass the default checks and demand these opsRequests to run
simultaneously.</p>
<p>Note: Once set, the <code>force</code> field is immutable and cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>enqueueOnForce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether opsRequest should continue to queue when &lsquo;force&rsquo; is set to true.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>Specifies the type of this operation. Supported types include &ldquo;Start&rdquo;, &ldquo;Stop&rdquo;, &ldquo;Restart&rdquo;, &ldquo;Switchover&rdquo;,
&ldquo;VerticalScaling&rdquo;, &ldquo;HorizontalScaling&rdquo;, &ldquo;VolumeExpansion&rdquo;, &ldquo;Reconfiguring&rdquo;, &ldquo;Upgrade&rdquo;, &ldquo;Backup&rdquo;, &ldquo;Restore&rdquo;,
&ldquo;Expose&rdquo;, &ldquo;RebuildInstance&rdquo;, &ldquo;Custom&rdquo;.</p>
<p>Note: This field is immutable once set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterSucceed</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the duration in seconds that an OpsRequest will remain in the system after successfully completing
(when <code>opsRequest.status.phase</code> is &ldquo;Succeed&rdquo;) before automatic deletion.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterUnsuccessfulCompletion</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the duration in seconds that an OpsRequest will remain in the system after completion
for any phase other than &ldquo;Succeed&rdquo; (e.g., &ldquo;Failed&rdquo;, &ldquo;Cancelled&rdquo;, &ldquo;Aborted&rdquo;) before automatic deletion.</p>
</td>
</tr>
<tr>
<td>
<code>preConditionDeadlineSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum time in seconds that the OpsRequest will wait for its pre-conditions to be met
before it aborts the operation.
If set to 0 (default), pre-conditions must be satisfied immediately for the OpsRequest to proceed.</p>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum duration (in seconds) that an opsRequest is allowed to run.
If the opsRequest runs longer than this duration, its phase will be marked as Aborted.
If this value is not set or set to 0, the timeout will be ignored and the opsRequest will run indefinitely.</p>
</td>
</tr>
<tr>
<td>
<code>SpecificOpsRequest</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">
SpecificOpsRequest
</a>
</em>
</td>
<td>
<p>
(Members of <code>SpecificOpsRequest</code> are embedded into this type.)
</p>
<p>Exactly one of its members must be set.</p>
</td>
</tr>
</tbody>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestStatus">
OpsRequestStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ActionTask">ActionTask
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail</a>)
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
<code>objectKey</code><br/>
<em>
string
</em>
</td>
<td>
<p>Represents the name of the task.</p>
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
<p>Represents the namespace where the task is deployed.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ActionTaskStatus">
ActionTaskStatus
</a>
</em>
</td>
<td>
<p>Indicates the current status of the task, including &ldquo;Processing&rdquo;, &ldquo;Failed&rdquo;, &ldquo;Succeed&rdquo;.</p>
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
<em>(Optional)</em>
<p>The name of the Pod that the task is associated with or operates on.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The count of retry attempts made for this task.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ActionTaskStatus">ActionTaskStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.ActionTask">ActionTask</a>)
</p>
<div>
<p>ActionTaskStatus defines the status of the task.</p>
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
</tr><tr><td><p>&#34;Processing&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Succeed&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Backup">Backup
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>backupName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the Backup custom resource.</p>
</td>
</tr>
<tr>
<td>
<code>backupPolicyName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the name of the BackupPolicy applied to perform this Backup.</p>
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
<em>(Optional)</em>
<p>Specifies the name of BackupMethod.
The specified BackupMethod must be defined in the BackupPolicy.</p>
</td>
</tr>
<tr>
<td>
<code>deletionPolicy</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines whether the backup contents stored in backup repository
should be deleted when the Backup custom resource is deleted.
Supported values are <code>Retain</code> and <code>Delete</code>.
- <code>Retain</code> means that the backup content and its physical snapshot on backup repository are kept.
- <code>Delete</code> means that the backup content and its physical snapshot on backup repository are deleted.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines the duration for which the Backup custom resources should be retained.</p>
<p>The controller will automatically remove all Backup objects that are older than the specified RetentionPeriod.
For example, RetentionPeriod of <code>30d</code> will keep only the Backup objects of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 2y</li>
<li>months: 6mo</li>
<li>days: 30d</li>
<li>hours: 12h</li>
<li>minutes: 30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m.
If not set, the Backup objects will be kept forever.</p>
<p>If the <code>deletionPolicy</code> is set to &lsquo;Delete&rsquo;, then the associated backup data will also be deleted
along with the Backup object.
Otherwise, only the Backup custom resource will be deleted.</p>
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
<p>If the specified BackupMethod is incremental, <code>parentBackupName</code> is required.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.ParameterPair
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of name-value pairs representing parameters and their corresponding values.
Parameters match the schema specified in the <code>actionset.spec.parametersSchema</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.BackupRefSpec">BackupRefSpec
</h3>
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
<code>ref</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.RefNamespaceName">
RefNamespaceName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to a reference backup that needs to be restored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.CompletionProbe">CompletionProbe
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of seconds to wait after the resource has been patched before initiating completion probes.
The default value is 5 seconds, with a minimum value of 1.</p>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the number of seconds after which the probe times out.
The default value is 60 seconds, with a minimum value of 1.</p>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the frequency (in seconds) at which the probe should be performed.
The default value is 5 seconds, with a minimum value of 1.</p>
</td>
</tr>
<tr>
<td>
<code>matchExpressions</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.MatchExpressions">
MatchExpressions
</a>
</em>
</td>
<td>
<p>Executes expressions regularly, based on the value of PeriodSeconds, to determine if the action has been completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ComponentInfo">ComponentInfo
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<code>componentDefinitionName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the ComponentDefinition.
The name can represent an exact name, a name prefix, or a regular expression pattern.</p>
<p>For example:</p>
<ul>
<li>&ldquo;mysql-8.0.30-v1alpha1&rdquo;: Matches the exact name &ldquo;mysql-8.0.30-v1alpha1&rdquo;</li>
<li>&ldquo;mysql-8.0.30&rdquo;: Matches all names starting with &ldquo;mysql-8.0.30&rdquo;</li>
<li>&rdquo;^mysql-8.0.\d&#123;1,2&#125;$&ldquo;: Matches all names starting with &ldquo;mysql-8.0.&rdquo; followed by one or two digits.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>accountName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the account name associated with the Component.
If set, the corresponding account username and password are injected into containers&rsquo; environment variables
<code>KB_ACCOUNT_USERNAME</code> and <code>KB_ACCOUNT_PASSWORD</code>.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the Service.
If set, the service name is injected as the <code>KB_COMP_SVC_NAME</code> environment variable in the containers,
and each service port is mapped to a corresponding environment variable named <code>KB_COMP_SVC_PORT_$(portName)</code>.
The <code>portName</code> is transformed by replacing &lsquo;-&rsquo; with &lsquo;_&rsquo; and converting to uppercase.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ComponentOps">ComponentOps
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.CustomOpsComponent">CustomOpsComponent</a>, <a href="#operations.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>, <a href="#operations.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance</a>, <a href="#operations.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure</a>, <a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>, <a href="#operations.kubeblocks.io/v1alpha1.UpgradeComponent">UpgradeComponent</a>, <a href="#operations.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>, <a href="#operations.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion</a>)
</p>
<div>
<p>ComponentOps specifies the Component to be operated on.</p>
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
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the Component as defined in the cluster.spec</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.CustomOps">CustomOps
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>opsDefinitionName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the OpsDefinition.</p>
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
<p>Specifies the name of the ServiceAccount to be used for executing the custom operation.</p>
</td>
</tr>
<tr>
<td>
<code>maxConcurrentComponents</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum number of components to be operated on concurrently to mitigate performance impact
on clusters with multiple components.</p>
<p>It accepts an absolute number (e.g., 5) or a percentage of components to execute in parallel (e.g., &ldquo;10%&rdquo;).
Percentages are rounded up to the nearest whole number of components.
For example, if &ldquo;10%&rdquo; results in less than one, it rounds up to 1.</p>
<p>When unspecified, all components are processed simultaneously by default.</p>
<p>Note: This feature is not implemented yet.</p>
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.CustomOpsComponent">
[]CustomOpsComponent
</a>
</em>
</td>
<td>
<p>Specifies the components and their parameters for executing custom actions as defined in OpsDefinition.
Requires at least one component.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.CustomOpsComponent">CustomOpsComponent
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.CustomOps">CustomOps</a>)
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Parameter">
[]Parameter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters that match the schema specified in the <code>opsDefinition.spec.parametersSchema</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.EnvVarRef">EnvVarRef
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsVarSource">OpsVarSource</a>)
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
<code>targetContainerName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the container name in the target Pod.
If not specified, the first container will be used by default.</p>
</td>
</tr>
<tr>
<td>
<code>envName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Defines the name of the environment variable.
This name can originate from an &lsquo;env&rsquo; entry or be a data key from an &lsquo;envFrom&rsquo; source.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Expose">Expose
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>switch</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ExposeSwitch">
ExposeSwitch
</a>
</em>
</td>
<td>
<p>Indicates whether the services will be exposed.
&lsquo;Enable&rsquo; exposes the services. while &lsquo;Disable&rsquo; removes the exposed Service.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsService">
[]OpsService
</a>
</em>
</td>
<td>
<p>Specifies a list of OpsService.
When an OpsService is exposed, a corresponding ClusterService will be added to <code>cluster.spec.services</code>.
On the other hand, when an OpsService is unexposed, the corresponding ClusterService will be removed
from <code>cluster.spec.services</code>.</p>
<p>Note: If <code>componentName</code> is not specified, the <code>ports</code> and <code>selector</code> fields must be provided
in each OpsService definition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ExposeSwitch">ExposeSwitch
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.Expose">Expose</a>)
</p>
<div>
<p>ExposeSwitch Specifies the switch for the expose operation. This switch can be used to enable or disable the expose operation.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Disable&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Enable&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.FailurePolicyType">FailurePolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
</p>
<div>
<p>FailurePolicyType specifies the type of failure policy.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Fail&#34;</p></td>
<td><p>FailurePolicyFail means that an error will be reported.</p>
</td>
</tr><tr><td><p>&#34;Ignore&#34;</p></td>
<td><p>FailurePolicyIgnore means that an error will be ignored but logged.</p>
</td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
</p>
<div>
<p>HorizontalScaling defines the parameters of a horizontal scaling operation.</p>
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Specifies the desired number of shards for the component.
This parameter is mutually exclusive with other parameters.</p>
</td>
</tr>
<tr>
<td>
<code>scaleOut</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ScaleOut">
ScaleOut
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the replica changes for scaling out components and instance templates,
and brings offline instances back online. Can be used in conjunction with the &ldquo;scaleIn&rdquo; operation.
Note: Any configuration that deletes instances is considered invalid.</p>
</td>
</tr>
<tr>
<td>
<code>scaleIn</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ScaleIn">
ScaleIn
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the replica changes for scaling in components and instance templates,
and takes specified instances offline. Can be used in conjunction with the &ldquo;scaleOut&rdquo; operation.
Note: Any configuration that creates instances is considered invalid.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Instance">Instance
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance</a>)
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
<p>Pod name of the instance.</p>
</td>
</tr>
<tr>
<td>
<code>targetNodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The instance will rebuild on the specified node.
If not set, it will rebuild on a random node.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.InstanceReplicasTemplate">InstanceReplicasTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.ReplicaChanger">ReplicaChanger</a>)
</p>
<div>
<p>InstanceReplicasTemplate defines the template for instance replicas.</p>
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
<p>Specifies the name of the instance template.</p>
</td>
</tr>
<tr>
<td>
<code>replicaChanges</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Specifies the replica changes for the instance template.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.InstanceResourceTemplate">InstanceResourceTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>)
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
<p>Refer to the instance template name of the component or sharding.</p>
</td>
</tr>
<tr>
<td>
<code>ResourceRequirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceRequirements</code> are embedded into this type.)
</p>
<p>Defines the computational resource size for vertical scaling.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.InstanceVolumeClaimTemplate">InstanceVolumeClaimTemplate
</h3>
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
<p>Refer to the instance template name of the component or sharding.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">
[]OpsRequestVolumeClaimTemplate
</a>
</em>
</td>
<td>
<p>volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.JSONPatchOperation">JSONPatchOperation
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
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
<code>op</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the type of JSON patch operation. It supports the following values: &lsquo;add&rsquo;, &lsquo;remove&rsquo;, &lsquo;replace&rsquo;.</p>
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
<p>Specifies the json patch path.</p>
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
<p>Specifies the value to be used in the JSON patch operation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.LastConfiguration">LastConfiguration</a>)
</p>
<div>
<p>LastComponentConfiguration can be used to track and compare the desired state of the Component over time.</p>
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
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the <code>replicas</code> of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the <code>shards</code> of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>ResourceRequirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceRequirements</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Records the resources of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">
[]OpsRequestVolumeClaimTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records volumes&rsquo; storage size of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/apps/v1.ClusterComponentService
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the ClusterComponentService list of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/apps/v1.InstanceTemplate
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the InstanceTemplate list of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>offlineInstances</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the offline instances of the Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>serviceVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the version of the Service expected to be provisioned by this Component prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>componentDefinitionName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the name of the ComponentDefinition prior to any changes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.LastConfiguration">LastConfiguration
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
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
<code>components</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.LastComponentConfiguration">
map[string]github.com/apecloud/kubeblocks/apis/operations/v1alpha1.LastComponentConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the configuration of each Component prior to any changes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.MatchExpressions">MatchExpressions
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.CompletionProbe">CompletionProbe</a>)
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
<code>failure</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a failure condition for an action using a Go template expression.
Should evaluate to either <code>true</code> or <code>false</code>.
The current resource object is parsed into the Go template.
for example, you can use &lsquo;&#123;&#123; eq .spec.replicas 1 &#125;&#125;&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>success</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a success condition for an action using a Go template expression.
Should evaluate to either <code>true</code> or <code>false</code>.
The current resource object is parsed into the Go template.
for example, using &lsquo;&#123;&#123; eq .spec.replicas 1 &#125;&#125;&rsquo;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsAction">OpsAction
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
</p>
<div>
<p>OpsAction specifies a custom action defined in OpsDefinition for execution in a &ldquo;Custom&rdquo; OpsRequest.</p>
<p>OpsAction can be of three types:</p>
<ul>
<li>workload: Creates a Job or Pod to run custom scripts, ideal for isolated or long-running tasks.</li>
<li>exec: Executes commands directly within an existing container using the kubectl exec interface,
suitable for immediate, short-lived operations.</li>
<li>resourceModifier: Modifies a K8s object using JSON patches, useful for updating the spec of some resource.</li>
</ul>
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
<p>Specifies the name of the OpsAction.</p>
</td>
</tr>
<tr>
<td>
<code>failurePolicy</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.FailurePolicyType">
FailurePolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the failure policy of the OpsAction.
Valid values are:</p>
<ul>
<li>&ldquo;Fail&rdquo;: Marks the entire OpsRequest as failed if the action fails.</li>
<li>&ldquo;Ignore&rdquo;: The OpsRequest continues processing despite the failure of the action.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters for the OpsAction. Their usage varies based on the action type:</p>
<ul>
<li>For &lsquo;workload&rsquo; or &lsquo;exec&rsquo; actions, parameters are injected as environment variables.</li>
<li>For &lsquo;resourceModifier&rsquo; actions, parameter can be referenced using $() in fields
<code>resourceModifier.completionProbe.matchExpressions</code> and <code>resourceModifier.jsonPatches[*].value</code>.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>workload</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsWorkloadAction">
OpsWorkloadAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration for a &lsquo;workload&rsquo; action.
This action leads to the creation of a K8s workload, such as a Pod or Job, to execute specified tasks.</p>
</td>
</tr>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsExecAction">
OpsExecAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration for a &lsquo;exec&rsquo; action.
It creates a Pod and invokes a &lsquo;kubectl exec&rsquo; to run command inside a specified container with the target Pod.</p>
</td>
</tr>
<tr>
<td>
<code>resourceModifier</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsResourceModifierAction">
OpsResourceModifierAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration for a &lsquo;resourceModifier&rsquo; action.
This action allows for modifications to existing K8s objects.</p>
<p>Note: This feature has not been implemented yet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>)
</p>
<div>
<p>OpsDefinitionSpec defines the desired state of OpsDefinition.</p>
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
<code>preConditions</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the actions for the operation.
if set, it will check the condition before the Component runs this operation.
Example:</p>
<pre><code class="language-yaml"> preConditions:
 - rule:
     expression: '&#123;&#123; eq .component.status.phase &quot;Running&quot; &#125;&#125;'
     message: Component is not in Running status.
</code></pre>
</td>
</tr>
<tr>
<td>
<code>podInfoExtractors</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PodInfoExtractor">
[]PodInfoExtractor
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of PodInfoExtractor, each designed to select a specific Pod and extract selected runtime info
from its PodSpec.
The extracted information, such as environment variables, volumes and tolerations, are then injected into
Jobs or Pods that execute the OpsActions defined in <code>actions</code>.</p>
</td>
</tr>
<tr>
<td>
<code>componentInfos</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentInfo">
[]ComponentInfo
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of ComponentDefinition for Components associated with this OpsDefinition.
It also includes connection credentials (address and account) for each Component.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the schema for validating the data types and value ranges of parameters in OpsActions before their usage.</p>
</td>
</tr>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsAction">
[]OpsAction
</a>
</em>
</td>
<td>
<p>Specifies a list of OpsAction where each customized action is executed sequentially.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsDefinitionStatus">OpsDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>)
</p>
<div>
<p>OpsDefinitionStatus defines the observed state of OpsDefinition</p>
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
<p>Represents the most recent generation observed of this OpsDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Phase">
Phase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the current state of the OpsDefinition.
Valid values are &ldquo;&rdquo;, &ldquo;Available&rdquo;, &ldquo;Unavailable&rdquo;.
When it equals to &ldquo;Available&rdquo;, the OpsDefinition is ready and can be used in a &ldquo;Custom&rdquo; OpsRequest.</p>
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
<h3 id="operations.kubeblocks.io/v1alpha1.OpsEnvVar">OpsEnvVar
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.PodInfoExtractor">PodInfoExtractor</a>)
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
<p>Specifies the name of the environment variable to be injected into Pods executing OpsActions.
It must conform to the C_IDENTIFIER format, which includes only alphanumeric characters and underscores, and cannot begin with a digit.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsVarSource">
OpsVarSource
</a>
</em>
</td>
<td>
<p>Specifies the source of the environment variable&rsquo;s value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsExecAction">OpsExecAction
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
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
<code>podInfoExtractorName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a PodInfoExtractor defined in the <code>opsDefinition.spec.podInfoExtractors</code>.</p>
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
<p>Specifies the number of retries allowed before marking the action as failed.</p>
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
<p>The command to be executed via &lsquo;kubectl exec &ndash;&rsquo;.</p>
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
<p>The name of the container in the target pod where the command should be executed.
This corresponds to the <code>-c &#123;containerName&#125;</code> option in <code>kubectl exec</code>.</p>
<p>If not set, the first container is used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsPhase">OpsPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
</p>
<div>
<p>OpsPhase defines opsRequest phase.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Aborted&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Cancelled&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Cancelling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Creating&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Succeed&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRecorder">OpsRecorder
</h3>
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
<p>name OpsRequest name</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>opsRequest type</p>
</td>
</tr>
<tr>
<td>
<code>inQueue</code><br/>
<em>
bool
</em>
</td>
<td>
<p>indicates whether the current opsRequest is in the queue</p>
</td>
</tr>
<tr>
<td>
<code>queueBySelf</code><br/>
<em>
bool
</em>
</td>
<td>
<p>indicates that the operation is queued for execution within its own-type scope.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequestBehaviour">OpsRequestBehaviour
</h3>
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
<code>FromClusterPhases</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/apps/v1.ClusterPhase
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ToClusterPhase</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/apps/v1.ClusterPhase
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
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
<code>phase</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/apps/v1.ComponentPhase
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the current phase of the Component, mirroring <code>cluster.status.components[componentName].phase</code>.</p>
</td>
</tr>
<tr>
<td>
<code>lastFailedTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the timestamp when the Component last transitioned to a &ldquo;Failed&rdquo; phase.</p>
</td>
</tr>
<tr>
<td>
<code>preCheck</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PreCheckResult">
PreCheckResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the result of the preConditions check of the opsRequest, which determines subsequent steps.</p>
</td>
</tr>
<tr>
<td>
<code>progressDetails</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ProgressStatusDetail">
[]ProgressStatusDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the progress details of objects or actions associated with the Component.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provides an explanation for the Component being in its current state.</p>
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
<p>Provides a human-readable message indicating details about this operation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>)
</p>
<div>
<p>OpsRequestSpec defines the desired state of OpsRequest</p>
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
<code>clusterName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the Cluster resource that this operation is targeting.</p>
</td>
</tr>
<tr>
<td>
<code>cancel</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the current operation should be canceled and terminated gracefully if it&rsquo;s in the
&ldquo;Pending&rdquo;, &ldquo;Creating&rdquo;, or &ldquo;Running&rdquo; state.</p>
<p>This field applies only to &ldquo;VerticalScaling&rdquo; and &ldquo;HorizontalScaling&rdquo; opsRequests.</p>
<p>Note: Setting <code>cancel</code> to true is irreversible; further modifications to this field are ineffective.</p>
</td>
</tr>
<tr>
<td>
<code>force</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Instructs the system to bypass pre-checks (including cluster state checks and customized pre-conditions hooks)
and immediately execute the opsRequest, except for the opsRequest of &lsquo;Start&rsquo; type, which will still undergo
pre-checks even if <code>force</code> is true.</p>
<p>This is useful for concurrent execution of &lsquo;VerticalScaling&rsquo; and &lsquo;HorizontalScaling&rsquo; opsRequests.
By setting <code>force</code> to true, you can bypass the default checks and demand these opsRequests to run
simultaneously.</p>
<p>Note: Once set, the <code>force</code> field is immutable and cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>enqueueOnForce</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether opsRequest should continue to queue when &lsquo;force&rsquo; is set to true.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>Specifies the type of this operation. Supported types include &ldquo;Start&rdquo;, &ldquo;Stop&rdquo;, &ldquo;Restart&rdquo;, &ldquo;Switchover&rdquo;,
&ldquo;VerticalScaling&rdquo;, &ldquo;HorizontalScaling&rdquo;, &ldquo;VolumeExpansion&rdquo;, &ldquo;Reconfiguring&rdquo;, &ldquo;Upgrade&rdquo;, &ldquo;Backup&rdquo;, &ldquo;Restore&rdquo;,
&ldquo;Expose&rdquo;, &ldquo;RebuildInstance&rdquo;, &ldquo;Custom&rdquo;.</p>
<p>Note: This field is immutable once set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterSucceed</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the duration in seconds that an OpsRequest will remain in the system after successfully completing
(when <code>opsRequest.status.phase</code> is &ldquo;Succeed&rdquo;) before automatic deletion.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterUnsuccessfulCompletion</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the duration in seconds that an OpsRequest will remain in the system after completion
for any phase other than &ldquo;Succeed&rdquo; (e.g., &ldquo;Failed&rdquo;, &ldquo;Cancelled&rdquo;, &ldquo;Aborted&rdquo;) before automatic deletion.</p>
</td>
</tr>
<tr>
<td>
<code>preConditionDeadlineSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum time in seconds that the OpsRequest will wait for its pre-conditions to be met
before it aborts the operation.
If set to 0 (default), pre-conditions must be satisfied immediately for the OpsRequest to proceed.</p>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the maximum duration (in seconds) that an opsRequest is allowed to run.
If the opsRequest runs longer than this duration, its phase will be marked as Aborted.
If this value is not set or set to 0, the timeout will be ignored and the opsRequest will run indefinitely.</p>
</td>
</tr>
<tr>
<td>
<code>SpecificOpsRequest</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">
SpecificOpsRequest
</a>
</em>
</td>
<td>
<p>
(Members of <code>SpecificOpsRequest</code> are embedded into this type.)
</p>
<p>Exactly one of its members must be set.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>)
</p>
<div>
<p>OpsRequestStatus represents the observed state of an OpsRequest.</p>
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
<code>clusterGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the cluster generation after the OpsRequest action has been handled.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsPhase">
OpsPhase
</a>
</em>
</td>
<td>
<p>Represents the phase of the OpsRequest.
Possible values include &ldquo;Pending&rdquo;, &ldquo;Creating&rdquo;, &ldquo;Running&rdquo;, &ldquo;Cancelling&rdquo;, &ldquo;Cancelled&rdquo;, &ldquo;Failed&rdquo;, &ldquo;Succeed&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>progress</code><br/>
<em>
string
</em>
</td>
<td>
<p>Represents the progress of the OpsRequest.</p>
</td>
</tr>
<tr>
<td>
<code>lastConfiguration</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.LastConfiguration">
LastConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the configuration prior to any changes.</p>
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">
map[string]github.com/apecloud/kubeblocks/apis/operations/v1alpha1.OpsRequestComponentStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the status information of Components changed due to the OpsRequest.</p>
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
<p>A collection of additional key-value pairs that provide supplementary information for the OpsRequest.</p>
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
<p>Records the time when the OpsRequest started processing.</p>
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
<p>Records the time when the OpsRequest was completed.</p>
</td>
</tr>
<tr>
<td>
<code>cancelTimestamp</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the time when the OpsRequest was cancelled.</p>
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
<p>Describes the detailed status of the OpsRequest.
Possible condition types include &ldquo;Cancelled&rdquo;, &ldquo;WaitForProgressing&rdquo;, &ldquo;Validated&rdquo;, &ldquo;Succeed&rdquo;, &ldquo;Failed&rdquo;, &ldquo;Restarting&rdquo;,
&ldquo;VerticalScaling&rdquo;, &ldquo;HorizontalScaling&rdquo;, &ldquo;VolumeExpanding&rdquo;, &ldquo;Reconfigure&rdquo;, &ldquo;Switchover&rdquo;, &ldquo;Stopping&rdquo;, &ldquo;Starting&rdquo;,
&ldquo;VersionUpgrading&rdquo;, &ldquo;Exposing&rdquo;, &ldquo;Backup&rdquo;, &ldquo;InstancesRebuilding&rdquo;, &ldquo;CustomOperation&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">OpsRequestVolumeClaimTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.InstanceVolumeClaimTemplate">InstanceVolumeClaimTemplate</a>, <a href="#operations.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>, <a href="#operations.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion</a>)
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
<code>storage</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<p>Specifies the desired storage size for the volume.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specify the name of the volumeClaimTemplate in the Component.
The specified name must match one of the volumeClaimTemplates defined
in the <code>clusterComponentSpec.volumeClaimTemplates</code> field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
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
<code>resource</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.TypedObjectRef">
TypedObjectRef
</a>
</em>
</td>
<td>
<p>Specifies the K8s object that is to be updated.</p>
</td>
</tr>
<tr>
<td>
<code>jsonPatches</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.JSONPatchOperation">
[]JSONPatchOperation
</a>
</em>
</td>
<td>
<p>Specifies a list of patches for modifying the object.</p>
</td>
</tr>
<tr>
<td>
<code>completionProbe</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.CompletionProbe">
CompletionProbe
</a>
</em>
</td>
<td>
<p>Specifies a method to determine if the action has been completed.</p>
<p>Note: This feature has not been implemented yet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsService">OpsService
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.Expose">Expose</a>)
</p>
<div>
<p>OpsService represents the parameters to dynamically create or remove a ClusterService in the <code>cluster.spec.services</code> array.</p>
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
<p>Specifies the name of the Service. This name is used to set <code>clusterService.name</code>.</p>
<p>Note: This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Contains cloud provider related parameters if ServiceType is LoadBalancer.</p>
<p>More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#serviceport-v1-core">
[]Kubernetes core/v1.ServicePort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies Port definitions that are to be exposed by a ClusterService.</p>
<p>If not specified, the Port definitions from non-NodePort and non-LoadBalancer type ComponentService
defined in the ComponentDefinition (<code>componentDefinition.spec.services</code>) will be used.
If no matching ComponentService is found, the expose operation will fail.</p>
<p>More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports">https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports</a></p>
</td>
</tr>
<tr>
<td>
<code>roleSelector</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a role to target with the service.
If specified, the service will only be exposed to pods with the matching role.</p>
<p>Note: If the component has roles, at least one of &lsquo;roleSelector&rsquo; or &lsquo;podSelector&rsquo; must be specified.
If both are specified, a pod must match both conditions to be selected.</p>
</td>
</tr>
<tr>
<td>
<code>podSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Routes service traffic to pods with matching label keys and values.
If specified, the service will only be exposed to pods matching the selector.</p>
<p>Note: If the component has roles, at least one of &lsquo;roleSelector&rsquo; or &lsquo;podSelector&rsquo; must be specified.
If both are specified, a pod must match both conditions to be selected.</p>
</td>
</tr>
<tr>
<td>
<code>serviceType</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#servicetype-v1-core">
Kubernetes core/v1.ServiceType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines how the Service is exposed. Defaults to &lsquo;ClusterIP&rsquo;.
Valid options are <code>ClusterIP</code>, <code>NodePort</code>, and <code>LoadBalancer</code>.</p>
<ul>
<li><code>ClusterIP</code>: allocates a cluster-internal IP address for load-balancing to endpoints.
Endpoints are determined by the selector or if that is not specified,
they are determined by manual construction of an Endpoints object or EndpointSlice objects.</li>
<li><code>NodePort</code>: builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.</li>
<li><code>LoadBalancer</code>: builds on NodePort and creates an external load-balancer (if supported in the current cloud)
which routes to the same endpoints as the clusterIP.</li>
</ul>
<p>Note: although K8s Service type allows the &lsquo;ExternalName&rsquo; type, it is not a valid option for the expose operation.</p>
<p>For more info, see:
<a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a>.</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#ipfamily-v1-core">
[]Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of IP families (e.g., IPv4, IPv6) assigned to this Service.</p>
<p>Usually assigned automatically based on the cluster configuration and the <code>ipFamilyPolicy</code> field.
If specified manually, the requested IP family must be available in the cluster and allowed by the <code>ipFamilyPolicy</code>.
If the requested IP family is not available or not allowed, the Service creation will fail.</p>
<p>Valid values:</p>
<ul>
<li>&ldquo;IPv4&rdquo;</li>
<li>&ldquo;IPv6&rdquo;</li>
</ul>
<p>This field may hold a maximum of two entries (dual-stack families, in either order).</p>
<p>Common combinations of <code>ipFamilies</code> and <code>ipFamilyPolicy</code> are:</p>
<ul>
<li>ipFamilies=[] + ipFamilyPolicy=&ldquo;PreferDualStack&rdquo; :
The Service prefers dual-stack but can fall back to single-stack if the cluster does not support dual-stack.
The IP family is automatically assigned based on the cluster configuration.</li>
<li>ipFamilies=[&ldquo;IPV4&rdquo;,&ldquo;IPV6&rdquo;] + ipFamilyPolicy=&ldquo;RequiredDualStack&rdquo; :
The Service requires dual-stack and will only be created if the cluster supports both IPv4 and IPv6.
The primary IP family is IPV4.</li>
<li>ipFamilies=[&ldquo;IPV6&rdquo;,&ldquo;IPV4&rdquo;] + ipFamilyPolicy=&ldquo;RequiredDualStack&rdquo; :
The Service requires dual-stack and will only be created if the cluster supports both IPv4 and IPv6.
The primary IP family is IPV6.</li>
<li>ipFamilies=[&ldquo;IPV4&rdquo;] + ipFamilyPolicy=&ldquo;SingleStack&rdquo; :
The Service uses a single-stack with IPv4 only.</li>
<li>ipFamilies=[&ldquo;IPV6&rdquo;] + ipFamilyPolicy=&ldquo;SingleStack&rdquo; :
The Service uses a single-stack with IPv6 only.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>ipFamilyPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#ipfamilypolicy-v1-core">
Kubernetes core/v1.IPFamilyPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether the Service should use a single IP family (SingleStack) or two IP families (DualStack).</p>
<p>Possible values:</p>
<ul>
<li>&lsquo;SingleStack&rsquo; (default) : The Service uses a single IP family.
If no value is provided, IPFamilyPolicy defaults to SingleStack.</li>
<li>&lsquo;PreferDualStack&rsquo; : The Service prefers to use two IP families on dual-stack configured clusters
or a single IP family on single-stack clusters.</li>
<li>&lsquo;RequiredDualStack&rsquo; : The Service requires two IP families on dual-stack configured clusters.
If the cluster is not configured for dual-stack, the Service creation fails.</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsType">OpsType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRecorder">OpsRecorder</a>, <a href="#operations.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>OpsType defines operation types.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Backup&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Custom&#34;</p></td>
<td><p>RebuildInstance rebuilding an instance is very useful when a node is offline or an instance is unrecoverable.</p>
</td>
</tr><tr><td><p>&#34;Expose&#34;</p></td>
<td><p>StartType the start operation will start the pods which is deleted in stop operation.</p>
</td>
</tr><tr><td><p>&#34;HorizontalScaling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;RebuildInstance&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Reconfiguring&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Restart&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Restore&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Start&#34;</p></td>
<td><p>StopType the stop operation will delete all pods in a cluster concurrently.</p>
</td>
</tr><tr><td><p>&#34;Stop&#34;</p></td>
<td><p>RestartType the restart operation is a special case of the rolling update operation.</p>
</td>
</tr><tr><td><p>&#34;Switchover&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Upgrade&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;VerticalScaling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;VolumeExpansion&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsVarSource">OpsVarSource
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsEnvVar">OpsEnvVar</a>)
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
<code>envRef</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.EnvVarRef">
EnvVarRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a reference to a specific environment variable within a container.
Used to specify the source of the variable, which can be either &ldquo;env&rdquo; or &ldquo;envFrom&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>fieldPath</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectfieldselector-v1-core">
Kubernetes core/v1.ObjectFieldSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the JSONPath expression pointing to the specific data within the JSON structure of the target Pod.
It is used to extract precise data locations for operations on the Pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsWorkloadAction">OpsWorkloadAction
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
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
<code>type</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsWorkloadType">
OpsWorkloadType
</a>
</em>
</td>
<td>
<p>Defines the workload type of the action. Valid values include &ldquo;Job&rdquo; and &ldquo;Pod&rdquo;.</p>
<ul>
<li>&ldquo;Job&rdquo;: Creates a Job to execute the action.</li>
<li>&ldquo;Pod&rdquo;: Creates a Pod to execute the action.
Note: unlike Jobs, manually deleting a Pod does not affect the <code>backoffLimit</code>.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>podInfoExtractorName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a PodInfoExtractor defined in the <code>opsDefinition.spec.podInfoExtractors</code>.</p>
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
<p>Specifies the number of retries allowed before marking the action as failed.</p>
</td>
</tr>
<tr>
<td>
<code>podSpec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podspec-v1-core">
Kubernetes core/v1.PodSpec
</a>
</em>
</td>
<td>
<p>Specifies the PodSpec of the &lsquo;workload&rsquo; action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.OpsWorkloadType">OpsWorkloadType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsWorkloadAction">OpsWorkloadAction</a>)
</p>
<div>
<p>OpsWorkloadType policy after action failure.</p>
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
</tr><tr><td><p>&#34;Pod&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Parameter">Parameter
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.CustomOpsComponent">CustomOpsComponent</a>)
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
<p>Specifies the identifier of the parameter as defined in the OpsDefinition.</p>
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
<p>Holds the data associated with the parameter.
If the parameter type is an array, the format should be &ldquo;v1,v2,v3&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ParameterSource">
ParameterSource
</a>
</em>
</td>
<td>
<p>Source for the parameter&rsquo;s value. Cannot be used if value is not empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ParameterPair">ParameterPair
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure</a>)
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
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
<p>Represents the name of the parameter that is to be updated.</p>
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
<p>Represents the parameter values that are to be updated.
If set to nil, the parameter defined by the Key field will be removed from the configuration file.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ParameterSource">ParameterSource
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.Parameter">Parameter</a>)
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
<code>configMapKeyRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#configmapkeyselector-v1-core">
Kubernetes core/v1.ConfigMapKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a key of a ConfigMap.</p>
</td>
</tr>
<tr>
<td>
<code>secretKeyRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a key of a Secret.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ParametersSchema">ParametersSchema
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<code>openAPIV3Schema</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jsonschemaprops-v1-apiextensions-k8s-io">
Kubernetes api extensions v1.JSONSchemaProps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the schema for parameters using the OpenAPI v3.
The supported property types include:
- string
- number
- integer
- array: Note that only items of string type are supported.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionStatus">OpsDefinitionStatus</a>)
</p>
<div>
<p>Phase represents the current status of the ClusterDefinition CR.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Available&#34;</p></td>
<td><p>AvailablePhase indicates that the object is in an available state.</p>
</td>
</tr><tr><td><p>&#34;Unavailable&#34;</p></td>
<td><p>UnavailablePhase indicates that the object is in an unavailable state.</p>
</td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PodInfoExtractor">PodInfoExtractor
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<p>Specifies the name of the PodInfoExtractor.</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsEnvVar">
[]OpsEnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of environment variables to be extracted from a selected Pod,
and injected into the containers executing each OpsAction.</p>
</td>
</tr>
<tr>
<td>
<code>podSelector</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PodSelector">
PodSelector
</a>
</em>
</td>
<td>
<p>Used to select the target Pod from which environment variables and volumes are extracted from its PodSpec.</p>
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
<p>Specifies a list of volumes, along with their respective mount points, that are to be extracted from a selected Pod,
and mounted onto the containers executing each OpsAction.
This allows the containers to access shared or persistent data necessary for the operation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PodSelectionPolicy">PodSelectionPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>)
</p>
<div>
<p>PodSelectionPolicy pod selection strategy.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;All&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Any&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PodSelector">PodSelector
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.PodInfoExtractor">PodInfoExtractor</a>)
</p>
<div>
<p>PodSelector selects the target Pod from which environment variables and volumes are extracted from its PodSpec.</p>
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
<em>(Optional)</em>
<p>Specifies the role of the target Pod.</p>
</td>
</tr>
<tr>
<td>
<code>multiPodSelectionPolicy</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.PodSelectionPolicy">
PodSelectionPolicy
</a>
</em>
</td>
<td>
<p>Defines the policy for selecting the target pod when multiple pods match the podSelector.
It can be either &lsquo;Any&rsquo; (select any one pod that matches the podSelector)
or &lsquo;All&rsquo; (select all pods that match the podSelector).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PointInTimeRefSpec">PointInTimeRefSpec
</h3>
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
<code>time</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to the specific time point for restoration, with UTC as the time zone.</p>
</td>
</tr>
<tr>
<td>
<code>ref</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.RefNamespaceName">
RefNamespaceName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Refers to a reference source cluster that needs to be restored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PreCheckResult">PreCheckResult
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
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
<code>pass</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Indicates whether the preCheck operation passed or failed.</p>
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
<p>Provides explanations related to the preCheck result in a human-readable format.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.PreCondition">PreCondition
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<code>rule</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Rule">
Rule
</a>
</em>
</td>
<td>
<p>Specifies the conditions that must be met for the operation to execute.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ProgressStatus">ProgressStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail</a>)
</p>
<div>
<p>ProgressStatus defines the status of the opsRequest progress.</p>
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
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Processing&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Succeed&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
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
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the group to which the current object belongs to.</p>
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
<em>(Optional)</em>
<p><code>objectKey</code> uniquely identifies the object, which can be any K8s object, like a Pod, Job, Component, or PVC.
Either <code>objectKey</code> or <code>actionName</code> must be provided.</p>
</td>
</tr>
<tr>
<td>
<code>actionName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the name of an OpsAction, as defined in <code>opsDefinition.spec.actions[*].name</code>.
Either <code>objectKey</code> or <code>actionName</code> must be provided.</p>
</td>
</tr>
<tr>
<td>
<code>actionTasks</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ActionTask">
[]ActionTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists the tasks, such as Jobs or Pods, that carry out the action.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ProgressStatus">
ProgressStatus
</a>
</em>
</td>
<td>
<p>Represents the current processing state of the object, including &ldquo;Processing&rdquo;, &ldquo;Pending&rdquo;, &ldquo;Failed&rdquo;, &ldquo;Succeed&rdquo;</p>
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
<p>Provides a human-readable explanation of the object&rsquo;s condition.</p>
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
<p>Records the start time of object processing.</p>
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
<p>Records the completion time of object processing.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Instance">
[]Instance
</a>
</em>
</td>
<td>
<p>Specifies the instances (Pods) that need to be rebuilt, typically operating as standbys.</p>
</td>
</tr>
<tr>
<td>
<code>inPlace</code><br/>
<em>
bool
</em>
</td>
<td>
<p>When it is set to true, the instance will be rebuilt in-place.
If false, a new pod will be created. Once the new pod is ready to serve,
the instance that require rebuilding will be taken offline.</p>
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
<em>(Optional)</em>
<p>Indicates the name of the Backup custom resource from which to recover the instance.
Defaults to an empty PersistentVolume if unspecified.</p>
<p>Note:
- Only full physical backups are supported for multi-replica Components (e.g., &lsquo;xtrabackup&rsquo; for MySQL).
- Logical backups (e.g., &lsquo;mysqldump&rsquo; for MySQL) are unsupported in the current version.</p>
</td>
</tr>
<tr>
<td>
<code>sourceBackupTargetName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>When multiple source targets exist of the backup, you must specify the source target to restore.</p>
</td>
</tr>
<tr>
<td>
<code>restoreEnv</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines container environment variables for the restore process.
merged with the ones specified in the Backup and ActionSet resources.</p>
<p>Merge priority: Restore env &gt; Backup env &gt; ActionSet env.</p>
<p>Purpose: Some databases require different configurations when being restored as a standby
compared to being restored as a primary.
For example, when restoring MySQL as a replica, you need to set <code>skip_slave_start=&quot;ON&quot;</code> for 5.7
or <code>skip_replica_start=&quot;ON&quot;</code> for 8.0.
Allowing environment variables to be passed in makes it more convenient to control these behavioral differences
during the restore process.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
</p>
<div>
<p>Reconfigure defines the parameters for updating a Component&rsquo;s configuration.</p>
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ParameterPair">
[]ParameterPair
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of key-value pairs representing parameters and their corresponding values
within a single configuration file.
This field is used to override or set the values of parameters without modifying the entire configuration file.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.RefNamespaceName">RefNamespaceName
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.BackupRefSpec">BackupRefSpec</a>, <a href="#operations.kubeblocks.io/v1alpha1.PointInTimeRefSpec">PointInTimeRefSpec</a>)
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
<p>Refers to the specific name of the resource.</p>
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
<em>(Optional)</em>
<p>Refers to the specific namespace of the resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ReplicaChanger">ReplicaChanger
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.ScaleIn">ScaleIn</a>, <a href="#operations.kubeblocks.io/v1alpha1.ScaleOut">ScaleOut</a>)
</p>
<div>
<p>ReplicaChanger defines the parameters for changing the number of replicas.</p>
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
<code>replicaChanges</code><br/>
<em>
int32
</em>
</td>
<td>
<p>Specifies the replica changes for the component.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.InstanceReplicasTemplate">
[]InstanceReplicasTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Modifies the desired replicas count for existing InstanceTemplate.
if the inst</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Restore">Restore
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>backupName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the Backup custom resource.</p>
</td>
</tr>
<tr>
<td>
<code>backupNamespace</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the namespace of the backup custom resource. If not specified, the namespace of the opsRequest will be used.</p>
</td>
</tr>
<tr>
<td>
<code>restorePointInTime</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the point in time to which the restore should be performed.
Supported time formats:</p>
<ul>
<li>RFC3339 format, e.g. &ldquo;2023-11-25T18:52:53Z&rdquo;</li>
<li>A human-readable date-time format, e.g. &ldquo;Jul 25,2023 18:52:53 UTC+0800&rdquo;</li>
</ul>
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
<code>volumeRestorePolicy</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the policy for restoring volume claims of a Component&rsquo;s Pods.
It determines whether the volume claims should be restored sequentially (one by one) or in parallel (all at once).
Support values:</p>
<ul>
<li>&ldquo;Serial&rdquo;</li>
<li>&ldquo;Parallel&rdquo;</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>deferPostReadyUntilClusterRunning</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Controls the timing of PostReady actions during the recovery process.</p>
<p>If false (default), PostReady actions execute when the Component reaches the &ldquo;Running&rdquo; state.
If true, PostReady actions are delayed until the entire Cluster is &ldquo;Running,&rdquo;
ensuring the cluster&rsquo;s overall stability before proceeding.</p>
<p>This setting is useful for coordinating PostReady operations across the Cluster for optimal cluster conditions.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.ParameterPair
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of name-value pairs representing parameters and their corresponding values.
Parameters match the schema specified in the <code>actionset.spec.parametersSchema</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Rule">Rule
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.PreCondition">PreCondition</a>)
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
<code>expression</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies a Go template expression that determines how the operation can be executed.
The return value must be either <code>true</code> or <code>false</code>.
Available built-in objects that can be referenced in the expression include:</p>
<ul>
<li><code>params</code>: Input parameters.</li>
<li><code>cluster</code>: The referenced Cluster object.</li>
<li><code>component</code>: The referenced Component object.</li>
</ul>
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
<p>Specifies the error or status message reported if the <code>expression</code> does not evaluate to <code>true</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ScaleIn">ScaleIn
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>)
</p>
<div>
<p>ScaleIn defines the configuration for a scale-in operation.</p>
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
<code>ReplicaChanger</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ReplicaChanger">
ReplicaChanger
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReplicaChanger</code> are embedded into this type.)
</p>
<p>Modifies the replicas of the component and instance templates.</p>
</td>
</tr>
<tr>
<td>
<code>onlineInstancesToOffline</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the instance names that need to be taken offline.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.ScaleOut">ScaleOut
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>)
</p>
<div>
<p>ScaleOut defines the configuration for a scale-out operation.</p>
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
<code>ReplicaChanger</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ReplicaChanger">
ReplicaChanger
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReplicaChanger</code> are embedded into this type.)
</p>
<p>Modifies the replicas of the component and instance templates.</p>
</td>
</tr>
<tr>
<td>
<code>newInstances</code><br/>
<em>
[]github.com/apecloud/kubeblocks/apis/apps/v1.InstanceTemplate
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the configuration for new instances added during scaling, including resource requirements, labels, annotations, etc.
New instances are created based on the provided instance templates.</p>
</td>
</tr>
<tr>
<td>
<code>offlineInstancesToOnline</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the instances in the offline list to bring back online.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
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
<code>upgrade</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Upgrade">
Upgrade
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the desired new version of the Cluster.</p>
<p>Note: This field is immutable once set.</p>
</td>
</tr>
<tr>
<td>
<code>horizontalScaling</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.HorizontalScaling">
[]HorizontalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists HorizontalScaling objects, each specifying scaling requirements for a Component,
including desired replica changes, configurations for new instances, modifications for existing instances,
and take offline/online the specified instances.</p>
</td>
</tr>
<tr>
<td>
<code>volumeExpansion</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.VolumeExpansion">
[]VolumeExpansion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists VolumeExpansion objects, each specifying a component and its corresponding volumeClaimTemplates
that requires storage expansion.</p>
</td>
</tr>
<tr>
<td>
<code>start</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
[]ComponentOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Components to be started. If empty, all components will be started.</p>
</td>
</tr>
<tr>
<td>
<code>stop</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
[]ComponentOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Components to be stopped. If empty, all components will be stopped.</p>
</td>
</tr>
<tr>
<td>
<code>restart</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
[]ComponentOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Components to be restarted.</p>
</td>
</tr>
<tr>
<td>
<code>switchover</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Switchover">
[]Switchover
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Switchover objects, each specifying a Component to perform the switchover operation.</p>
</td>
</tr>
<tr>
<td>
<code>verticalScaling</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.VerticalScaling">
[]VerticalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists VerticalScaling objects, each specifying a component and its desired compute resources for vertical scaling.</p>
</td>
</tr>
<tr>
<td>
<code>reconfigures</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Reconfigure">
[]Reconfigure
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Reconfigure objects, each specifying a Component and its configuration updates.</p>
</td>
</tr>
<tr>
<td>
<code>expose</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Expose">
[]Expose
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists Expose objects, each specifying a Component and its services to be exposed.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Backup">
Backup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters to back up a Cluster.</p>
</td>
</tr>
<tr>
<td>
<code>restore</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.Restore">
Restore
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters to restore a Cluster.
Note that this restore operation will roll back cluster services.</p>
</td>
</tr>
<tr>
<td>
<code>rebuildFrom</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.RebuildInstance">
[]RebuildInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the parameters to rebuild some instances.
Rebuilding an instance involves restoring its data from a backup or another database replica.
The instances being rebuilt usually serve as standby in the cluster.
Hence, rebuilding instances is often also referred to as &ldquo;standby reconstruction&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>custom</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.CustomOps">
CustomOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a custom operation defined by OpsDefinition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Switchover">Switchover
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
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
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the Component as defined in the cluster.spec.</p>
</td>
</tr>
<tr>
<td>
<code>componentObjectName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the Component object.</p>
</td>
</tr>
<tr>
<td>
<code>instanceName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the instance whose role will be transferred. A typical usage is to transfer the leader role
in a consensus system.</p>
</td>
</tr>
<tr>
<td>
<code>candidateName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If CandidateName is specified, the role will be transferred to this instance.
The name must match one of the pods in the component.
Refer to ComponentDefinition&rsquo;s Swtichover lifecycle action for more details.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.TypedObjectRef">TypedObjectRef
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
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
<code>apiGroup</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the group for the resource being referenced.
If not specified, the referenced Kind must belong to the core API group.
For all third-party types, this is mandatory.</p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the type of resource being referenced.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Indicates the name of the resource being referenced.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.UpdatedParameters">UpdatedParameters
</h3>
<div>
<p>UpdatedParameters holds details about the modifications made to configuration parameters.
Example:</p>
<pre><code class="language-yaml">updatedParameters:
	updatedKeys:
	  my.cnf: '&#123;&quot;mysqld&quot;:&#123;&quot;max_connections&quot;:&quot;100&quot;&#125;&#125;'
</code></pre>
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
<code>addedKeys</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maps newly added configuration files to their content.</p>
</td>
</tr>
<tr>
<td>
<code>deletedKeys</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists the name of configuration files that have been deleted.</p>
</td>
</tr>
<tr>
<td>
<code>updatedKeys</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maps the name of configuration files to their updated content, detailing the changes made.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.Upgrade">Upgrade
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
</p>
<div>
<p>Upgrade defines the parameters for an upgrade operation.</p>
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
<code>components</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.UpgradeComponent">
[]UpgradeComponent
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lists components to be upgrade based on desired ComponentDefinition and ServiceVersion.
From the perspective of cluster API, the reasonable combinations should be:
1. (comp-def, service-ver) - upgrade to the specified service version and component definition, the user takes the responsibility to ensure that they are compatible.
2. (&ldquo;&rdquo;, service-ver) - upgrade to the specified service version, let the operator choose the latest compatible component definition.
3. (comp-def, &ldquo;&rdquo;) - upgrade to the specified component definition, let the operator choose the latest compatible service version.
4. (&ldquo;&rdquo;, &ldquo;&rdquo;) - upgrade to the latest service version and component definition, the operator will ensure the compatibility between the selected versions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.UpgradeComponent">UpgradeComponent
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.Upgrade">Upgrade</a>)
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>componentDefinitionName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the ComponentDefinition, only exact matches are supported.</p>
</td>
</tr>
<tr>
<td>
<code>serviceVersion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the version of the Service expected to be provisioned by this Component.
Referring to the ServiceVersion defined by the ComponentDefinition and ComponentVersion.
And ServiceVersion in ClusterComponentSpec is optional, when no version is specified,
use the latest available version in ComponentVersion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
</p>
<div>
<p>VerticalScaling refers to the process of adjusting compute resources (e.g., CPU, memory) allocated to a Component.
It defines the parameters required for the operation.</p>
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>ResourceRequirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceRequirements</code> are embedded into this type.)
</p>
<p>Defines the desired compute resources of the Component&rsquo;s instances.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.InstanceResourceTemplate">
[]InstanceResourceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the desired compute resources of the instance template that need to vertical scale.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion
</h3>
<p>
(<em>Appears on:</em><a href="#operations.kubeblocks.io/v1alpha1.SpecificOpsRequest">SpecificOpsRequest</a>)
</p>
<div>
<p>VolumeExpansion encapsulates the parameters required for a volume expansion operation.</p>
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
<code>ComponentOps</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
<p>Specifies the name of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#operations.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">
[]OpsRequestVolumeClaimTemplate
</a>
</em>
</td>
<td>
<p>Specifies a list of OpsRequestVolumeClaimTemplate objects, defining the volumeClaimTemplates
that are used to expand the storage and the desired storage size for each one.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>