---
title: Cluster API Reference
description: Cluster API Reference
keywords: [cluster, api]
sidebar_position: 1
sidebar_label: Cluster
---
<br />
<p>Packages:</p>
<ul>
<li>
<a href="#apps.kubeblocks.io%2fv1alpha1">apps.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="apps.kubeblocks.io/v1alpha1">apps.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.Cluster">Cluster</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinition">ClusterDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamily">ClusterFamily</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplate">ClusterTemplate</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinition">ComponentClassDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraint">ComponentResourceConstraint</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>
</li></ul>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate
</h3>
<div>
<p>BackupPolicyTemplate is the Schema for the BackupPolicyTemplates API (defined by provider)</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>BackupPolicyTemplate</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">
BackupPolicyTemplateSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupPolicies</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">
[]BackupPolicy
</a>
</em>
</td>
<td>
<p>backupPolicies is a list of backup policy template for the specified componentDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>identifier</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identifier is a unique identifier for this BackupPolicyTemplate.<br />this identifier will be the suffix of the automatically generated backupPolicy name.<br />and must be added when multiple BackupPolicyTemplates exist,<br />otherwise the generated backupPolicy override will occur.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateStatus">
BackupPolicyTemplateStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Cluster">Cluster
</h3>
<div>
<p>Cluster is the Schema for the clusters API.</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Cluster</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">
ClusterSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Cluster referencing ClusterDefinition name. This is an immutable attribute.</p><br />
</td>
</tr>
<tr>
<td>
<code>clusterVersionRef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster referencing ClusterVersion name.</p><br />
</td>
</tr>
<tr>
<td>
<code>terminationPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TerminationPolicyType">
TerminationPolicyType
</a>
</em>
</td>
<td>
<p>Cluster termination policy. Valid values are DoNotTerminate, Halt, Delete, WipeOut.<br />DoNotTerminate will block delete operation.<br />Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.<br />Delete is based on Halt and deletes PVCs.<br />WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">
[]ClusterComponentSpec
</a>
</em>
</td>
<td>
<p>List of componentSpecs you want to replace in ClusterDefinition and ClusterVersion. It will replace the field in ClusterDefinition&rsquo;s and ClusterVersion&rsquo;s component if type is matching.</p><br />
</td>
</tr>
<tr>
<td>
<code>tenancy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TenancyType">
TenancyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>tenancy describes how pods are distributed across node.<br />SharedNode means multiple pods may share the same node.<br />DedicatedNode means each pod runs on their own dedicated node.</p><br />
</td>
</tr>
<tr>
<td>
<code>availabilityPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.AvailabilityPolicyType">
AvailabilityPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>availabilityPolicy describes the availability policy, including zone, node, and none.</p><br />
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Affinity">
Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>affinity is a group of affinity scheduling rules.</p><br />
</td>
</tr>
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
<p>tolerations are attached to tolerate any taint that matches the triple <code>key,value,effect</code> using the matching operator <code>operator</code>.</p><br />
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
<em>(Optional)</em>
<p>replicas specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterResources">
ClusterResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>resources specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>storage</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterStorage">
ClusterStorage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>storage specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterMode">
ClusterMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>mode specifies the mode of this cluster, the value of this can be one of the following: Standalone, Replication, RaftGroup.</p><br />
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>customized parameters that is used in different clusterdefinition</p><br />
</td>
</tr>
<tr>
<td>
<code>monitor</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterMonitor">
ClusterMonitor
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>monitor specifies the configuration of monitor</p><br />
</td>
</tr>
<tr>
<td>
<code>network</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterNetwork">
ClusterNetwork
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>network specifies the configuration of network</p><br />
</td>
</tr>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterBackup">
ClusterBackup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>cluster backup configuration.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterStatus">
ClusterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinition">ClusterDefinition
</h3>
<div>
<p>ClusterDefinition is the Schema for the clusterdefinitions API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClusterDefinition</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionSpec">
ClusterDefinitionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster definition type defines well known application cluster type, e.g. mysql/redis/mongodb</p><br />
</td>
</tr>
<tr>
<td>
<code>componentDefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">
[]ClusterComponentDefinition
</a>
</em>
</td>
<td>
<p>componentDefs provides cluster components definitions.</p><br />
</td>
</tr>
<tr>
<td>
<code>connectionCredential</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Connection credential template used for creating a connection credential<br />secret for cluster.apps.kubeblocks.io object.</p><br /><br /><p>Built-in objects are:<br />- <code>$(RANDOM_PASSWD)</code> - random 8 characters.<br />- <code>$(UUID)</code> - generate a random UUID v4 string.<br />- <code>$(UUID_B64)</code> - generate a random UUID v4 BASE64 encoded string.<br />- <code>$(UUID_STR_B64)</code> - generate a random UUID v4 string then BASE64 encoded.<br />- <code>$(UUID_HEX)</code> - generate a random UUID v4 HEX representation.<br />- <code>$(HEADLESS_SVC_FQDN)</code> - headless service FQDN placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc,<br />   where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;<br />- <code>$(SVC_FQDN)</code> - service FQDN  placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,<br />   where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;<br />- <code>$(SVC_PORT_&#123;PORT-NAME&#125;)</code> - a ServicePort&rsquo;s port value with specified port name, i.e, a servicePort JSON struct:<br />   <code>&#123;&quot;name&quot;: &quot;mysql&quot;, &quot;targetPort&quot;: &quot;mysqlContainerPort&quot;, &quot;port&quot;: 3306&#125;</code>, and &ldquo;$(SVC_PORT_mysql)&rdquo; in the<br />   connection credential value is 3306.</p><br />
</td>
</tr>
<tr>
<td>
<code>clusterFamilyRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>reference name of clusterfamily</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionStatus">
ClusterDefinitionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterFamily">ClusterFamily
</h3>
<div>
<p>ClusterFamily is the Schema for the clusterfamilies API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClusterFamily</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilySpec">
ClusterFamilySpec
</a>
</em>
</td>
<td>
<p>spec of clusterfamily which defines what kind of clustertemplate to use in some conditions</p><br />
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterTemplateRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRef">
[]ClusterFamilyTemplateRef
</a>
</em>
</td>
<td>
<p>list of clustertemplates</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilyStatus">
ClusterFamilyStatus
</a>
</em>
</td>
<td>
<p>status of clusterfamily</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTemplate">ClusterTemplate
</h3>
<div>
<p>ClusterTemplate is the Schema for the clustertemplates API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClusterTemplate</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplateSpec">
ClusterTemplateSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>componentSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">
[]ClusterComponentSpec
</a>
</em>
</td>
<td>
<p>default value of cluster.spec.componentSpecs</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplateStatus">
ClusterTemplateStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion
</h3>
<div>
<p>ClusterVersion is the Schema for the ClusterVersions API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ClusterVersion</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionSpec">
ClusterVersionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>ref ClusterDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentVersions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">
[]ClusterComponentVersion
</a>
</em>
</td>
<td>
<p>List of components&rsquo; containers versioning context, i.e., container image ID, container commands, args., and environments.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionStatus">
ClusterVersionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClassDefinition">ComponentClassDefinition
</h3>
<div>
<p>ComponentClassDefinition is the Schema for the componentclassdefinitions API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ComponentClassDefinition</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionSpec">
ComponentClassDefinitionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>groups</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassGroup">
[]ComponentClassGroup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>group defines a list of class series that conform to the same constraint.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionStatus">
ComponentClassDefinitionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceConstraint">ComponentResourceConstraint
</h3>
<div>
<p>ComponentResourceConstraint is the Schema for the componentresourceconstraints API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ComponentResourceConstraint</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSpec">
ComponentResourceConstraintSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>rules</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">
[]ResourceConstraintRule
</a>
</em>
</td>
<td>
<p>Component resource constraint rules.</p><br />
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterResourceConstraintSelector">
[]ClusterResourceConstraintSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>selector is used to bind the resource constraint to cluster definitions.</p><br />
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint
</h3>
<div>
<p>ConfigConstraint is the Schema for the configconstraint API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ConfigConstraint</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">
ConfigConstraintSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>reloadOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">
ReloadOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reloadOptions indicates whether the process supports reload.<br />if set, the controller will determine the behavior of the engine instance based on the configuration templates,<br />restart or reload depending on whether any parameters in the StaticParameters have been modified.</p><br />
</td>
</tr>
<tr>
<td>
<code>toolsImageSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ToolsImageSpec">
ToolsImageSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>toolConfig used to config init container.</p><br />
</td>
</tr>
<tr>
<td>
<code>downwardAPIOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.DownwardAPIOption">
[]DownwardAPIOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>downwardAPIOptions is used to watch pod fields.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptConfigs, list of ScriptConfig, witch these scripts can be used by volume trigger,downward trigger, or tool image</p><br />
</td>
</tr>
<tr>
<td>
<code>cfgSchemaTopLevelName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>cfgSchemaTopLevelName is cue type name, which generates openapi schema.</p><br />
</td>
</tr>
<tr>
<td>
<code>configurationSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomParametersValidation">
CustomParametersValidation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configurationSchema imposes restrictions on database parameter&rsquo;s rule.</p><br />
</td>
</tr>
<tr>
<td>
<code>staticParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>staticParameters, list of StaticParameter, modifications of them trigger a process restart.</p><br />
</td>
</tr>
<tr>
<td>
<code>dynamicParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.</p><br />
</td>
</tr>
<tr>
<td>
<code>immutableParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>immutableParameters describes parameters that prohibit user from modification.</p><br />
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
<p>selector is used to match the label on the pod,<br />for example, a pod of the primary is match on the patroni cluster.</p><br />
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>formatterConfig describes the format of the configuration file, the controller<br />1. parses configuration file<br />2. analyzes the modified parameters<br />3. applies corresponding policies.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintStatus">
ConfigConstraintStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest
</h3>
<div>
<p>OpsRequest is the Schema for the opsrequests API</p><br />
</div>
<table>
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
<code>apps.kubeblocks.io/v1alpha1</code>
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
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">
OpsRequestSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterRef references clusterDefinition.</p><br />
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
<p>cancel defines the action to cancel the Pending/Creating/Running opsRequest, supported types: [VerticalScaling, HorizontalScaling].<br />once cancel is set to true, this opsRequest will be canceled and modifying this property again will not take effect.</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>type defines the operation type.</p><br />
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
<p>ttlSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.</p><br />
</td>
</tr>
<tr>
<td>
<code>upgrade</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Upgrade">
Upgrade
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>upgrade specifies the cluster version by specifying clusterVersionRef.</p><br />
</td>
</tr>
<tr>
<td>
<code>horizontalScaling</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HorizontalScaling">
[]HorizontalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>horizontalScaling defines what component need to horizontal scale the specified replicas.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeExpansion</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VolumeExpansion">
[]VolumeExpansion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeExpansion defines what component and volumeClaimTemplate need to expand the specified storage.</p><br />
</td>
</tr>
<tr>
<td>
<code>restart</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
[]ComponentOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>restart the specified components.</p><br />
</td>
</tr>
<tr>
<td>
<code>switchover</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Switchover">
[]Switchover
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>switchover the specified components.</p><br />
</td>
</tr>
<tr>
<td>
<code>verticalScaling</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">
[]VerticalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>verticalScaling defines what component need to vertical scale the specified compute resources.</p><br />
</td>
</tr>
<tr>
<td>
<code>reconfigure</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">
Reconfigure
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reconfigure defines the variables that need to input when updating configuration.</p><br />
</td>
</tr>
<tr>
<td>
<code>expose</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Expose">
[]Expose
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>expose defines services the component needs to expose.</p><br />
</td>
</tr>
<tr>
<td>
<code>restoreFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RestoreFromSpec">
RestoreFromSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>cluster RestoreFrom backup or point in time</p><br />
</td>
</tr>
<tr>
<td>
<code>ttlSecondsBeforeAbort</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ttlSecondsBeforeAbort OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.<br />If not specified, the default value is 0, which means that the start-conditions must be met immediately.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">
ScriptSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptSpec defines the script to be executed.</p><br />
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequestStatus">
OpsRequestStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.AccessMode">AccessMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConsensusMember">ConsensusMember</a>, <a href="#apps.kubeblocks.io/v1alpha1.ConsensusMemberStatus">ConsensusMemberStatus</a>)
</p>
<div>
<p>AccessMode defines SVC access mode enums.</p><br />
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
<h3 id="apps.kubeblocks.io/v1alpha1.AccountName">AccountName
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccountConfig">SystemAccountConfig</a>)
</p>
<div>
<p>AccountName defines system account names.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;kbadmin&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;kbdataprotection&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;kbmonitoring&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;kbprobe&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;kbreplicator&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Affinity">Affinity
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<code>podAntiAffinity</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodAntiAffinity">
PodAntiAffinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>podAntiAffinity describes the anti-affinity level of pods within a component.<br />Preferred means try spread pods by <code>TopologyKeys</code>.<br />Required means must spread pods by <code>TopologyKeys</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>topologyKeys</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>topologyKey is the key of node labels.<br />Nodes that have a label with this key and identical values are considered to be in the same topology.<br />It&rsquo;s used as the topology domain for pod anti-affinity and pod spread constraint.<br />Some well-known label keys, such as &ldquo;kubernetes.io/hostname&rdquo; and &ldquo;topology.kubernetes.io/zone&rdquo;<br />are often used as TopologyKey, as well as any other custom label key.</p><br />
</td>
</tr>
<tr>
<td>
<code>nodeLabels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>nodeLabels describes that pods must be scheduled to the nodes with the specified node labels.</p><br />
</td>
</tr>
<tr>
<td>
<code>tenancy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TenancyType">
TenancyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>tenancy describes how pods are distributed across node.<br />SharedNode means multiple pods may share the same node.<br />DedicatedNode means each pod runs on their own dedicated node.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.AvailabilityPolicyType">AvailabilityPolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>AvailabilityPolicyType for cluster affinity policy.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;node&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;none&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;zone&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec</a>)
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
<code>componentDefRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentDefRef references componentDef defined in ClusterDefinition spec. Need to<br />comply with IANA Service Naming rule.</p><br />
</td>
</tr>
<tr>
<td>
<code>retention</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RetentionSpec">
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
<a href="#apps.kubeblocks.io/v1alpha1.Schedule">
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
<a href="#apps.kubeblocks.io/v1alpha1.SnapshotPolicy">
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
<a href="#apps.kubeblocks.io/v1alpha1.CommonBackupPolicy">
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
<a href="#apps.kubeblocks.io/v1alpha1.CommonBackupPolicy">
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
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyHook">BackupPolicyHook
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>)
</p>
<div>
<p>BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupPolicies</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">
[]BackupPolicy
</a>
</em>
</td>
<td>
<p>backupPolicies is a list of backup policy template for the specified componentDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>identifier</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identifier is a unique identifier for this BackupPolicyTemplate.<br />this identifier will be the suffix of the automatically generated backupPolicy name.<br />and must be added when multiple BackupPolicyTemplates exist,<br />otherwise the generated backupPolicy override will occur.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateStatus">BackupPolicyTemplateStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>)
</p>
<div>
<p>BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupRefSpec">BackupRefSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RestoreFromSpec">RestoreFromSpec</a>)
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
<code>ref</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RefNamespaceName">
RefNamespaceName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>specify a reference backup to restore</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupStatusUpdate">BackupStatusUpdate
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.BackupStatusUpdateStage">
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
<h3 id="apps.kubeblocks.io/v1alpha1.BackupStatusUpdateStage">BackupStatusUpdateStage
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupStatusUpdate">BackupStatusUpdate</a>)
</p>
<div>
<p>BackupStatusUpdateStage defines the stage of backup status update.</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.BaseBackupType">BaseBackupType
(<code>string</code> alias)</h3>
<div>
<p>BaseBackupType the base backup type, keep synchronized with the BaseBackupType of the data protection API.</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CommonBackupPolicy">CommonBackupPolicy</a>, <a href="#apps.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.TargetInstance">
TargetInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>target instance for backup.</p><br />
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
<a href="#apps.kubeblocks.io/v1alpha1.BackupStatusUpdate">
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
<h3 id="apps.kubeblocks.io/v1alpha1.CPUConstraint">CPUConstraint
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">ResourceConstraintRule</a>)
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
<code>max</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range<br />must be multiple times of Step. It&rsquo;s useful to define a large number of valid values without defining them one by<br />one. Please see the documentation for Step for some examples.<br />If Slots is specified, Max, Min, and Step are ignored</p><br />
</td>
</tr>
<tr>
<td>
<code>min</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range<br />must be multiple times of Step. It&rsquo;s useful to define a large number of valid values without defining them one by<br />one. Please see the documentation for Step for some examples.<br />If Slots is specified, Max, Min, and Step are ignored</p><br />
</td>
</tr>
<tr>
<td>
<code>step</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum granularity of vcpu cores, [Min, Max] defines a range for valid vcpu cores and the value in this range must be<br />multiple times of Step.<br />For example:<br />1. Min is 2, Max is 8, Step is 2, and the valid vcpu core is &#123;2, 4, 6, 8&#125;.<br />2. Min is 0.5, Max is 2, Step is 0.5, and the valid vcpu core is &#123;0.5, 1, 1.5, 2&#125;.</p><br />
</td>
</tr>
<tr>
<td>
<code>slots</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
[]Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The valid vcpu cores, it&rsquo;s useful if you want to define valid vcpu cores explicitly.<br />If Slots is specified, Max, Min, and Step are ignored</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CfgFileFormat">CfgFileFormat
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.FormatterConfig">FormatterConfig</a>)
</p>
<div>
<p>CfgFileFormat defines formatter of configuration files.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;dotenv&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;hcl&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ini&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;json&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;properties&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;props-plus&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;redis&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;toml&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;xml&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;yaml&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CfgReloadType">CfgReloadType
(<code>string</code> alias)</h3>
<div>
<p>CfgReloadType defines reload method.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;http&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;sql&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;exec&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;tpl&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;signal&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClassDefRef">ClassDefRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>, <a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>)
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
<p>Name refers to the name of the ComponentClassDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>class</code><br/>
<em>
string
</em>
</td>
<td>
<p>Class refers to the name of the class that is defined in the ComponentClassDefinition.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterBackup">ClusterBackup
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<p>enabled defines whether to enable automated backup.</p><br />
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
<p>retentionPeriod is a time string ending with the &rsquo;d&rsquo;|&rsquo;D&rsquo;|&lsquo;h&rsquo;|&lsquo;H&rsquo; character to describe how long<br />the Backup should be retained. if not set, will be retained forever.</p><br />
</td>
</tr>
<tr>
<td>
<code>method</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.BackupMethod
</em>
</td>
<td>
<p>backup method, support: snapshot, backupTool.</p><br />
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
<em>(Optional)</em>
<p>the cron expression for schedule, the timezone is in UTC. see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p><br />
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the backup job<br />if it misses scheduled time for any reason.</p><br />
</td>
</tr>
<tr>
<td>
<code>repoName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>repoName is the name of the backupRepo, if not set, will use the default backupRepo.</p><br />
</td>
</tr>
<tr>
<td>
<code>pitrEnabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>pitrEnabled defines whether to enable point-in-time recovery.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionSpec">ClusterDefinitionSpec</a>)
</p>
<div>
<p>ClusterComponentDefinition provides a workload component specification template,<br />with attributes that strongly work with stateful workloads and day-2 operations<br />behaviors.</p><br />
</div>
<table>
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
<p>A component definition name, this name could be used as default name of <code>Cluster.spec.componentSpecs.name</code>,<br />and so this name is need to conform with same validation rules as <code>Cluster.spec.componentSpecs.name</code>, that<br />is currently comply with IANA Service Naming rule. This name will apply to &ldquo;apps.kubeblocks.io/component-name&rdquo;<br />object label value.</p><br />
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The description of component definition.</p><br />
</td>
</tr>
<tr>
<td>
<code>workloadType</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.WorkloadType">
WorkloadType
</a>
</em>
</td>
<td>
<p>workloadType defines type of the workload.<br />Stateless is a stateless workload type used to describe stateless applications.<br />Stateful is a stateful workload type used to describe common stateful applications.<br />Consensus is a stateful workload type used to describe applications based on consensus protocols, common consensus protocols such as raft and paxos.<br />Replication is a stateful workload type used to describe applications based on the primary-secondary data replication protocol.</p><br />
</td>
</tr>
<tr>
<td>
<code>characterType</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>characterType defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql)<br />KubeBlocks will generate proper monitor configs for well-known characterType when builtIn is true.</p><br />
</td>
</tr>
<tr>
<td>
<code>configSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The configSpec field provided by provider, and<br />finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">
[]ComponentTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The scriptSpec field provided by provider, and<br />finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.</p><br />
</td>
</tr>
<tr>
<td>
<code>probes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbes">
ClusterDefinitionProbes
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>probes setting for healthy checks.</p><br />
</td>
</tr>
<tr>
<td>
<code>monitor</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MonitorConfig">
MonitorConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>monitor is monitoring config which provided by provider.</p><br />
</td>
</tr>
<tr>
<td>
<code>logConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LogConfig">
[]LogConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>logConfigs is detail log file config which provided by provider.</p><br />
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
<em>(Optional)</em>
<p>podSpec define pod spec template of the cluster component.</p><br />
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceSpec">
ServiceSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>service defines the behavior of a service spec.<br />provide read-write service when WorkloadType is Consensus.</p><br />
</td>
</tr>
<tr>
<td>
<code>statelessSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.StatelessSetSpec">
StatelessSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>statelessSpec defines stateless related spec if workloadType is Stateless.</p><br />
</td>
</tr>
<tr>
<td>
<code>statefulSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">
StatefulSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>statefulSpec defines stateful related spec if workloadType is Stateful.</p><br />
</td>
</tr>
<tr>
<td>
<code>consensusSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusSetSpec">
ConsensusSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>consensusSpec defines consensus related spec if workloadType is Consensus, required if workloadType is Consensus.</p><br />
</td>
</tr>
<tr>
<td>
<code>replicationSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicationSetSpec">
ReplicationSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>replicationSpec defines replication related spec if workloadType is Replication.</p><br />
</td>
</tr>
<tr>
<td>
<code>horizontalScalePolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HorizontalScalePolicy">
HorizontalScalePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>horizontalScalePolicy controls the behavior of horizontal scale.</p><br />
</td>
</tr>
<tr>
<td>
<code>systemAccounts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">
SystemAccountSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Statement to create system account.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VolumeTypeSpec">
[]VolumeTypeSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeTypes is used to describe the purpose of the volumes<br />mapping the name of the VolumeMounts in the PodSpec.Container field,<br />such as data volume, log volume, etc.<br />When backing up the volume, the volume can be correctly backed up<br />according to the volumeType.</p><br /><br /><p>For example:<br /> <code>name: data, type: data</code> means that the volume named <code>data</code> is used to store <code>data</code>.<br /> <code>name: binlog, type: log</code> means that the volume named <code>binlog</code> is used to store <code>log</code>.</p><br /><br /><p>NOTE:<br />  When volumeTypes is not defined, the backup function will not be supported,<br />even if a persistent volume has been specified.</p><br />
</td>
</tr>
<tr>
<td>
<code>customLabelSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomLabelSpec">
[]CustomLabelSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>customLabelSpecs is used for custom label tags which you want to add to the component resources.</p><br />
</td>
</tr>
<tr>
<td>
<code>switchoverSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SwitchoverSpec">
SwitchoverSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>switchoverSpec defines command to do switchover.<br />in particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeProtectionSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VolumeProtectionSpec">
VolumeProtectionSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>componentDefRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefRef">
[]ComponentDefRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentDefRef is used to inject values from other components into the current component.<br />values will be saved and updated in a configmap and mounted to the current component.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentPhase">ClusterComponentPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
</p>
<div>
<p>ClusterComponentPhase defines the Cluster CR .status.components.phase</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Abnormal&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Creating&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
<td><p>Abnormal is a sub-state of failed, where one or more workload pods is not in &ldquo;Running&rdquo; phase.</p><br /></td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentService">ClusterComponentService
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.Expose">Expose</a>, <a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>)
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
<p>Service name</p><br />
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
<p>serviceType determines how the Service is exposed. Valid<br />options are ClusterIP, NodePort, and LoadBalancer.<br />&ldquo;ClusterIP&rdquo; allocates a cluster-internal IP address for load-balancing<br />to endpoints. Endpoints are determined by the selector or if that is not<br />specified, they are determined by manual construction of an Endpoints object or<br />EndpointSlice objects. If clusterIP is &ldquo;None&rdquo;, no virtual IP is<br />allocated and the endpoints are published as a set of endpoints rather<br />than a virtual IP.<br />&ldquo;NodePort&rdquo; builds on ClusterIP and allocates a port on every node which<br />routes to the same endpoints as the clusterIP.<br />&ldquo;LoadBalancer&rdquo; builds on NodePort and creates an external load-balancer<br />(if supported in the current cloud) which routes to the same endpoints<br />as the clusterIP.<br />More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a>.</p><br />
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
<p>If ServiceType is LoadBalancer, cloud provider related parameters can be put here<br />More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplateSpec">ClusterTemplateSpec</a>)
</p>
<div>
<p>ClusterComponentSpec defines the cluster component spec.</p><br />
</div>
<table>
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
<p>name defines cluster&rsquo;s component name, this name is also part of Service DNS name, so this name will<br />comply with IANA Service Naming rule.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentDefRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentDefRef references componentDef defined in ClusterDefinition spec. Need to<br />comply with IANA Service Naming rule.</p><br />
</td>
</tr>
<tr>
<td>
<code>classDefRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClassDefRef">
ClassDefRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>classDefRef references the class defined in ComponentClassDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>monitor</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>monitor is a switch to enable monitoring and is set as false by default.<br />KubeBlocks provides an extension mechanism to support component level monitoring,<br />which will scrape metrics auto or manually from servers in component and export<br />metrics to Time Series Database.</p><br />
</td>
</tr>
<tr>
<td>
<code>enabledLogs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>enabledLogs indicates which log file takes effect in the database cluster.<br />element is the log type which is defined in cluster definition logConfig.name,<br />and will set relative variables about this log type in database kernel.</p><br />
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
<p>Component replicas. The default value is used in ClusterDefinition spec if not specified.</p><br />
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Affinity">
Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>affinity describes affinities specified by users.</p><br />
</td>
</tr>
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
<p>Component tolerations will override ClusterSpec.Tolerations if specified.</p><br />
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
<p>Resources requests and limits of workload.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVolumeClaimTemplate">
[]ClusterComponentVolumeClaimTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.</p><br />
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentService">
[]ClusterComponentService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services expose endpoints that can be accessed by clients.</p><br />
</td>
</tr>
<tr>
<td>
<code>switchPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterSwitchPolicy">
ClusterSwitchPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>switchPolicy defines the strategy for switchover and failover when workloadType is Replication.</p><br />
</td>
</tr>
<tr>
<td>
<code>tls</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enables or disables TLS certs.</p><br />
</td>
</tr>
<tr>
<td>
<code>issuer</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Issuer">
Issuer
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>issuer defines provider context for TLS certs.<br />required when TLS enabled</p><br />
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
<p>serviceAccountName is the name of the ServiceAccount that running component depends on.</p><br />
</td>
</tr>
<tr>
<td>
<code>noCreatePDB</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>noCreatePDB defines the PodDisruptionBudget creation behavior and is set to true if creation of PodDisruptionBudget<br />for this component is not needed. It defaults to false.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterStatus">ClusterStatus</a>)
</p>
<div>
<p>ClusterComponentStatus records components status.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentPhase">
ClusterComponentPhase
</a>
</em>
</td>
<td>
<p>phase describes the phase of the component and the detail information of the phases are as following:<br />Running: the component is running. [terminal state]<br />Stopped: the component is stopped, as no running pod. [terminal state]<br />Failed: the component is unavailable, i.e. all pods are not ready for Stateless/Stateful component and<br />Leader/Primary pod is not ready for Consensus/Replication component. [terminal state]<br />Abnormal: the component is running but part of its pods are not ready.<br />Leader/Primary pod is ready for Consensus/Replication component. [terminal state]<br />Creating: the component has entered creating process.<br />Updating: the component has entered updating process, triggered by Spec. updated.</p><br />
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentMessageMap">
ComponentMessageMap
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>message records the component details message in current phase.<br />Keys are podName or deployName or statefulSetName. The format is <code>ObjectKind/Name</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>podsReady</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>podsReady checks if all pods of the component are ready.</p><br />
</td>
</tr>
<tr>
<td>
<code>podsReadyTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>podsReadyTime what time point of all component pods are ready,<br />this time is the ready time of the last component pod.</p><br />
</td>
</tr>
<tr>
<td>
<code>consensusSetStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusSetStatus">
ConsensusSetStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>consensusSetStatus specifies the mapping of role and pod name.</p><br />
</td>
</tr>
<tr>
<td>
<code>replicationSetStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicationSetStatus">
ReplicationSetStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>replicationSetStatus specifies the mapping of role and pod name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionSpec">ClusterVersionSpec</a>)
</p>
<div>
<p>ClusterComponentVersion is an application version component spec.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>componentDefRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentDefRef reference one of the cluster component definition names in ClusterDefinition API (spec.componentDefs.name).</p><br />
</td>
</tr>
<tr>
<td>
<code>configSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configSpecs defines a configuration extension mechanism to handle configuration differences between versions,<br />the configTemplateRefs field, together with configTemplateRefs in the ClusterDefinition,<br />determines the final configuration file.</p><br />
</td>
</tr>
<tr>
<td>
<code>systemAccountSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SystemAccountShortSpec">
SystemAccountShortSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>systemAccountSpec define image for the component to connect database or engines.<br />It overrides <code>image</code> and <code>env</code> attributes defined in ClusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.<br />To clean default envs settings, set <code>SystemAccountSpec.CmdExecutorConfig.Env</code> to empty list.</p><br />
</td>
</tr>
<tr>
<td>
<code>versionsContext</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VersionsContext">
VersionsContext
</a>
</em>
</td>
<td>
<p>versionContext defines containers images&rsquo; context for component versions,<br />this value replaces ClusterDefinition.spec.componentDefs.podSpec.[initContainers | containers]</p><br />
</td>
</tr>
<tr>
<td>
<code>switchoverSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SwitchoverShortSpec">
SwitchoverShortSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>switchoverSpec defines images for the component to do switchover.<br />It overrides <code>image</code> and <code>env</code> attributes defined in ClusterDefinition.spec.componentDefs.SwitchoverSpec.CommandExecutorEnvItem.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentVolumeClaimTemplate">ClusterComponentVolumeClaimTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>)
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
<p>Reference <code>ClusterDefinition.spec.componentDefs.containers.volumeMounts.name</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PersistentVolumeClaimSpec">
PersistentVolumeClaimSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>spec defines the desired characteristics of a volume requested by a pod author.</p><br />
<br/>
<br/>
<table>
<tr>
<td>
<code>accessModes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumeaccessmode-v1-core">
[]Kubernetes core/v1.PersistentVolumeAccessMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>accessModes contains the desired access modes the volume should have.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1</a>.</p><br />
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
<p>resources represents the minimum resources the volume should have.<br />If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements<br />that are lower than previous value but must still be higher than capacity recorded in the<br />status field of the claim.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources">https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources</a>.</p><br />
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
<p>storageClassName is the name of the StorageClass required by the claim.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1</a>.</p><br />
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbe">ClusterDefinitionProbe
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbes">ClusterDefinitionProbes</a>)
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
<code>periodSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<p>How often (in seconds) to perform the probe.</p><br />
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
<p>Number of seconds after which the probe times out. Defaults to 1 second.</p><br />
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
<p>Minimum consecutive failures for the probe to be considered failed after having succeeded.</p><br />
</td>
</tr>
<tr>
<td>
<code>commands</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbeCMDs">
ClusterDefinitionProbeCMDs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>commands used to execute for probe.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbeCMDs">ClusterDefinitionProbeCMDs
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbe">ClusterDefinitionProbe</a>)
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
<code>writes</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Write check executed on probe sidecar, used to check workload&rsquo;s allow write access.</p><br />
</td>
</tr>
<tr>
<td>
<code>queries</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Read check executed on probe sidecar, used to check workload&rsquo;s readonly access.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbes">ClusterDefinitionProbes
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>runningProbe</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbe">
ClusterDefinitionProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Probe for DB running check.</p><br />
</td>
</tr>
<tr>
<td>
<code>statusProbe</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbe">
ClusterDefinitionProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Probe for DB status check.</p><br />
</td>
</tr>
<tr>
<td>
<code>roleProbe</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionProbe">
ClusterDefinitionProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Probe for DB role changed check.</p><br />
</td>
</tr>
<tr>
<td>
<code>roleProbeTimeoutAfterPodsReady</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>roleProbeTimeoutAfterPodsReady(in seconds), when all pods of the component are ready,<br />it will detect whether the application is available in the pod.<br />if pods exceed the InitializationTimeoutSeconds time without a role label,<br />this component will enter the Failed/Abnormal phase.<br />Note that this configuration will only take effect if the component supports RoleProbe<br />and will not affect the life cycle of the pod. default values are 60 seconds.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinitionSpec">ClusterDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinition">ClusterDefinition</a>)
</p>
<div>
<p>ClusterDefinitionSpec defines the desired state of ClusterDefinition</p><br />
</div>
<table>
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
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster definition type defines well known application cluster type, e.g. mysql/redis/mongodb</p><br />
</td>
</tr>
<tr>
<td>
<code>componentDefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">
[]ClusterComponentDefinition
</a>
</em>
</td>
<td>
<p>componentDefs provides cluster components definitions.</p><br />
</td>
</tr>
<tr>
<td>
<code>connectionCredential</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Connection credential template used for creating a connection credential<br />secret for cluster.apps.kubeblocks.io object.</p><br /><br /><p>Built-in objects are:<br />- <code>$(RANDOM_PASSWD)</code> - random 8 characters.<br />- <code>$(UUID)</code> - generate a random UUID v4 string.<br />- <code>$(UUID_B64)</code> - generate a random UUID v4 BASE64 encoded string.<br />- <code>$(UUID_STR_B64)</code> - generate a random UUID v4 string then BASE64 encoded.<br />- <code>$(UUID_HEX)</code> - generate a random UUID v4 HEX representation.<br />- <code>$(HEADLESS_SVC_FQDN)</code> - headless service FQDN placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc,<br />   where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;<br />- <code>$(SVC_FQDN)</code> - service FQDN  placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,<br />   where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;<br />- <code>$(SVC_PORT_&#123;PORT-NAME&#125;)</code> - a ServicePort&rsquo;s port value with specified port name, i.e, a servicePort JSON struct:<br />   <code>&#123;&quot;name&quot;: &quot;mysql&quot;, &quot;targetPort&quot;: &quot;mysqlContainerPort&quot;, &quot;port&quot;: 3306&#125;</code>, and &ldquo;$(SVC_PORT_mysql)&rdquo; in the<br />   connection credential value is 3306.</p><br />
</td>
</tr>
<tr>
<td>
<code>clusterFamilyRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>reference name of clusterfamily</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterDefinitionStatus">ClusterDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinition">ClusterDefinition</a>)
</p>
<div>
<p>ClusterDefinitionStatus defines the observed state of ClusterDefinition</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.Phase">
Phase
</a>
</em>
</td>
<td>
<p>ClusterDefinition phase, valid values are <code>empty</code>, <code>Available</code>, &lsquo;Unavailable`.<br />Available is ClusterDefinition become available, and can be referenced for co-related objects.</p><br />
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
<p>Extra message in current phase</p><br />
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
<p>observedGeneration is the most recent generation observed for this<br />ClusterDefinition. It corresponds to the ClusterDefinition&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterFamilySpec">ClusterFamilySpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterFamily">ClusterFamily</a>)
</p>
<div>
<p>ClusterFamilySpec defines the desired state of ClusterFamily</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterTemplateRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRef">
[]ClusterFamilyTemplateRef
</a>
</em>
</td>
<td>
<p>list of clustertemplates</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterFamilyStatus">ClusterFamilyStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterFamily">ClusterFamily</a>)
</p>
<div>
<p>ClusterFamilyStatus defines the observed state of ClusterFamily</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRef">ClusterFamilyTemplateRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilySpec">ClusterFamilySpec</a>)
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
<p>key defines which parameters in cluster should be checked, if key equals to value, the clustertemplate defined in templateRef will be chosen</p><br />
</td>
</tr>
<tr>
<td>
<code>expression</code><br/>
<em>
string
</em>
</td>
<td>
<p>if the evaluated result equals to value, the clustertemplate defined in templateRef will be chosen, when this field is set, key will be ignored</p><br />
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
<p>the value of the key or the expression</p><br />
</td>
</tr>
<tr>
<td>
<code>templateRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clustertemplate name referenced to, when value is match</p><br />
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRefSelector">
[]ClusterFamilyTemplateRefSelector
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRefSelector">ClusterFamilyTemplateRefSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterFamilyTemplateRef">ClusterFamilyTemplateRef</a>)
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
</td>
</tr>
<tr>
<td>
<code>expression</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterMode">ClusterMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<tbody><tr><td><p>&#34;raftGroup&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;replication&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;standalone&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterMonitor">ClusterMonitor
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<code>monitoringInterval</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>monitoringInterval specifies interval of monitoring, no monitor if set to 0</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterNetwork">ClusterNetwork
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<code>hostNetworkAccessible</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>hostNetworkAccessible specifies whether host network is accessible. It defaults to false</p><br />
</td>
</tr>
<tr>
<td>
<code>publiclyAccessible</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>publiclyAccessible specifies whether it is publicly accessible. It defaults to false</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterPhase">ClusterPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterStatus">ClusterStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestBehaviour">OpsRequestBehaviour</a>)
</p>
<div>
<p>ClusterPhase defines the Cluster CR .status.phase</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Abnormal&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Creating&#34;</p></td>
<td><p>Abnormal is a sub-state of failed, where one of the cluster components has &ldquo;Failed&rdquo; or &ldquo;Abnormal&rdquo; status phase.</p><br /></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>REVIEW/TODO: AbnormalClusterPhase provides hybrid, consider remove it if possible</p><br /></td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterResourceConstraintSelector">ClusterResourceConstraintSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSpec">ComponentResourceConstraintSpec</a>)
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
<code>clusterDefRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterDefRef is the name of the cluster definition.</p><br />
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSelector">
[]ComponentResourceConstraintSelector
</a>
</em>
</td>
<td>
<p>selector is used to bind the resource constraint to components.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterResources">ClusterResources
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<code>cpu</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>cpu resource needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p><br />
</td>
</tr>
<tr>
<td>
<code>memory</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>memory resource needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Cluster">Cluster</a>)
</p>
<div>
<p>ClusterSpec defines the desired state of Cluster.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Cluster referencing ClusterDefinition name. This is an immutable attribute.</p><br />
</td>
</tr>
<tr>
<td>
<code>clusterVersionRef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster referencing ClusterVersion name.</p><br />
</td>
</tr>
<tr>
<td>
<code>terminationPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TerminationPolicyType">
TerminationPolicyType
</a>
</em>
</td>
<td>
<p>Cluster termination policy. Valid values are DoNotTerminate, Halt, Delete, WipeOut.<br />DoNotTerminate will block delete operation.<br />Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.<br />Delete is based on Halt and deletes PVCs.<br />WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">
[]ClusterComponentSpec
</a>
</em>
</td>
<td>
<p>List of componentSpecs you want to replace in ClusterDefinition and ClusterVersion. It will replace the field in ClusterDefinition&rsquo;s and ClusterVersion&rsquo;s component if type is matching.</p><br />
</td>
</tr>
<tr>
<td>
<code>tenancy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TenancyType">
TenancyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>tenancy describes how pods are distributed across node.<br />SharedNode means multiple pods may share the same node.<br />DedicatedNode means each pod runs on their own dedicated node.</p><br />
</td>
</tr>
<tr>
<td>
<code>availabilityPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.AvailabilityPolicyType">
AvailabilityPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>availabilityPolicy describes the availability policy, including zone, node, and none.</p><br />
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Affinity">
Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>affinity is a group of affinity scheduling rules.</p><br />
</td>
</tr>
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
<p>tolerations are attached to tolerate any taint that matches the triple <code>key,value,effect</code> using the matching operator <code>operator</code>.</p><br />
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
<em>(Optional)</em>
<p>replicas specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterResources">
ClusterResources
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>resources specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>storage</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterStorage">
ClusterStorage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>storage specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified, this value will be ignored.</p><br />
</td>
</tr>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterMode">
ClusterMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>mode specifies the mode of this cluster, the value of this can be one of the following: Standalone, Replication, RaftGroup.</p><br />
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>customized parameters that is used in different clusterdefinition</p><br />
</td>
</tr>
<tr>
<td>
<code>monitor</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterMonitor">
ClusterMonitor
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>monitor specifies the configuration of monitor</p><br />
</td>
</tr>
<tr>
<td>
<code>network</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterNetwork">
ClusterNetwork
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>network specifies the configuration of network</p><br />
</td>
</tr>
<tr>
<td>
<code>backup</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterBackup">
ClusterBackup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>cluster backup configuration.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterStatus">ClusterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Cluster">Cluster</a>)
</p>
<div>
<p>ClusterStatus defines the observed state of Cluster.</p><br />
</div>
<table>
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
<p>observedGeneration is the most recent generation observed for this<br />Cluster. It corresponds to the Cluster&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterPhase">
ClusterPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>phase describes the phase of the Cluster, the detail information of the phases are as following:<br />Running: cluster is running, all its components are available. [terminal state]<br />Stopped: cluster has stopped, all its components are stopped. [terminal state]<br />Failed: cluster is unavailable. [terminal state]<br />Abnormal: Cluster is still running, but part of its components are Abnormal/Failed. [terminal state]<br />Creating: Cluster has entered creating process.<br />Updating: Cluster has entered updating process, triggered by Spec. updated.</p><br />
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
<p>message describes cluster details message in current phase.</p><br />
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">
map[string]github.com/apecloud/kubeblocks/apis/apps/v1alpha1.ClusterComponentStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>components record the current status information of all components of the cluster.</p><br />
</td>
</tr>
<tr>
<td>
<code>clusterDefGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterDefGeneration represents the generation number of ClusterDefinition referenced.</p><br />
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
<p>Describe current state of cluster API Resource, like warning.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterStorage">ClusterStorage
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
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
<code>size</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>storage size needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterSwitchPolicy">ClusterSwitchPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.SwitchPolicyType">
SwitchPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterSwitchPolicy defines type of the switchPolicy when workloadType is Replication.<br />MaximumAvailability: [WIP] when the primary is active, do switch if the synchronization delay = 0 in the user-defined lagProbe data delay detection logic, otherwise do not switch. The primary is down, switch immediately. It will be available in future versions.<br />MaximumDataProtection: [WIP] when the primary is active, do switch if synchronization delay = 0 in the user-defined lagProbe data lag detection logic, otherwise do not switch. If the primary is down, if it can be judged that the primary and secondary data are consistent, then do the switch, otherwise do not switch. It will be available in future versions.<br />Noop: KubeBlocks will not perform high-availability switching on components. Users need to implement HA by themselves or integrate open source HA solution.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTemplateSpec">ClusterTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplate">ClusterTemplate</a>)
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
<code>componentSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">
[]ClusterComponentSpec
</a>
</em>
</td>
<td>
<p>default value of cluster.spec.componentSpecs</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTemplateStatus">ClusterTemplateStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterTemplate">ClusterTemplate</a>)
</p>
<div>
<p>ClusterTemplateStatus defines the observed state of ClusterTemplate</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterVersionSpec">ClusterVersionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>)
</p>
<div>
<p>ClusterVersionSpec defines the desired state of ClusterVersion</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>ref ClusterDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentVersions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">
[]ClusterComponentVersion
</a>
</em>
</td>
<td>
<p>List of components&rsquo; containers versioning context, i.e., container image ID, container commands, args., and environments.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterVersionStatus">ClusterVersionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>)
</p>
<div>
<p>ClusterVersionStatus defines the observed state of ClusterVersion</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.Phase">
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
<p>A human readable message indicating details about why the ClusterVersion is in this phase.</p><br />
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
<tr>
<td>
<code>clusterDefGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterDefGeneration represents the generation number of ClusterDefinition referenced.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">CmdExecutorConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">SwitchoverAction</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec</a>)
</p>
<div>
<p>CmdExecutorConfig specifies how to perform creation and deletion statements.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>CommandExecutorEnvItem</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CommandExecutorEnvItem">
CommandExecutorEnvItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>CommandExecutorEnvItem</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>CommandExecutorItem</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CommandExecutorItem">
CommandExecutorItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>CommandExecutorItem</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CommandExecutorEnvItem">CommandExecutorEnvItem
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">CmdExecutorConfig</a>, <a href="#apps.kubeblocks.io/v1alpha1.SwitchoverShortSpec">SwitchoverShortSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccountShortSpec">SystemAccountShortSpec</a>)
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
<p>image for Connector when executing the command.</p><br />
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
<p>envs is a list of environment variables.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CommandExecutorItem">CommandExecutorItem
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">CmdExecutorConfig</a>)
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
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>command to perform statements.</p><br />
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
<p>args is used to perform statements.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CommonBackupPolicy">CommonBackupPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.BasePolicy">
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClass">ComponentClass
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionStatus">ComponentClassDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentClassSeries">ComponentClassSeries</a>)
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
<p>name is the class name</p><br />
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
<p>args are variable&rsquo;s value</p><br />
</td>
</tr>
<tr>
<td>
<code>cpu</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the CPU of the class</p><br />
</td>
</tr>
<tr>
<td>
<code>memory</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>the memory of the class</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionSpec">ComponentClassDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinition">ComponentClassDefinition</a>)
</p>
<div>
<p>ComponentClassDefinitionSpec defines the desired state of ComponentClassDefinition</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>groups</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassGroup">
[]ComponentClassGroup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>group defines a list of class series that conform to the same constraint.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionStatus">ComponentClassDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinition">ComponentClassDefinition</a>)
</p>
<div>
<p>ComponentClassDefinitionStatus defines the observed state of ComponentClassDefinition</p><br />
</div>
<table>
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
<p>observedGeneration is the most recent generation observed for this<br />ComponentClassDefinition. It corresponds to the ComponentClassDefinition&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
</td>
</tr>
<tr>
<td>
<code>classes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClass">
[]ComponentClass
</a>
</em>
</td>
<td>
<p>classes is the list of classes that have been observed for this ComponentClassDefinition</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClassGroup">ComponentClassGroup
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinitionSpec">ComponentClassDefinitionSpec</a>)
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
<code>template</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>template is a class definition template that uses the Go template syntax and allows for variable declaration.<br />When defining a class in Series, specifying the variable&rsquo;s value is sufficient, as the complete class<br />definition will be generated through rendering the template.</p><br /><br /><p>For example:<br />	template: |<br />	  cpu: &ldquo;&#123;&#123; or .cpu 1 &#125;&#125;&rdquo;<br />	  memory: &ldquo;&#123;&#123; or .memory 4 &#125;&#125;Gi&rdquo;</p><br />
</td>
</tr>
<tr>
<td>
<code>vars</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>vars defines the variables declared in the template and will be used to generating the complete class definition by<br />render the template.</p><br />
</td>
</tr>
<tr>
<td>
<code>series</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassSeries">
[]ComponentClassSeries
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>series is a series of class definitions.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentClassSeries">ComponentClassSeries
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentClassGroup">ComponentClassGroup</a>)
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
<code>namingTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>namingTemplate is a template that uses the Go template syntax and allows for referencing variables defined<br />in ComponentClassGroup.Template. This enables dynamic generation of class names.<br />For example:<br />name: &ldquo;general-&#123;&#123; .cpu &#125;&#125;c&#123;&#123; .memory &#125;&#125;g&rdquo;</p><br />
</td>
</tr>
<tr>
<td>
<code>classes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClass">
[]ComponentClass
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>classes are definitions of classes that come in two forms. In the first form, only ComponentClass.Args<br />need to be defined, and the complete class definition is generated by rendering the ComponentClassGroup.Template<br />and Name. In the second form, the Name, CPU and Memory must be defined.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>)
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
<code>ComponentTemplateSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">
ComponentTemplateSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentTemplateSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>keys</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify a list of keys.<br />If empty, ConfigConstraint takes effect for all keys in configmap.</p><br />
</td>
</tr>
<tr>
<td>
<code>lazyRenderedConfigSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LazyRenderedTemplateSpec">
LazyRenderedTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>lazyRenderedConfigSpec is optional: specify the secondary rendered config spec.</p><br />
</td>
</tr>
<tr>
<td>
<code>constraintRef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify the name of the referenced the configuration constraints object.</p><br />
</td>
</tr>
<tr>
<td>
<code>asEnvFrom</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>asEnvFrom is optional: the list of containers will be injected into EnvFrom.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefRef">ComponentDefRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
</p>
<div>
<p>ComponentDefRef is used to select the component and its fields to be referenced.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>componentDefName</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentDefName is the name of the componentDef to select.</p><br />
</td>
</tr>
<tr>
<td>
<code>failurePolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.FailurePolicyType">
FailurePolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>failurePolicy is the failure policy of the component.<br />If failed to find the component, the failure policy will be used.</p><br />
</td>
</tr>
<tr>
<td>
<code>componentRefEnv</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentRefEnv">
[]ComponentRefEnv
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentRefEnv specifies a list of values to be injected as env variables to each component.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentMessageMap">ComponentMessageMap
(<code>map[string]string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>)
</p>
<div>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentNameSet">ComponentNameSet
(<code>map[string]struct{}</code> alias)</h3>
<div>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentOps">ComponentOps
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Expose">Expose</a>, <a href="#apps.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure</a>, <a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">ScriptSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.Switchover">Switchover</a>, <a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>, <a href="#apps.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion</a>)
</p>
<div>
<p>ComponentOps defines the common variables of component scope operations.</p><br />
</div>
<table>
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
<p>componentName cluster component name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentRefEnv">ComponentRefEnv
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefRef">ComponentDefRef</a>)
</p>
<div>
<p>ComponentRefEnv specifies name and value of an env.</p><br />
</div>
<table>
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
<p>name is the name of the env to be injected, and it must be a C identifier.</p><br />
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
<p>value is the value of the env to be injected.</p><br />
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentValueFrom">
ComponentValueFrom
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>valueFrom specifies the source of the env to be injected.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSelector">ComponentResourceConstraintSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterResourceConstraintSelector">ClusterResourceConstraintSelector</a>)
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
<code>componentDefRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentDefRef is the name of the component definition in the cluster definition.</p><br />
</td>
</tr>
<tr>
<td>
<code>rules</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>rules are the constraint rules that will be applied to the component.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSpec">ComponentResourceConstraintSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraint">ComponentResourceConstraint</a>)
</p>
<div>
<p>ComponentResourceConstraintSpec defines the desired state of ComponentResourceConstraint</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>rules</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">
[]ResourceConstraintRule
</a>
</em>
</td>
<td>
<p>Component resource constraint rules.</p><br />
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterResourceConstraintSelector">
[]ClusterResourceConstraintSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>selector is used to bind the resource constraint to cluster definitions.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceKey">ComponentResourceKey
(<code>string</code> alias)</h3>
<div>
<p>ComponentResourceKey defines the resource key of component, such as pod/pvc.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;pods&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">ComponentTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec</a>)
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
<p>Specify the name of configuration template.</p><br />
</td>
</tr>
<tr>
<td>
<code>templateRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specify the name of the referenced the configuration template ConfigMap object.</p><br />
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
<p>Specify the namespace of the referenced the configuration template ConfigMap object.<br />An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p><br />
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
<p>volumeName is the volume name of PodTemplate, which the configuration file produced through the configuration<br />template will be mounted to the corresponding volume. Must be a DNS_LABEL name.<br />The volume name must be defined in podSpec.containers[*].volumeMounts.</p><br />
</td>
</tr>
<tr>
<td>
<code>defaultMode</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>defaultMode is optional: mode bits used to set permissions on created files by default.<br />Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.<br />YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.<br />Defaults to 0644.<br />Directories within the path are not affected by this setting.<br />This might be in conflict with other options that affect the file<br />mode, like fsGroup, and the result can be other mode bits set.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentValueFrom">ComponentValueFrom
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentRefEnv">ComponentRefEnv</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentValueFromType">
ComponentValueFromType
</a>
</em>
</td>
<td>
<p>type is the type of the source to select. There are three types: <code>FieldRef</code>, <code>ServiceRef</code>, <code>HeadlessServiceRef</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>fieldPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>fieldRef is the jsonpath of the source to select when type is <code>FieldRef</code>.<br />there are two objects registered in the jsonpath: <code>componentDef</code> and <code>components</code>.<br />componentDef is the component definition object specified in <code>componentRef.componentDefName</code>.<br />components is the component list objects referring to the component definition object.</p><br />
</td>
</tr>
<tr>
<td>
<code>format</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>format is the format of each headless service address.<br />there are three builtin variables can be used as placerholder: $POD_ORDINAL, $POD_FQDN, $POD_NAME<br />$POD_ORDINAL is the ordinal of the pod.<br />$POD_FQDN is the fully qualified domain name of the pod.<br />$POD_NAME is the name of the pod</p><br />
</td>
</tr>
<tr>
<td>
<code>joinWith</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>joinWith is the string to join the values of headless service addresses.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentValueFromType">ComponentValueFromType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentValueFrom">ComponentValueFrom</a>)
</p>
<div>
<p>ComponentValueFromType specifies the type of component value from.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;FieldRef&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;HeadlessServiceRef&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ServiceRef&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraintPhase">ConfigConstraintPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintStatus">ConfigConstraintStatus</a>)
</p>
<div>
<p>ConfigConstraintPhase defines the ConfigConstraint  CR .status.phase</p><br />
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
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Unavailable&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint</a>)
</p>
<div>
<p>ConfigConstraintSpec defines the desired state of ConfigConstraint</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>reloadOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">
ReloadOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reloadOptions indicates whether the process supports reload.<br />if set, the controller will determine the behavior of the engine instance based on the configuration templates,<br />restart or reload depending on whether any parameters in the StaticParameters have been modified.</p><br />
</td>
</tr>
<tr>
<td>
<code>toolsImageSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ToolsImageSpec">
ToolsImageSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>toolConfig used to config init container.</p><br />
</td>
</tr>
<tr>
<td>
<code>downwardAPIOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.DownwardAPIOption">
[]DownwardAPIOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>downwardAPIOptions is used to watch pod fields.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptConfigs, list of ScriptConfig, witch these scripts can be used by volume trigger,downward trigger, or tool image</p><br />
</td>
</tr>
<tr>
<td>
<code>cfgSchemaTopLevelName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>cfgSchemaTopLevelName is cue type name, which generates openapi schema.</p><br />
</td>
</tr>
<tr>
<td>
<code>configurationSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomParametersValidation">
CustomParametersValidation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configurationSchema imposes restrictions on database parameter&rsquo;s rule.</p><br />
</td>
</tr>
<tr>
<td>
<code>staticParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>staticParameters, list of StaticParameter, modifications of them trigger a process restart.</p><br />
</td>
</tr>
<tr>
<td>
<code>dynamicParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.</p><br />
</td>
</tr>
<tr>
<td>
<code>immutableParameters</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>immutableParameters describes parameters that prohibit user from modification.</p><br />
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
<p>selector is used to match the label on the pod,<br />for example, a pod of the primary is match on the patroni cluster.</p><br />
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>formatterConfig describes the format of the configuration file, the controller<br />1. parses configuration file<br />2. analyzes the modified parameters<br />3. applies corresponding policies.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraintStatus">ConfigConstraintStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint</a>)
</p>
<div>
<p>ConfigConstraintStatus defines the observed state of ConfigConstraint.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintPhase">
ConfigConstraintPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>phase is status of configuration template, when set to CCAvailablePhase, it can be referenced by ClusterDefinition or ClusterVersion.</p><br />
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
<p>message field describes the reasons of abnormal status.</p><br />
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
<p>observedGeneration is the latest generation observed for this<br />ClusterDefinition. It refers to the ConfigConstraint&rsquo;s generation, which is<br />updated by the API Server.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Configuration">Configuration
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure</a>)
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
<p>name is a config template name.</p><br />
</td>
</tr>
<tr>
<td>
<code>keys</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ParameterConfig">
[]ParameterConfig
</a>
</em>
</td>
<td>
<p>keys is used to set the parameters to be updated.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationStatus">ConfigurationStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus</a>)
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
<p>name is a config template name.</p><br />
</td>
</tr>
<tr>
<td>
<code>updatePolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UpgradePolicy">
UpgradePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>updatePolicy describes the policy of reconfiguring.</p><br />
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>status describes the current state of the reconfiguring state machine.</p><br />
</td>
</tr>
<tr>
<td>
<code>succeedCount</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>succeedCount describes the number of successful reconfiguring.</p><br />
</td>
</tr>
<tr>
<td>
<code>expectedCount</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>expectedCount describes the number of expected reconfiguring.</p><br />
</td>
</tr>
<tr>
<td>
<code>lastStatus</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>lastStatus describes the last status for the reconfiguring controller.</p><br />
</td>
</tr>
<tr>
<td>
<code>lastAppliedConfiguration</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastAppliedConfiguration describes the last configuration.</p><br />
</td>
</tr>
<tr>
<td>
<code>updatedParameters</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UpdatedParameters">
UpdatedParameters
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>updatedParameters describes the updated parameters.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConnectionCredentialKey">ConnectionCredentialKey
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance</a>)
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
<code>passwordKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>the key of password in the ConnectionCredential secret.<br />if not set, the default key is &ldquo;password&rdquo;.</p><br />
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
<em>(Optional)</em>
<p>the key of username in the ConnectionCredential secret.<br />if not set, the default key is &ldquo;username&rdquo;.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConsensusMember">ConsensusMember
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConsensusSetSpec">ConsensusSetSpec</a>)
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
<p>name, role name.</p><br />
</td>
</tr>
<tr>
<td>
<code>accessMode</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.AccessMode">
AccessMode
</a>
</em>
</td>
<td>
<p>accessMode, what service this member capable.</p><br />
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
<em>(Optional)</em>
<p>replicas, number of Pods of this role.<br />default 1 for Leader<br />default 0 for Learner<br />default Cluster.spec.componentSpec[*].Replicas - Leader.Replicas - Learner.Replicas for Followers</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConsensusMemberStatus">ConsensusMemberStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConsensusSetStatus">ConsensusSetStatus</a>)
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
<p>Defines the role name.</p><br />
</td>
</tr>
<tr>
<td>
<code>accessMode</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.AccessMode">
AccessMode
</a>
</em>
</td>
<td>
<p>accessMode defines what service this pod provides.</p><br />
</td>
</tr>
<tr>
<td>
<code>pod</code><br/>
<em>
string
</em>
</td>
<td>
<p>Pod name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConsensusSetSpec">ConsensusSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>StatefulSetSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">
StatefulSetSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>StatefulSetSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>leader</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMember">
ConsensusMember
</a>
</em>
</td>
<td>
<p>leader, one single leader.</p><br />
</td>
</tr>
<tr>
<td>
<code>followers</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMember">
[]ConsensusMember
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>followers, has voting right but not Leader.</p><br />
</td>
</tr>
<tr>
<td>
<code>learner</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMember">
ConsensusMember
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>learner, no voting right.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConsensusSetStatus">ConsensusSetStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>)
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
<code>leader</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMemberStatus">
ConsensusMemberStatus
</a>
</em>
</td>
<td>
<p>Leader status.</p><br />
</td>
</tr>
<tr>
<td>
<code>followers</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMemberStatus">
[]ConsensusMemberStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Followers status.</p><br />
</td>
</tr>
<tr>
<td>
<code>learner</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConsensusMemberStatus">
ConsensusMemberStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Learner status.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CustomLabelSpec">CustomLabelSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<p>key name of label</p><br />
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
<p>value of label</p><br />
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.GVKResource">
[]GVKResource
</a>
</em>
</td>
<td>
<p>resources defines the resources to be labeled.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CustomParametersValidation">CustomParametersValidation
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
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
<code>schema</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jsonschemaprops-v1-apiextensions-k8s-io">
Kubernetes api extensions v1.JSONSchemaProps
</a>
</em>
</td>
<td>
<p>schema provides a way for providers to validate the changed parameters through json.</p><br />
</td>
</tr>
<tr>
<td>
<code>cue</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>cue that to let provider verify user configuration through cue language.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.DownwardAPIOption">DownwardAPIOption
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
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
<p>Specify the name of the field.</p><br />
</td>
</tr>
<tr>
<td>
<code>mountPoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>mountPoint is the mount point of the scripts file.</p><br />
</td>
</tr>
<tr>
<td>
<code>items</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#downwardapivolumefile-v1-core">
[]Kubernetes core/v1.DownwardAPIVolumeFile
</a>
</em>
</td>
<td>
<p>Items is a list of downward API volume file</p><br />
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
<em>(Optional)</em>
<p>command used to execute for downwrad api.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ExporterConfig">ExporterConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MonitorConfig">MonitorConfig</a>)
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
<code>scrapePort</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<p>scrapePort is exporter port for Time Series Database to scrape metrics.</p><br />
</td>
</tr>
<tr>
<td>
<code>scrapePath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>scrapePath is exporter url path for Time Series Database to scrape metrics.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Expose">Expose
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentService">
[]ClusterComponentService
</a>
</em>
</td>
<td>
<p>Setting the list of services to be exposed.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.FailurePolicyType">FailurePolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefRef">ComponentDefRef</a>)
</p>
<div>
<p>FailurePolicyType specifies the type of failure policy</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Fail&#34;</p></td>
<td><p>ReportError means that an error will be reported.</p><br /></td>
</tr><tr><td><p>&#34;Ignore&#34;</p></td>
<td><p>Ignore means that an error will be ignored but logged.</p><br /></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.FormatterConfig">FormatterConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
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
<code>FormatterOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.FormatterOptions">
FormatterOptions
</a>
</em>
</td>
<td>
<p>
(Members of <code>FormatterOptions</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>The FormatterOptions represents the special options of configuration file.<br />This is optional for now. If not specified.</p><br />
</td>
</tr>
<tr>
<td>
<code>format</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CfgFileFormat">
CfgFileFormat
</a>
</em>
</td>
<td>
<p>The configuration file format. Valid values are ini, xml, yaml, json,<br />hcl, dotenv, properties and toml.</p><br /><br /><p>ini: a configuration file that consists of a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: <a href="https://en.wikipedia.org/wiki/INI_file">https://en.wikipedia.org/wiki/INI_file</a><br />xml: reference wiki: <a href="https://en.wikipedia.org/wiki/XML">https://en.wikipedia.org/wiki/XML</a><br />yaml: a configuration file support for complex data types and structures.<br />json: reference wiki: <a href="https://en.wikipedia.org/wiki/JSON">https://en.wikipedia.org/wiki/JSON</a><br />hcl: : The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: <a href="https://www.linode.com/docs/guides/introduction-to-hcl/">https://www.linode.com/docs/guides/introduction-to-hcl/</a><br />dotenv: this was a plain text file with simple key–value pairs, reference wiki: <a href="https://en.wikipedia.org/wiki/Configuration_file#MS-DOS">https://en.wikipedia.org/wiki/Configuration_file#MS-DOS</a><br />properties: a file extension mainly used in Java, reference wiki: <a href="https://en.wikipedia.org/wiki/.properties">https://en.wikipedia.org/wiki/.properties</a><br />toml: reference wiki: <a href="https://en.wikipedia.org/wiki/TOML">https://en.wikipedia.org/wiki/TOML</a><br />props-plus: a file extension mainly used in Java, support CamelCase(e.g: brokerMaxConnectionsPerIp)</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.FormatterOptions">FormatterOptions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.FormatterConfig">FormatterConfig</a>)
</p>
<div>
<p>FormatterOptions represents the special options of configuration file.<br />Only one of its members may be specified.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>iniConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.IniConfig">
IniConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>iniConfig represents the ini options.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.GVKResource">GVKResource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CustomLabelSpec">CustomLabelSpec</a>)
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
<code>gvk</code><br/>
<em>
string
</em>
</td>
<td>
<p>gvk is Group/Version/Kind, for example &ldquo;v1/Pod&rdquo;, &ldquo;apps/v1/StatefulSet&rdquo;, etc.<br />when the gvk resource filtered by the selector already exists, if there is no corresponding custom label, it will be added, and if label already exists, it will be updated.</p><br />
</td>
</tr>
<tr>
<td>
<code>selector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>selector is a label query over a set of resources.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HScaleDataClonePolicyType">HScaleDataClonePolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.HorizontalScalePolicy">HorizontalScalePolicy</a>)
</p>
<div>
<p>HScaleDataClonePolicyType defines data clone policy when horizontal scaling.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CloneVolume&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Snapshot&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;None&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HorizontalScalePolicy">HorizontalScalePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.HScaleDataClonePolicyType">
HScaleDataClonePolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>type controls what kind of data synchronization do when component scale out.<br />Policy is in enum of &#123;None, CloneVolume&#125;. The default policy is <code>None</code>.<br />None: Default policy, create empty volume and no data clone.<br />CloneVolume: Do data clone to newly scaled pods. Prefer to use volume snapshot first,<br />        and will try backup tool if volume snapshot is not enabled, finally<br />	       report error if both above cannot work.<br />Snapshot: Deprecated, alias for CloneVolume.</p><br />
</td>
</tr>
<tr>
<td>
<code>backupPolicyTemplateName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>BackupPolicyTemplateName reference the backup policy template.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeMountsName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeMountsName defines which volumeMount of the container to do backup,<br />only work if Type is not None<br />if not specified, the 1st volumeMount will be chosen</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>HorizontalScaling defines the variables of horizontal scaling operation</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
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
<p>replicas for the workloads.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.IniConfig">IniConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.FormatterOptions">FormatterOptions</a>)
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
<code>sectionName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>sectionName describes ini section.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Issuer">Issuer
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>)
</p>
<div>
<p>Issuer defines Tls certs issuer</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.IssuerName">
IssuerName
</a>
</em>
</td>
<td>
<p>Name of issuer.<br />Options supported:<br />- KubeBlocks - Certificates signed by KubeBlocks Operator.<br />- UserProvided - User provided own CA-signed certificates.</p><br />
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TLSSecretRef">
TLSSecretRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secretRef. TLS certs Secret reference<br />required when from is UserProvided</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.IssuerName">IssuerName
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Issuer">Issuer</a>)
</p>
<div>
<p>IssuerName defines Tls certs issuer name</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;KubeBlocks&#34;</p></td>
<td><p>IssuerKubeBlocks Certificates signed by KubeBlocks Operator.</p><br /></td>
</tr><tr><td><p>&#34;UserProvided&#34;</p></td>
<td><p>IssuerUserProvided User provided own CA-signed certificates.</p><br /></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.KBAccountType">KBAccountType
(<code>byte</code> alias)</h3>
<div>
<p>KBAccountType is used for bitwise operation.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>0</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.LastConfiguration">LastConfiguration</a>)
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
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>replicas are the last replicas of the component.</p><br />
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
<p>the last resources of the component.</p><br />
</td>
</tr>
<tr>
<td>
<code>classDefRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClassDefRef">
ClassDefRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>classDefRef reference class defined in ComponentClassDefinition.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">
[]OpsRequestVolumeClaimTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeClaimTemplates records the last volumeClaimTemplates of the component.</p><br />
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentService">
[]ClusterComponentService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>services records the last services of the component.</p><br />
</td>
</tr>
<tr>
<td>
<code>targetResources</code><br/>
<em>
map[github.com/apecloud/kubeblocks/apis/apps/v1alpha1.ComponentResourceKey][]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>targetResources records the affecting target resources information for the component.<br />resource key is in list of [pods].</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LastConfiguration">LastConfiguration
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
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
<code>clusterVersionRef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterVersionRef references ClusterVersion name.</p><br />
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">
map[string]github.com/apecloud/kubeblocks/apis/apps/v1alpha1.LastComponentConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>components records last configuration of the component.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LazyRenderedTemplateSpec">LazyRenderedTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec</a>)
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
<code>templateRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specify the name of the referenced the configuration template ConfigMap object.</p><br />
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
<p>Specify the namespace of the referenced the configuration template ConfigMap object.<br />An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p><br />
</td>
</tr>
<tr>
<td>
<code>policy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MergedPolicy">
MergedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>policy defines how to merge external imported templates into component templates.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LetterCase">LetterCase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PasswordConfig">PasswordConfig</a>)
</p>
<div>
<p>LetterCase defines cases to use in password generation.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;LowerCases&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;MixedCases&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;UpperCases&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LogConfig">LogConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<p>name log type name, such as slow for MySQL slow log file.</p><br />
</td>
</tr>
<tr>
<td>
<code>filePathPattern</code><br/>
<em>
string
</em>
</td>
<td>
<p>filePathPattern log file path pattern which indicate how to find this file<br />corresponding to variable (log path) in database kernel. please don&rsquo;t set this casually.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MemoryConstraint">MemoryConstraint
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">ResourceConstraintRule</a>)
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
<code>sizePerCPU</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The size of memory per vcpu core.<br />For example: 1Gi, 200Mi.<br />If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignore.</p><br />
</td>
</tr>
<tr>
<td>
<code>maxPerCPU</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.<br />It is useful on GCP as the ratio between the CPU and memory may be a range.<br />If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.<br />Reference: <a href="https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types">https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types</a></p><br />
</td>
</tr>
<tr>
<td>
<code>minPerCPU</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.<br />It is useful on GCP as the ratio between the CPU and memory may be a range.<br />If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.<br />Reference: <a href="https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types">https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types</a></p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MergedPolicy">MergedPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.LazyRenderedTemplateSpec">LazyRenderedTemplateSpec</a>)
</p>
<div>
<p>MergedPolicy defines how to merge external imported templates into component templates.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;none&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;add&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;patch&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;replace&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MonitorConfig">MonitorConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>builtIn</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>builtIn is a switch to enable KubeBlocks builtIn monitoring.<br />If BuiltIn is set to true, monitor metrics will be scraped automatically.<br />If BuiltIn is set to false, the provider should set ExporterConfig and Sidecar container own.</p><br />
</td>
</tr>
<tr>
<td>
<code>exporterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ExporterConfig">
ExporterConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>exporterConfig provided by provider, which specify necessary information to Time Series Database.<br />exporterConfig is valid when builtIn is false.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsPhase">OpsPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
</p>
<div>
<p>OpsPhase defines opsRequest phase.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Cancelled&#34;</p></td>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRecorder">OpsRecorder
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
<p>name OpsRequest name</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>clusterPhase the cluster phase when the OpsRequest is running</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestBehaviour">OpsRequestBehaviour
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterPhase">
[]ClusterPhase
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ToClusterPhase</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterPhase">
ClusterPhase
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ProcessingReasonInClusterCondition</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentPhase">
ClusterComponentPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>phase describes the component phase, reference Cluster.status.component.phase.</p><br />
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
<p>lastFailedTime is the last time the component phase transitioned to Failed or Abnormal.</p><br />
</td>
</tr>
<tr>
<td>
<code>progressDetails</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProgressStatusDetail">
[]ProgressStatusDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>progressDetails describes the progress details of the component for this operation.</p><br />
</td>
</tr>
<tr>
<td>
<code>workloadType</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.WorkloadType">
WorkloadType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>workloadType references workload type of component in ClusterDefinition.</p><br />
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
<p>reason describes the reason for the component phase.</p><br />
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
<p>message is a human-readable message indicating details about this operation.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>)
</p>
<div>
<p>OpsRequestSpec defines the desired state of OpsRequest</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterRef references clusterDefinition.</p><br />
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
<p>cancel defines the action to cancel the Pending/Creating/Running opsRequest, supported types: [VerticalScaling, HorizontalScaling].<br />once cancel is set to true, this opsRequest will be canceled and modifying this property again will not take effect.</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsType">
OpsType
</a>
</em>
</td>
<td>
<p>type defines the operation type.</p><br />
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
<p>ttlSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.</p><br />
</td>
</tr>
<tr>
<td>
<code>upgrade</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Upgrade">
Upgrade
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>upgrade specifies the cluster version by specifying clusterVersionRef.</p><br />
</td>
</tr>
<tr>
<td>
<code>horizontalScaling</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HorizontalScaling">
[]HorizontalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>horizontalScaling defines what component need to horizontal scale the specified replicas.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumeExpansion</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VolumeExpansion">
[]VolumeExpansion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeExpansion defines what component and volumeClaimTemplate need to expand the specified storage.</p><br />
</td>
</tr>
<tr>
<td>
<code>restart</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
[]ComponentOps
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>restart the specified components.</p><br />
</td>
</tr>
<tr>
<td>
<code>switchover</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Switchover">
[]Switchover
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>switchover the specified components.</p><br />
</td>
</tr>
<tr>
<td>
<code>verticalScaling</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">
[]VerticalScaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>verticalScaling defines what component need to vertical scale the specified compute resources.</p><br />
</td>
</tr>
<tr>
<td>
<code>reconfigure</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">
Reconfigure
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reconfigure defines the variables that need to input when updating configuration.</p><br />
</td>
</tr>
<tr>
<td>
<code>expose</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Expose">
[]Expose
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>expose defines services the component needs to expose.</p><br />
</td>
</tr>
<tr>
<td>
<code>restoreFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RestoreFromSpec">
RestoreFromSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>cluster RestoreFrom backup or point in time</p><br />
</td>
</tr>
<tr>
<td>
<code>ttlSecondsBeforeAbort</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ttlSecondsBeforeAbort OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.<br />If not specified, the default value is 0, which means that the start-conditions must be met immediately.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">
ScriptSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptSpec defines the script to be executed.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>)
</p>
<div>
<p>OpsRequestStatus defines the observed state of OpsRequest</p><br />
</div>
<table>
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
<p>ClusterGeneration records the cluster generation after handling the opsRequest action.</p><br />
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsPhase">
OpsPhase
</a>
</em>
</td>
<td>
<p>phase describes OpsRequest phase.</p><br />
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
</td>
</tr>
<tr>
<td>
<code>lastConfiguration</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LastConfiguration">
LastConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>lastConfiguration records the last configuration before this operation take effected.</p><br />
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">
map[string]github.com/apecloud/kubeblocks/apis/apps/v1alpha1.OpsRequestComponentStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>components defines the recorded the status information of changed components for operation request.</p><br />
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
<p>startTimestamp The time when the OpsRequest started processing.</p><br />
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
<p>completionTimestamp defines the OpsRequest completion time.</p><br />
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
<p>CancelTimestamp defines cancel time.</p><br />
</td>
</tr>
<tr>
<td>
<code>reconfiguringStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReconfiguringStatus">
ReconfiguringStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reconfiguringStatus defines the status information of reconfiguring.</p><br />
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
<p>conditions describes opsRequest detail status.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">OpsRequestVolumeClaimTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>, <a href="#apps.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion</a>)
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
<p>Request storage size.</p><br />
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
<p>name references volumeClaimTemplate name from cluster components.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsType">OpsType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRecorder">OpsRecorder</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>OpsType defines operation types.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;DataScript&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Expose&#34;</p></td>
<td><p>StartType the start operation will start the pods which is deleted in stop operation.</p><br /></td>
</tr><tr><td><p>&#34;HorizontalScaling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Reconfiguring&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Restart&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Start&#34;</p></td>
<td><p>StopType the stop operation will delete all pods in a cluster concurrently.</p><br /></td>
</tr><tr><td><p>&#34;Stop&#34;</p></td>
<td><p>RestartType the restart operation is a special case of the rolling update operation.</p><br /></td>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ParameterConfig">ParameterConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Configuration">Configuration</a>)
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
<p>key indicates the key name of ConfigMap.</p><br />
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ParameterPair">
[]ParameterPair
</a>
</em>
</td>
<td>
<p>Setting the list of parameters for a single configuration file.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ParameterPair">ParameterPair
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ParameterConfig">ParameterConfig</a>)
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
<p>key is name of the parameter to be updated.</p><br />
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
<p>parameter values to be updated.<br />if set nil, the parameter defined by the key field will be deleted from the configuration file.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PasswordConfig">PasswordConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec</a>)
</p>
<div>
<p>PasswordConfig helps provide to customize complexity of password generation pattern.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>length</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>length defines the length of password.</p><br />
</td>
</tr>
<tr>
<td>
<code>numDigits</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>numDigits defines number of digits.</p><br />
</td>
</tr>
<tr>
<td>
<code>numSymbols</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>numSymbols defines number of symbols.</p><br />
</td>
</tr>
<tr>
<td>
<code>letterCase</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LetterCase">
LetterCase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>letterCase defines to use lower-cases, upper-cases or mixed-cases of letters.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PersistentVolumeClaimSpec">PersistentVolumeClaimSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVolumeClaimTemplate">ClusterComponentVolumeClaimTemplate</a>)
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
<code>accessModes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumeaccessmode-v1-core">
[]Kubernetes core/v1.PersistentVolumeAccessMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>accessModes contains the desired access modes the volume should have.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1</a>.</p><br />
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
<p>resources represents the minimum resources the volume should have.<br />If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements<br />that are lower than previous value but must still be higher than capacity recorded in the<br />status field of the claim.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources">https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources</a>.</p><br />
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
<p>storageClassName is the name of the StorageClass required by the claim.<br />More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1</a>.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionStatus">ClusterDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionStatus">ClusterVersionStatus</a>)
</p>
<div>
<p>Phase defines the ClusterDefinition and ClusterVersion  CR .status.phase</p><br />
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
<h3 id="apps.kubeblocks.io/v1alpha1.PodAntiAffinity">PodAntiAffinity
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Affinity">Affinity</a>)
</p>
<div>
<p>PodAntiAffinity defines pod anti-affinity strategy.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Preferred&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Required&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PointInTimeRefSpec">PointInTimeRefSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RestoreFromSpec">RestoreFromSpec</a>)
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
<code>time</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>specify the time point to restore, with UTC as the time zone.</p><br />
</td>
</tr>
<tr>
<td>
<code>ref</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RefNamespaceName">
RefNamespaceName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>specify a reference source cluster to restore</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProgressStatus">ProgressStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail</a>)
</p>
<div>
<p>ProgressStatus defines the status of the opsRequest progress.</p><br />
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
<h3 id="apps.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
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
<p>group describes which group the current object belongs to.<br />if the objects of a component belong to the same group, we can ignore it.</p><br />
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
<p>objectKey is the unique key of the object.</p><br />
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProgressStatus">
ProgressStatus
</a>
</em>
</td>
<td>
<p>status describes the state of processing the object.</p><br />
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
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>startTime is the start time of object processing.</p><br />
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
<p>endTime is the completion time of object processing.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProtectedVolume">ProtectedVolume
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VolumeProtectionSpec">VolumeProtectionSpec</a>)
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
<p>Name of volume to protect.</p><br />
</td>
</tr>
<tr>
<td>
<code>highWatermark</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume specified high watermark threshold, it will override the component level threshold.<br />If the value is invalid, it will be ignored and the component level threshold will be used.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccountConfig">SystemAccountConfig</a>)
</p>
<div>
<p>ProvisionPolicy defines the policy details for creating accounts.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicyType">
ProvisionPolicyType
</a>
</em>
</td>
<td>
<p>type defines the way to provision an account, either <code>CreateByStmt</code> or <code>ReferToExisting</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>scope</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProvisionScope">
ProvisionScope
</a>
</em>
</td>
<td>
<p>scope is the scope to provision account, and the scope could be <code>AnyPods</code> or <code>AllPods</code>.</p><br />
</td>
</tr>
<tr>
<td>
<code>statements</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProvisionStatements">
ProvisionStatements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>statements will be used when Type is CreateByStmt.</p><br />
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProvisionSecretRef">
ProvisionSecretRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secretRef will be used when Type is ReferToExisting.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionPolicyType">ProvisionPolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>)
</p>
<div>
<p>ProvisionPolicyType defines the policy for creating accounts.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CreateByStmt&#34;</p></td>
<td><p>CreateByStmt will create account w.r.t. deletion and creation statement given by provider.</p><br /></td>
</tr><tr><td><p>&#34;ReferToExisting&#34;</p></td>
<td><p>ReferToExisting will not create account, but create a secret by copying data from referred secret file.</p><br /></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionScope">ProvisionScope
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>)
</p>
<div>
<p>ProvisionScope defines the scope (within component) of provision.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;AllPods&#34;</p></td>
<td><p>AllPods will create accounts for all pods belong to the component.</p><br /></td>
</tr><tr><td><p>&#34;AnyPods&#34;</p></td>
<td><p>AndyPods will only create accounts on one pod.</p><br /></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionSecretRef">ProvisionSecretRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>)
</p>
<div>
<p>ProvisionSecretRef defines the information of secret referred to.</p><br />
</div>
<table>
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
<p>name refers to the name of the secret.</p><br />
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
<p>namespace refers to the namespace of the secret.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionStatements">ProvisionStatements
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>)
</p>
<div>
<p>ProvisionStatements defines the statements used to create accounts.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>creation</code><br/>
<em>
string
</em>
</td>
<td>
<p>creation specifies statement how to create this account with required privileges.</p><br />
</td>
</tr>
<tr>
<td>
<code>update</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>update specifies statement how to update account&rsquo;s password.</p><br />
</td>
</tr>
<tr>
<td>
<code>deletion</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>deletion specifies statement how to delete this account.<br />Used in combination with <code>CreateionStatement</code> to delete the account before create it.<br />For instance, one usually uses <code>drop user if exists</code> statement followed by <code>create user</code> statement to create an account.<br />Deprecated: this field is deprecated, use <code>update</code> instead.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>configurations</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Configuration">
[]Configuration
</a>
</em>
</td>
<td>
<p>configurations defines which components perform the operation.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestStatus">OpsRequestStatus</a>)
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
<code>configurationStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationStatus">
[]ConfigurationStatus
</a>
</em>
</td>
<td>
<p>configurationStatus describes the status of the component reconfiguring.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RefNamespaceName">RefNamespaceName
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupRefSpec">BackupRefSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.PointInTimeRefSpec">PointInTimeRefSpec</a>)
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
<p>specified the name</p><br />
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
<p>specified the namespace</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>ReloadOptions defines reload options<br />Only one of its members may be specified.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>unixSignalTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UnixSignalTrigger">
UnixSignalTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>unixSignalTrigger used to reload by sending a signal.</p><br />
</td>
</tr>
<tr>
<td>
<code>shellTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ShellTrigger">
ShellTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>shellTrigger performs the reload command.</p><br />
</td>
</tr>
<tr>
<td>
<code>tplScriptTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TPLScriptTrigger">
TPLScriptTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>goTplTrigger performs the reload command.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicationMemberStatus">ReplicationMemberStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReplicationSetStatus">ReplicationSetStatus</a>)
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
<code>pod</code><br/>
<em>
string
</em>
</td>
<td>
<p>Pod name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicationSetSpec">ReplicationSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>StatefulSetSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">
StatefulSetSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>StatefulSetSpec</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicationSetStatus">ReplicationSetStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>)
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
<code>primary</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicationMemberStatus">
ReplicationMemberStatus
</a>
</em>
</td>
<td>
<p>Primary status.</p><br />
</td>
</tr>
<tr>
<td>
<code>secondaries</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicationMemberStatus">
[]ReplicationMemberStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secondaries status.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">ResourceConstraintRule
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSpec">ComponentResourceConstraintSpec</a>)
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
<p>The name of the constraint.</p><br />
</td>
</tr>
<tr>
<td>
<code>cpu</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CPUConstraint">
CPUConstraint
</a>
</em>
</td>
<td>
<p>The constraint for vcpu cores.</p><br />
</td>
</tr>
<tr>
<td>
<code>memory</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MemoryConstraint">
MemoryConstraint
</a>
</em>
</td>
<td>
<p>The constraint for memory size.</p><br />
</td>
</tr>
<tr>
<td>
<code>storage</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.StorageConstraint">
StorageConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The constraint for storage size.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RestoreFromSpec">RestoreFromSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
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
<code>backup</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupRefSpec">
[]BackupRefSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>use the backup name and component name for restore, support for multiple components&rsquo; recovery.</p><br />
</td>
</tr>
<tr>
<td>
<code>pointInTime</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PointInTimeRefSpec">
PointInTimeRefSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>specified the point in time to recovery</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RetentionSpec">RetentionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.Schedule">Schedule
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.SchedulePolicy">
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
<a href="#apps.kubeblocks.io/v1alpha1.SchedulePolicy">
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
<a href="#apps.kubeblocks.io/v1alpha1.SchedulePolicy">
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
<h3 id="apps.kubeblocks.io/v1alpha1.SchedulePolicy">SchedulePolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Schedule">Schedule</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptConfig">ScriptConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.TPLScriptTrigger">TPLScriptTrigger</a>)
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
<code>scriptConfigMapRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>scriptConfigMapRef used to execute for reload.</p><br />
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
<p>Specify the namespace of the referenced the tpl script ConfigMap object.<br />An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptFrom">ScriptFrom
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">ScriptSpec</a>)
</p>
<div>
<p>ScriptFrom defines the script to be executed from configMap or secret.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>configMapRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#configmapkeyselector-v1-core">
[]Kubernetes core/v1.ConfigMapKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configMapRef defines the configMap to be executed.</p><br />
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretkeyselector-v1-core">
[]Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secretRef defines the secret to be executed.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptSecret">ScriptSecret
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">ScriptSpec</a>)
</p>
<div>
<p>ScriptSecret defines the secret to be used to execute the script.</p><br />
</div>
<table>
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
<p>name is the name of the secret.</p><br />
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
<em>(Optional)</em>
<p>usernameKey field is used to specify the username of the secret.</p><br />
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
<em>(Optional)</em>
<p>passwordKey field is used to specify the password of the secret.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptSpec">ScriptSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>ScriptSpec defines the script to be executed. It is not a general purpose script executor.<br />It is designed to execute the script to perform some specific operations, such as create database, create user, etc.<br />It is applicable for engines, such as MySQL, PostgreSQL, Redis, MongoDB, etc.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
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
<p>exec command with image, by default use the image of kubeblocks-datascript.</p><br />
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptSecret">
ScriptSecret
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secret defines the secret to be used to execute the script.<br />If not specified, the default cluster root credential secret will be used.</p><br />
</td>
</tr>
<tr>
<td>
<code>script</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>script defines the script to be executed.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptFrom">
ScriptFrom
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptFrom defines the script to be executed from configMap or secret.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptSpecSelector">ScriptSpecSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">SwitchoverAction</a>)
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
<p>ScriptSpec name of the referent, refer to componentDefs[x].scriptSpecs[y].Name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServicePort">ServicePort
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceSpec">ServiceSpec</a>)
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
<p>The name of this port within the service. This must be a DNS_LABEL.<br />All ports within a ServiceSpec must have unique names. When considering<br />the endpoints for a Service, this must match the &lsquo;name&rsquo; field in the<br />EndpointPort.</p><br />
</td>
</tr>
<tr>
<td>
<code>protocol</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#protocol-v1-core">
Kubernetes core/v1.Protocol
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The IP protocol for this port. Supports &ldquo;TCP&rdquo;, &ldquo;UDP&rdquo;, and &ldquo;SCTP&rdquo;.<br />Default is TCP.</p><br />
</td>
</tr>
<tr>
<td>
<code>appProtocol</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The application protocol for this port.<br />This field follows standard Kubernetes label syntax.<br />Un-prefixed names are reserved for IANA standard service names (as per<br />RFC-6335 and <a href="https://www.iana.org/assignments/service-names)">https://www.iana.org/assignments/service-names)</a>.<br />Non-standard protocols should use prefixed names such as<br />mycompany.com/my-custom-protocol.</p><br />
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int32
</em>
</td>
<td>
<p>The port that will be exposed by this service.</p><br />
</td>
</tr>
<tr>
<td>
<code>targetPort</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number or name of the port to access on the pods targeted by the service.<br />Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.<br />If this is a string, it will be looked up as a named port in the<br />target Pod&rsquo;s container ports. If this is not specified, the value<br />of the &lsquo;port&rsquo; field is used (an identity map).<br />This field is ignored for services with clusterIP=None, and should be<br />omitted or set equal to the &lsquo;port&rsquo; field.<br />More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service">https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service</a></p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceSpec">ServiceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>ports</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServicePort">
[]ServicePort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The list of ports that are exposed by this service.<br />More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ShellTrigger">ShellTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>)
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
<code>command</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>command used to execute for reload.</p><br />
</td>
</tr>
<tr>
<td>
<code>sync</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify synchronize updates parameters to the config manager.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SignalType">SignalType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.UnixSignalTrigger">UnixSignalTrigger</a>)
</p>
<div>
<p>SignalType defines which signals are valid.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;SIGABRT&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGALRM&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGBUS&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGCHLD&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGCONT&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGFPE&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGHUP&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGILL&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGINT&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGIO&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGKILL&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGPIPE&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGPROF&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGPWR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGQUIT&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGSEGV&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGSTKFLT&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGSTOP&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGSYS&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGTERM&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGTRAP&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGTSTP&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGTTIN&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGTTOU&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGURG&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGUSR1&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGUSR2&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGVTALRM&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGWINCH&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGXCPU&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SIGXFSZ&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SnapshotPolicy">SnapshotPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.BasePolicy">
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
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyHook">
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
<h3 id="apps.kubeblocks.io/v1alpha1.StatefulSetSpec">StatefulSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ConsensusSetSpec">ConsensusSetSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ReplicationSetSpec">ReplicationSetSpec</a>)
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
<code>updateStrategy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UpdateStrategy">
UpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>updateStrategy, Pods update strategy.<br />In case of workloadType=Consensus the update strategy will be following:</p><br /><br /><p>serial: update Pods one by one that guarantee minimum component unavailable time.<br />		Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader<br />bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time.<br />		Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader, keep majority online all the time.<br />parallel: force parallel</p><br />
</td>
</tr>
<tr>
<td>
<code>llPodManagementPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podmanagementpolicytype-v1-apps">
Kubernetes apps/v1.PodManagementPolicyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>llPodManagementPolicy is the low-level controls how pods are created during initial scale up,<br />when replacing pods on nodes, or when scaling down.<br /><code>OrderedReady</code> policy specify where pods are created in increasing order (pod-0, then<br />pod-1, etc) and the controller will wait until each pod is ready before<br />continuing. When scaling down, the pods are removed in the opposite order.<br /><code>Parallel</code> policy specify create pods in parallel<br />to match the desired scale without waiting, and on scale down will delete<br />all pods at once.</p><br />
</td>
</tr>
<tr>
<td>
<code>llUpdateStrategy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#statefulsetupdatestrategy-v1-apps">
Kubernetes apps/v1.StatefulSetUpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>llUpdateStrategy indicates the low-level StatefulSetUpdateStrategy that will be<br />employed to update Pods in the StatefulSet when a revision is made to<br />Template. Will ignore <code>updateStrategy</code> attribute if provided.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.StatefulSetWorkload">StatefulSetWorkload
</h3>
<div>
<p>StatefulSetWorkload interface</p><br />
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.StatelessSetSpec">StatelessSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>updateStrategy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#deploymentstrategy-v1-apps">
Kubernetes apps/v1.DeploymentStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>updateStrategy defines the underlying deployment strategy to use to replace existing pods with new ones.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.StorageConstraint">StorageConstraint
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ResourceConstraintRule">ResourceConstraintRule</a>)
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
<code>min</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum size of storage.</p><br />
</td>
</tr>
<tr>
<td>
<code>max</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#quantity-resource-core">
Kubernetes resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum size of storage.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SwitchPolicyType">SwitchPolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSwitchPolicy">ClusterSwitchPolicy</a>)
</p>
<div>
<p>SwitchPolicyType defines switchPolicy type.<br />Currently, only Noop is supported. MaximumAvailability and MaximumDataProtection will be supported in the future.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;MaximumAvailability&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;MaximumDataProtection&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Noop&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SwitchStepRole">SwitchStepRole
(<code>string</code> alias)</h3>
<div>
<p>SwitchStepRole defines the role to execute the switch command.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;NewPrimary&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;OldPrimary&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Secondaries&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Switchover">Switchover
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
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
<p>instanceName is used to specify the candidate primary or leader instanceName for switchover.<br />If instanceName is set to &ldquo;<em>&rdquo;, it means that no specific primary or leader is specified for the switchover,<br />and the switchoverAction defined in clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate will be executed,<br />It is required that clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate is not empty.<br />If instanceName is set to a valid instanceName other than &ldquo;</em>&rdquo;, it means that a specific candidate primary or leader is specified for the switchover.<br />the value of instanceName can be obtained using <code>kbcli cluster list-instances</code>, any other value is invalid.<br />In this case, the <code>switchoverAction</code> defined in clusterDefinition.componentDefs[x].switchoverSpec.withCandidate will be executed,<br />and it is required that clusterDefinition.componentDefs[x].switchoverSpec.withCandidate is not empty.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SwitchoverAction">SwitchoverAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SwitchoverSpec">SwitchoverSpec</a>)
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
<code>cmdExecutorConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">
CmdExecutorConfig
</a>
</em>
</td>
<td>
<p>cmdExecutorConfig is the executor configuration of the switchover command.</p><br />
</td>
</tr>
<tr>
<td>
<code>scriptSpecSelectors</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptSpecSelector">
[]ScriptSpecSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.<br />Once ScriptSpecSelectors is defined, the scripts defined in scriptSpecs can be referenced in the SwitchoverAction.CmdExecutorConfig.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SwitchoverShortSpec">SwitchoverShortSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>)
</p>
<div>
<p>SwitchoverShortSpec is a short version of SwitchoverSpec, with only CommandExecutorEnvItem field.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cmdExecutorConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CommandExecutorEnvItem">
CommandExecutorEnvItem
</a>
</em>
</td>
<td>
<p>CmdExecutorConfig is the command executor config.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SwitchoverSpec">SwitchoverSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>withCandidate</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">
SwitchoverAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>withCandidate corresponds to the switchover of the specified candidate primary or leader instance.</p><br />
</td>
</tr>
<tr>
<td>
<code>withoutCandidate</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">
SwitchoverAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>withoutCandidate corresponds to a switchover that does not specify a candidate primary or leader instance.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SystemAccountConfig">SystemAccountConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec</a>)
</p>
<div>
<p>SystemAccountConfig specifies how to create and delete system accounts.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.AccountName">
AccountName
</a>
</em>
</td>
<td>
<p>name is the name of a system account.</p><br />
</td>
</tr>
<tr>
<td>
<code>provisionPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">
ProvisionPolicy
</a>
</em>
</td>
<td>
<p>provisionPolicy defines how to create account.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SystemAccountShortSpec">SystemAccountShortSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>)
</p>
<div>
<p>SystemAccountShortSpec is a short version of SystemAccountSpec, with only CmdExecutorConfig field.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cmdExecutorConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CommandExecutorEnvItem">
CommandExecutorEnvItem
</a>
</em>
</td>
<td>
<p>cmdExecutorConfig configs how to get client SDK and perform statements.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
</p>
<div>
<p>SystemAccountSpec specifies information to create system accounts.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cmdExecutorConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">
CmdExecutorConfig
</a>
</em>
</td>
<td>
<p>cmdExecutorConfig configs how to get client SDK and perform statements.</p><br />
</td>
</tr>
<tr>
<td>
<code>passwordConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PasswordConfig">
PasswordConfig
</a>
</em>
</td>
<td>
<p>passwordConfig defines the pattern to generate password.</p><br />
</td>
</tr>
<tr>
<td>
<code>accounts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SystemAccountConfig">
[]SystemAccountConfig
</a>
</em>
</td>
<td>
<p>accounts defines system account config settings.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TLSSecretRef">TLSSecretRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Issuer">Issuer</a>)
</p>
<div>
<p>TLSSecretRef defines Secret contains Tls certs</p><br />
</div>
<table>
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
<p>Name of the Secret</p><br />
</td>
</tr>
<tr>
<td>
<code>ca</code><br/>
<em>
string
</em>
</td>
<td>
<p>CA cert key in Secret</p><br />
</td>
</tr>
<tr>
<td>
<code>cert</code><br/>
<em>
string
</em>
</td>
<td>
<p>Cert key in Secret</p><br />
</td>
</tr>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
<p>Key of TLS private key in Secret</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TPLScriptTrigger">TPLScriptTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>)
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
<code>ScriptConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ScriptConfig">
ScriptConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>ScriptConfig</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>sync</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify synchronize updates parameters to the config manager.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BasePolicy">BasePolicy</a>)
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
<em>(Optional)</em>
<p>select instance of corresponding role for backup, role are:<br />- the name of Leader/Follower/Leaner for Consensus component.<br />- primary or secondary for Replication component.<br />finally, invalid role of the component will be ignored.<br />such as if workload type is Replication and component&rsquo;s replicas is 1,<br />the secondary role is invalid. and it also will be ignored when component is Stateful/Stateless.<br />the role will be transformed to a role LabelSelector for BackupPolicy&rsquo;s target attribute.</p><br />
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
<p>refer to spec.componentDef.systemAccounts.accounts[*].name in ClusterDefinition.<br />the secret created by this account will be used to connect the database.<br />if not set, the secret created by spec.ConnectionCredential of the ClusterDefinition will be used.<br />it will be transformed to a secret for BackupPolicy&rsquo;s target secret.</p><br />
</td>
</tr>
<tr>
<td>
<code>connectionCredentialKey</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConnectionCredentialKey">
ConnectionCredentialKey
</a>
</em>
</td>
<td>
<p>connectionCredentialKey defines connection credential key in secret<br />which created by spec.ConnectionCredential of the ClusterDefinition.<br />it will be ignored when &ldquo;account&rdquo; is set.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TenancyType">TenancyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Affinity">Affinity</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>TenancyType for cluster tenant resources.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;DedicatedNode&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;SharedNode&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TerminationPolicyType">TerminationPolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>TerminationPolicyType defines termination policy types.</p><br />
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
</tr><tr><td><p>&#34;DoNotTerminate&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Halt&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;WipeOut&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ToolConfig">ToolConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ToolsImageSpec">ToolsImageSpec</a>)
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
<p>Specify the name of initContainer. Must be a DNS_LABEL name.</p><br />
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
<p>tools Container image name.</p><br />
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
<p>exec used to execute for init containers.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ToolsImageSpec">ToolsImageSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
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
<code>mountPoint</code><br/>
<em>
string
</em>
</td>
<td>
<p>mountPoint is the mount point of the scripts file.</p><br />
</td>
</tr>
<tr>
<td>
<code>toolConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ToolConfig">
[]ToolConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>toolConfig used to config init container.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UnixSignalTrigger">UnixSignalTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>)
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
<code>signal</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SignalType">
SignalType
</a>
</em>
</td>
<td>
<p>signal is valid for unix signal.<br />e.g: SIGHUP<br />url: ../../internal/configuration/configmap/handler.go:allUnixSignals</p><br />
</td>
</tr>
<tr>
<td>
<code>processName</code><br/>
<em>
string
</em>
</td>
<td>
<p>processName is process name, sends unix signal to proc.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UpdateStrategy">UpdateStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">StatefulSetSpec</a>)
</p>
<div>
<p>UpdateStrategy defines Cluster Component update strategy.</p><br />
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
<h3 id="apps.kubeblocks.io/v1alpha1.UpdatedParameters">UpdatedParameters
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationStatus">ConfigurationStatus</a>)
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
<code>addedKeys</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>addedKeys describes the key added.</p><br />
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
<p>deletedKeys describes the key deleted.</p><br />
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
<p>updatedKeys describes the key updated.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Upgrade">Upgrade
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>Upgrade defines the variables of upgrade operation.</p><br />
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterVersionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>clusterVersionRef references ClusterVersion name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UpgradePolicy">UpgradePolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationStatus">ConfigurationStatus</a>)
</p>
<div>
<p>UpgradePolicy defines the policy of reconfiguring.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;autoReload&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;none&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;simple&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;operatorSyncUpdate&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;parallel&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;rolling&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VersionsContext">VersionsContext
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>)
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
<code>initContainers</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provide ClusterDefinition.spec.componentDefs.podSpec.initContainers override<br />values, typical scenarios are application container image updates.</p><br />
</td>
</tr>
<tr>
<td>
<code>containers</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provide ClusterDefinition.spec.componentDefs.podSpec.containers override<br />values, typical scenarios are application container image updates.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>VerticalScaling defines the variables that need to input when scaling compute resources.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
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
<p>resources specifies the computing resource size of verticalScaling.</p><br />
</td>
</tr>
<tr>
<td>
<code>classDefRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClassDefRef">
ClassDefRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>classDefRef reference class defined in ComponentClassDefinition.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>)
</p>
<div>
<p>VolumeExpansion defines the variables of volume expansion operation.</p><br />
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentOps">
ComponentOps
</a>
</em>
</td>
<td>
<p>
(Members of <code>ComponentOps</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeClaimTemplates</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequestVolumeClaimTemplate">
[]OpsRequestVolumeClaimTemplate
</a>
</em>
</td>
<td>
<p>volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VolumeProtectionSpec">VolumeProtectionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<code>highWatermark</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>The high watermark threshold for volume space usage.<br />If there is any specified volumes who&rsquo;s space usage is over the threshold, the pre-defined &ldquo;LOCK&rdquo; action<br />will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance<br />as read-only. And after that, if all volumes&rsquo; space usage drops under the threshold later, the pre-defined<br />&ldquo;UNLOCK&rdquo; action will be performed to recover the service normally.</p><br />
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ProtectedVolume">
[]ProtectedVolume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes to protect.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VolumeType">VolumeType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VolumeTypeSpec">VolumeTypeSpec</a>)
</p>
<div>
<p>VolumeType defines volume type for backup data or log.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;data&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;log&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VolumeTypeSpec">VolumeTypeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
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
<p>name definition is the same as the name of the VolumeMounts field in PodSpec.Container,<br />similar to the relations of Volumes[<em>].name and VolumesMounts[</em>].name in Pod.Spec.</p><br />
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VolumeType">
VolumeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>type is in enum of &#123;data, log&#125;.<br />VolumeTypeData: the volume is for the persistent data storage.<br />VolumeTypeLog: the volume is for the persistent log storage.</p><br />
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.WorkloadType">WorkloadType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
</p>
<div>
<p>WorkloadType defines ClusterDefinition&rsquo;s component workload type.</p><br />
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Consensus&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Replication&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stateful&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stateless&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
