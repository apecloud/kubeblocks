---
title: Add-On API Reference
description: Add-On API Reference
keywords: [add-on, api]
sidebar_position: 3
sidebar_label: Add-On
---
<br />
<p>Packages:</p>
<ul>
<li>
<a href="#workloads.kubeblocks.io%2fv1alpha1">workloads.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="workloads.kubeblocks.io/v1alpha1">workloads.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachine">ReplicatedStateMachine</a>
</li></ul>
<h3 id="workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachine">ReplicatedStateMachine
</h3>
<div>
<p>ReplicatedStateMachine is the Schema for the replicatedstatemachines API.</p>
</div>
<table>
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
<code>workloads.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ReplicatedStateMachine</code></td>
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
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">
ReplicatedStateMachineSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>replicas is the desired number of replicas of the given Template.
These are replicas in the sense that they are instantiations of the
same Template, but individual replicas also have a consistent identity.
If unspecified, defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum number of seconds for which a newly created pod should be ready
without any of its container crashing for it to be considered available.
Defaults to 0 (pod will be considered available as soon as it is ready)</p>
</td>
</tr>
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
<p>selector is a label query over pods that should match the replica count.
It must match the pod template&rsquo;s labels.
More info: <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors">https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors</a></p>
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
<p>serviceName is the name of the service that governs this StatefulSet.
This service must exist before the StatefulSet, and is responsible for
the network identity of the set. Pods get DNS/hostnames that follow the
pattern: pod-specific-string.serviceName.default.svc.cluster.local
where &ldquo;pod-specific-string&rdquo; is managed by the StatefulSet controller.</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#service-v1-core">
Kubernetes core/v1.Service
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>service defines the behavior of a service spec.
provides read-write service
<a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
</td>
</tr>
<tr>
<td>
<code>alternativeServices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#service-v1-core">
[]Kubernetes core/v1.Service
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AlternativeServices defines Alternative Services selector pattern specifier.
can be used for creating Readonly service.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podtemplatespec-v1-core">
Kubernetes core/v1.PodTemplateSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumeclaim-v1-core">
[]Kubernetes core/v1.PersistentVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeClaimTemplates is a list of claims that pods are allowed to reference.
The ReplicatedStateMachine controller is responsible for mapping network identities to
claims in a way that maintains the identity of a pod. Every claim in
this list must have at least one matching (by name) volumeMount in one
container in the template. A claim in this list takes precedence over
any volumes in the template, with the same name.</p>
</td>
</tr>
<tr>
<td>
<code>podManagementPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podmanagementpolicytype-v1-apps">
Kubernetes apps/v1.PodManagementPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>podManagementPolicy controls how pods are created during initial scale up,
when replacing pods on nodes, or when scaling down. The default policy is
<code>OrderedReady</code>, where pods are created in increasing order (pod-0, then
pod-1, etc) and the controller will wait until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.
The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#statefulsetupdatestrategy-v1-apps">
Kubernetes apps/v1.StatefulSetUpdateStrategy
</a>
</em>
</td>
<td>
<p>updateStrategy indicates the StatefulSetUpdateStrategy that will be
employed to update Pods in the RSM when a revision is made to
Template.
UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicaRole">
[]ReplicaRole
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles, a list of roles defined in the system.</p>
</td>
</tr>
<tr>
<td>
<code>roleProbe</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.RoleProbe">
RoleProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleProbe provides method to probe role.</p>
</td>
</tr>
<tr>
<td>
<code>membershipReconfiguration</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.MembershipReconfiguration">
MembershipReconfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MembershipReconfiguration provides actions to do membership dynamic reconfiguration.</p>
</td>
</tr>
<tr>
<td>
<code>memberUpdateStrategy</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.MemberUpdateStrategy">
MemberUpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberUpdateStrategy, Members(Pods) update strategy.
serial: update Members one by one that guarantee minimum component unavailable time.
Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader
bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader, keep majority online all the time.
parallel: force parallel</p>
</td>
</tr>
<tr>
<td>
<code>paused</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Paused indicates that the rsm is paused, means the reconciliation of this rsm object will be paused.</p>
</td>
</tr>
<tr>
<td>
<code>credential</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Credential">
Credential
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credential used to connect to DB engine</p>
</td>
</tr>
<tr>
<td>
<code>rsmTransformPolicy</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.RsmTransformPolicy">
RsmTransformPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RsmTransformPolicy defines the policy generate sts using rsm. Passed from cluster.
ToSts: rsm transform to statefulSet
ToPod: rsm transform to pod</p>
</td>
</tr>
<tr>
<td>
<code>nodeAssignment</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.NodeAssignment">
[]NodeAssignment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeAssignment defines the expected assignment of nodes.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineStatus">
ReplicatedStateMachineStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.AccessMode">AccessMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicaRole">ReplicaRole</a>)
</p>
<div>
<p>AccessMode defines SVC access mode enums.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;None&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ReadWrite&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Readonly&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.Action">Action
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.MembershipReconfiguration">MembershipReconfiguration</a>, <a href="#workloads.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe</a>)
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
<em>(Optional)</em>
<p>utility image contains command that can be used to retrieve of process role info</p>
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
<p>Command will be executed in Container to retrieve or process role info</p>
</td>
</tr>
<tr>
<td>
<code>args</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Args is used to perform statements.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.Credential">Credential
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
<code>username</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<p>Username
variable name will be KB_RSM_USERNAME</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<p>Password
variable name will be KB_RSM_PASSWORD</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.CredentialVar">CredentialVar
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.Credential">Credential</a>)
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
<code>value</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Variable references $(VAR_NAME) are expanded
using the previously defined environment variables in the container and
any service environment variables. If a variable cannot be resolved,
the reference in the input string will be unchanged. Double $$ are reduced
to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
&ldquo;$$(VAR_NAME)&rdquo; will produce the string literal &ldquo;$(VAR_NAME)&rdquo;.
Escaped references will never be expanded, regardless of whether the variable
exists or not.
Defaults to &ldquo;&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvarsource-v1-core">
Kubernetes core/v1.EnvVarSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source for the environment variable&rsquo;s value. Cannot be used if value is not empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.MemberStatus">MemberStatus
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineStatus">ReplicatedStateMachineStatus</a>)
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
<code>podName</code><br/>
<em>
string
</em>
</td>
<td>
<p>PodName pod name.</p>
</td>
</tr>
<tr>
<td>
<code>role</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicaRole">
ReplicaRole
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>readyWithoutPrimary</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Is it required for rsm to have at least one primary pod to be ready.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.MemberUpdateStrategy">MemberUpdateStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
</p>
<div>
<p>MemberUpdateStrategy defines Cluster Component update strategy.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;BestEffortParallel&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Parallel&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Serial&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.MembershipReconfiguration">MembershipReconfiguration
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
<code>switchoverAction</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Environment variables can be used in all following Actions:</p>
<ul>
<li>KB_RSM_USERNAME username part of credential</li>
<li>KB_RSM_PASSWORD password part of credential</li>
<li>KB_RSM_LEADER_HOST leader host</li>
<li>KB_RSM_TARGET_HOST target host</li>
<li>KB_RSM_SERVICE_PORT port</li>
</ul>
<p>SwitchoverAction specifies how to do switchover
latest <a href="https://busybox.net/">BusyBox</a> image will be used if Image not configured</p>
</td>
</tr>
<tr>
<td>
<code>memberJoinAction</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberJoinAction specifies how to add member
previous none-nil action&rsquo;s Image will be used if not configured</p>
</td>
</tr>
<tr>
<td>
<code>memberLeaveAction</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberLeaveAction specifies how to remove member
previous none-nil action&rsquo;s Image will be used if not configured</p>
</td>
</tr>
<tr>
<td>
<code>logSyncAction</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogSyncAction specifies how to trigger the new member to start log syncing
previous none-nil action&rsquo;s Image will be used if not configured</p>
</td>
</tr>
<tr>
<td>
<code>promoteAction</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PromoteAction specifies how to tell the cluster that the new member can join voting now
previous none-nil action&rsquo;s Image will be used if not configured</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.NodeAssignment">NodeAssignment
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
<p>Name defines the name of statefulSet that needs to allocate node.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSpec</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.NodeSpec">
NodeSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSpec defines the detailed node info that will assign to the statefulSet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.NodeSpec">NodeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.NodeAssignment">NodeAssignment</a>)
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
<code>nodeName</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#NodeName">
k8s.io/apimachinery/pkg/types.NodeName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.ReplicaRole">ReplicaRole
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.MemberStatus">MemberStatus</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
<p>Name, role name.</p>
</td>
</tr>
<tr>
<td>
<code>accessMode</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.AccessMode">
AccessMode
</a>
</em>
</td>
<td>
<p>AccessMode, what service this member capable.</p>
</td>
</tr>
<tr>
<td>
<code>canVote</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>CanVote, whether this member has voting rights</p>
</td>
</tr>
<tr>
<td>
<code>isLeader</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>IsLeader, whether this member is the leader</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachine">ReplicatedStateMachine</a>)
</p>
<div>
<p>ReplicatedStateMachineSpec defines the desired state of ReplicatedStateMachine</p>
</div>
<table>
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
<p>replicas is the desired number of replicas of the given Template.
These are replicas in the sense that they are instantiations of the
same Template, but individual replicas also have a consistent identity.
If unspecified, defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>minReadySeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum number of seconds for which a newly created pod should be ready
without any of its container crashing for it to be considered available.
Defaults to 0 (pod will be considered available as soon as it is ready)</p>
</td>
</tr>
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
<p>selector is a label query over pods that should match the replica count.
It must match the pod template&rsquo;s labels.
More info: <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors">https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors</a></p>
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
<p>serviceName is the name of the service that governs this StatefulSet.
This service must exist before the StatefulSet, and is responsible for
the network identity of the set. Pods get DNS/hostnames that follow the
pattern: pod-specific-string.serviceName.default.svc.cluster.local
where &ldquo;pod-specific-string&rdquo; is managed by the StatefulSet controller.</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#service-v1-core">
Kubernetes core/v1.Service
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>service defines the behavior of a service spec.
provides read-write service
<a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
</td>
</tr>
<tr>
<td>
<code>alternativeServices</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#service-v1-core">
[]Kubernetes core/v1.Service
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AlternativeServices defines Alternative Services selector pattern specifier.
can be used for creating Readonly service.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podtemplatespec-v1-core">
Kubernetes core/v1.PodTemplateSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumeclaim-v1-core">
[]Kubernetes core/v1.PersistentVolumeClaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeClaimTemplates is a list of claims that pods are allowed to reference.
The ReplicatedStateMachine controller is responsible for mapping network identities to
claims in a way that maintains the identity of a pod. Every claim in
this list must have at least one matching (by name) volumeMount in one
container in the template. A claim in this list takes precedence over
any volumes in the template, with the same name.</p>
</td>
</tr>
<tr>
<td>
<code>podManagementPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podmanagementpolicytype-v1-apps">
Kubernetes apps/v1.PodManagementPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>podManagementPolicy controls how pods are created during initial scale up,
when replacing pods on nodes, or when scaling down. The default policy is
<code>OrderedReady</code>, where pods are created in increasing order (pod-0, then
pod-1, etc) and the controller will wait until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.
The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#statefulsetupdatestrategy-v1-apps">
Kubernetes apps/v1.StatefulSetUpdateStrategy
</a>
</em>
</td>
<td>
<p>updateStrategy indicates the StatefulSetUpdateStrategy that will be
employed to update Pods in the RSM when a revision is made to
Template.
UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.ReplicaRole">
[]ReplicaRole
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles, a list of roles defined in the system.</p>
</td>
</tr>
<tr>
<td>
<code>roleProbe</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.RoleProbe">
RoleProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleProbe provides method to probe role.</p>
</td>
</tr>
<tr>
<td>
<code>membershipReconfiguration</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.MembershipReconfiguration">
MembershipReconfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MembershipReconfiguration provides actions to do membership dynamic reconfiguration.</p>
</td>
</tr>
<tr>
<td>
<code>memberUpdateStrategy</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.MemberUpdateStrategy">
MemberUpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberUpdateStrategy, Members(Pods) update strategy.
serial: update Members one by one that guarantee minimum component unavailable time.
Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader
bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader, keep majority online all the time.
parallel: force parallel</p>
</td>
</tr>
<tr>
<td>
<code>paused</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Paused indicates that the rsm is paused, means the reconciliation of this rsm object will be paused.</p>
</td>
</tr>
<tr>
<td>
<code>credential</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Credential">
Credential
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credential used to connect to DB engine</p>
</td>
</tr>
<tr>
<td>
<code>rsmTransformPolicy</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.RsmTransformPolicy">
RsmTransformPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RsmTransformPolicy defines the policy generate sts using rsm. Passed from cluster.
ToSts: rsm transform to statefulSet
ToPod: rsm transform to pod</p>
</td>
</tr>
<tr>
<td>
<code>nodeAssignment</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.NodeAssignment">
[]NodeAssignment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeAssignment defines the expected assignment of nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineStatus">ReplicatedStateMachineStatus
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachine">ReplicatedStateMachine</a>)
</p>
<div>
<p>ReplicatedStateMachineStatus defines the observed state of ReplicatedStateMachine</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>StatefulSetStatus</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#statefulsetstatus-v1-apps">
Kubernetes apps/v1.StatefulSetStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>StatefulSetStatus</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>initReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>InitReplicas is the number of pods(members) when cluster first initialized
it&rsquo;s set to spec.Replicas at object creation time and never changes</p>
</td>
</tr>
<tr>
<td>
<code>readyInitReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReadyInitReplicas is the number of pods(members) already in MembersStatus in the cluster initialization stage
will never change once equals to InitReplicas</p>
</td>
</tr>
<tr>
<td>
<code>currentGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>CurrentGeneration, if not empty, indicates the version of the RSM used to generate the underlying workload</p>
</td>
</tr>
<tr>
<td>
<code>membersStatus</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.MemberStatus">
[]MemberStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>members&rsquo; status.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
</p>
<div>
<p>RoleProbe defines how to observe role</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>builtinHandlerName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>BuiltinHandler specifies the builtin handler name to use to probe the role of the main container.
current available handlers: mysql, postgres, mongodb, redis, etcd, kafka.
use CustomHandler to define your own role probe function if none of them satisfies the requirement.</p>
</td>
</tr>
<tr>
<td>
<code>customHandler</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.Action">
[]Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CustomHandler defines the custom way to do role probe.
if the BuiltinHandler satisfies the requirement, use it instead.</p>
<p>how the actions defined here works:</p>
<p>Actions will be taken in serial.
after all actions done, the final output should be a single string of the role name defined in spec.Roles
latest <a href="https://busybox.net/">BusyBox</a> image will be used if Image not configured
Environment variables can be used in Command:</p>
<ul>
<li>v_KB_RSM_LAST<em>STDOUT stdout from last action, watch &lsquo;v</em>&rsquo; prefixed</li>
<li>KB_RSM_USERNAME username part of credential</li>
<li>KB_RSM_PASSWORD password part of credential</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>initialDelaySeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number of seconds after the container has started before role probe has started.</p>
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
<p>Number of seconds after which the probe times out.
Defaults to 1 second. Minimum value is 1.</p>
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
<p>How often (in seconds) to perform the probe.
Default to 2 seconds. Minimum value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>successThreshold</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum consecutive successes for the probe to be considered successful after having failed.
Defaults to 1. Minimum value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>failureThreshold</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Minimum consecutive failures for the probe to be considered failed after having succeeded.
Defaults to 3. Minimum value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>roleUpdateMechanism</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.RoleUpdateMechanism">
RoleUpdateMechanism
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleUpdateMechanism specifies the way how pod role label being updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.RoleUpdateMechanism">RoleUpdateMechanism
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe</a>)
</p>
<div>
<p>RoleUpdateMechanism defines the way how pod role label being updated.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;DirectAPIServerEventUpdate&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ReadinessProbeEventUpdate&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.RsmTransformPolicy">RsmTransformPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
</p>
<div>
<p>RsmTransformPolicy defines rsm transform type
ToSts and ToPod is supported</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ToPod&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ToSts&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
