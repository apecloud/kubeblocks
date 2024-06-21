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
<li>
<a href="#workloads.kubeblocks.io%2fv1alpha1">workloads.kubeblocks.io/v1alpha1</a>
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
<a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.Component">Component</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentClassDefinition">ComponentClassDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraint">ComponentResourceConstraint</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.Configuration">Configuration</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptor">ServiceDescriptor</a>
</li></ul>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate
</h3>
<div>
<p>BackupPolicyTemplate is the Schema for the BackupPolicyTemplates API (defined by provider)</p>
</div>
<table>
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
<p>clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.</p>
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
<p>backupPolicies is a list of backup policy template for the specified componentDefinition.</p>
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
<p>Identifier is a unique identifier for this BackupPolicyTemplate.
this identifier will be the suffix of the automatically generated backupPolicy name.
and must be added when multiple BackupPolicyTemplates exist,
otherwise the generated backupPolicy override will occur.</p>
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
<p>Cluster is the Schema for the clusters API.</p>
</div>
<table>
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
<em>(Optional)</em>
<p>Cluster referencing ClusterDefinition name. This is an immutable attribute.
If ClusterDefRef is not specified, ComponentDef must be specified for each Component in ComponentSpecs.</p>
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
<p>Cluster referencing ClusterVersion name.</p>
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
<p>Cluster termination policy. Valid values are <code>DoNotTerminate</code>, <code>Halt</code>, <code>Delete</code>, <code>WipeOut</code>.</p>
<ul>
<li>DoNotTerminate will block delete operation.</li>
<li>Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.</li>
<li>Delete is based on Halt and deletes PVCs.</li>
<li>WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>shardingSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ShardingSpec">
[]ShardingSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ShardingSpec which is used to define components with a sharding topology structure that make up a cluster.
ShardingSpecs and ComponentSpecs cannot both be empty at the same time.</p>
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
<em>(Optional)</em>
<p>List of componentSpec which is used to define the components that make up a cluster.
ComponentSpecs and ShardingSpecs cannot both be empty at the same time.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterService">
[]ClusterService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>services defines the services to access a cluster.</p>
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
<p>affinity is a group of affinity scheduling rules.</p>
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
<p>tolerations are attached to tolerate any taint that matches the triple <code>key,value,effect</code> using the matching operator <code>operator</code>.</p>
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
<p>tenancy describes how pods are distributed across node.
SharedNode means multiple pods may share the same node.
DedicatedNode means each pod runs on their own dedicated node.</p>
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
<p>availabilityPolicy describes the availability policy, including zone, node, and none.</p>
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
<p>replicas specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified, this value will be ignored.</p>
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
<p>resources specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified, this value will be ignored.</p>
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
<p>storage specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified, this value will be ignored.</p>
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
<p>monitor specifies the configuration of monitor</p>
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
<p>network specifies the configuration of network</p>
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
<p>cluster backup configuration.</p>
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
<p>ClusterDefinition is the Schema for the clusterdefinitions API</p>
</div>
<table>
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
<p>Cluster definition type defines well known application cluster type, e.g. mysql/redis/mongodb</p>
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
<p>componentDefs provides cluster components definitions.</p>
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
<p>Connection credential template used for creating a connection credential
secret for cluster.apps.kubeblocks.io object.</p>
<p>Built-in objects are:</p>
<ul>
<li><code>$(RANDOM_PASSWD)</code> random 8 characters.
dfjksgks</li>
<li><code>$(STRONG_RANDOM_PASSWD)</code> random 16 characters, with mixed cases, digits and symbols.</li>
<li><code>$(UUID)</code> generate a random UUID v4 string.</li>
<li><code>$(UUID_B64)</code> generate a random UUID v4 BASE64 encoded string.</li>
<li><code>$(UUID_STR_B64)</code> generate a random UUID v4 string then BASE64 encoded.</li>
<li><code>$(UUID_HEX)</code> generate a random UUID v4 HEX representation.</li>
<li><code>$(HEADLESS_SVC_FQDN)</code> headless service FQDN placeholder, value pattern is <code>$(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc</code>,
where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;</li>
<li><code>$(SVC_FQDN)</code> service FQDN placeholder, value pattern is <code>$(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc</code>,
where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;</li>
<li><code>$(SVC_PORT_&#123;PORT-NAME&#125;)</code> is ServicePort&rsquo;s port value with specified port name, i.e, a servicePort JSON struct:
<code>&#123;&quot;name&quot;: &quot;mysql&quot;, &quot;targetPort&quot;: &quot;mysqlContainerPort&quot;, &quot;port&quot;: 3306&#125;</code>, and <code>$(SVC_PORT_mysql)</code> in the
connection credential value is 3306.</li>
</ul>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion
</h3>
<div>
<p>ClusterVersion is the Schema for the ClusterVersions API</p>
</div>
<table>
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
<p>ref ClusterDefinition.</p>
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
<p>List of components&rsquo; containers versioning context, i.e., container image ID, container commands, args, and environments.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.Component">Component
</h3>
<div>
<p>Component is the Schema for the components API</p>
</div>
<table>
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
<td><code>Component</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">
ComponentSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>compDef</code><br/>
<em>
string
</em>
</td>
<td>
<p>compDef is the name of the referenced componentDefinition.</p>
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
<p>classDefRef references the class defined in ComponentClassDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRef">
[]ServiceRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:</p>
<ul>
<li>Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.</li>
<li>Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.</li>
</ul>
<p>Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies Labels to override or add for underlying Pods.</p>
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
<p>Specifies Annotations to override or add for underlying Pods.</p>
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
<p>List of environment variables to add.</p>
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
<p>Resources requests and limits of workload.</p>
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
<p>VolumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of volumes to override.</p>
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
<p>Component replicas. The default value is used in ClusterDefinition spec if not specified.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>monitor is a switch to enable monitoring and is set as false by default.
KubeBlocks provides an extension mechanism to support component level monitoring,
which will scrape metrics auto or manually from servers in component and export
metrics to Time Series Database.</p>
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
<p>enabledLogs indicates which log file takes effect in the database cluster.
element is the log type which is defined in ComponentDefinition logConfig.name,
and will set relative variables about this log type in database kernel.</p>
</td>
</tr>
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
<p>serviceAccountName is the name of the ServiceAccount that running component depends on.</p>
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
</td>
</tr>
<tr>
<td>
<code>tlsConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TLSConfig">
TLSConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>RsmTransformPolicy defines the policy generate sts using rsm.
ToSts: rsm transform to statefulSet
ToPod: rsm transform to pods</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#NodeName">
[]k8s.io/apimachinery/pkg/types.NodeName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes defines the list of nodes that pods can schedule
If the RsmTransformPolicy is specified as OneToMul,the list of nodes will be used. If the list of nodes is empty,
no specific node will be assigned. However, if the list of node is filled, all pods will be evenly scheduled
across the nodes in the list.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Instances defines the list of instance to be deleted priorly</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentStatus">
ComponentStatus
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
<p>ComponentClassDefinition is the Schema for the componentclassdefinitions API</p>
</div>
<table>
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
<p>group defines a list of class series that conform to the same constraint.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition
</h3>
<div>
<p>ComponentDefinition is the Schema for the componentdefinitions API</p>
</div>
<table>
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
<td><code>ComponentDefinition</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">
ComponentDefinitionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>provider</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider is the name of the component provider.</p>
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
<p>Description is a brief description of the component.</p>
</td>
</tr>
<tr>
<td>
<code>serviceKind</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceKind defines what kind of well-known service that the component provides (e.g., MySQL, Redis, ETCD, case insensitive).
Cannot be updated.</p>
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
<p>ServiceVersion defines the version of the well-known service that the component provides.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>runtime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podspec-v1-core">
Kubernetes core/v1.PodSpec
</a>
</em>
</td>
<td>
<p>Runtime defines primarily runtime information for the component, including:</p>
<ul>
<li>Init containers</li>
<li>Containers
<ul>
<li>Image</li>
<li>Commands</li>
<li>Args</li>
<li>Envs</li>
<li>Mounts</li>
<li>Ports</li>
<li>Security context</li>
<li>Probes</li>
<li>Lifecycle</li>
</ul></li>
<li>Volumes</li>
</ul>
<p>CPU and memory resource limits, as well as scheduling settings (affinity, toleration, priority), should not be configured within this structure.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>vars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.EnvVar">
[]EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Vars represents user-defined variables.</p>
<p>These variables can be utilized as environment variables for Pods and Actions, or to render the templates of config and script.
When used as environment variables, these variables are placed in front of the environment variables declared in the Pod.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVolume">
[]ComponentVolume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes defines the persistent volumes needed by the component.
The users are responsible for providing these volumes when creating a component instance.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentService">
[]ComponentService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services defines endpoints that can be used to access the component service to manage the component.</p>
<p>In addition, a reserved headless service will be created by default, with the name pattern <code>&#123;clusterName&#125;-&#123;componentName&#125;-headless</code>.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The configs field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.
Cannot be updated.</p>
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
<p>LogConfigs is detail log file config which provided by provider.
Cannot be updated.</p>
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
<p>Monitor is monitoring config which provided by provider.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>scripts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">
[]ComponentTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The scripts field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>policyRules</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#policyrule-v1-rbac">
[]Kubernetes rbac/v1.PolicyRule
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PolicyRules defines the namespaced policy rules required by the component.
If any rule application fails (e.g., due to lack of permissions), the provisioning of the component instance will also fail.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels defines static labels that will be patched to all k8s resources created for the component.
If a label key conflicts with any other system labels or user-specified labels, it will be silently ignored.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>replicasLimit</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicasLimit">
ReplicasLimit
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReplicasLimit defines the limit of valid replicas supported.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>systemAccounts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SystemAccount">
[]SystemAccount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemAccounts defines the pre-defined system accounts required to manage the component.
Cannot be updated.</p>
</td>
</tr>
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
<p>UpdateStrategy defines the strategy for updating the component instance.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicaRole">
[]ReplicaRole
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles defines all the roles that the component can assume.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>roleArbitrator</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RoleArbitrator">
RoleArbitrator
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleArbitrator defines the strategy for electing the component&rsquo;s active role.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycleActions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">
ComponentLifecycleActions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LifecycleActions defines the operational actions that needed to interoperate with the component
service and processes for lifecycle management.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefDeclarations</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefDeclaration">
[]ServiceRefDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceRefDeclarations is used to declare the service reference of the current component.
Cannot be updated.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionStatus">
ComponentDefinitionStatus
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
<p>ComponentResourceConstraint is the Schema for the componentresourceconstraints API</p>
</div>
<table>
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
<p>Component resource constraint rules.</p>
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
<p>selector is used to bind the resource constraint to cluster definitions based on ClusterDefinition API.</p>
</td>
</tr>
<tr>
<td>
<code>componentSelector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSelector">
[]ComponentResourceConstraintSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentSelector is used to bind the resource constraint to components based on ComponentDefinition API.</p>
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
<p>ConfigConstraint is the Schema for the configconstraint API</p>
</div>
<table>
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
<p>reloadOptions indicates whether the process supports reload.</p>
<p>if set, the controller will determine the behavior of the engine instance based on the configuration templates,
restart or reload depending on whether any parameters in the StaticParameters have been modified.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicActionCanBeMerged</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the dynamic reload action and restart action can be merged to a restart action.</p>
<p>When a batch of parameters updates incur both restart &amp; dynamic reload, it works as:
- set to true, the two actions merged to only one restart action
- set to false, the two actions cannot be merged, the actions executed in order [dynamic reload, restart]</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the policy for selecting the parameters of dynamic reload actions.</p>
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
<p>toolConfig used to config init container.</p>
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
<p>downwardAPIOptions is used to watch pod fields.</p>
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
<p>scriptConfigs, list of ScriptConfig, witch these scripts can be used by volume trigger,downward trigger, or tool image</p>
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
<p>cfgSchemaTopLevelName is cue type name, which generates openapi schema.</p>
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
<p>configurationSchema imposes restrictions on database parameter&rsquo;s rule.</p>
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
<p>staticParameters, list of StaticParameter, modifications of them trigger a process restart.</p>
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
<p>dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.</p>
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
<p>immutableParameters describes parameters that prohibit user from modification.</p>
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
<p>selector is used to match the label on the pod,
for example, a pod of the primary is match on the patroni cluster.</p>
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
<p>formatterConfig describes the format of the configuration file, the controller will:</p>
<ol>
<li>parses configuration file</li>
<li>analyzes the modified parameters</li>
<li>applies corresponding policies.</li>
</ol>
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
<h3 id="apps.kubeblocks.io/v1alpha1.Configuration">Configuration
</h3>
<div>
<p>Configuration is the Schema for the configurations API</p>
</div>
<table>
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
<td><code>Configuration</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationSpec">
ConfigurationSpec
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
<p>clusterRef references Cluster name.</p>
</td>
</tr>
<tr>
<td>
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentName is cluster component name.</p>
</td>
</tr>
<tr>
<td>
<code>configItemDetails</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">
[]ConfigurationItemDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>customConfigurationItems describes user-defined config template.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationStatus">
ConfigurationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition
</h3>
<div>
<p>OpsDefinition is the Schema for the opsdefinitions API</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">
OpsDefinitionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>componentDefinitionRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionRef">
[]ComponentDefinitionRef
</a>
</em>
</td>
<td>
<p>Specifies the types of componentDefinitions that are supported by the operation.
It can refer to some variables of the componentDefinition.
If set, any component that does not meet the conditions will be intercepted.</p>
</td>
</tr>
<tr>
<td>
<code>varsRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarsRef">
VarsRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the environment variables that need to be referenced from the target component pod, and will be injected into the job&rsquo;s containers.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the schema used for validation, pruning, and defaulting.</p>
</td>
</tr>
<tr>
<td>
<code>jobSpec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jobspec-v1-batch">
Kubernetes batch/v1.JobSpec
</a>
</em>
</td>
<td>
<p>Describes the job specification for the operation.</p>
</td>
</tr>
<tr>
<td>
<code>preConditions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the job for the operation.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionStatus">
OpsDefinitionStatus
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
<p>References the cluster object.</p>
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
<p>Defines the action to cancel the <code>Pending/Creating/Running</code> opsRequest, supported types: <code>VerticalScaling/HorizontalScaling</code>.
Once set to true, this opsRequest will be canceled and modifying this property again will not take effect.</p>
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
<p>Defines the operation type.</p>
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
<p>OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.</p>
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
<p>Specifies the cluster version by specifying clusterVersionRef.</p>
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
<p>Defines what component need to horizontal scale the specified replicas.</p>
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
<p>Note: Quantity struct can not do immutable check by CEL.
Defines what component and volumeClaimTemplate need to expand the specified storage.</p>
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
<p>Restarts the specified components.</p>
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
<p>Switches over the specified components.</p>
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
<p>Note: Quantity struct can not do immutable check by CEL.
Defines what component need to vertical scale the specified compute resources.</p>
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
<p>Deprecated: replace by reconfigures.
Defines the variables that need to input when updating configuration.</p>
</td>
</tr>
<tr>
<td>
<code>reconfigures</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">
[]Reconfigure
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the variables that need to input when updating configuration.</p>
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
<p>Defines services the component needs to expose.</p>
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
<p>Cluster RestoreFrom backup or point in time.</p>
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
<p>OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.
If not specified, the default value is 0, which means that the start-conditions must be met immediately.</p>
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
<p>Defines the script to be executed.</p>
</td>
</tr>
<tr>
<td>
<code>rebuildFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RebuildInstance">
[]RebuildInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the instances that require re-creation.</p>
</td>
</tr>
<tr>
<td>
<code>backupSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupSpec">
BackupSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines how to backup the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>restoreSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RestoreSpec">
RestoreSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines how to restore the cluster.
Note that this restore operation will roll back cluster services.</p>
</td>
</tr>
<tr>
<td>
<code>customSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomOpsSpec">
CustomOpsSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a custom operation as defined by OpsDefinition.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceDescriptor">ServiceDescriptor
</h3>
<div>
<p>ServiceDescriptor is the Schema for the servicedescriptors API</p>
</div>
<table>
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
<td><code>ServiceDescriptor</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorSpec">
ServiceDescriptorSpec
</a>
</em>
</td>
<td>
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
<p>service kind, indicating the type or nature of the service. It should be well-known application cluster type, e.g. &#123;mysql, redis, mongodb&#125;.
The serviceKind is case-insensitive and supports abbreviations for some well-known databases.
For example, both <code>zk</code> and <code>zookeeper</code> will be considered as a ZooKeeper cluster, and <code>pg</code>, <code>postgres</code>, <code>postgresql</code> will all be considered as a PostgreSQL cluster.</p>
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
<p>The version of the service reference.</p>
</td>
</tr>
<tr>
<td>
<code>endpoint</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>endpoint is the endpoint of the service connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>auth</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConnectionCredentialAuth">
ConnectionCredentialAuth
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>auth is the auth of the service connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>port is the port of the service connection credential.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorStatus">
ServiceDescriptorStatus
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
<h3 id="apps.kubeblocks.io/v1alpha1.AccountName">AccountName
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccountConfig">SystemAccountConfig</a>)
</p>
<div>
<p>AccountName defines system account names.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.Action">Action
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentSwitchover">ComponentSwitchover</a>, <a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">LifecycleActionHandler</a>)
</p>
<div>
<p>Action defines an operational action that can be performed by a component instance.
There are some pre-defined environment variables that can be used when writing action commands, check @BuiltInVars for reference.</p>
<p>An action is considered successful if it returns 0 (or HTTP 200 for HTTP(s) actions). Any other return value or
HTTP status code is considered as a failure, and the action may be retried based on the configured retry policy.</p>
<ul>
<li>If an action exceeds the specified timeout duration, it will be terminated, and the action is considered failed.</li>
<li>If an action produces any data as output, it should be written to stdout (or included in the HTTP response payload for HTTP(s) actions).</li>
<li>If an action encounters any errors, error messages should be written to stderr (or included in the HTTP response payload with a non-200 HTTP status code).</li>
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
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image defines the container image to run the action.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ExecAction">
ExecAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Exec specifies the action to take.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>http</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HTTPAction">
HTTPAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTP specifies the http request to perform.
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
<p>List of environment variables to set in the container.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodSelector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TargetPodSelector">
TargetPodSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetPodSelector defines the way that how to select the target Pod where the action will be performed,
if there may not have a target replica by default.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>matchingKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MatchingKey uses to select the target pod(s) actually.
If the selector is AnyReplica or AllReplicas, the matchingKey will be ignored.
If the selector is RoleSelector, any replica which has the same role with matchingKey will be chosen.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Container defines the name of the container within the target Pod where the action will be executed.
If specified, it must be one of container declared in @Runtime.
If not specified, the first container declared in @Runtime will be used.
Cannot be updated.</p>
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
<p>TimeoutSeconds defines the timeout duration for the action in seconds.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>retryPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RetryPolicy">
RetryPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryPolicy defines the strategy for retrying the action in case of failure.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>preCondition</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreConditionType">
PreConditionType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PreCondition defines the condition when the action will be executed.</p>
<ul>
<li>Immediately: The Action is executed immediately after the Component object is created,
without guaranteeing the availability of the Component and its underlying resources. only after the action is successfully executed will the Component&rsquo;s state turn to ready.</li>
<li>RuntimeReady: The Action is executed after the Component object is created and once all underlying Runtimes are ready.
only after the action is successfully executed will the Component&rsquo;s state turn to ready.</li>
<li>ComponentReady: The Action is executed after the Component object is created and once the Component is ready.
the execution process does not impact the state of the Component and the Cluster.</li>
<li>ClusterReady: The Action is executed after the Cluster object is created and once the Cluster is ready.</li>
</ul>
<p>the execution process does not impact the state of the Component and the Cluster.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Affinity">Affinity
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>)
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
<p>podAntiAffinity describes the anti-affinity level of pods within a component.
Preferred means try spread pods by <code>TopologyKeys</code>.
Required means must spread pods by <code>TopologyKeys</code>.</p>
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
<p>topologyKey is the key of node labels.
Nodes that have a label with this key and identical values are considered to be in the same topology.
It&rsquo;s used as the topology domain for pod anti-affinity and pod spread constraint.
Some well-known label keys, such as <code>kubernetes.io/hostname</code> and <code>topology.kubernetes.io/zone</code>
are often used as TopologyKey, as well as any other custom label key.</p>
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
<p>nodeLabels describes that pods must be scheduled to the nodes with the specified node labels.</p>
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
<p>tenancy describes how pods are distributed across node.
SharedNode means multiple pods may share the same node.
DedicatedNode means each pod runs on their own dedicated node.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.AutoTrigger">AutoTrigger
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
<code>processName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>processName is process name</p>
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
<p>AvailabilityPolicyType for cluster affinity policy.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod
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
<code>BackupMethod</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.BackupMethod
</em>
</td>
<td>
<p>
(Members of <code>BackupMethod</code> are embedded into this type.)
</p>
</td>
</tr>
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
<p>target instance for backup.</p>
</td>
</tr>
<tr>
<td>
<code>envMapping</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.EnvMappingVar">
[]EnvMappingVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>envMapping defines the variables of cluster mapped to env values&rsquo; keys.</p>
</td>
</tr>
</tbody>
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
<em>(Optional)</em>
<p>componentDefRef references componentDef defined in ClusterDefinition spec. Need to
comply with IANA Service Naming rule.</p>
</td>
</tr>
<tr>
<td>
<code>componentDefs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentDef references componentDefinition. Need to
comply with IANA Service Naming rule.</p>
</td>
</tr>
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
<p>target instance for backup.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulePolicy">
[]SchedulePolicy
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
<code>backupMethods</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupMethod">
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
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">BackupPolicyTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate</a>)
</p>
<div>
<p>BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate</p>
</div>
<table>
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
<p>clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.</p>
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
<p>backupPolicies is a list of backup policy template for the specified componentDefinition.</p>
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
<p>Identifier is a unique identifier for this BackupPolicyTemplate.
this identifier will be the suffix of the automatically generated backupPolicy name.
and must be added when multiple BackupPolicyTemplates exist,
otherwise the generated backupPolicy override will occur.</p>
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
<p>BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate</p>
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
<p>Refers to a reference backup that needs to be restored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupSpec">BackupSpec
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
<code>backupName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the backup.</p>
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
<p>Indicates the backupPolicy applied to perform this backup.</p>
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
<p>Defines the backup method that is defined in backupPolicy.</p>
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
should be deleted when the backup custom resource is deleted.
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
<p>Determines a duration up to which the backup should be kept.
Controller will remove all backups that are older than the RetentionPeriod.
For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 2y</li>
<li>months: 6mo</li>
<li>days: 30d</li>
<li>hours: 12h</li>
<li>minutes: 30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m.
If not set, the backup will be kept forever.</p>
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
<p>If backupType is incremental, parentBackupName is required.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.BackupStatusUpdateStage">BackupStatusUpdateStage
(<code>string</code> alias)</h3>
<div>
<p>BackupStatusUpdateStage defines the stage of backup status update.</p>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.BaseBackupType">BaseBackupType
(<code>string</code> alias)</h3>
<div>
<p>BaseBackupType the base backup type, keep synchronized with the BaseBackupType of the data protection API.</p>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.BuiltinActionHandlerType">BuiltinActionHandlerType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">LifecycleActionHandler</a>)
</p>
<div>
<p>BuiltinActionHandlerType defines build-in action handlers provided by Lorry.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;apecloud-postgresql&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;custom&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;etcd&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;mongodb&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;mysql&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;oceanbase&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;official-postgresql&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;polardbx&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;postgresql&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;redis&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;unknown&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;wesql&#34;</p></td>
<td></td>
</tr></tbody>
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
<p>The maximum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range
must be multiple times of Step. It&rsquo;s useful to define a large number of valid values without defining them one by
one. Please see the documentation for Step for some examples.
If Slots is specified, Max, Min, and Step are ignored</p>
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
<p>The minimum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range
must be multiple times of Step. It&rsquo;s useful to define a large number of valid values without defining them one by
one. Please see the documentation for Step for some examples.
If Slots is specified, Max, Min, and Step are ignored</p>
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
<p>The minimum granularity of vcpu cores, [Min, Max] defines a range for valid vcpu cores and the value in this range must be
multiple times of Step.
For example:
1. Min is 2, Max is 8, Step is 2, and the valid vcpu core is &#123;2, 4, 6, 8&#125;.
2. Min is 0.5, Max is 2, Step is 0.5, and the valid vcpu core is &#123;0.5, 1, 1.5, 2&#125;.</p>
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
<p>The valid vcpu cores, it&rsquo;s useful if you want to define valid vcpu cores explicitly.
If Slots is specified, Max, Min, and Step are ignored</p>
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
<p>CfgFileFormat defines formatter of configuration files.</p>
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
<p>CfgReloadType defines reload method.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;auto&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;http&#34;</p></td>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>, <a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>)
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
<p>Name refers to the name of the ComponentClassDefinition.</p>
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
<p>Class refers to the name of the class that is defined in the ComponentClassDefinition.</p>
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
<p>enabled defines whether to enable automated backup.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.RetentionPeriod
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
<tr>
<td>
<code>method</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>backup method name to use, that is defined in backupPolicy.</p>
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
<p>the cron expression for schedule, the timezone is in UTC. see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
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
<p>startingDeadlineMinutes defines the deadline in minutes for starting the backup job
if it misses scheduled time for any reason.</p>
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
<p>repoName is the name of the backupRepo, if not set, will use the default backupRepo.</p>
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
<p>pitrEnabled defines whether to enable point-in-time recovery.</p>
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
<p>ClusterComponentDefinition provides a workload component specification template,
with attributes that strongly work with stateful workloads and day-2 operations
behaviors.</p>
</div>
<table>
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
<p>A component definition name, this name could be used as default name of <code>Cluster.spec.componentSpecs.name</code>,
and so this name is need to conform with same validation rules as <code>Cluster.spec.componentSpecs.name</code>, that
is currently comply with IANA Service Naming rule. This name will apply to &ldquo;apps.kubeblocks.io/component-name&rdquo;
object label value.</p>
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
<p>The description of component definition.</p>
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
<p>workloadType defines type of the workload.</p>
<ul>
<li><code>Stateless</code> is a stateless workload type used to describe stateless applications.</li>
<li><code>Stateful</code> is a stateful workload type used to describe common stateful applications.</li>
<li><code>Consensus</code> is a stateful workload type used to describe applications based on consensus protocols, common consensus protocols such as raft and paxos.</li>
<li><code>Replication</code> is a stateful workload type used to describe applications based on the primary-secondary data replication protocol.</li>
</ul>
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
<p>characterType defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql).
KubeBlocks will generate proper monitor configs for well-known characterType when builtIn is true.</p>
<p>It will also be used in role probe to decide which probe engine to use.
current available candidates are: mysql, postgres, mongodb, redis, etcd, kafka.</p>
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
<p>The configSpec field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.</p>
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
<p>The scriptSpec field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.</p>
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
<p>probes setting for healthy checks.</p>
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
<p>monitor is monitoring config which provided by provider.</p>
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
<p>logConfigs is detail log file config which provided by provider.</p>
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
<p>podSpec define pod spec template of the cluster component.</p>
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
<p>service defines the behavior of a service spec.
provide read-write service when WorkloadType is Consensus.</p>
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
<p>statelessSpec defines stateless related spec if workloadType is Stateless.</p>
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
<p>statefulSpec defines stateful related spec if workloadType is Stateful.</p>
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
<p>consensusSpec defines consensus related spec if workloadType is Consensus, required if workloadType is Consensus.</p>
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
<p>replicationSpec defines replication related spec if workloadType is Replication.</p>
</td>
</tr>
<tr>
<td>
<code>rsmSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">
RSMSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RSMSpec defines workload related spec of this component.
start from KB 0.7.0, RSM(ReplicatedStateMachineSpec) will be the underlying CR which powers all kinds of workload in KB.
RSM is an enhanced stateful workload extension dedicated for heavy-state workloads like databases.</p>
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
<p>horizontalScalePolicy controls the behavior of horizontal scale.</p>
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
<p>Statement to create system account.</p>
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
<p>volumeTypes is used to describe the purpose of the volumes
mapping the name of the VolumeMounts in the PodSpec.Container field,
such as data volume, log volume, etc.
When backing up the volume, the volume can be correctly backed up
according to the volumeType.</p>
<p>For example:</p>
<ul>
<li><code>name: data, type: data</code> means that the volume named <code>data</code> is used to store <code>data</code>.</li>
<li><code>name: binlog, type: log</code> means that the volume named <code>binlog</code> is used to store <code>log</code>.</li>
</ul>
<p>NOTE: When volumeTypes is not defined, the backup function will not be supported,
even if a persistent volume has been specified.</p>
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
<p>customLabelSpecs is used for custom label tags which you want to add to the component resources.</p>
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
<p>switchoverSpec defines command to do switchover.
in particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.</p>
</td>
</tr>
<tr>
<td>
<code>postStartSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PostStartAction">
PostStartAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>postStartSpec defines the command to be executed when the component is ready, and the command will only be executed once after the component becomes ready.</p>
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
<p>componentDefRef is used to inject values from other components into the current component.
values will be saved and updated in a configmap and mounted to the current component.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefDeclarations</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefDeclaration">
[]ServiceRefDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>serviceRefDeclarations is used to declare the service reference of the current component.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentPhase">ClusterComponentPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentStatus">ComponentStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestComponentStatus">OpsRequestComponentStatus</a>)
</p>
<div>
<p>ClusterComponentPhase defines the Cluster CR .status.components.phase</p>
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
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stopping&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentService">ClusterComponentService
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>)
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
<p>Service name</p>
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
<p>serviceType determines how the Service is exposed. Valid
options are ClusterIP, NodePort, and LoadBalancer.</p>
<ul>
<li><code>ClusterIP</code> allocates a cluster-internal IP address for load-balancing
to endpoints. Endpoints are determined by the selector or if that is not
specified, they are determined by manual construction of an Endpoints object or
EndpointSlice objects. If clusterIP is &ldquo;None&rdquo;, no virtual IP is
allocated and the endpoints are published as a set of endpoints rather
than a virtual IP.</li>
<li><code>NodePort</code> builds on ClusterIP and allocates a port on every node which
routes to the same endpoints as the clusterIP.</li>
<li><code>LoadBalancer</code> builds on NodePort and creates an external load-balancer
(if supported in the current cloud) which routes to the same endpoints
as the clusterIP.</li>
</ul>
<p>More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a>.</p>
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
<p>If ServiceType is LoadBalancer, cloud provider related parameters can be put here
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ShardingSpec">ShardingSpec</a>)
</p>
<div>
<p>ClusterComponentSpec defines the cluster component spec.</p>
</div>
<table>
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
<p>name defines cluster&rsquo;s component name, this name is also part of Service DNS name, so this name will comply with IANA Service Naming rule.
When ClusterComponentSpec is referenced as a template, name is optional. Otherwise, it is required.</p>
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
<em>(Optional)</em>
<p>componentDefRef references componentDef defined in ClusterDefinition spec. Need to comply with IANA Service Naming rule.</p>
</td>
</tr>
<tr>
<td>
<code>componentDef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentDef references the name of the ComponentDefinition.
If both componentDefRef and componentDef are provided, the componentDef will take precedence over componentDefRef.</p>
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
<p>classDefRef references the class defined in ComponentClassDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRef">
[]ServiceRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:</p>
<ul>
<li>Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.</li>
<li>Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.</li>
</ul>
<p>Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.</p>
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
<p>monitor is a switch to enable monitoring and is set as false by default.
KubeBlocks provides an extension mechanism to support component level monitoring,
which will scrape metrics auto or manually from servers in component and export
metrics to Time Series Database.</p>
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
<p>enabledLogs indicates which log file takes effect in the database cluster.
element is the log type which is defined in cluster definition logConfig.name,
and will set relative variables about this log type in database kernel.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies Labels to override or add for underlying Pods.</p>
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
<p>Specifies Annotations to override or add for underlying Pods.</p>
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
<p>List of environment variables to add.
These environment variables will be placed after the environment variables declared in the Pod.</p>
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
<p>Component replicas.</p>
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
<p>affinity describes affinities specified by users.</p>
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
<p>Component tolerations will override ClusterSpec.Tolerations if specified.</p>
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
<p>Resources requests and limits of workload.</p>
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
<p>volumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of volumes to override.</p>
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
<p>Services expose endpoints that can be accessed by clients.</p>
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
<p>switchPolicy defines the strategy for switchover and failover when workloadType is Replication.</p>
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
<p>Enables or disables TLS certs.</p>
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
<p>issuer defines provider context for TLS certs.
required when TLS enabled</p>
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
<p>serviceAccountName is the name of the ServiceAccount that running component depends on.</p>
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
<p>noCreatePDB defines the PodDisruptionBudget creation behavior and is set to true if creation of PodDisruptionBudget
for this component is not needed. It defaults to false.</p>
</td>
</tr>
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
<p>updateStrategy defines the update strategy for the component.</p>
</td>
</tr>
<tr>
<td>
<code>userResourceRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UserResourceRefs">
UserResourceRefs
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>userResourceRefs defines the user-defined volumes.</p>
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
<p>RsmTransformPolicy defines the policy generate sts using rsm.
ToSts: rsm transforms to statefulSet
ToPod: rsm transforms to pods</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#NodeName">
[]k8s.io/apimachinery/pkg/types.NodeName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes defines the list of nodes that pods can schedule
If the RsmTransformPolicy is specified as ToPod,the list of nodes will be used. If the list of nodes is empty,
no specific node will be assigned. However, if the list of node is filled, all pods will be evenly scheduled
across the nodes in the list.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Instances defines the list of instance to be deleted priorly
If the RsmTransformPolicy is specified as ToPod,the list of instances will be used.</p>
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
<p>ClusterComponentStatus records components status.</p>
</div>
<table>
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
<p>phase describes the phase of the component and the detail information of the phases are as following:</p>
<ul>
<li>Creating: <code>Creating</code> is a special <code>Updating</code> with previous phase <code>empty</code>(means &ldquo;&rdquo;) or <code>Creating</code>.</li>
<li>Running: component replicas &gt; 0 and all pod specs are latest with a Running state.</li>
<li>Updating: component replicas &gt; 0 and has no failed pods. the component is being updated.</li>
<li>Abnormal: component replicas &gt; 0 but having some failed pods. the component basically works but in a fragile state.</li>
<li>Failed: component replicas &gt; 0 but having some failed pods. the component doesn&rsquo;t work anymore.</li>
<li>Stopping: component replicas = 0 and has terminating pods.</li>
<li>Stopped: component replicas = 0 and all pods have been deleted.</li>
<li>Deleting: the component is being deleted.</li>
</ul>
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
<p>message records the component details message in current phase.
Keys are podName or deployName or statefulSetName. The format is <code>ObjectKind/Name</code>.</p>
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
<p>podsReady checks if all pods of the component are ready.</p>
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
<p>podsReadyTime what time point of all component pods are ready,
this time is the ready time of the last component pod.</p>
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
<p>consensusSetStatus specifies the mapping of role and pod name.</p>
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
<p>replicationSetStatus specifies the mapping of role and pod name.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionSpec">ClusterVersionSpec</a>)
</p>
<div>
<p>ClusterComponentVersion is an application version component spec.</p>
</div>
<table>
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
<p>componentDefRef reference one of the cluster component definition names in ClusterDefinition API (spec.componentDefs.name).</p>
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
<p>configSpecs defines a configuration extension mechanism to handle configuration differences between versions,
the configTemplateRefs field, together with configTemplateRefs in the ClusterDefinition,
determines the final configuration file.</p>
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
<p>systemAccountSpec define image for the component to connect database or engines.
It overrides <code>image</code> and <code>env</code> attributes defined in ClusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.
To clean default envs settings, set <code>SystemAccountSpec.CmdExecutorConfig.Env</code> to empty list.</p>
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
<p>versionContext defines containers images&rsquo; context for component versions,
this value replaces ClusterDefinition.spec.componentDefs.podSpec.[initContainers | containers]</p>
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
<p>switchoverSpec defines images for the component to do switchover.
It overrides <code>image</code> and <code>env</code> attributes defined in ClusterDefinition.spec.componentDefs.SwitchoverSpec.CommandExecutorEnvItem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterComponentVolumeClaimTemplate">ClusterComponentVolumeClaimTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>)
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
<p>Reference <code>ClusterDefinition.spec.componentDefs.containers.volumeMounts.name</code>.</p>
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
<p>spec defines the desired characteristics of a volume requested by a pod author.</p>
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
<p>accessModes contains the desired access modes the volume should have.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1</a>.</p>
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
<p>resources represents the minimum resources the volume should have.
If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
that are lower than previous value but must still be higher than capacity recorded in the
status field of the claim.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources">https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources</a>.</p>
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
<p>storageClassName is the name of the StorageClass required by the claim.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1</a>.</p>
</td>
</tr>
<tr>
<td>
<code>volumeMode</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumemode-v1-core">
Kubernetes core/v1.PersistentVolumeMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeMode defines what type of volume is required by the claim.</p>
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
<p>How often (in seconds) to perform the probe.</p>
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
<p>Number of seconds after which the probe times out. Defaults to 1 second.</p>
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
<p>Minimum consecutive failures for the probe to be considered failed after having succeeded.</p>
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
<p>commands used to execute for probe.</p>
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
<p>Write check executed on probe sidecar, used to check workload&rsquo;s allow write access.</p>
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
<p>Read check executed on probe sidecar, used to check workload&rsquo;s readonly access.</p>
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
<p>Probe for DB running check.</p>
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
<p>Probe for DB status check.</p>
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
<p>Probe for DB role changed check.</p>
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
<p>roleProbeTimeoutAfterPodsReady(in seconds), when all pods of the component are ready,
it will detect whether the application is available in the pod.
if pods exceed the InitializationTimeoutSeconds time without a role label,
this component will enter the Failed/Abnormal phase.</p>
<p>Note that this configuration will only take effect if the component supports RoleProbe
and will not affect the life cycle of the pod. default values are 60 seconds.</p>
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
<p>ClusterDefinitionSpec defines the desired state of ClusterDefinition</p>
</div>
<table>
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
<p>Cluster definition type defines well known application cluster type, e.g. mysql/redis/mongodb</p>
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
<p>componentDefs provides cluster components definitions.</p>
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
<p>Connection credential template used for creating a connection credential
secret for cluster.apps.kubeblocks.io object.</p>
<p>Built-in objects are:</p>
<ul>
<li><code>$(RANDOM_PASSWD)</code> random 8 characters.
dfjksgks</li>
<li><code>$(STRONG_RANDOM_PASSWD)</code> random 16 characters, with mixed cases, digits and symbols.</li>
<li><code>$(UUID)</code> generate a random UUID v4 string.</li>
<li><code>$(UUID_B64)</code> generate a random UUID v4 BASE64 encoded string.</li>
<li><code>$(UUID_STR_B64)</code> generate a random UUID v4 string then BASE64 encoded.</li>
<li><code>$(UUID_HEX)</code> generate a random UUID v4 HEX representation.</li>
<li><code>$(HEADLESS_SVC_FQDN)</code> headless service FQDN placeholder, value pattern is <code>$(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc</code>,
where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;</li>
<li><code>$(SVC_FQDN)</code> service FQDN placeholder, value pattern is <code>$(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc</code>,
where 1ST_COMP_NAME is the 1st component that provide <code>ClusterDefinition.spec.componentDefs[].service</code> attribute;</li>
<li><code>$(SVC_PORT_&#123;PORT-NAME&#125;)</code> is ServicePort&rsquo;s port value with specified port name, i.e, a servicePort JSON struct:
<code>&#123;&quot;name&quot;: &quot;mysql&quot;, &quot;targetPort&quot;: &quot;mysqlContainerPort&quot;, &quot;port&quot;: 3306&#125;</code>, and <code>$(SVC_PORT_mysql)</code> in the
connection credential value is 3306.</li>
</ul>
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
<p>ClusterDefinitionStatus defines the observed state of ClusterDefinition</p>
</div>
<table>
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
<p>ClusterDefinition phase, valid values are <code>empty</code>, <code>Available</code>, <code>Unavailable</code>.
Available is ClusterDefinition become available, and can be referenced for co-related objects.</p>
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
<p>Extra message in current phase</p>
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
ClusterDefinition. It corresponds to the ClusterDefinition&rsquo;s generation, which is
updated on mutation by the API Server.</p>
</td>
</tr>
</tbody>
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
<p>monitoringInterval specifies interval of monitoring, no monitor if set to 0</p>
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
<p>hostNetworkAccessible specifies whether host network is accessible. It defaults to false</p>
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
<p>publiclyAccessible specifies whether it is publicly accessible. It defaults to false</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterObjectReference">ClusterObjectReference
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CredentialVarSelector">CredentialVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVarSelector">ServiceRefVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceVarSelector">ServiceVarSelector</a>)
</p>
<div>
<p>ClusterObjectReference defines information to let you locate the referenced object inside the same cluster.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>compDef</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CompDef specifies the definition used by the component that the referent object resident in.
If not specified, the component itself will be used.</p>
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
<em>(Optional)</em>
<p>Name of the referent object.</p>
</td>
</tr>
<tr>
<td>
<code>optional</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify whether the object must be defined.</p>
</td>
</tr>
<tr>
<td>
<code>multipleClusterObjectOption</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectOption">
MultipleClusterObjectOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>This option defines the behavior when multiple component objects match the specified @CompDef.
If not provided, an error will be raised when handling multiple matches.</p>
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
<p>ClusterPhase defines the Cluster CR .status.phase</p>
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
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Stopping&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
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
<p>clusterDefRef is the name of the cluster definition.</p>
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
<p>selector is used to bind the resource constraint to components.</p>
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
<p>cpu resource needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>memory resource needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterService">ClusterService
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>ClusterService defines the service of a cluster.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Service</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Service">
Service
</a>
</em>
</td>
<td>
<p>
(Members of <code>Service</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>shardingSelector</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShardingSelector extends the ServiceSpec.Selector by allowing you to specify a sharding name
defined in Cluster.Spec.ShardingSpecs[x].Name as selectors for the service.
ShardingSelector and ComponentSelector cannot be set at the same time.</p>
</td>
</tr>
<tr>
<td>
<code>componentSelector</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComponentSelector extends the ServiceSpec.Selector by allowing you to specify a component as selectors for the service.
ComponentSelector and ShardingSelector cannot be set at the same time.</p>
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
<p>ClusterSpec defines the desired state of Cluster.</p>
</div>
<table>
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
<em>(Optional)</em>
<p>Cluster referencing ClusterDefinition name. This is an immutable attribute.
If ClusterDefRef is not specified, ComponentDef must be specified for each Component in ComponentSpecs.</p>
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
<p>Cluster referencing ClusterVersion name.</p>
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
<p>Cluster termination policy. Valid values are <code>DoNotTerminate</code>, <code>Halt</code>, <code>Delete</code>, <code>WipeOut</code>.</p>
<ul>
<li>DoNotTerminate will block delete operation.</li>
<li>Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.</li>
<li>Delete is based on Halt and deletes PVCs.</li>
<li>WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>shardingSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ShardingSpec">
[]ShardingSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of ShardingSpec which is used to define components with a sharding topology structure that make up a cluster.
ShardingSpecs and ComponentSpecs cannot both be empty at the same time.</p>
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
<em>(Optional)</em>
<p>List of componentSpec which is used to define the components that make up a cluster.
ComponentSpecs and ShardingSpecs cannot both be empty at the same time.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterService">
[]ClusterService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>services defines the services to access a cluster.</p>
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
<p>affinity is a group of affinity scheduling rules.</p>
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
<p>tolerations are attached to tolerate any taint that matches the triple <code>key,value,effect</code> using the matching operator <code>operator</code>.</p>
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
<p>tenancy describes how pods are distributed across node.
SharedNode means multiple pods may share the same node.
DedicatedNode means each pod runs on their own dedicated node.</p>
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
<p>availabilityPolicy describes the availability policy, including zone, node, and none.</p>
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
<p>replicas specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified, this value will be ignored.</p>
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
<p>resources specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified, this value will be ignored.</p>
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
<p>storage specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified, this value will be ignored.</p>
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
<p>monitor specifies the configuration of monitor</p>
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
<p>network specifies the configuration of network</p>
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
<p>cluster backup configuration.</p>
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
<p>ClusterStatus defines the observed state of Cluster.</p>
</div>
<table>
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
Cluster. It corresponds to the Cluster&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>phase describes the phase of the Cluster, the detail information of the phases are as following:</p>
<ul>
<li>Creating: all components are in <code>Creating</code> phase.</li>
<li>Running: all components are in <code>Running</code> phase, means the cluster is working well.</li>
<li>Updating: all components are in <code>Creating</code>, <code>Running</code> or <code>Updating</code> phase,
and at least one component is in <code>Creating</code> or <code>Updating</code> phase, means the cluster is doing an update.</li>
<li>Stopping: at least one component is in <code>Stopping</code> phase, means the cluster is in a stop progress.</li>
<li>Stopped: all components are in <code>Stopped</code> phase, means the cluster has stopped and didn&rsquo;t provide any function anymore.</li>
<li>Failed: all components are in <code>Failed</code> phase, means the cluster is unavailable.</li>
<li>Abnormal: some components are in <code>Failed</code> or <code>Abnormal</code> phase, means the cluster in a fragile state. troubleshoot need to be done.</li>
<li>Deleting: the cluster is being deleted.</li>
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
<em>(Optional)</em>
<p>message describes cluster details message in current phase.</p>
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
<p>components record the current status information of all components of the cluster.</p>
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
<p>clusterDefGeneration represents the generation number of ClusterDefinition referenced.</p>
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
<p>Describe current state of cluster API Resource, like warning.</p>
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
<p>storage size needed, more info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>clusterSwitchPolicy defines type of the switchPolicy when workloadType is Replication.</p>
<ul>
<li>MaximumAvailability: [WIP] when the primary is active, do switch if the synchronization delay = 0 in the user-defined lagProbe data delay detection logic, otherwise do not switch. The primary is down, switch immediately. It will be available in future versions.</li>
<li>MaximumDataProtection: [WIP] when the primary is active, do switch if synchronization delay = 0 in the user-defined lagProbe data lag detection logic, otherwise do not switch. If the primary is down, if it can be judged that the primary and secondary data are consistent, then do the switch, otherwise do not switch. It will be available in future versions.</li>
<li>Noop: KubeBlocks will not perform high-availability switching on components. Users need to implement HA by themselves or integrate open source HA solution.</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterVersionSpec">ClusterVersionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>)
</p>
<div>
<p>ClusterVersionSpec defines the desired state of ClusterVersion</p>
</div>
<table>
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
<p>ref ClusterDefinition.</p>
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
<p>List of components&rsquo; containers versioning context, i.e., container image ID, container commands, args, and environments.</p>
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
<p>ClusterVersionStatus defines the observed state of ClusterVersion</p>
</div>
<table>
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
<p>A human readable message indicating details about why the ClusterVersion is in this phase.</p>
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
<p>generation number</p>
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
<p>clusterDefGeneration represents the generation number of ClusterDefinition referenced.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">CmdExecutorConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PostStartAction">PostStartAction</a>, <a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">SwitchoverAction</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec</a>)
</p>
<div>
<p>CmdExecutorConfig specifies how to perform creation and deletion statements.</p>
</div>
<table>
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
<p>image for Connector when executing the command.</p>
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
<p>envs is a list of environment variables.</p>
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
<p>command to perform statements.</p>
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
<p>args is used to perform statements.</p>
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
<p>name is the class name</p>
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
<p>args are variable&rsquo;s value</p>
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
<p>the CPU of the class</p>
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
<p>the memory of the class</p>
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
<p>ComponentClassDefinitionSpec defines the desired state of ComponentClassDefinition</p>
</div>
<table>
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
<p>group defines a list of class series that conform to the same constraint.</p>
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
<p>ComponentClassDefinitionStatus defines the observed state of ComponentClassDefinition</p>
</div>
<table>
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
ComponentClassDefinition. It corresponds to the ComponentClassDefinition&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>classes is the list of classes that have been observed for this ComponentClassDefinition</p>
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
<p>template is a class definition template that uses the Go template syntax and allows for variable declaration.
When defining a class in Series, specifying the variable&rsquo;s value is sufficient, as the complete class
definition will be generated through rendering the template.</p>
<p>For example:</p>
<pre><code class="language-yaml">template: |
cpu: &quot;&#123;&#123; or .cpu 1 &#125;&#125;&quot;
memory: &quot;&#123;&#123; or .memory 4 &#125;&#125;Gi&quot;
</code></pre>
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
<p>vars defines the variables declared in the template and will be used to generating the complete class definition by
render the template.</p>
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
<p>series is a series of class definitions.</p>
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
<p>namingTemplate is a template that uses the Go template syntax and allows for referencing variables defined
in ComponentClassGroup.Template. This enables dynamic generation of class names.
For example:
name: &ldquo;general-&#123;&#123; .cpu &#125;&#125;c&#123;&#123; .memory &#125;&#125;g&rdquo;</p>
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
<p>classes are definitions of classes that come in two forms. In the first form, only ComponentClass.Args
need to be defined, and the complete class definition is generated by rendering the ComponentClassGroup.Template
and Name. In the second form, the Name, CPU and Memory must be defined.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">ConfigurationItemDetail</a>)
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
<p>Specify a list of keys.
If empty, ConfigConstraint takes effect for all keys in configmap.</p>
</td>
</tr>
<tr>
<td>
<code>legacyRenderedConfigSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LegacyRenderedTemplateSpec">
LegacyRenderedTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>lazyRenderedConfigSpec is optional: specify the secondary rendered config spec.</p>
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
<p>Specify the name of the referenced the configuration constraints object.</p>
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
<p>asEnvFrom is optional: the list of containers will be injected into EnvFrom.</p>
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
<p>ComponentDefRef is used to select the component and its fields to be referenced.</p>
</div>
<table>
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
<p>componentDefName is the name of the componentDef to select.</p>
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
<p>failurePolicy is the failure policy of the component.
If failed to find the component, the failure policy will be used.</p>
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
<p>componentRefEnv specifies a list of values to be injected as env variables to each component.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinitionRef">ComponentDefinitionRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<p>Refers to the name of the component definition. This is a required field with a maximum length of 32 characters.</p>
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
<p>Represents the account name of the component.
If provided, the account username and password will be injected into the job environment variables <code>KB_ACCOUNT_USERNAME</code> and <code>KB_ACCOUNT_PASSWORD</code>.</p>
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
<p>References the name of the service.
If provided, the service name and ports will be mapped to the job environment variables <code>KB_COMP_SVC_NAME</code> and <code>KB_COMP_SVC_PORT_$(portName)</code>.
Note that the portName will replace the characters &lsquo;-&rsquo; with &lsquo;_&rsquo; and convert to uppercase.</p>
</td>
</tr>
<tr>
<td>
<code>varsRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarsRef">
VarsRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the environment variables that need to be referenced from the target component pod and will be injected into the job&rsquo;s containers.
If this field is set, the global &ldquo;varsRef&rdquo; will be ignored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition</a>)
</p>
<div>
<p>ComponentDefinitionSpec provides a workload component specification with attributes that strongly work with stateful workloads and day-2 operation behaviors.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>provider</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider is the name of the component provider.</p>
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
<p>Description is a brief description of the component.</p>
</td>
</tr>
<tr>
<td>
<code>serviceKind</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceKind defines what kind of well-known service that the component provides (e.g., MySQL, Redis, ETCD, case insensitive).
Cannot be updated.</p>
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
<p>ServiceVersion defines the version of the well-known service that the component provides.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>runtime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#podspec-v1-core">
Kubernetes core/v1.PodSpec
</a>
</em>
</td>
<td>
<p>Runtime defines primarily runtime information for the component, including:</p>
<ul>
<li>Init containers</li>
<li>Containers
<ul>
<li>Image</li>
<li>Commands</li>
<li>Args</li>
<li>Envs</li>
<li>Mounts</li>
<li>Ports</li>
<li>Security context</li>
<li>Probes</li>
<li>Lifecycle</li>
</ul></li>
<li>Volumes</li>
</ul>
<p>CPU and memory resource limits, as well as scheduling settings (affinity, toleration, priority), should not be configured within this structure.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>vars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.EnvVar">
[]EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Vars represents user-defined variables.</p>
<p>These variables can be utilized as environment variables for Pods and Actions, or to render the templates of config and script.
When used as environment variables, these variables are placed in front of the environment variables declared in the Pod.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVolume">
[]ComponentVolume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volumes defines the persistent volumes needed by the component.
The users are responsible for providing these volumes when creating a component instance.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentService">
[]ComponentService
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services defines endpoints that can be used to access the component service to manage the component.</p>
<p>In addition, a reserved headless service will be created by default, with the name pattern <code>&#123;clusterName&#125;-&#123;componentName&#125;-headless</code>.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The configs field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.
Cannot be updated.</p>
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
<p>LogConfigs is detail log file config which provided by provider.
Cannot be updated.</p>
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
<p>Monitor is monitoring config which provided by provider.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>scripts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">
[]ComponentTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The scripts field provided by provider, and
finally this configTemplateRefs will be rendered into the user&rsquo;s own configuration file according to the user&rsquo;s cluster.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>policyRules</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#policyrule-v1-rbac">
[]Kubernetes rbac/v1.PolicyRule
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PolicyRules defines the namespaced policy rules required by the component.
If any rule application fails (e.g., due to lack of permissions), the provisioning of the component instance will also fail.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels defines static labels that will be patched to all k8s resources created for the component.
If a label key conflicts with any other system labels or user-specified labels, it will be silently ignored.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>replicasLimit</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicasLimit">
ReplicasLimit
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReplicasLimit defines the limit of valid replicas supported.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>systemAccounts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SystemAccount">
[]SystemAccount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemAccounts defines the pre-defined system accounts required to manage the component.
Cannot be updated.</p>
</td>
</tr>
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
<p>UpdateStrategy defines the strategy for updating the component instance.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReplicaRole">
[]ReplicaRole
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles defines all the roles that the component can assume.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>roleArbitrator</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RoleArbitrator">
RoleArbitrator
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleArbitrator defines the strategy for electing the component&rsquo;s active role.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycleActions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">
ComponentLifecycleActions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LifecycleActions defines the operational actions that needed to interoperate with the component
service and processes for lifecycle management.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefDeclarations</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefDeclaration">
[]ServiceRefDeclaration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceRefDeclarations is used to declare the service reference of the current component.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinitionStatus">ComponentDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition</a>)
</p>
<div>
<p>ComponentDefinitionStatus defines the observed state of ComponentDefinition.</p>
</div>
<table>
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
<p>ObservedGeneration is the most recent generation observed for this ComponentDefinition.</p>
</td>
</tr>
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
<p>Phase valid values are `<code>,</code>Available<code>,</code>Unavailable`.
Available is ComponentDefinition become available, and can be used for co-related objects.</p>
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
<p>Extra message for current phase.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>ComponentLifecycleActions defines a set of operational actions for interacting with component services and processes.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>postProvision</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PostProvision defines the actions to be executed and the corresponding policy when a component is created.
You can define the preCondition for executing PostProvision using Action.PreCondition. The default PostProvision action preCondition is ComponentReady.
The PostProvision Action will be executed only once.
Dedicated env vars for the action:</p>
<ul>
<li>KB_CLUSTER_COMPONENT_LIST: The list of all components in the cluster, joined by &lsquo;,&rsquo; (e.g., &ldquo;comp1,comp2&rdquo;).</li>
<li>KB_CLUSTER_COMPONENT_POD_NAME_LIST: The list of all pods name in this component, joined by &lsquo;,&rsquo; (e.g., &ldquo;pod1,pod2&rdquo;).</li>
<li>KB_CLUSTER_COMPONENT_POD_IP_LIST: The list of pod IPs where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by &lsquo;,&rsquo; (e.g., &ldquo;podIp1,podIp2&rdquo;).</li>
<li>KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: The list of hostName where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by &lsquo;,&rsquo; (e.g., &ldquo;hostName1,hostName2&rdquo;).</li>
<li>KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: The list of host IPs where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by &lsquo;,&rsquo; (e.g., &ldquo;hostIp1,hostIp2&rdquo;).</li>
</ul>
<p>Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>preTerminate</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PreTerminate defines the actions to be executed when a component is terminated due to an API request.
The PreTerminate Action will be executed only once. Upon receiving a scale-down command for the Component, it is executed immediately.
Only after the preTerminate action is successfully executed, the destruction of the Component and its underlying resources proceeds.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>roleProbe</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RoleProbe">
RoleProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RoleProbe defines the mechanism to probe the role of replicas periodically. The specified action will be
executed by Lorry at the configured interval. If the execution is successful, the output will be used as
the replica&rsquo;s assigned role, and the role must be one of the names defined in the componentdefinition roles.
The output will be compared with the last successful result.  If there is a change, a role change event will
be created to notify the controller and trigger updating the replica&rsquo;s role.
Defining a RoleProbe is required if roles are configured for the component. Otherwise, the replicas&rsquo; pods will
lack role information after the cluster is created, and services will not route to the replica correctly.</p>
<p>The following dedicated environment variables are available for the action:</p>
<ul>
<li>KB_POD_FQDN: The pod FQDN of the replica to check the role.</li>
<li>KB_SERVICE_PORT: The port on which the DB service listens.</li>
<li>KB_SERVICE_USER: The username used to access the DB service and retrieve the role information with sufficient privileges.</li>
<li>KB_SERVICE_PASSWORD: The password of the user used to access the DB service and retrieve the role information.</li>
</ul>
<p>Output of the action:
- ROLE: the role of the replica. It must be one of the names defined in the roles.
- ERROR: Any error message if the action fails.</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>switchover</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentSwitchover">
ComponentSwitchover
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Switchover defines how to proactively switch the current leader to a new replica to minimize the impact on availability.
This action is typically invoked when the leader is about to become unavailable due to events, such as:</p>
<ul>
<li>switchover</li>
<li>stop</li>
<li>restart</li>
<li>scale-in</li>
</ul>
<p>Dedicated env vars for the action:</p>
<ul>
<li>KB_SWITCHOVER_CANDIDATE_NAME: The name of the new candidate replica&rsquo;s Pod. It may be empty.</li>
<li>KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new candidate replica. It may be empty.</li>
<li>KB_LEADER_POD_IP: The IP address of the original leader&rsquo;s Pod before switchover.</li>
<li>KB_LEADER_POD_NAME: The name of the original leader&rsquo;s Pod before switchover.</li>
<li>KB_LEADER_POD_FQDN: The FQDN of the original leader&rsquo;s Pod before switchover.</li>
</ul>
<p>The env vars with following prefix are deprecated and will be removed in the future:</p>
<ul>
<li>KB_REPLICATION_PRIMARY<em>POD</em>: The prefix of the environment variables of the original primary&rsquo;s Pod before switchover.</li>
<li>KB_CONSENSUS_LEADER<em>POD</em>: The prefix of the environment variables of the original leader&rsquo;s Pod before switchover.</li>
</ul>
<p>Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>memberJoin</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberJoin defines how to add a new replica to the replication group.
This action is typically invoked when a new replica needs to be added, such as during scale-out.
The function does not specify or constrain the role of the new member. The role assignment
is handled by the scripts implemented in the action commands. This provides flexibility
as the new member can be automatically scaled and assigned a role based on the cluster&rsquo;s needs.</p>
<p>The following dedicated environment variables are available for the action:</p>
<ul>
<li>KB_SERVICE_PORT: The port on which the DB service listens.</li>
<li>KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.</li>
<li>KB_SERVICE_PASSWORD: The password of the user used to access the DB service .</li>
<li>KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod.</li>
<li>KB_MEMBER_ADDRESSES: The addresses of all members.</li>
<li>KB_NEW_MEMBER_POD_NAME: The name of the new member&rsquo;s Pod.</li>
<li>KB_NEW_MEMBER_POD_IP: The name of the new member&rsquo;s Pod.</li>
</ul>
<p>Output of the action:
- ERROR: Any error message if the action fails.</p>
<p>For example, the following command can be used to add a new OBServer to the OceanBase Cluster in zone1:
command:
- bash
- -c
- |
ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
HOST=$(echo $ADDRESS | cut -d &lsquo;:&rsquo; -f 1)
PORT=$(echo $ADDRESS | cut -d &lsquo;:&rsquo; -f 2)
CLIENT=&ldquo;mysql -u $KB_SERVICE_USER -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e&rdquo;
$CLIENT &ldquo;ALTER SYSTEM ADD SERVER &lsquo;$KB_NEW_MEMBER_POD_IP:$KB_SERVICE_PORT&rsquo; ZONE &lsquo;zone1&rsquo;&rdquo;</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>memberLeave</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemberLeave defines how to remove a replica from the replication group.
This action is typically invoked when a replica needs to be removed, such as during scale-in.
It may involve configuration updates and notifying other members about the departure,
but it is advisable to avoid performing data migration within this action.</p>
<p>The following dedicated environment variables are available for the action:</p>
<ul>
<li>KB_SERVICE_PORT: The port on which the DB service listens.</li>
<li>KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.</li>
<li>KB_SERVICE_PASSWORD: The password of the user used to access the DB service.</li>
<li>KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod.</li>
<li>KB_MEMBER_ADDRESSES: The addresses of all members.</li>
<li>KB_LEAVE_MEMBER_POD_NAME: The name of the leave member&rsquo;s Pod.</li>
<li>KB_LEAVE_MEMBER_POD_IP: The IP of the leave member&rsquo;s Pod.</li>
</ul>
<p>Output of the action:
- ERROR: Any error message if the action fails.</p>
<p>For example, the following command can be used to delete a OBServer from the OceanBase Cluster in zone1:
command:
- bash
- -c
- |
ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
HOST=$(echo $ADDRESS | cut -d &lsquo;:&rsquo; -f 1)
PORT=$(echo $ADDRESS | cut -d &lsquo;:&rsquo; -f 2)
CLIENT=&ldquo;mysql -u $KB_SERVICE_USER  -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e&rdquo;
$CLIENT &ldquo;ALTER SYSTEM DELETE SERVER &lsquo;$KB_NEW_MEMBER_POD_IP:$KB_SERVICE_PORT&rsquo; ZONE &lsquo;zone1&rsquo;&rdquo;</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>readonly</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Readonly defines how to set a replica service as read-only.
This action is used to protect a replica in case of volume space exhaustion or excessive traffic.</p>
<p>The following dedicated environment variables are available for the action:</p>
<ul>
<li>KB_POD_FQDN: The FQDN of the replica pod to check the role.</li>
<li>KB_SERVICE_PORT: The port on which the DB service listens.</li>
<li>KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.</li>
<li>KB_SERVICE_PASSWORD: The password of the user used to access the DB service.</li>
</ul>
<p>Output of the action:
- ERROR: Any error message if the action fails.</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>readwrite</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Readwrite defines how to set a replica service as read-write.</p>
<p>The following dedicated environment variables are available for the action:</p>
<ul>
<li>KB_POD_FQDN: The FQDN of the replica pod to check the role.</li>
<li>KB_SERVICE_PORT: The port on which the DB service listens.</li>
<li>KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.</li>
<li>KB_SERVICE_PASSWORD: The password of the user used to access the DB service.</li>
</ul>
<p>Output of the action:
- ERROR: Any error message if the action fails.</p>
<p>This field cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>dataPopulate</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataPopulate defines how to populate the data to create new replicas.
This action is typically used when a new replica needs to be constructed, such as:</p>
<ul>
<li>scale-out</li>
<li>rebuild</li>
<li>clone</li>
</ul>
<p>It should write the valid data to stdout without including any extraneous information.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>dataAssemble</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataAssemble defines how to assemble data synchronized from external before starting the service for a new replica.
This action is typically used when creating a new replica, such as:</p>
<ul>
<li>scale-out</li>
<li>rebuild</li>
<li>clone</li>
</ul>
<p>The data will be streamed in via stdin. If any error occurs during the assembly process,
the action must be able to guarantee idempotence to allow for retries from the beginning.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>reconfigure</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reconfigure defines how to notify the replica service that there is a configuration update.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>accountProvision</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccountProvision defines how to provision accounts.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentMessageMap">ComponentMessageMap
(<code>map[string]string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentStatus">ComponentStatus</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Expose">Expose</a>, <a href="#apps.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance</a>, <a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">Reconfigure</a>, <a href="#apps.kubeblocks.io/v1alpha1.ScriptSpec">ScriptSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.Switchover">Switchover</a>, <a href="#apps.kubeblocks.io/v1alpha1.VerticalScaling">VerticalScaling</a>, <a href="#apps.kubeblocks.io/v1alpha1.VolumeExpansion">VolumeExpansion</a>)
</p>
<div>
<p>ComponentOps represents the common variables required for operations within the scope of a component.</p>
</div>
<table>
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
<p>Specifies the name of the cluster component.</p>
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
<p>ComponentRefEnv specifies name and value of an env.</p>
</div>
<table>
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
<p>name is the name of the env to be injected, and it must be a C identifier.</p>
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
<p>value is the value of the env to be injected.</p>
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
<p>valueFrom specifies the source of the env to be injected.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSelector">ComponentResourceConstraintSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterResourceConstraintSelector">ClusterResourceConstraintSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSpec">ComponentResourceConstraintSpec</a>)
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
<p>In versions prior to KB 0.8.0, ComponentDefRef is the name of the component definition in the ClusterDefinition.
In KB 0.8.0 and later versions, ComponentDefRef is the name of ComponentDefinition.</p>
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
<p>rules are the constraint rules that will be applied to the component.</p>
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
<p>ComponentResourceConstraintSpec defines the desired state of ComponentResourceConstraint</p>
</div>
<table>
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
<p>Component resource constraint rules.</p>
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
<p>selector is used to bind the resource constraint to cluster definitions based on ClusterDefinition API.</p>
</td>
</tr>
<tr>
<td>
<code>componentSelector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentResourceConstraintSelector">
[]ComponentResourceConstraintSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>componentSelector is used to bind the resource constraint to components based on ComponentDefinition API.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentResourceKey">ComponentResourceKey
(<code>string</code> alias)</h3>
<div>
<p>ComponentResourceKey defines the resource key of component, such as pod/pvc.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentService">ComponentService
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<code>Service</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Service">
Service
</a>
</em>
</td>
<td>
<p>
(Members of <code>Service</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>generatePodOrdinalService</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>GeneratePodOrdinalService indicates whether to create a corresponding Service for each Pod of the selected Component.
If sets to true, a set of Service will be automatically generated for each Pod. And Service.RoleSelector will be ignored.
They can be referred to by adding the PodOrdinal to the defined ServiceName with named pattern <code>$(Service.ServiceName)-$(PodOrdinal)</code>.
And the Service.Name will also be generated with named pattern <code>$(Service.Name)-$(PodOrdinal)</code>.
The PodOrdinal is zero-based, and the number of generated Services is equal to the number of replicas of the Component.
For example, a Service might be defined as follows:</p>
<pre><code class="language-yaml">name: my-service
serviceName: my-service
generatePodOrdinalService: true
spec:
type: NodePort
ports:
- name: http
port: 80
targetPort: 8080
</code></pre>
<p>Assuming that the Component has 3 replicas, then three services would be generated: my-service-0, my-service-1, and my-service-2, each pointing to its respective Pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Component">Component</a>)
</p>
<div>
<p>ComponentSpec defines the desired state of Component</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>compDef</code><br/>
<em>
string
</em>
</td>
<td>
<p>compDef is the name of the referenced componentDefinition.</p>
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
<p>classDefRef references the class defined in ComponentClassDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRef">
[]ServiceRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>serviceRefs define service references for the current component. Based on the referenced services, they can be categorized into two types:</p>
<ul>
<li>Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks. They can be Kubernetes-based or non-Kubernetes services. For external services, you need to provide an additional ServiceDescriptor object to establish the service binding.</li>
<li>Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters. You can bind to these services by specifying the name of the hosting cluster.</li>
</ul>
<p>Each type of service reference requires specific configurations and bindings to establish the connection and interaction with the respective services.
It should be noted that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to the same Cluster or ServiceDescriptor.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies Labels to override or add for underlying Pods.</p>
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
<p>Specifies Annotations to override or add for underlying Pods.</p>
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
<p>List of environment variables to add.</p>
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
<p>Resources requests and limits of workload.</p>
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
<p>VolumeClaimTemplates information for statefulset.spec.volumeClaimTemplates.</p>
</td>
</tr>
<tr>
<td>
<code>volumes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of volumes to override.</p>
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
<p>Component replicas. The default value is used in ClusterDefinition spec if not specified.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
[]ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>monitor is a switch to enable monitoring and is set as false by default.
KubeBlocks provides an extension mechanism to support component level monitoring,
which will scrape metrics auto or manually from servers in component and export
metrics to Time Series Database.</p>
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
<p>enabledLogs indicates which log file takes effect in the database cluster.
element is the log type which is defined in ComponentDefinition logConfig.name,
and will set relative variables about this log type in database kernel.</p>
</td>
</tr>
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
<p>serviceAccountName is the name of the ServiceAccount that running component depends on.</p>
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
</td>
</tr>
<tr>
<td>
<code>tlsConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TLSConfig">
TLSConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>RsmTransformPolicy defines the policy generate sts using rsm.
ToSts: rsm transform to statefulSet
ToPod: rsm transform to pods</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#NodeName">
[]k8s.io/apimachinery/pkg/types.NodeName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes defines the list of nodes that pods can schedule
If the RsmTransformPolicy is specified as OneToMul,the list of nodes will be used. If the list of nodes is empty,
no specific node will be assigned. However, if the list of node is filled, all pods will be evenly scheduled
across the nodes in the list.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Instances defines the list of instance to be deleted priorly</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentStatus">ComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Component">Component</a>)
</p>
<div>
<p>ComponentStatus defines the observed state of Component</p>
</div>
<table>
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
<p>observedGeneration is the most recent generation observed for this Component.
It corresponds to the Cluster&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>Describe current state of component API Resource, like warning.</p>
</td>
</tr>
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
<p>phase describes the phase of the component and the detail information of the phases are as following:</p>
<ul>
<li>Creating: <code>Creating</code> is a special <code>Updating</code> with previous phase <code>empty</code>(means &ldquo;&rdquo;) or <code>Creating</code>.</li>
<li>Running: component replicas &gt; 0 and all pod specs are latest with a Running state.</li>
<li>Updating: component replicas &gt; 0 and has no failed pods. the component is being updated.</li>
<li>Abnormal: component replicas &gt; 0 but having some failed pods. the component basically works but in a fragile state.</li>
<li>Failed: component replicas &gt; 0 but having some failed pods. the component doesn&rsquo;t work anymore.</li>
<li>Stopping: component replicas = 0 and has terminating pods.</li>
<li>Stopped: component replicas = 0 and all pods have been deleted.</li>
<li>Deleting: the component is being deleted.</li>
</ul>
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
<p>message records the component details message in current phase.
Keys are podName or deployName or statefulSetName. The format is <code>ObjectKind/Name</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentSwitchover">ComponentSwitchover
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>withCandidate corresponds to the switchover of the specified candidate primary or leader instance.
Currently, only Action.Exec is supported, Action.HTTP is not supported.</p>
</td>
</tr>
<tr>
<td>
<code>withoutCandidate</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>withoutCandidate corresponds to a switchover that does not specify a candidate primary or leader instance.
Currently, only Action.Exec is supported, Action.HTTP is not supported.</p>
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
<p>scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
Once ScriptSpecSelectors is defined, the scripts defined in scripts can be referenced in the Action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentTemplateSpec">ComponentTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>Specify the name of configuration template.</p>
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
<p>Specify the name of the referenced the configuration template ConfigMap object.</p>
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
<p>Specify the namespace of the referenced the configuration template ConfigMap object.
An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p>
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
<p>volumeName is the volume name of PodTemplate, which the configuration file produced through the configuration
template will be mounted to the corresponding volume. Must be a DNS_LABEL name.
The volume name must be defined in podSpec.containers[*].volumeMounts.</p>
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
<p>defaultMode is optional: mode bits used to set permissions on created files by default.</p>
<p>Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
Defaults to 0644.</p>
<p>Directories within the path are not affected by this setting.
This might be in conflict with other options that affect the file
mode, like fsGroup, and the result can be other mode bits set.</p>
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
<p>type is the type of the source to select. There are three types: <code>FieldRef</code>, <code>ServiceRef</code>, <code>HeadlessServiceRef</code>.</p>
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
<p>fieldRef is the jsonpath of the source to select when type is <code>FieldRef</code>.
there are two objects registered in the jsonpath: <code>componentDef</code> and <code>components</code>:</p>
<ul>
<li>componentDef is the component definition object specified in <code>componentRef.componentDefName</code>.</li>
<li>components is the component list objects referring to the component definition object.</li>
</ul>
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
<p>format is the format of each headless service address.
there are three builtin variables can be used as placeholder: $POD_ORDINAL, $POD_FQDN, $POD_NAME</p>
<ul>
<li>$POD_ORDINAL is the ordinal of the pod.</li>
<li>$POD_FQDN is the fully qualified domain name of the pod.</li>
<li>$POD_NAME is the name of the pod</li>
</ul>
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
<p>joinWith is the string to join the values of headless service addresses.</p>
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
<p>ComponentValueFromType specifies the type of component value from.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVolume">ComponentVolume
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>The Name of the volume.
Must be a DNS_LABEL and unique within the pod.
More info: <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names">https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names</a>
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>needSnapshot</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>NeedSnapshot indicates whether the volume need to snapshot when making a backup for the component.
Cannot be updated.</p>
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
<p>HighWatermark defines the high watermark threshold for the volume space usage.</p>
<p>If there is any specified volumes who&rsquo;s space usage is over the threshold, the pre-defined &ldquo;LOCK&rdquo; action
will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
as read-only. And after that, if all volumes&rsquo; space usage drops under the threshold later, the pre-defined
&ldquo;UNLOCK&rdquo; action will be performed to recover the service normally.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraintPhase">ConfigConstraintPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintStatus">ConfigConstraintStatus</a>)
</p>
<div>
<p>ConfigConstraintPhase defines the ConfigConstraint  CR .status.phase</p>
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
<p>ConfigConstraintSpec defines the desired state of ConfigConstraint</p>
</div>
<table>
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
<p>reloadOptions indicates whether the process supports reload.</p>
<p>if set, the controller will determine the behavior of the engine instance based on the configuration templates,
restart or reload depending on whether any parameters in the StaticParameters have been modified.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicActionCanBeMerged</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the dynamic reload action and restart action can be merged to a restart action.</p>
<p>When a batch of parameters updates incur both restart &amp; dynamic reload, it works as:
- set to true, the two actions merged to only one restart action
- set to false, the two actions cannot be merged, the actions executed in order [dynamic reload, restart]</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the policy for selecting the parameters of dynamic reload actions.</p>
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
<p>toolConfig used to config init container.</p>
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
<p>downwardAPIOptions is used to watch pod fields.</p>
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
<p>scriptConfigs, list of ScriptConfig, witch these scripts can be used by volume trigger,downward trigger, or tool image</p>
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
<p>cfgSchemaTopLevelName is cue type name, which generates openapi schema.</p>
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
<p>configurationSchema imposes restrictions on database parameter&rsquo;s rule.</p>
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
<p>staticParameters, list of StaticParameter, modifications of them trigger a process restart.</p>
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
<p>dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.</p>
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
<p>immutableParameters describes parameters that prohibit user from modification.</p>
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
<p>selector is used to match the label on the pod,
for example, a pod of the primary is match on the patroni cluster.</p>
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
<p>formatterConfig describes the format of the configuration file, the controller will:</p>
<ol>
<li>parses configuration file</li>
<li>analyzes the modified parameters</li>
<li>applies corresponding policies.</li>
</ol>
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
<p>ConfigConstraintStatus defines the observed state of ConfigConstraint.</p>
</div>
<table>
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
<p>phase is status of configuration template, when set to CCAvailablePhase, it can be referenced by ClusterDefinition or ClusterVersion.</p>
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
<p>message field describes the reasons of abnormal status.</p>
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
<p>observedGeneration is the latest generation observed for this
ClusterDefinition. It refers to the ConfigConstraint&rsquo;s generation, which is
updated by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigMapRef">ConfigMapRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.UserResourceRefs">UserResourceRefs</a>)
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
<code>ResourceMeta</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ResourceMeta">
ResourceMeta
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceMeta</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>configMap</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#configmapvolumesource-v1-core">
Kubernetes core/v1.ConfigMapVolumeSource
</a>
</em>
</td>
<td>
<p>configMap defines the configmap volume source.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigParams">ConfigParams
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">ConfigurationItemDetail</a>)
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
<code>content</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>fileContent indicates the configuration file content.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]*string
</em>
</td>
<td>
<em>(Optional)</em>
<p>updated parameters for a single configuration file.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigTemplateExtension">ConfigTemplateExtension
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">ConfigurationItemDetail</a>, <a href="#apps.kubeblocks.io/v1alpha1.LegacyRenderedTemplateSpec">LegacyRenderedTemplateSpec</a>)
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
<p>Specify the name of the referenced the configuration template ConfigMap object.</p>
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
<p>Specify the namespace of the referenced the configuration template ConfigMap object.
An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p>
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
<p>policy defines how to merge external imported templates into component templates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationItem">ConfigurationItem
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
<p>Specifies the name of the configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>policy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.UpgradePolicy">
UpgradePolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the upgrade policy for the configuration. This field is optional.</p>
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
<p>Sets the parameters to be updated. It should contain at least one item. The keys are merged and retained during patch operations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">ConfigurationItemDetail
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationSpec">ConfigurationSpec</a>)
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
<p>Specify the name of configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: Please use payload instead.
version is the version of configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>payload</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Payload">
Payload
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Payload holds the configuration-related rerender.</p>
</td>
</tr>
<tr>
<td>
<code>configSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">
ComponentConfigSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configSpec is used to set the configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>importTemplateRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigTemplateExtension">
ConfigTemplateExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify the configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>configFileParams</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigParams">
map[string]github.com/apecloud/kubeblocks/apis/apps/v1alpha1.ConfigParams
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configFileParams is used to set the parameters to be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationItemDetailStatus">ConfigurationItemDetailStatus
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
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>name is a config template name.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationPhase">
ConfigurationPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>phase is status of configurationItem.</p>
</td>
</tr>
<tr>
<td>
<code>lastDoneRevision</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>lastDoneRevision is the last done revision of configurationItem.</p>
</td>
</tr>
<tr>
<td>
<code>updateRevision</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>updateRevision is the update revision of configurationItem.</p>
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
<p>message field describes the reasons of abnormal status.</p>
</td>
</tr>
<tr>
<td>
<code>reconcileDetail</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReconcileDetail">
ReconcileDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>reconcileDetail describes the details of the configuration change execution.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationItemStatus">ConfigurationItemStatus
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
<p>Specifies the name of the configuration template.</p>
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
<p>Defines the policy for reconfiguration.</p>
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
<p>Indicates the current state of the reconfiguration state machine.</p>
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
<p>Provides details about the operation.</p>
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
<p>Counts the number of successful reconfigurations.</p>
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
<p>Specifies the number of expected reconfigurations.</p>
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
<p>Records the last status of the reconfiguration controller.</p>
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
<p>Stores the last applied configuration.</p>
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
<p>Contains the updated parameters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationPhase">ConfigurationPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetailStatus">ConfigurationItemDetailStatus</a>)
</p>
<div>
<p>ConfigurationPhase defines the Configuration FSM phase</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Creating&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;FailedAndPause&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;FailedAndRetry&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Finished&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Init&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;MergeFailed&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Merged&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Upgrading&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationSpec">ConfigurationSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Configuration">Configuration</a>)
</p>
<div>
<p>ConfigurationSpec defines the desired state of Configuration</p>
</div>
<table>
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
<p>clusterRef references Cluster name.</p>
</td>
</tr>
<tr>
<td>
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<p>componentName is cluster component name.</p>
</td>
</tr>
<tr>
<td>
<code>configItemDetails</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">
[]ConfigurationItemDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>customConfigurationItems describes user-defined config template.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigurationStatus">ConfigurationStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Configuration">Configuration</a>)
</p>
<div>
<p>ConfigurationStatus defines the observed state of Configuration</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>message field describes the reasons of abnormal status.</p>
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
<p>observedGeneration is the latest generation observed for this
ClusterDefinition. It refers to the ConfigConstraint&rsquo;s generation, which is
updated by the API Server.</p>
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
<p>conditions describes opsRequest detail status.</p>
</td>
</tr>
<tr>
<td>
<code>configurationStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetailStatus">
[]ConfigurationItemDetailStatus
</a>
</em>
</td>
<td>
<p>configurationStatus describes the status of the component reconfiguring.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConnectionCredentialAuth">ConnectionCredentialAuth
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorSpec">ServiceDescriptorSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>service connection based-on username and password credential.</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>service connection based-on username and password credential.</p>
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
<p>the key of password in the ConnectionCredential secret.
if not set, the default key is &ldquo;password&rdquo;.</p>
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
<p>the key of username in the ConnectionCredential secret.
if not set, the default key is &ldquo;username&rdquo;.</p>
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
<p>name, role name.</p>
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
<p>accessMode, what service this member capable.</p>
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
<p>replicas, number of Pods of this role.</p>
<ul>
<li>default 1 for Leader</li>
<li>default 0 for Learner</li>
<li>default <code>Cluster.spec.componentSpec[*].Replicas - Leader.Replicas - Learner.Replicas</code> for Followers</li>
</ul>
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
<p>Defines the role name.</p>
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
<p>accessMode defines what service this pod provides.</p>
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
<p>Pod name.</p>
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
<p>leader, one single leader.</p>
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
<p>followers, has voting right but not Leader.</p>
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
<p>learner, no voting right.</p>
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
<p>Leader status.</p>
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
<p>Followers status.</p>
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
<p>Learner status.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CredentialVar">CredentialVar
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConnectionCredentialAuth">ConnectionCredentialAuth</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorSpec">ServiceDescriptorSpec</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.CredentialVarSelector">CredentialVarSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarSource">VarSource</a>)
</p>
<div>
<p>CredentialVarSelector selects a var from a Credential (SystemAccount).</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ClusterObjectReference</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterObjectReference">
ClusterObjectReference
</a>
</em>
</td>
<td>
<p>
(Members of <code>ClusterObjectReference</code> are embedded into this type.)
</p>
<p>The Credential (SystemAccount) to select from.</p>
</td>
</tr>
<tr>
<td>
<code>CredentialVars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVars">
CredentialVars
</a>
</em>
</td>
<td>
<p>
(Members of <code>CredentialVars</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CredentialVars">CredentialVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CredentialVarSelector">CredentialVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVars">ServiceRefVars</a>)
</p>
<div>
<p>CredentialVars defines the vars can be referenced from a Credential (SystemAccount).
!!!!! CredentialVars will only be used as environment variables for Pods &amp; Actions, and will not be used to render the templates.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>key name of label</p>
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
<p>value of label</p>
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
<p>resources defines the resources to be labeled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CustomOpsSpec">CustomOpsSpec
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
<code>componentName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Refers to the name of the cluster component.</p>
</td>
</tr>
<tr>
<td>
<code>opsDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Is a reference to an OpsDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>params</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the input for this operation as declared in the opsDefinition.spec.parametersSchema.
It will create corresponding jobs for each array element.
If the param type is an array, the format must be &ldquo;v1,v2,v3&rdquo;.</p>
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
<p>schema provides a way for providers to validate the changed parameters through json.</p>
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
<p>cue that to let provider verify user configuration through cue language.</p>
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
<p>Specify the name of the field.</p>
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
<p>mountPoint is the mount point of the scripts file.</p>
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
<p>Items is a list of downward API volume file</p>
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
<p>command used to execute for downwrad api.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.DynamicParameterSelectedPolicy">DynamicParameterSelectedPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>DynamicParameterSelectedPolicy determines how to select the parameters of dynamic reload actions</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;all&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;dynamic&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.EnvMappingVar">EnvMappingVar
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>)
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
<p>env key which needs to mapping.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ValueFrom">
ValueFrom
</a>
</em>
</td>
<td>
<p>valueFrom defines source of the env value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.EnvVar">EnvVar
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>EnvVar represents a variable present in the env of Pod/Action or the template of config/script.</p>
</div>
<table>
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
<p>Name of the variable. Must be a C_IDENTIFIER.</p>
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
<p>Variable references $(VAR_NAME) are expanded using the previously defined variables in the current context.</p>
<p>If a variable cannot be resolved, the reference in the input string will be unchanged.
Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.</p>
<ul>
<li>&rdquo;$$(VAR_NAME)&rdquo; will produce the string literal &ldquo;$(VAR_NAME)&rdquo;.</li>
</ul>
<p>Escaped references will never be expanded, regardless of whether the variable exists or not.
Defaults to &ldquo;&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarSource">
VarSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source for the variable&rsquo;s value. Cannot be used if value is not empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.EnvVarRef">EnvVarRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsVarSource">OpsVarSource</a>)
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
<em>(Optional)</em>
<p>Specifies the name of the container as defined in the componentDefinition or as injected by the kubeBlocks controller.
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
<p>Defines the name of the environment variable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ExecAction">ExecAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
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
<em>(Optional)</em>
<p>Command is the command line to execute inside the container, the working directory for the
command  is root (&lsquo;/&rsquo;) in the container&rsquo;s filesystem. The command is simply exec&rsquo;d, it is
not run inside a shell, so traditional shell instructions (&lsquo;|&rsquo;, etc) won&rsquo;t work. To use
a shell, you need to explicitly call out to that shell.
Exit status of 0 is treated as live/healthy and non-zero is unhealthy.</p>
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
<p>args is used to perform statements.</p>
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
<p>scrapePort is exporter port for Time Series Database to scrape metrics.</p>
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
<p>scrapePath is exporter url path for Time Series Database to scrape metrics.</p>
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
<code>switch</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ExposeSwitch">
ExposeSwitch
</a>
</em>
</td>
<td>
<p>Controls the expose operation.
If set to Enable, the corresponding service will be exposed. Conversely, if set to Disable, the service will be removed.</p>
</td>
</tr>
<tr>
<td>
<code>services</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsService">
[]OpsService
</a>
</em>
</td>
<td>
<p>A list of services that are to be exposed or removed.
If componentNamem is not specified, each <code>OpsService</code> in the list must specify ports and selectors.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ExposeSwitch">ExposeSwitch
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Expose">Expose</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.FailurePolicyType">FailurePolicyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefRef">ComponentDefRef</a>)
</p>
<div>
<p>FailurePolicyType specifies the type of failure policy</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Fail&#34;</p></td>
<td><p>ReportError means that an error will be reported.</p>
</td>
</tr><tr><td><p>&#34;Ignore&#34;</p></td>
<td><p>Ignore means that an error will be ignored but logged.</p>
</td>
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
<p>The FormatterOptions represents the special options of configuration file.
This is optional for now. If not specified.</p>
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
<p>The configuration file format. Valid values are <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>,
<code>hcl</code>, <code>dotenv</code>, <code>properties</code> and <code>toml</code>.</p>
<ul>
<li>ini: a configuration file that consists of a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: <a href="https://en.wikipedia.org/wiki/INI_file">https://en.wikipedia.org/wiki/INI_file</a></li>
<li>xml: reference wiki: <a href="https://en.wikipedia.org/wiki/XML">https://en.wikipedia.org/wiki/XML</a></li>
<li>yaml: a configuration file support for complex data types and structures.</li>
<li>json: reference wiki: <a href="https://en.wikipedia.org/wiki/JSON">https://en.wikipedia.org/wiki/JSON</a></li>
<li>hcl: The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: <a href="https://www.linode.com/docs/guides/introduction-to-hcl/">https://www.linode.com/docs/guides/introduction-to-hcl/</a></li>
<li>dotenv: this was a plain text file with simple key–value pairs, reference wiki: <a href="https://en.wikipedia.org/wiki/Configuration_file#MS-DOS">https://en.wikipedia.org/wiki/Configuration_file#MS-DOS</a></li>
<li>properties: a file extension mainly used in Java, reference wiki: <a href="https://en.wikipedia.org/wiki/.properties">https://en.wikipedia.org/wiki/.properties</a></li>
<li>toml: reference wiki: <a href="https://en.wikipedia.org/wiki/TOML">https://en.wikipedia.org/wiki/TOML</a></li>
<li>props-plus: a file extension mainly used in Java, support CamelCase(e.g: brokerMaxConnectionsPerIp)</li>
</ul>
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
<p>FormatterOptions represents the special options of configuration file.
Only one of its members may be specified.</p>
</div>
<table>
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
<p>iniConfig represents the ini options.</p>
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
<p>gvk is Group/Version/Kind, for example &ldquo;v1/Pod&rdquo;, &ldquo;apps/v1/StatefulSet&rdquo;, etc.</p>
<p>when the gvk resource filtered by the selector already exists, if there is no corresponding custom label, it will be added, and if label already exists, it will be updated.</p>
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
<p>selector is a label query over a set of resources.</p>
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
<p>HScaleDataClonePolicyType defines data clone policy when horizontal scaling.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.HTTPAction">HTTPAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
</p>
<div>
<p>HTTPAction describes an action based on HTTP requests.</p>
</div>
<table>
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
<p>Path to access on the HTTP server.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<p>Name or number of the port to access on the container.
Number must be in the range 1 to 65535.
Name must be an IANA_SVC_NAME.</p>
</td>
</tr>
<tr>
<td>
<code>host</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Host name to connect to, defaults to the pod IP. You probably want to set
&ldquo;Host&rdquo; in httpHeaders instead.</p>
</td>
</tr>
<tr>
<td>
<code>scheme</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#urischeme-v1-core">
Kubernetes core/v1.URIScheme
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheme to use for connecting to the host.
Defaults to HTTP.</p>
</td>
</tr>
<tr>
<td>
<code>method</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Method represents the HTTP request method, which can be one of the standard HTTP methods like &ldquo;GET,&rdquo; &ldquo;POST,&rdquo; &ldquo;PUT,&rdquo; etc.
Defaults to Get.</p>
</td>
</tr>
<tr>
<td>
<code>httpHeaders</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#httpheader-v1-core">
[]Kubernetes core/v1.HTTPHeader
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Custom headers to set in the request. HTTP allows repeated headers.</p>
</td>
</tr>
</tbody>
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
<p>type controls what kind of data synchronization do when component scale out.
Policy is in enum of &#123;None, CloneVolume&#125;. The default policy is <code>None</code>.</p>
<ul>
<li>None: Default policy, create empty volume and no data clone.</li>
<li>CloneVolume: Do data clone to newly scaled pods. Prefer to use volume snapshot first,
and will try backup tool if volume snapshot is not enabled, finally report error if both above cannot work.</li>
<li>Snapshot: Deprecated, alias for CloneVolume.</li>
</ul>
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
<p>BackupPolicyTemplateName reference the backup policy template.</p>
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
<p>volumeMountsName defines which volumeMount of the container to do backup,
only work if Type is not None
if not specified, the 1st volumeMount will be chosen</p>
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
<p>HorizontalScaling defines the variables of horizontal scaling operation</p>
</div>
<table>
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
<p>Specifies the number of replicas for the workloads.</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#NodeName">
[]k8s.io/apimachinery/pkg/types.NodeName
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the list of nodes where pods can be scheduled during a scale-up operation.
If the RsmTransformPolicy is set to ToPod and the expected number of replicas is greater than the current number,
the list of Nodes will be used. If the list of Nodes is empty, pods will not be assigned to any specific node.
However, if the list of Nodes is populated, pods will be evenly distributed across the nodes in the list during scale-up.</p>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the names of instances that the rsm should prioritize for scale-down operations.
If the RsmTransformPolicy is set to ToPod and the expected number of replicas is less than the current number,
the list of Instances will be used.</p>
<ul>
<li><code>current replicas - expected replicas &gt; len(Instances)</code>: Scale down from the list of Instances priorly, the others
will select from NodeAssignment.</li>
<li><code>current replicas - expected replicas &lt; len(Instances)</code>: Scale down from the list of Instances.</li>
<li><code>current replicas - expected replicas &lt; len(Instances)</code>: Scale down from a part of Instances.</li>
</ul>
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
<p>sectionName describes ini section.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Instance">Instance
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance</a>)
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
<p>The instance will rebuild on the specified node when the instance uses local PersistentVolume as the storage disk.
If not set, it will rebuild on a random node.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Issuer">Issuer
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.TLSConfig">TLSConfig</a>)
</p>
<div>
<p>Issuer defines Tls certs issuer</p>
</div>
<table>
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
<p>Name of issuer.
Options supported:</p>
<ul>
<li>KubeBlocks - Certificates signed by KubeBlocks Operator.</li>
<li>UserProvided - User provided own CA-signed certificates.</li>
</ul>
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
<p>secretRef. TLS certs Secret reference
required when from is UserProvided</p>
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
<p>IssuerName defines Tls certs issuer name</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;KubeBlocks&#34;</p></td>
<td><p>IssuerKubeBlocks Certificates signed by KubeBlocks Operator.</p>
</td>
</tr><tr><td><p>&#34;UserProvided&#34;</p></td>
<td><p>IssuerUserProvided User provided own CA-signed certificates.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.KBAccountType">KBAccountType
(<code>byte</code> alias)</h3>
<div>
<p>KBAccountType is used for bitwise operation.</p>
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
<p>Represents the last replicas of the component.</p>
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
<p>Represents the last resources of the component.</p>
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
<p>References a class defined in ComponentClassDefinition.</p>
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
<p>Records the last volumeClaimTemplates of the component.</p>
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
<p>Records the last services of the component.</p>
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
<p>Records the information about the target resources affected by the component.
The resource key is in the list of [pods].</p>
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
<p>Specifies the reference to the ClusterVersion name.</p>
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
<p>Records the last configuration of the component.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LegacyRenderedTemplateSpec">LegacyRenderedTemplateSpec
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
<code>ConfigTemplateExtension</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigTemplateExtension">
ConfigTemplateExtension
</a>
</em>
</td>
<td>
<p>
(Members of <code>ConfigTemplateExtension</code> are embedded into this type.)
</p>
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
<p>LetterCase defines cases to use in password generation.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">LifecycleActionHandler
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions</a>, <a href="#apps.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe</a>)
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
<code>builtinHandler</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BuiltinActionHandlerType">
BuiltinActionHandlerType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>builtinHandler specifies the builtin action handler name to do the action.
the BuiltinHandler within the same ComponentLifecycleActions should be consistent. Details can be queried through official documentation in the future.
use CustomHandler to define your own actions if none of them satisfies the requirement.</p>
</td>
</tr>
<tr>
<td>
<code>customHandler</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Action">
Action
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>customHandler defines the custom way to do action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LogConfig">LogConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>name log type name, such as slow for MySQL slow log file.</p>
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
<p>filePathPattern log file path pattern which indicate how to find this file
corresponding to variable (log path) in database kernel. please don&rsquo;t set this casually.</p>
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
<p>The size of memory per vcpu core.
For example: 1Gi, 200Mi.
If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignore.</p>
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
<p>The maximum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.
It is useful on GCP as the ratio between the CPU and memory may be a range.
If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.
Reference: <a href="https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types">https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types</a></p>
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
<p>The minimum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.
It is useful on GCP as the ratio between the CPU and memory may be a range.
If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.
Reference: <a href="https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types">https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MergedPolicy">MergedPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigTemplateExtension">ConfigTemplateExtension</a>)
</p>
<div>
<p>MergedPolicy defines how to merge external imported templates into component templates.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>builtIn is a switch to enable KubeBlocks builtIn monitoring.
If BuiltIn is set to true, monitor metrics will be scraped automatically.
If BuiltIn is set to false, the provider should set ExporterConfig and Sidecar container own.</p>
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
<p>exporterConfig provided by provider, which specify necessary information to Time Series Database.
exporterConfig is valid when builtIn is false.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MultipleClusterObjectCombinedOption">MultipleClusterObjectCombinedOption
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectOption">MultipleClusterObjectOption</a>)
</p>
<div>
<p>MultipleClusterObjectCombinedOption defines options for handling combined variables.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>newVarSuffix</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If set, the existing variable will be kept, and a new variable will be defined with the specified suffix
in pattern: $(var.name)_$(suffix).
The new variable will be auto-created and placed behind the existing one.
If not set, the existing variable will be reused with the value format defined below.</p>
</td>
</tr>
<tr>
<td>
<code>valueFormat</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectValueFormat">
MultipleClusterObjectValueFormat
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The format of the value that the operator will use to compose values from multiple components.</p>
</td>
</tr>
<tr>
<td>
<code>flattenFormat</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectValueFormatFlatten">
MultipleClusterObjectValueFormatFlatten
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The flatten format, default is: $(comp-name-1):value,$(comp-name-2):value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MultipleClusterObjectOption">MultipleClusterObjectOption
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterObjectReference">ClusterObjectReference</a>)
</p>
<div>
<p>MultipleClusterObjectOption defines the options for handling multiple cluster objects matched.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>strategy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectStrategy">
MultipleClusterObjectStrategy
</a>
</em>
</td>
<td>
<p>Define the strategy for handling multiple cluster objects.</p>
</td>
</tr>
<tr>
<td>
<code>combinedOption</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectCombinedOption">
MultipleClusterObjectCombinedOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Define the options for handling combined variables.
Valid only when the strategy is set to &ldquo;combined&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MultipleClusterObjectStrategy">MultipleClusterObjectStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectOption">MultipleClusterObjectOption</a>)
</p>
<div>
<p>MultipleClusterObjectStrategy defines the strategy for handling multiple cluster objects.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;combined&#34;</p></td>
<td><p>MultipleClusterObjectStrategyCombined - the values from all matched components will be combined into a single
variable using the specified option.</p>
</td>
</tr><tr><td><p>&#34;individual&#34;</p></td>
<td><p>MultipleClusterObjectStrategyIndividual - each matched component will have its individual variable with its name
as the suffix.
This is required when referencing credential variables that cannot be passed by values.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MultipleClusterObjectValueFormat">MultipleClusterObjectValueFormat
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectCombinedOption">MultipleClusterObjectCombinedOption</a>)
</p>
<div>
<p>MultipleClusterObjectValueFormat defines the format details for the value.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Flatten&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MultipleClusterObjectValueFormatFlatten">MultipleClusterObjectValueFormatFlatten
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MultipleClusterObjectCombinedOption">MultipleClusterObjectCombinedOption</a>)
</p>
<div>
<p>MultipleClusterObjectValueFormatFlatten defines the flatten format for the value.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>delimiter</code><br/>
<em>
string
</em>
</td>
<td>
<p>Pair delimiter.</p>
</td>
</tr>
<tr>
<td>
<code>keyValueDelimiter</code><br/>
<em>
string
</em>
</td>
<td>
<p>Key-value delimiter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.NamedVar">NamedVar
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceVars">ServiceVars</a>)
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
</td>
</tr>
<tr>
<td>
<code>option</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>)
</p>
<div>
<p>OpsDefinitionSpec defines the desired state of OpsDefinition</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>componentDefinitionRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionRef">
[]ComponentDefinitionRef
</a>
</em>
</td>
<td>
<p>Specifies the types of componentDefinitions that are supported by the operation.
It can refer to some variables of the componentDefinition.
If set, any component that does not meet the conditions will be intercepted.</p>
</td>
</tr>
<tr>
<td>
<code>varsRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarsRef">
VarsRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the environment variables that need to be referenced from the target component pod, and will be injected into the job&rsquo;s containers.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the schema used for validation, pruning, and defaulting.</p>
</td>
</tr>
<tr>
<td>
<code>jobSpec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jobspec-v1-batch">
Kubernetes batch/v1.JobSpec
</a>
</em>
</td>
<td>
<p>Describes the job specification for the operation.</p>
</td>
</tr>
<tr>
<td>
<code>preConditions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the job for the operation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsDefinitionStatus">OpsDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>)
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
<p>Refers to the most recent generation observed for this OpsDefinition.</p>
</td>
</tr>
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
<p>Represents the current state of the OpsDefinition. Valid values are `<code>,</code>Available<code>,</code>Unavailable<code>.
When the state is</code>Available`, the OpsDefinition is ready and can be used for related objects.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsEnvVar">OpsEnvVar
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarsRef">VarsRef</a>)
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
<p>Specifies the name of the variable. This must be a C_IDENTIFIER.</p>
</td>
</tr>
<tr>
<td>
<code>valueFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsVarSource">
OpsVarSource
</a>
</em>
</td>
<td>
<p>Defines the source for the variable&rsquo;s value.</p>
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
<p>OpsPhase defines opsRequest phase.</p>
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
<p>name OpsRequest name</p>
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
<p>Describes the component phase, referencing Cluster.status.component.phase.</p>
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
<p>Indicates the last time the component phase transitioned to Failed or Abnormal.</p>
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
<p>Describes the progress details of the component for this operation.</p>
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
<p>References the workload type of component in ClusterDefinition.</p>
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
<p>Describes the reason for the component phase.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsRequestSpec">OpsRequestSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>)
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
<code>clusterRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>References the cluster object.</p>
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
<p>Defines the action to cancel the <code>Pending/Creating/Running</code> opsRequest, supported types: <code>VerticalScaling/HorizontalScaling</code>.
Once set to true, this opsRequest will be canceled and modifying this property again will not take effect.</p>
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
<p>Defines the operation type.</p>
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
<p>OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.</p>
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
<p>Specifies the cluster version by specifying clusterVersionRef.</p>
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
<p>Defines what component need to horizontal scale the specified replicas.</p>
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
<p>Note: Quantity struct can not do immutable check by CEL.
Defines what component and volumeClaimTemplate need to expand the specified storage.</p>
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
<p>Restarts the specified components.</p>
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
<p>Switches over the specified components.</p>
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
<p>Note: Quantity struct can not do immutable check by CEL.
Defines what component need to vertical scale the specified compute resources.</p>
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
<p>Deprecated: replace by reconfigures.
Defines the variables that need to input when updating configuration.</p>
</td>
</tr>
<tr>
<td>
<code>reconfigures</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Reconfigure">
[]Reconfigure
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the variables that need to input when updating configuration.</p>
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
<p>Defines services the component needs to expose.</p>
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
<p>Cluster RestoreFrom backup or point in time.</p>
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
<p>OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.
If not specified, the default value is 0, which means that the start-conditions must be met immediately.</p>
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
<p>Defines the script to be executed.</p>
</td>
</tr>
<tr>
<td>
<code>rebuildFrom</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RebuildInstance">
[]RebuildInstance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the instances that require re-creation.</p>
</td>
</tr>
<tr>
<td>
<code>backupSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BackupSpec">
BackupSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines how to backup the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>restoreSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RestoreSpec">
RestoreSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines how to restore the cluster.
Note that this restore operation will roll back cluster services.</p>
</td>
</tr>
<tr>
<td>
<code>customSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomOpsSpec">
CustomOpsSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a custom operation as defined by OpsDefinition.</p>
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
<p>Specifies the cluster generation after the OpsRequest action has been handled.</p>
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
<p>Defines the phase of the OpsRequest.</p>
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
<a href="#apps.kubeblocks.io/v1alpha1.LastConfiguration">
LastConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the last configuration before this operation took effect.</p>
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
<p>Records the status information of components changed due to the operation request.</p>
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
<p>Indicates the time when the OpsRequest started processing.</p>
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
<p>Specifies the time when the OpsRequest was completed.</p>
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
<p>Defines the time when the OpsRequest was cancelled.</p>
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
<p>Deprecated: Replaced by ReconfiguringStatusAsComponent.
Defines the status information of reconfiguring.</p>
</td>
</tr>
<tr>
<td>
<code>reconfiguringStatusAsComponent</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ReconfiguringStatus">
map[string]*github.com/apecloud/kubeblocks/apis/apps/v1alpha1.ReconfiguringStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the status information of reconfiguring.</p>
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
<p>Describes the detailed status of the OpsRequest.</p>
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
<p>Specifies the requested storage size for the volume.</p>
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
<p>A reference to the volumeClaimTemplate name from the cluster components.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsService">OpsService
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Expose">Expose</a>)
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
<p>Specifies the name of the service. This name is used by others to refer to this service (e.g., connection credential).
Note: This field cannot be updated.</p>
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
<p>Contains cloud provider related parameters if ServiceType is LoadBalancer.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p>
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
<p>Lists the ports that are exposed by this service.
If not provided, the default Services Ports defined in the ClusterDefinition or ComponentDefinition that are neither of NodePort nor LoadBalancer service type will be used.
If there is no corresponding Service defined in the ClusterDefinition or ComponentDefinition, the expose operation will fail.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
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
<p>Allows you to specify a defined role as a selector for the service, extending the ServiceSpec.Selector.</p>
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
<p>Routes service traffic to pods with label keys and values matching this selector.
If empty or not present, the service is assumed to have an external process managing its endpoints, which Kubernetes will not modify.
This only applies to types ClusterIP, NodePort, and LoadBalancer and is ignored if type is ExternalName.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/">https://kubernetes.io/docs/concepts/services-networking/service/</a></p>
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
<p>Determines how the Service is exposed. Defaults to ClusterIP. Valid options are ExternalName, ClusterIP, NodePort, and LoadBalancer.
- <code>ClusterIP</code> allocates a cluster-internal IP address for load-balancing to endpoints.
- <code>NodePort</code> builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.
- <code>LoadBalancer</code> builds on NodePort and creates an external load-balancer (if supported in the current cloud) which routes to the same endpoints as the clusterIP.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a>.</p>
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
<p>IPFamilies is a list of IP families (e.g. IPv4, IPv6) assigned to this
service. This field is usually assigned automatically based on cluster
configuration and the ipFamilyPolicy field. If this field is specified
manually, the requested family is available in the cluster,
and ipFamilyPolicy allows it, it will be used; otherwise creation of
the service will fail. This field is conditionally mutable: it allows
for adding or removing a secondary IP family, but it does not allow
changing the primary IP family of the Service. Valid values are &ldquo;IPv4&rdquo;
and &ldquo;IPv6&rdquo;.  This field only applies to Services of types ClusterIP,
NodePort, and LoadBalancer, and does apply to &ldquo;headless&rdquo; services.
This field will be wiped when updating a Service to type ExternalName.</p>
<p>This field may hold a maximum of two entries (dual-stack families, in
either order).  These families must correspond to the values of the
clusterIPs field, if specified. Both clusterIPs and ipFamilies are
governed by the ipFamilyPolicy field.</p>
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
<p>IPFamilyPolicy represents the dual-stack-ness requested or required by
this Service. If there is no value provided, then this field will be set
to SingleStack. Services can be &ldquo;SingleStack&rdquo; (a single IP family),
&ldquo;PreferDualStack&rdquo; (two IP families on dual-stack configured clusters or
a single IP family on single-stack clusters), or &ldquo;RequireDualStack&rdquo;
(two IP families on dual-stack configured clusters, otherwise fail). The
ipFamilies and clusterIPs fields depend on the value of this field. This
field will be wiped when updating a service to type ExternalName.</p>
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
<td><p>DataScriptType the data script operation will execute the data script against the cluster.</p>
</td>
</tr><tr><td><p>&#34;Custom&#34;</p></td>
<td><p>RebuildInstance rebuilding an instance is very useful when a node is offline or an instance is unrecoverable.</p>
</td>
</tr><tr><td><p>&#34;DataScript&#34;</p></td>
<td></td>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsVarSource">OpsVarSource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsEnvVar">OpsEnvVar</a>)
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
<code>envVarRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.EnvVarRef">
EnvVarRef
</a>
</em>
</td>
<td>
<p>Specifies a reference to a specific environment variable within a container.
Used to specify the source of the variable, which can be either &ldquo;env&rdquo; or &ldquo;envFrom&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ParameterConfig">ParameterConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItem">ConfigurationItem</a>)
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
<p>Represents the unique identifier for the ConfigMap.</p>
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
<em>(Optional)</em>
<p>Defines a list of key-value pairs for a single configuration file.
These parameters are used to update the specified configuration settings.</p>
</td>
</tr>
<tr>
<td>
<code>fileContent</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the content of the configuration file.
This field is used to update the entire content of the file.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ParametersSchema">ParametersSchema
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<p>Defines the OpenAPI v3 schema used for the parameter schema.
The supported property types include:
- string
- number
- integer
- array: Note that only items of string type are supported.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PasswordConfig">PasswordConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SystemAccount">SystemAccount</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccountSpec">SystemAccountSpec</a>)
</p>
<div>
<p>PasswordConfig helps provide to customize complexity of password generation pattern.</p>
</div>
<table>
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
<p>length defines the length of password.</p>
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
<p>numDigits defines number of digits.</p>
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
<p>numSymbols defines number of symbols.</p>
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
<p>letterCase defines to use lower-cases, upper-cases or mixed-cases of letters.</p>
</td>
</tr>
<tr>
<td>
<code>seed</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>seed specifies the seed used to generate the account&rsquo;s password.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Payload">Payload
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetail">ConfigurationItemDetail</a>)
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
<code>-</code><br/>
<em>
map[string]any
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>accessModes contains the desired access modes the volume should have.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1</a>.</p>
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
<p>resources represents the minimum resources the volume should have.
If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
that are lower than previous value but must still be higher than capacity recorded in the
status field of the claim.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources">https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources</a>.</p>
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
<p>storageClassName is the name of the StorageClass required by the claim.
More info: <a href="https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1">https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1</a>.</p>
</td>
</tr>
<tr>
<td>
<code>volumeMode</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#persistentvolumemode-v1-core">
Kubernetes core/v1.PersistentVolumeMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>volumeMode defines what type of volume is required by the claim.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionStatus">ClusterDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionStatus">ClusterVersionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionStatus">ComponentDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionStatus">OpsDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorStatus">ServiceDescriptorStatus</a>)
</p>
<div>
<p>Phase defines the ClusterDefinition and ClusterVersion  CR .status.phase</p>
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
<p>PodAntiAffinity defines pod anti-affinity strategy.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.PodSelectionStrategy">PodSelectionStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarsRef">VarsRef</a>)
</p>
<div>
<p>PodSelectionStrategy pod selection strategy.</p>
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
</tr><tr><td><p>&#34;PreferredAvailable&#34;</p></td>
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
<p>Refers to the specific time point for restoration, with UTC as the time zone.</p>
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
<p>Refers to a reference source cluster that needs to be restored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PostStartAction">PostStartAction
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
<code>cmdExecutorConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CmdExecutorConfig">
CmdExecutorConfig
</a>
</em>
</td>
<td>
<p>cmdExecutorConfig is the executor configuration of the post-start command.</p>
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
<p>scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
Once ScriptSpecSelectors is defined, the scripts defined in scriptSpecs can be referenced in the PostStartAction.CmdExecutorConfig.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PreCondition">PreCondition
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.Rule">
Rule
</a>
</em>
</td>
<td>
<p>Defines the conditions under which the operation can be executed.</p>
</td>
</tr>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreConditionExec">
PreConditionExec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents a job that will be run to execute the PreCondition.
The operation will only be executed if the job is successful.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PreConditionExec">PreConditionExec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PreCondition">PreCondition</a>)
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
<p>Specifies the name of the Docker image to be used for the execution.</p>
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
<p>Defines the environment variables to be set in the container.</p>
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
<p>Specifies the commands to be executed in the container.</p>
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
<p>Represents the arguments to be passed to the command in the container.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PreConditionType">PreConditionType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
</p>
<div>
<p>PreConditionType defines the preCondition type of the action execution.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ClusterReady&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ComponentReady&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Immediately&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;RuntimeReady&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProgressStatus">ProgressStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail</a>)
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
<p>Specifies the group to which the current object belongs.
If the objects of a component belong to the same group, they can be ignored.</p>
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
<p>Represents the unique key of the object.</p>
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
<p>Indicates the state of processing the object.</p>
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
<p>Provides a human-readable message detailing the condition of the object.</p>
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
<p>Represents the start time of object processing.</p>
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
<p>Represents the completion time of object processing.</p>
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
<p>Name of volume to protect.</p>
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
<p>Volume specified high watermark threshold, it will override the component level threshold.
If the value is invalid, it will be ignored and the component level threshold will be used.</p>
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
<p>ProvisionPolicy defines the policy details for creating accounts.</p>
</div>
<table>
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
<p>type defines the way to provision an account, either <code>CreateByStmt</code> or <code>ReferToExisting</code>.</p>
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
<p>scope is the scope to provision account, and the scope could be <code>AnyPods</code> or <code>AllPods</code>.</p>
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
<p>statements will be used when Type is CreateByStmt.</p>
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
<p>secretRef will be used when Type is ReferToExisting.</p>
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
<p>ProvisionPolicyType defines the policy for creating accounts.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CreateByStmt&#34;</p></td>
<td><p>CreateByStmt will create account w.r.t. deletion and creation statement given by provider.</p>
</td>
</tr><tr><td><p>&#34;ReferToExisting&#34;</p></td>
<td><p>ReferToExisting will not create account, but create a secret by copying data from referred secret file.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionScope">ProvisionScope
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>)
</p>
<div>
<p>ProvisionScope defines the scope (within component) of provision.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;AllPods&#34;</p></td>
<td><p>AllPods will create accounts for all pods belong to the component.</p>
</td>
</tr><tr><td><p>&#34;AnyPods&#34;</p></td>
<td><p>AnyPods will only create accounts on one pod.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionSecretRef">ProvisionSecretRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccount">SystemAccount</a>)
</p>
<div>
<p>ProvisionSecretRef defines the information of secret referred to.</p>
</div>
<table>
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
<p>name refers to the name of the secret.</p>
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
<p>namespace refers to the namespace of the secret.</p>
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
<p>ProvisionStatements defines the statements used to create accounts.</p>
</div>
<table>
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
<p>creation specifies statement how to create this account with required privileges.</p>
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
<p>update specifies statement how to update account&rsquo;s password.</p>
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
<p>deletion specifies statement how to delete this account.
Used in combination with <code>CreateionStatement</code> to delete the account before create it.
For instance, one usually uses <code>drop user if exists</code> statement followed by <code>create user</code> statement to create an account.
Deprecated: this field is deprecated, use <code>update</code> instead.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec
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
<p>MemberUpdateStrategy, Members(Pods) update strategy.</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.
<code>Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader</code></li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
<code>Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader</code>, keep majority online all the time.</li>
<li>parallel: force parallel</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RebuildInstance">RebuildInstance
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
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Instance">
[]Instance
</a>
</em>
</td>
<td>
<p>Defines the instances that need to be rebuilt.</p>
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
<p>Indicates the name of the backup from which to recover. Currently, only a full physical backup is supported
unless your component only has one replica. Such as &lsquo;xtrabackup&rsquo; is full physical backup for mysql and &lsquo;mysqldump&rsquo; is not.
And if no specified backupName, the instance will be recreated with empty &lsquo;PersistentVolumes&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>envForRestore</code><br/>
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
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReconcileDetail">ReconcileDetail
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemDetailStatus">ConfigurationItemDetailStatus</a>)
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
<code>policy</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>policy is the policy of the latest execution.</p>
</td>
</tr>
<tr>
<td>
<code>execResult</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>execResult is the result of the latest execution.</p>
</td>
</tr>
<tr>
<td>
<code>currentRevision</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>currentRevision is the current revision of configurationItem.</p>
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
<p>succeedCount is the number of pods for which configuration changes were successfully executed.</p>
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
<p>expectedCount is the number of pods that need to be executed for configuration changes.</p>
</td>
</tr>
<tr>
<td>
<code>errMessage</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>errMessage is the error message when the configuration change execution fails.</p>
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
<p>Reconfigure represents the variables required for updating a configuration.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItem">
[]ConfigurationItem
</a>
</em>
</td>
<td>
<p>Specifies the components that will perform the operation.</p>
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
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the reconfiguring detail status.</p>
</td>
</tr>
<tr>
<td>
<code>configurationStatus</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemStatus">
[]ConfigurationItemStatus
</a>
</em>
</td>
<td>
<p>Describes the status of the component reconfiguring.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>ReloadOptions defines reload options
Only one of its members may be specified.</p>
</div>
<table>
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
<p>unixSignalTrigger used to reload by sending a signal.</p>
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
<p>shellTrigger performs the reload command.</p>
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
<p>goTplTrigger performs the reload command.</p>
</td>
</tr>
<tr>
<td>
<code>autoTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.AutoTrigger">
AutoTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>autoTrigger performs the reload command.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicaRole">ReplicaRole
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>ReplicaRole represents a role that can be assumed by a component instance.</p>
</div>
<table>
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
<p>Name of the role. It will apply to &ldquo;apps.kubeblocks.io/role&rdquo; object label value.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>serviceable</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Serviceable indicates whether a replica with this role can provide services.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>writable</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Writable indicates whether a replica with this role is allowed to write data.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>votable</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Votable indicates whether a replica with this role is allowed to vote.
Cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicasLimit">ReplicasLimit
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>ReplicasLimit defines the limit of valid replicas supported.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>minReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>The minimum limit of replicas.</p>
</td>
</tr>
<tr>
<td>
<code>maxReplicas</code><br/>
<em>
int32
</em>
</td>
<td>
<p>The maximum limit of replicas.</p>
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
<p>Pod name.</p>
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
<p>Primary status.</p>
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
<p>Secondaries status.</p>
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
<p>The name of the constraint.</p>
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
<p>The constraint for vcpu cores.</p>
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
<p>The constraint for memory size.</p>
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
<p>The constraint for storage size.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ResourceMeta">ResourceMeta
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigMapRef">ConfigMapRef</a>, <a href="#apps.kubeblocks.io/v1alpha1.SecretRef">SecretRef</a>)
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
<p>name is the name of the referenced the Configmap/Secret object.</p>
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
<p>mountPath is the path at which to mount the volume.</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>subPath is a relative file path within the volume to mount.</p>
</td>
</tr>
<tr>
<td>
<code>asVolumeFrom</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>asVolumeFrom defines the list of containers where volumeMounts will be injected into.</p>
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
<p>Refers to the backup name and component name used for restoration. Supports recovery of multiple components.</p>
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
<p>Refers to the specific point in time for recovery.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RestoreSpec">RestoreSpec
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
<code>backupName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of the backup.</p>
</td>
</tr>
<tr>
<td>
<code>effectiveCommonComponentDef</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Indicates if this backup will be restored for all components which refer to common ComponentDefinition.</p>
</td>
</tr>
<tr>
<td>
<code>restoreTimeStr</code><br/>
<em>
string
</em>
</td>
<td>
<p>Defines the point in time to restore.</p>
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
<p>Specifies the volume claim restore policy, support values: [Serial, Parallel]</p>
</td>
</tr>
<tr>
<td>
<code>doReadyRestoreAfterClusterRunning</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If set to true, the recovery process in the PostReady phase will be performed after the cluster is running successfully.
otherwise, it will be performed after component is running.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RetryPolicy">RetryPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
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
<code>maxRetries</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxRetries specifies the maximum number of times the action should be retried.</p>
</td>
</tr>
<tr>
<td>
<code>retryInterval</code><br/>
<em>
time.Duration
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryInterval specifies the interval between retry attempts.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RoleArbitrator">RoleArbitrator
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>RoleArbitrator defines how to arbitrate the role of replicas.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;External&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Lorry&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions</a>)
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
<code>LifecycleActionHandler</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<p>
(Members of <code>LifecycleActionHandler</code> are embedded into this type.)
</p>
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
<p>Number of seconds after the container has started before liveness probes are initiated.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
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
Defaults to 1 second. Minimum value is 1.
More info: <a href="https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes">https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes</a></p>
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
Default to 10 seconds. Minimum value is 1.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Rule">Rule
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PreCondition">PreCondition</a>)
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
<p>Defines how the operation can be executed using a Go template expression.
Should return either <code>true</code> or <code>false</code>. The built-in objects available for use in the expression include:
- <code>params</code>: These are the input parameters.
- <code>cluster</code>: This is the referenced cluster object.
- <code>component</code>: This is the referenced component object.</p>
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
<p>Reported if the rule is not matched.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SchedulePolicy">SchedulePolicy
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
github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.RetentionPeriod
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
<li>minutes: 	30m
You can also combine the above durations. For example: 30d12h30m</li>
</ul>
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
<p>scriptConfigMapRef used to execute for reload.</p>
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
<p>Specify the namespace of the referenced the tpl script ConfigMap object.
An empty namespace is equivalent to the &ldquo;default&rdquo; namespace.</p>
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
<p>ScriptFrom represents the script that is to be executed from a configMap or a secret.</p>
</div>
<table>
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
<p>Specifies the configMap that is to be executed.</p>
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
<p>Specifies the secret that is to be executed.</p>
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
<p>ScriptSecret represents the secret that is used to execute the script.</p>
</div>
<table>
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
<p>Specifies the name of the secret.</p>
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
<p>Used to specify the username part of the secret.</p>
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
<p>Used to specify the password part of the secret.</p>
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
<p>ScriptSpec is designed to execute specific operations such as creating a database or user.
It is not a general-purpose script executor and is applicable for engines like MySQL, PostgreSQL, Redis, MongoDB, etc.</p>
</div>
<table>
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
<p>Specifies the image to be used for the exec command. By default, the image of kubeblocks-datascript is used.</p>
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
<p>Defines the secret to be used to execute the script. If not specified, the default cluster root credential secret is used.</p>
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
<p>Defines the script to be executed.</p>
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
<p>Defines the script to be executed from a configMap or secret.</p>
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
<em>(Optional)</em>
<p>By default, KubeBlocks will execute the script on the primary pod with role=leader.
Exceptions exist, such as Redis, which does not synchronize account information between primary and secondary.
In such cases, the script needs to be executed on all pods matching the selector.
Indicates the components on which the script is executed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ScriptSpecSelector">ScriptSpecSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentSwitchover">ComponentSwitchover</a>, <a href="#apps.kubeblocks.io/v1alpha1.PostStartAction">PostStartAction</a>, <a href="#apps.kubeblocks.io/v1alpha1.SwitchoverAction">SwitchoverAction</a>)
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
<p>ScriptSpec name of the referent, refer to componentDefs[x].scriptSpecs[y].Name.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SecretRef">SecretRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.UserResourceRefs">UserResourceRefs</a>)
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
<code>ResourceMeta</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ResourceMeta">
ResourceMeta
</a>
</em>
</td>
<td>
<p>
(Members of <code>ResourceMeta</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretvolumesource-v1-core">
Kubernetes core/v1.SecretVolumeSource
</a>
</em>
</td>
<td>
<p>secret defines the secret volume source.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Service">Service
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterService">ClusterService</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentService">ComponentService</a>)
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
<p>Name defines the name of the service.
otherwise, it indicates the name of the service.
Others can refer to this service by its name. (e.g., connection credential)
Cannot be updated.</p>
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
<p>ServiceName defines the name of the underlying service object.
If not specified, the default service name with different patterns will be used:</p>
<ul>
<li>CLUSTER_NAME: for cluster-level services</li>
<li>CLUSTER_NAME-COMPONENT_NAME: for component-level services</li>
</ul>
<p>Only one default service name is allowed.
Cannot be updated.</p>
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
<p>If ServiceType is LoadBalancer, cloud provider related parameters can be put here
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p>
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#servicespec-v1-core">
Kubernetes core/v1.ServiceSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the behavior of a service.
<a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
<br/>
<br/>
<table>
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
<p>The list of ports that are exposed by this service.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
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
<p>Route service traffic to pods with label keys and values matching this
selector. If empty or not present, the service is assumed to have an
external process managing its endpoints, which Kubernetes will not
modify. Only applies to types ClusterIP, NodePort, and LoadBalancer.
Ignored if type is ExternalName.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/">https://kubernetes.io/docs/concepts/services-networking/service/</a></p>
</td>
</tr>
<tr>
<td>
<code>clusterIP</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterIP is the IP address of the service and is usually assigned
randomly. If an address is specified manually, is in-range (as per
system configuration), and is not in use, it will be allocated to the
service; otherwise creation of the service will fail. This field may not
be changed through updates unless the type field is also being changed
to ExternalName (which requires this field to be blank) or the type
field is being changed from ExternalName (in which case this field may
optionally be specified, as describe above).  Valid values are &ldquo;None&rdquo;,
empty string (&ldquo;&rdquo;), or a valid IP address. Setting this to &ldquo;None&rdquo; makes a
&ldquo;headless service&rdquo; (no virtual IP), which is useful when direct endpoint
connections are preferred and proxying is not required.  Only applies to
types ClusterIP, NodePort, and LoadBalancer. If this field is specified
when creating a Service of type ExternalName, creation will fail. This
field will be wiped when updating a Service to type ExternalName.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
</td>
</tr>
<tr>
<td>
<code>clusterIPs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterIPs is a list of IP addresses assigned to this service, and are
usually assigned randomly.  If an address is specified manually, is
in-range (as per system configuration), and is not in use, it will be
allocated to the service; otherwise creation of the service will fail.
This field may not be changed through updates unless the type field is
also being changed to ExternalName (which requires this field to be
empty) or the type field is being changed from ExternalName (in which
case this field may optionally be specified, as describe above).  Valid
values are &ldquo;None&rdquo;, empty string (&ldquo;&rdquo;), or a valid IP address.  Setting
this to &ldquo;None&rdquo; makes a &ldquo;headless service&rdquo; (no virtual IP), which is
useful when direct endpoint connections are preferred and proxying is
not required.  Only applies to types ClusterIP, NodePort, and
LoadBalancer. If this field is specified when creating a Service of type
ExternalName, creation will fail. This field will be wiped when updating
a Service to type ExternalName.  If this field is not specified, it will
be initialized from the clusterIP field.  If this field is specified,
clients must ensure that clusterIPs[0] and clusterIP have the same
value.</p>
<p>This field may hold a maximum of two entries (dual-stack IPs, in either order).
These IPs must correspond to the values of the ipFamilies field. Both
clusterIPs and ipFamilies are governed by the ipFamilyPolicy field.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#servicetype-v1-core">
Kubernetes core/v1.ServiceType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>type determines how the Service is exposed. Defaults to ClusterIP. Valid
options are ExternalName, ClusterIP, NodePort, and LoadBalancer.
&ldquo;ClusterIP&rdquo; allocates a cluster-internal IP address for load-balancing
to endpoints. Endpoints are determined by the selector or if that is not
specified, by manual construction of an Endpoints object or
EndpointSlice objects. If clusterIP is &ldquo;None&rdquo;, no virtual IP is
allocated and the endpoints are published as a set of endpoints rather
than a virtual IP.
&ldquo;NodePort&rdquo; builds on ClusterIP and allocates a port on every node which
routes to the same endpoints as the clusterIP.
&ldquo;LoadBalancer&rdquo; builds on NodePort and creates an external load-balancer
(if supported in the current cloud) which routes to the same endpoints
as the clusterIP.
&ldquo;ExternalName&rdquo; aliases this service to the specified externalName.
Several other fields do not apply to ExternalName services.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types">https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types</a></p>
</td>
</tr>
<tr>
<td>
<code>externalIPs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>externalIPs is a list of IP addresses for which nodes in the cluster
will also accept traffic for this service.  These IPs are not managed by
Kubernetes.  The user is responsible for ensuring that traffic arrives
at a node with this IP.  A common example is external load-balancers
that are not part of the Kubernetes system.</p>
</td>
</tr>
<tr>
<td>
<code>sessionAffinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#serviceaffinity-v1-core">
Kubernetes core/v1.ServiceAffinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Supports &ldquo;ClientIP&rdquo; and &ldquo;None&rdquo;. Used to maintain session affinity.
Enable client IP based session affinity.
Must be ClientIP or None.
Defaults to None.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerIP</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Only applies to Service Type: LoadBalancer.
This feature depends on whether the underlying cloud-provider supports specifying
the loadBalancerIP when a load balancer is created.
This field will be ignored if the cloud-provider does not support the feature.
Deprecated: This field was under-specified and its meaning varies across implementations.
Using it is non-portable and it may not support dual-stack.
Users are encouraged to use implementation-specific annotations when available.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerSourceRanges</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If specified and supported by the platform, this will restrict traffic through the cloud-provider
load-balancer will be restricted to the specified client IPs. This field will be ignored if the
cloud-provider does not support the feature.&rdquo;
More info: <a href="https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/">https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/</a></p>
</td>
</tr>
<tr>
<td>
<code>externalName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>externalName is the external reference that discovery mechanisms will
return as an alias for this service (e.g. a DNS CNAME record). No
proxying will be involved.  Must be a lowercase RFC-1123 hostname
(<a href="https://tools.ietf.org/html/rfc1123">https://tools.ietf.org/html/rfc1123</a>) and requires <code>type</code> to be &ldquo;ExternalName&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>externalTrafficPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#serviceexternaltrafficpolicy-v1-core">
Kubernetes core/v1.ServiceExternalTrafficPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>externalTrafficPolicy describes how nodes distribute service traffic they
receive on one of the Service&rsquo;s &ldquo;externally-facing&rdquo; addresses (NodePorts,
ExternalIPs, and LoadBalancer IPs). If set to &ldquo;Local&rdquo;, the proxy will configure
the service in a way that assumes that external load balancers will take care
of balancing the service traffic between nodes, and so each node will deliver
traffic only to the node-local endpoints of the service, without masquerading
the client source IP. (Traffic mistakenly sent to a node with no endpoints will
be dropped.) The default value, &ldquo;Cluster&rdquo;, uses the standard behavior of
routing to all endpoints evenly (possibly modified by topology and other
features). Note that traffic sent to an External IP or LoadBalancer IP from
within the cluster will always get &ldquo;Cluster&rdquo; semantics, but clients sending to
a NodePort from within the cluster may need to take traffic policy into account
when picking a node.</p>
</td>
</tr>
<tr>
<td>
<code>healthCheckNodePort</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>healthCheckNodePort specifies the healthcheck nodePort for the service.
This only applies when type is set to LoadBalancer and
externalTrafficPolicy is set to Local. If a value is specified, is
in-range, and is not in use, it will be used.  If not specified, a value
will be automatically allocated.  External systems (e.g. load-balancers)
can use this port to determine if a given node holds endpoints for this
service or not.  If this field is specified when creating a Service
which does not need it, creation will fail. This field will be wiped
when updating a Service to no longer need it (e.g. changing type).
This field cannot be updated once set.</p>
</td>
</tr>
<tr>
<td>
<code>publishNotReadyAddresses</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>publishNotReadyAddresses indicates that any agent which deals with endpoints for this
Service should disregard any indications of ready/not-ready.
The primary use case for setting this field is for a StatefulSet&rsquo;s Headless Service to
propagate SRV DNS records for its Pods for the purpose of peer discovery.
The Kubernetes controllers that generate Endpoints and EndpointSlice resources for
Services interpret this to mean that all endpoints are considered &ldquo;ready&rdquo; even if the
Pods themselves are not. Agents which consume only Kubernetes generated endpoints
through the Endpoints or EndpointSlice resources can safely assume this behavior.</p>
</td>
</tr>
<tr>
<td>
<code>sessionAffinityConfig</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#sessionaffinityconfig-v1-core">
Kubernetes core/v1.SessionAffinityConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>sessionAffinityConfig contains the configurations of session affinity.</p>
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
<p>IPFamilies is a list of IP families (e.g. IPv4, IPv6) assigned to this
service. This field is usually assigned automatically based on cluster
configuration and the ipFamilyPolicy field. If this field is specified
manually, the requested family is available in the cluster,
and ipFamilyPolicy allows it, it will be used; otherwise creation of
the service will fail. This field is conditionally mutable: it allows
for adding or removing a secondary IP family, but it does not allow
changing the primary IP family of the Service. Valid values are &ldquo;IPv4&rdquo;
and &ldquo;IPv6&rdquo;.  This field only applies to Services of types ClusterIP,
NodePort, and LoadBalancer, and does apply to &ldquo;headless&rdquo; services.
This field will be wiped when updating a Service to type ExternalName.</p>
<p>This field may hold a maximum of two entries (dual-stack families, in
either order).  These families must correspond to the values of the
clusterIPs field, if specified. Both clusterIPs and ipFamilies are
governed by the ipFamilyPolicy field.</p>
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
<p>IPFamilyPolicy represents the dual-stack-ness requested or required by
this Service. If there is no value provided, then this field will be set
to SingleStack. Services can be &ldquo;SingleStack&rdquo; (a single IP family),
&ldquo;PreferDualStack&rdquo; (two IP families on dual-stack configured clusters or
a single IP family on single-stack clusters), or &ldquo;RequireDualStack&rdquo;
(two IP families on dual-stack configured clusters, otherwise fail). The
ipFamilies and clusterIPs fields depend on the value of this field. This
field will be wiped when updating a service to type ExternalName.</p>
</td>
</tr>
<tr>
<td>
<code>allocateLoadBalancerNodePorts</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>allocateLoadBalancerNodePorts defines if NodePorts will be automatically
allocated for services with type LoadBalancer.  Default is &ldquo;true&rdquo;. It
may be set to &ldquo;false&rdquo; if the cluster load-balancer does not rely on
NodePorts.  If the caller requests specific NodePorts (by specifying a
value), those requests will be respected, regardless of this field.
This field may only be set for services with type LoadBalancer and will
be cleared if the type is changed to any other type.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerClass</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>loadBalancerClass is the class of the load balancer implementation this Service belongs to.
If specified, the value of this field must be a label-style identifier, with an optional prefix,
e.g. &ldquo;internal-vip&rdquo; or &ldquo;example.com/internal-vip&rdquo;. Unprefixed names are reserved for end-users.
This field can only be set when the Service type is &lsquo;LoadBalancer&rsquo;. If not set, the default load
balancer implementation is used, today this is typically done through the cloud provider integration,
but should apply for any default implementation. If set, it is assumed that a load balancer
implementation is watching for Services with a matching class. Any default load balancer
implementation (e.g. cloud providers) should ignore Services that set this field.
This field can only be set when creating or updating a Service to type &lsquo;LoadBalancer&rsquo;.
Once set, it can not be changed. This field will be wiped when a service is updated to a non &lsquo;LoadBalancer&rsquo; type.</p>
</td>
</tr>
<tr>
<td>
<code>internalTrafficPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#serviceinternaltrafficpolicy-v1-core">
Kubernetes core/v1.ServiceInternalTrafficPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InternalTrafficPolicy describes how nodes distribute service traffic they
receive on the ClusterIP. If set to &ldquo;Local&rdquo;, the proxy will assume that pods
only want to talk to endpoints of the service on the same node as the pod,
dropping the traffic if there are no local endpoints. The default value,
&ldquo;Cluster&rdquo;, uses the standard behavior of routing to all endpoints evenly
(possibly modified by topology and other features).</p>
</td>
</tr>
</table>
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
<p>RoleSelector extends the ServiceSpec.Selector by allowing you to specify defined role as selector for the service.
if GeneratePodOrdinalService sets to true, RoleSelector will be ignored.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceDescriptorSpec">ServiceDescriptorSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptor">ServiceDescriptor</a>)
</p>
<div>
<p>ServiceDescriptorSpec defines the desired state of ServiceDescriptor</p>
</div>
<table>
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
<p>service kind, indicating the type or nature of the service. It should be well-known application cluster type, e.g. &#123;mysql, redis, mongodb&#125;.
The serviceKind is case-insensitive and supports abbreviations for some well-known databases.
For example, both <code>zk</code> and <code>zookeeper</code> will be considered as a ZooKeeper cluster, and <code>pg</code>, <code>postgres</code>, <code>postgresql</code> will all be considered as a PostgreSQL cluster.</p>
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
<p>The version of the service reference.</p>
</td>
</tr>
<tr>
<td>
<code>endpoint</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>endpoint is the endpoint of the service connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>auth</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConnectionCredentialAuth">
ConnectionCredentialAuth
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>auth is the auth of the service connection credential.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVar">
CredentialVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>port is the port of the service connection credential.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceDescriptorStatus">ServiceDescriptorStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptor">ServiceDescriptor</a>)
</p>
<div>
<p>ServiceDescriptorStatus defines the observed state of ServiceDescriptor</p>
</div>
<table>
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
<p>A human-readable message indicating details about why the ServiceConnectionCredential is in this phase.</p>
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
<p>generation number</p>
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
<p>The name of this port within the service. This must be a DNS_LABEL.
All ports within a ServiceSpec must have unique names. When considering
the endpoints for a Service, this must match the &lsquo;name&rsquo; field in the
EndpointPort.</p>
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
<p>The IP protocol for this port. Supports &ldquo;TCP&rdquo;, &ldquo;UDP&rdquo;, and &ldquo;SCTP&rdquo;.
Default is TCP.</p>
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
<p>The application protocol for this port.
This field follows standard Kubernetes label syntax.
Un-prefixed names are reserved for IANA standard service names (as per
RFC-6335 and <a href="https://www.iana.org/assignments/service-names)">https://www.iana.org/assignments/service-names)</a>.
Non-standard protocols should use prefixed names such as
mycompany.com/my-custom-protocol.</p>
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
<p>The port that will be exposed by this service.</p>
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
<p>Number or name of the port to access on the pods targeted by the service.</p>
<p>Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.</p>
<ul>
<li>If this is a string, it will be looked up as a named port in the target Pod&rsquo;s container ports.</li>
<li>If this is not specified, the value of the <code>port</code> field is used (an identity map).</li>
</ul>
<p>This field is ignored for services with clusterIP=None, and should be
omitted or set equal to the <code>port</code> field.</p>
<p>More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service">https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRef">ServiceRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>)
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
<p>name of the service reference declaration. references the serviceRefDeclaration name defined in clusterDefinition.componentDefs[<em>].serviceRefDeclarations[</em>].name</p>
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
<p>namespace defines the namespace of the referenced Cluster or the namespace of the referenced ServiceDescriptor object.
If not set, the referenced Cluster and ServiceDescriptor will be searched in the namespace of the current cluster by default.</p>
</td>
</tr>
<tr>
<td>
<code>cluster</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>When referencing a service provided by other KubeBlocks cluster, you need to provide the name of the Cluster being referenced.</p>
<p>By default, when other KubeBlocks Cluster are referenced, the ClusterDefinition.spec.connectionCredential secret corresponding to the referenced Cluster will be used to bind to the current component.
Currently, if a KubeBlocks cluster is to be referenced, the connection credential secret should include and correspond to the following fields: endpoint, port, username, and password.</p>
<p>Under this referencing approach, the ServiceKind and ServiceVersion of service reference declaration defined in the ClusterDefinition will not be validated.
If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.</p>
</td>
</tr>
<tr>
<td>
<code>serviceDescriptor</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>serviceDescriptor defines the service descriptor of the service provided by external sources.</p>
<p>When referencing a service provided by external sources, you need to provide the ServiceDescriptor object name to establish the service binding.
And serviceDescriptor is the name of the ServiceDescriptor object, furthermore, the <code>ServiceDescriptor.spec.serviceKind</code> and <code>ServiceDescriptor.spec.serviceVersion</code>
should match <code>clusterDefinition.componentDefs[*].serviceRefDeclarations[*].serviceRefDeclarationSpecs[*].serviceKind</code>
and the regular expression defines in <code>clusterDefinition.componentDefs[*].serviceRefDeclarations[*].serviceRefDeclarationSpecs[*].serviceVersion</code>.</p>
<p>If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefDeclaration">ServiceRefDeclaration
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>The name of the service reference declaration.
The service reference can come from an external service that is not part of KubeBlocks, or services provided by other KubeBlocks Cluster objects.
The specific type of service reference depends on the binding declaration when creates a Cluster.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefDeclarationSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefDeclarationSpec">
[]ServiceRefDeclarationSpec
</a>
</em>
</td>
<td>
<p>serviceRefDeclarationSpecs is a collection of service descriptions for a service reference declaration.</p>
<p>Each ServiceRefDeclarationSpec defines a service Kind and Version. When multiple ServiceRefDeclarationSpecs are defined,
it indicates that the ServiceRefDeclaration can be any one of the specified ServiceRefDeclarationSpecs.</p>
<p>For example, when the ServiceRefDeclaration is declared to require an OLTP database, which can be either MySQL or PostgreSQL,
you can define a ServiceRefDeclarationSpec for MySQL and another ServiceRefDeclarationSpec for PostgreSQL,
when referencing the service within the cluster, as long as the serviceKind and serviceVersion match either MySQL or PostgreSQL, it can be used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefDeclarationSpec">ServiceRefDeclarationSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceRefDeclaration">ServiceRefDeclaration</a>)
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
<code>serviceKind</code><br/>
<em>
string
</em>
</td>
<td>
<p>service kind, indicating the type or nature of the service. It should be well-known application cluster type, e.g. &#123;mysql, redis, mongodb&#125;.
The serviceKind is case-insensitive and supports abbreviations for some well-known databases.
For example, both <code>zk</code> and <code>zookeeper</code> will be considered as a ZooKeeper cluster, and <code>pg</code>, <code>postgres</code>, <code>postgresql</code> will all be considered as a PostgreSQL cluster.</p>
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
<p>The service version of the service reference. It is a regular expression that matches a version number pattern.
For example, <code>^8.0.8$</code>, <code>8.0.\d&#123;1,2&#125;$</code>, <code>^[v\-]*?(\d&#123;1,2&#125;\.)&#123;0,3&#125;\d&#123;1,2&#125;$</code></p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefVarSelector">ServiceRefVarSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarSource">VarSource</a>)
</p>
<div>
<p>ServiceRefVarSelector selects a var from a ServiceRefDeclaration.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ClusterObjectReference</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterObjectReference">
ClusterObjectReference
</a>
</em>
</td>
<td>
<p>
(Members of <code>ClusterObjectReference</code> are embedded into this type.)
</p>
<p>The ServiceRefDeclaration to select from.</p>
</td>
</tr>
<tr>
<td>
<code>ServiceRefVars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVars">
ServiceRefVars
</a>
</em>
</td>
<td>
<p>
(Members of <code>ServiceRefVars</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefVars">ServiceRefVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVarSelector">ServiceRefVarSelector</a>)
</p>
<div>
<p>ServiceRefVars defines the vars can be referenced from a ServiceRef.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>endpoint</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>CredentialVars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVars">
CredentialVars
</a>
</em>
</td>
<td>
<p>
(Members of <code>CredentialVars</code> are embedded into this type.)
</p>
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
<p>The list of ports that are exposed by this service.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies">https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceVarSelector">ServiceVarSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarSource">VarSource</a>)
</p>
<div>
<p>ServiceVarSelector selects a var from a Service.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ClusterObjectReference</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterObjectReference">
ClusterObjectReference
</a>
</em>
</td>
<td>
<p>
(Members of <code>ClusterObjectReference</code> are embedded into this type.)
</p>
<p>The Service to select from.
It can be referenced from the default headless service by setting the name to &ldquo;headless&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>ServiceVars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceVars">
ServiceVars
</a>
</em>
</td>
<td>
<p>
(Members of <code>ServiceVars</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>generatePodOrdinalServiceVar</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>GeneratePodOrdinalServiceVar indicates whether to create a corresponding ServiceVars reference variable for each Pod.
If set to true, a set of ServiceVars that can be referenced will be automatically generated for each Pod Ordinal.
They can be referred to by adding the PodOrdinal to the defined name template with named pattern <code>$(Vars[x].Name)_$(PodOrdinal)</code>.
For example, a ServiceVarRef might be defined as follows:</p>
<pre><code class="language-yaml">
name: MY_SERVICE_PORT
valueFrom:
serviceVarRef:
compDef: my-component-definition
name: my-service
optional: true
generatePodOrdinalServiceVar: true
port:
name: redis-sentinel
</code></pre>
<p>Assuming that the Component has 3 replicas, then you can reference the port of existing services named my-service-0, my-service-1,
and my-service-2 with $MY_SERVICE_PORT_0, $MY_SERVICE_PORT_1, and $MY_SERVICE_PORT_2, respectively.
It should be used in conjunction with Service.GeneratePodOrdinalService.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceVars">ServiceVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceVarSelector">ServiceVarSelector</a>)
</p>
<div>
<p>ServiceVars defines the vars can be referenced from a Service.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>host</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.VarOption">
VarOption
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.NamedVar">
NamedVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>nodePort</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.NamedVar">
NamedVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ShardingSpec">ShardingSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>ShardingSpec defines the sharding spec.</p>
</div>
<table>
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
<p>name defines sharding name, this name is also part of Service DNS name, so this name will comply with IANA Service Naming rule.
The name is also used to generate the name of the underlying components with the naming pattern <code>$(ShardingSpec.Name)-$(ShardID)</code>.
At the same time, the name of component template defined in ShardingSpec.Template.Name will be ignored.</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">
ClusterComponentSpec
</a>
</em>
</td>
<td>
<p>template defines the component template.
A ShardingSpec generates a set of components (also called shards) based on the component template, and this group of components or shards have the same specifications and definitions.</p>
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
<p>shards indicates the number of component, and these components have the same specifications and definitions.</p>
<p>It should be noted that the number of replicas for each component should be defined by template.replicas.
Moreover, the logical relationship between these components should be maintained by the components themselves,
KubeBlocks only provides the following capabilities for managing the lifecycle of sharding:</p>
<ol>
<li>When the number of shards increases, the postProvision Action defined in the ComponentDefinition will be executed if the conditions are met.</li>
<li>When the number of shards decreases, the preTerminate Action defined in the ComponentDefinition will be executed if the conditions are met.
Additionally, the resources and data associated with the corresponding Component will be deleted as well.</li>
</ol>
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
<p>command used to execute for reload.</p>
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
<p>Specify synchronize updates parameters to the config manager.</p>
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
<p>SignalType defines which signals are valid.</p>
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
<p>updateStrategy, Pods update strategy.
In case of workloadType=Consensus the update strategy will be following:</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.
<code>Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader</code></li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
<code>Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader</code>, keep majority online all the time.</li>
<li>parallel: force parallel</li>
</ul>
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
<p>llPodManagementPolicy is the low-level controls how pods are created during initial scale up,
when replacing pods on nodes, or when scaling down.</p>
<ul>
<li><code>OrderedReady</code> policy specify where pods are created in increasing order (pod-0, then
pod-1, etc) and the controller will wait until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.</li>
<li><code>Parallel</code> policy specify create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</li>
</ul>
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
<p>llUpdateStrategy indicates the low-level StatefulSetUpdateStrategy that will be
employed to update Pods in the StatefulSet when a revision is made to
Template. Will ignore <code>updateStrategy</code> attribute if provided.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.StatefulSetWorkload">StatefulSetWorkload
</h3>
<div>
<p>StatefulSetWorkload interface</p>
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
<p>updateStrategy defines the underlying deployment strategy to use to replace existing pods with new ones.</p>
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
<p>The minimum size of storage.</p>
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
<p>The maximum size of storage.</p>
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
<p>SwitchPolicyType defines switchPolicy type.
Currently, only Noop is supported. MaximumAvailability and MaximumDataProtection will be supported in the future.</p>
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
<p>Utilized to designate the candidate primary or leader instance for the switchover process.
If assigned &ldquo;*&rdquo;, it signifies that no specific primary or leader is designated for the switchover,
and the switchoverAction defined in <code>clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate</code> will be executed.</p>
<p>It is mandatory that <code>clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate</code> is not left blank.</p>
<p>If assigned a valid instance name other than &ldquo;*&rdquo;, it signifies that a specific candidate primary or leader is designated for the switchover.
The value can be retrieved using <code>kbcli cluster list-instances</code>, any other value is considered invalid.</p>
<p>In this scenario, the <code>switchoverAction</code> defined in clusterDefinition.componentDefs[x].switchoverSpec.withCandidate will be executed,
and it is mandatory that clusterDefinition.componentDefs[x].switchoverSpec.withCandidate is not left blank.</p>
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
<p>cmdExecutorConfig is the executor configuration of the switchover command.</p>
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
<p>scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
Once ScriptSpecSelectors is defined, the scripts defined in scriptSpecs can be referenced in the SwitchoverAction.CmdExecutorConfig.</p>
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
<p>SwitchoverShortSpec is a short version of SwitchoverSpec, with only CommandExecutorEnvItem field.</p>
</div>
<table>
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
<p>CmdExecutorConfig is the command executor config.</p>
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
<p>withCandidate corresponds to the switchover of the specified candidate primary or leader instance.</p>
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
<p>withoutCandidate corresponds to a switchover that does not specify a candidate primary or leader instance.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SystemAccount">SystemAccount
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
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
<p>The name of the account.
Others can refer to this account by the name.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>initAccount</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>InitAccount indicates whether this is the unique system initialization account (e.g., MySQL root).
Only one system init account is allowed.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>statement</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Statement specifies the statement used to create the account with required privileges.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>passwordGenerationPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PasswordConfig">
PasswordConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PasswordGenerationPolicy defines the policy for generating the account&rsquo;s password.
Cannot be updated.</p>
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
<p>SecretRef specifies the secret from which data will be copied to create the new account.
Cannot be updated.</p>
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
<p>SystemAccountConfig specifies how to create and delete system accounts.</p>
</div>
<table>
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
<p>name is the name of a system account.</p>
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
<p>provisionPolicy defines how to create account.</p>
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
<p>SystemAccountShortSpec is a short version of SystemAccountSpec, with only CmdExecutorConfig field.</p>
</div>
<table>
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
<p>cmdExecutorConfig configs how to get client SDK and perform statements.</p>
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
<p>SystemAccountSpec specifies information to create system accounts.</p>
</div>
<table>
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
<p>cmdExecutorConfig configs how to get client SDK and perform statements.</p>
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
<p>passwordConfig defines the pattern to generate password.</p>
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
<p>accounts defines system account config settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TLSConfig">TLSConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>)
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
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>TLSSecretRef defines Secret contains Tls certs</p>
</div>
<table>
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
<p>Name of the Secret</p>
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
<p>CA cert key in Secret</p>
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
<p>Cert key in Secret</p>
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
<p>Key of TLS private key in Secret</p>
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
<p>Specify synchronize updates parameters to the config manager.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TargetInstance">TargetInstance
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BackupMethod">BackupMethod</a>, <a href="#apps.kubeblocks.io/v1alpha1.BackupPolicy">BackupPolicy</a>)
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
<p>select instance of corresponding role for backup, role are:</p>
<ul>
<li>the name of Leader/Follower/Leaner for Consensus component.</li>
<li>primary or secondary for Replication component.</li>
</ul>
<p>finally, invalid role of the component will be ignored.
such as if workload type is Replication and component&rsquo;s replicas is 1,
the secondary role is invalid. and it also will be ignored when component is Stateful/Stateless.
the role will be transformed to a role LabelSelector for BackupPolicy&rsquo;s target attribute.</p>
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
<p>refer to spec.componentDef.systemAccounts.accounts[*].name in ClusterDefinition.
the secret created by this account will be used to connect the database.
if not set, the secret created by spec.ConnectionCredential of the ClusterDefinition will be used.
it will be transformed to a secret for BackupPolicy&rsquo;s target secret.</p>
</td>
</tr>
<tr>
<td>
<code>strategy</code><br/>
<em>
github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1.PodSelectionStrategy
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodSelectionStrategy specifies the strategy to select when multiple pods are
selected for backup target.
Valid values are:</p>
<ul>
<li>Any: select any one pod that match the labelsSelector.</li>
<li>All: select all pods that match the labelsSelector.</li>
</ul>
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
<em>(Optional)</em>
<p>connectionCredentialKey defines connection credential key in secret
which created by spec.ConnectionCredential of the ClusterDefinition.
it will be ignored when &ldquo;account&rdquo; is set.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TargetPodSelector">TargetPodSelector
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
</p>
<div>
<p>TargetPodSelector defines how to select pod(s) to execute a action.</p>
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
</tr><tr><td><p>&#34;Ordinal&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Role&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.TenancyType">TenancyType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Affinity">Affinity</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>TenancyType for cluster tenant resources.</p>
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
<p>TerminationPolicyType defines termination policy types.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Delete&#34;</p></td>
<td><p>Delete is based on Halt and deletes PVCs.</p>
</td>
</tr><tr><td><p>&#34;DoNotTerminate&#34;</p></td>
<td><p>DoNotTerminate will block delete operation.</p>
</td>
</tr><tr><td><p>&#34;Halt&#34;</p></td>
<td><p>Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.</p>
</td>
</tr><tr><td><p>&#34;WipeOut&#34;</p></td>
<td><p>WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.</p>
</td>
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
<p>Specify the name of initContainer. Must be a DNS_LABEL name.</p>
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
<p>tools Container image name.</p>
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
<p>exec used to execute for init containers.</p>
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
<p>mountPoint is the mount point of the scripts file.</p>
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
<p>toolConfig used to config init container.</p>
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
<p>signal is valid for unix signal.
e.g: SIGHUP
url: ../../pkg/configuration/configmap/handler.go:allUnixSignals</p>
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
<p>processName is process name, sends unix signal to proc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UpdateStrategy">UpdateStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">StatefulSetSpec</a>)
</p>
<div>
<p>UpdateStrategy defines Cluster Component update strategy.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemStatus">ConfigurationItemStatus</a>)
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
<p>Lists the keys that have been added.</p>
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
<p>Lists the keys that have been deleted.</p>
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
<p>Lists the keys that have been updated.</p>
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
<p>Upgrade represents the parameters required for an upgrade operation.</p>
</div>
<table>
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
<p>A reference to the name of the ClusterVersion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UpgradePolicy">UpgradePolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItem">ConfigurationItem</a>, <a href="#apps.kubeblocks.io/v1alpha1.ConfigurationItemStatus">ConfigurationItemStatus</a>)
</p>
<div>
<p>UpgradePolicy defines the policy of reconfiguring.</p>
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
</tr><tr><td><p>&#34;dynamicReloadBeginRestart&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;none&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;simple&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;parallel&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;rolling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;operatorSyncUpdate&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.UserResourceRefs">UserResourceRefs
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
<code>secretRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SecretRef">
[]SecretRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>secretRefs defines the user-defined secrets.</p>
</td>
</tr>
<tr>
<td>
<code>configMapRefs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConfigMapRef">
[]ConfigMapRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>configMapRefs defines the user-defined configmaps.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ValueFrom">ValueFrom
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.EnvMappingVar">EnvMappingVar</a>)
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
<a href="#apps.kubeblocks.io/v1alpha1.ValueMapping">
[]ValueMapping
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>mapped ClusterVersionRef to env value.</p>
</td>
</tr>
<tr>
<td>
<code>componentDef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ValueMapping">
[]ValueMapping
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>mapped ComponentDefinition to env value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ValueMapping">ValueMapping
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ValueFrom">ValueFrom</a>)
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
<code>names</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>the array of ClusterVersion name which can be mapped to the env value.</p>
</td>
</tr>
<tr>
<td>
<code>mappingValue</code><br/>
<em>
string
</em>
</td>
<td>
<p>mapping value for the specified ClusterVersion names.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VarOption">VarOption
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CredentialVars">CredentialVars</a>, <a href="#apps.kubeblocks.io/v1alpha1.NamedVar">NamedVar</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVars">ServiceRefVars</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceVars">ServiceVars</a>)
</p>
<div>
<p>VarOption defines whether a variable is required or optional.</p>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.VarSource">VarSource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.EnvVar">EnvVar</a>)
</p>
<div>
<p>VarSource represents a source for the value of an EnvVar.</p>
</div>
<table>
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
<tr>
<td>
<code>serviceVarRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceVarSelector">
ServiceVarSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a defined var of a Service.</p>
</td>
</tr>
<tr>
<td>
<code>credentialVarRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CredentialVarSelector">
CredentialVarSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a defined var of a Credential (SystemAccount).</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefVarRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVarSelector">
ServiceRefVarSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a defined var of a ServiceRef.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VarsRef">VarsRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionRef">ComponentDefinitionRef</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionSpec">OpsDefinitionSpec</a>)
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
<code>podSelectionStrategy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodSelectionStrategy">
PodSelectionStrategy
</a>
</em>
</td>
<td>
<p>Defines the method to select the target component pod for variable references.
The strategy can be either &lsquo;PreferredAvailable&rsquo; which prioritizes the selection of available pods,
or &lsquo;Available&rsquo; which selects only available pods and terminates the operation if none are found.</p>
</td>
</tr>
<tr>
<td>
<code>vars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsEnvVar">
[]OpsEnvVar
</a>
</em>
</td>
<td>
<p>Represents a list of environment variables to be set in the job&rsquo;s container.</p>
</td>
</tr>
</tbody>
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
<p>Provide ClusterDefinition.spec.componentDefs.podSpec.initContainers override
values, typical scenarios are application container image updates.</p>
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
<p>Provide ClusterDefinition.spec.componentDefs.podSpec.containers override
values, typical scenarios are application container image updates.</p>
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
<p>VerticalScaling defines the parameters required for scaling compute resources.</p>
</div>
<table>
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
<p>Defines the computational resource size for vertical scaling.</p>
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
<p>A reference to a class defined in ComponentClassDefinition.</p>
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
<p>volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.</p>
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
<p>The high watermark threshold for volume space usage.
If there is any specified volumes who&rsquo;s space usage is over the threshold, the pre-defined &ldquo;LOCK&rdquo; action
will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
as read-only. And after that, if all volumes&rsquo; space usage drops under the threshold later, the pre-defined
&ldquo;UNLOCK&rdquo; action will be performed to recover the service normally.</p>
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
<p>Volumes to protect.</p>
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
<p>VolumeType defines volume type for backup data or log.</p>
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
<p>name definition is the same as the name of the VolumeMounts field in PodSpec.Container,
similar to the relations of Volumes[<em>].name and VolumesMounts[</em>].name in Pod.Spec.</p>
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
<p>type is in enum of &#123;data, log&#125;.
VolumeTypeData: the volume is for the persistent data storage.
VolumeTypeLog: the volume is for the persistent log storage.</p>
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
<p>WorkloadType defines ClusterDefinition&rsquo;s component workload type.</p>
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
when replacing pods on nodes, or when scaling down.</p>
<ul>
<li>The default policy is <code>OrderedReady</code>, where pods are created in increasing order (pod-0, then
pod-1, etc) and the controller will wait until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.</li>
<li>The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</li>
</ul>
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
<p>MemberUpdateStrategy, Members(Pods) update strategy.</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.
<code>Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader</code></li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
<code>Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader</code>, keep majority online all the time.</li>
<li>parallel: force parallel</li>
</ul>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineStatus">ReplicatedStateMachineStatus</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.MemberStatus">MemberStatus</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
when replacing pods on nodes, or when scaling down.</p>
<ul>
<li>The default policy is <code>OrderedReady</code>, where pods are created in increasing order (pod-0, then
pod-1, etc) and the controller will wait until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.</li>
<li>The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</li>
</ul>
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
<p>MemberUpdateStrategy, Members(Pods) update strategy.</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.
<code>Learner -&gt; Follower(with AccessMode=none) -&gt; Follower(with AccessMode=readonly) -&gt; Follower(with AccessMode=readWrite) -&gt; Leader</code></li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
<code>Learner, Follower(minority) in parallel -&gt; Follower(majority) -&gt; Leader</code>, keep majority online all the time.</li>
<li>parallel: force parallel</li>
</ul>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.ReplicatedStateMachineSpec">ReplicatedStateMachineSpec</a>)
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
