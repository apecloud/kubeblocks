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
<a href="#apps.kubeblocks.io%2fv1beta1">apps.kubeblocks.io/v1beta1</a>
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
<a href="#apps.kubeblocks.io/v1alpha1.Cluster">Cluster</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinition">ClusterDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterVersion">ClusterVersion</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.Component">Component</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersion">ComponentVersion</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.OpsDefinition">OpsDefinition</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.OpsRequest">OpsRequest</a>
</li><li>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptor">ServiceDescriptor</a>
</li></ul>
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
<p>Specifies the name of the ClusterDefinition to use when creating a Cluster.</p>
<p>This field enables users to create a Cluster using a specific ClusterDefinition and topology.
The specified ClusterDefinition determines the components to be used based on its defined topology,
allowing for the utilization of predefined configurations and behaviors.</p>
<p>Advanced users may opt for more precise control by directly referencing specific
ComponentDefinitions for each component within <code>componentSpecs[*].componentDef</code>.
This method offers granular control over the composition of the Cluster.</p>
<p>If this field is not provided, each component must be explicitly defined in <code>componentSpecs[*].componentDef</code>.</p>
<p>Note: Once set, this field cannot be modified; it is immutable.</p>
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
<p>Refers to the ClusterVersion name.</p>
<p>Deprecated since v0.9, use ComponentVersion instead.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>topology</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the ClusterTopology to be used when creating the Cluster.</p>
<p>This field defines which set of components, as outlined in the ClusterDefinition, will be used to
construct the Cluster based on the named topology.
The ClusterDefinition may list multiple topologies under <code>clusterdefinition.spec.topologies[*]</code>,
each tailored to different use cases or environments.</p>
<p>If <code>topology</code> is not specified, the Cluster will default to the topology defined in the ClusterDefinition.</p>
<p>Note: Once set during the Cluster creation, the Topology field cannot be modified.
It establishes the initial composition and structure of the Cluster and is intended for one-time configuration.</p>
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
<p>Specifies the behavior when a Cluster is deleted.
It defines how resources, data, and backups associated with a Cluster are managed during termination.
Choose a policy based on the desired level of resource cleanup and data preservation:</p>
<ul>
<li><code>DoNotTerminate</code>: Prevents deletion of the Cluster. This policy ensures that all resources remain intact.</li>
<li><code>Halt</code>: Deletes Cluster resources like Pods and Services but retains Persistent Volume Claims (PVCs),
allowing for data preservation while stopping other operations.</li>
<li><code>Delete</code>: Extends the <code>Halt</code> policy by also removing PVCs, leading to a thorough cleanup while
removing all persistent data.</li>
<li><code>WipeOut</code>: An aggressive policy that deletes all Cluster resources, including volume snapshots and
backups in external storage.
This results in complete data removal and should be used cautiously, primarily in non-production environments
to avoid irreversible data loss.</li>
</ul>
<p>Warning: Choosing an inappropriate termination policy can result in significant data loss.
The <code>WipeOut</code> policy is particularly risky in production environments due to its irreversible nature.</p>
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
<p>Specifies a list of ShardingSpec objects that configure the sharding topology for components of a Cluster.
Each ShardingSpec corresponds to a group of components organized into shards,
with each shard containing multiple replicas.
Components within a shard are based on a common ClusterComponentSpec template,
ensuring that all components in a shard have identical configurations as per the template.</p>
<p>This field supports dynamic scaling by facilitating the addition or removal of shards based on
the specified number in each ShardingSpec.</p>
<p>Note: <code>shardingSpecs</code> and <code>componentSpecs</code> cannot both be empty; at least one must be defined to configure a cluster.</p>
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
<p>Specifies a list of ClusterComponentSpec objects used to define the individual components that make up a Cluster.
This field allows for detailed configuration of each component within the Cluster.</p>
<p>Note: <code>shardingSpecs</code> and <code>componentSpecs</code> cannot both be empty; at least one must be defined to configure a cluster.</p>
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
<p>Defines the list of services that are exposed by a Cluster.
This field allows selected components, either from <code>componentSpecs</code> or <code>shardingSpecs</code>,
to be exposed as cluster-level services.</p>
<p>Services defined here can be referenced by other clusters using the ServiceRefClusterSelector.</p>
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
<p>Defines a set of node affinity scheduling rules for the Cluster&rsquo;s Pods.
This field helps control the placement of Pods on nodes within the cluster.</p>
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
<p>An array that specifies tolerations attached to the Cluster&rsquo;s Pods,
allowing them to be scheduled onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>schedulingPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulingPolicy">
SchedulingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling policy for the cluster.</p>
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
<p>Specifies the backup configuration of the Cluster.</p>
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
<p>Describes how pods are distributed across node.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Describes the availability policy, including zone, node, and none.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>The configuration of network.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines RuntimeClassName for all Pods managed by this cluster.</p>
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
<p>ClusterDefinition is the Schema for the ClusterDefinition API</p>
</div>
<table>
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
<p>Specifies the well-known database type, such as mysql, redis, or mongodb.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<em>(Optional)</em>
<p>Provides the definitions for the cluster components.</p>
<p>Deprecated since v0.9.
Components should now be individually defined using ComponentDefinition and
collectively referenced via <code>topology.components</code>.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Connection credential template used for creating a connection credential secret for cluster objects.</p>
<p>Built-in objects are:</p>
<ul>
<li><code>$(RANDOM_PASSWD)</code> random 8 characters.</li>
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
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>topologies</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTopology">
[]ClusterTopology
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Topologies defines all possible topologies within the cluster.</p>
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
<p>ClusterVersion is the Schema for the ClusterVersions API.</p>
<p>Deprecated: ClusterVersion has been replaced by ComponentVersion since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Specifies a reference to the ClusterDefinition.</p>
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
<p>Contains a list of versioning contexts for the components&rsquo; containers.</p>
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
<p>Component is a derived sub-object of a user-submitted Cluster object.
It is an internal object, and users should not modify Component objects;
they are intended only for observing the status of Components within the system.</p>
</div>
<table>
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
<p>Specifies the name of the referenced ComponentDefinition.</p>
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
<p>ServiceVersion specifies the version of the service expected to be provisioned by this component.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).</p>
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
<p>Defines a list of ServiceRef for a Component, allowing it to connect and interact with other services.
These services can be external or managed by the same KubeBlocks operator, categorized as follows:</p>
<ol>
<li><p>External Services:</p>
<ul>
<li>Not managed by KubeBlocks. These could be services outside KubeBlocks or non-Kubernetes services.</li>
<li>Connection requires a ServiceDescriptor providing details for service binding.</li>
</ul></li>
<li><p>KubeBlocks Services:</p>
<ul>
<li>Managed within the same KubeBlocks environment.</li>
<li>Service binding is achieved by specifying cluster names in the service references,
with configurations handled by the KubeBlocks operator.</li>
</ul></li>
</ol>
<p>ServiceRef maintains cluster-level semantic consistency; references with the same <code>serviceRef.name</code>
within the same cluster are treated as identical.
Only bindings to the same cluster or ServiceDescriptor are allowed within a cluster.</p>
<p>Example:</p>
<pre><code class="language-yaml">serviceRefs:
- name: &quot;redis-sentinel&quot;
serviceDescriptor:
name: &quot;external-redis-sentinel&quot;
- name: &quot;postgres-cluster&quot;
cluster:
name: &quot;my-postgres-cluster&quot;
</code></pre>
<p>The example above includes references to an external Redis Sentinel service and a PostgreSQL cluster managed by KubeBlocks.</p>
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
<p>Specifies the resources required by the Component.
It allows defining the CPU, memory requirements and limits for the Component&rsquo;s containers.</p>
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
<p>Specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component.
Each template specifies the desired characteristics of a persistent volume, such as storage class,
size, and access modes.
These templates are used to dynamically provision persistent volumes for the Component when it is deployed.</p>
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
<p>Overrides services defined in referenced ComponentDefinition and exposes endpoints that can be accessed
by clients.</p>
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
<p>Each component supports running multiple replicas to provide high availability and persistence.
This field can be used to specify the desired number of replicas.</p>
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
<p>Reserved field for future use.</p>
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
<p>Specifies which types of logs should be collected for the Cluster.
The log types are defined in the <code>componentDefinition.spec.logConfigs</code> field with the LogConfig entries.</p>
<p>The elements in the <code>enabledLogs</code> array correspond to the names of the LogConfig entries.
For example, if the <code>componentDefinition.spec.logConfigs</code> defines LogConfig entries with
names &ldquo;slow_query_log&rdquo; and &ldquo;error_log&rdquo;,
you can enable the collection of these logs by including their names in the <code>enabledLogs</code> array:
enabledLogs: [&ldquo;slow_query_log&rdquo;, &ldquo;error_log&rdquo;]</p>
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
<p>Specifies the name of the ServiceAccount required by the running Component.
This ServiceAccount is used to grant necessary permissions for the Component&rsquo;s Pods to interact
with other Kubernetes resources, such as modifying pod labels or sending events.</p>
<p>Defaults:
If not specified, KubeBlocks automatically assigns a default ServiceAccount named &ldquo;kb-&#123;cluster.name&#125;&rdquo;,
bound to a default role defined during KubeBlocks installation.</p>
<p>Future Changes:
Future versions might change the default ServiceAccount creation strategy to one per Component,
potentially revising the naming to &ldquo;kb-&#123;cluster.name&#125;-&#123;component.name&#125;&rdquo;.</p>
<p>Users can override the automatic ServiceAccount assignment by explicitly setting the name of
an existed ServiceAccount in this field.</p>
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
<p>Specifies a group of affinity scheduling rules for the Component.
It allows users to control how the Component&rsquo;s Pods are scheduled onto nodes in the cluster.</p>
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
<p>Allows the Component to be scheduled onto nodes with matching taints.
It is an array of tolerations that are attached to the Component&rsquo;s Pods.</p>
<p>Each toleration consists of a <code>key</code>, <code>value</code>, <code>effect</code>, and <code>operator</code>.
The <code>key</code>, <code>value</code>, and <code>effect</code> define the taint that the toleration matches.
The <code>operator</code> specifies how the toleration matches the taint.</p>
<p>If a node has a taint that matches a toleration, the Component&rsquo;s pods can be scheduled onto that node.
This allows the Component&rsquo;s Pods to run on nodes that have been tainted to prevent regular Pods from being scheduled.</p>
</td>
</tr>
<tr>
<td>
<code>schedulingPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulingPolicy">
SchedulingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling policy for the component.</p>
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
<p>Specifies the TLS configuration for the component, including:</p>
<ul>
<li>A boolean flag that indicates whether the component should use Transport Layer Security (TLS) for secure communication.</li>
<li>An optional field that specifies the configuration for the TLS certificates issuer when TLS is enabled.
It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allows for the customization of configuration values for each instance within a component.
An Instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
While instances typically share a common configuration as defined in the ClusterComponentSpec,
they can require unique settings in various scenarios:</p>
<p>For example:
- A database component might require different resource allocations for primary and secondary instances,
with primaries needing more resources.
- During a rolling upgrade, a component may first update the image for one or a few instances,
and then update the remaining instances after verifying that the updated instances are functioning correctly.</p>
<p>InstanceTemplate allows for specifying these unique configurations per instance.
Each instance&rsquo;s name is constructed using the pattern: $(component.name)-$(template.name)-$(ordinal),
starting with an ordinal of 0.
It is crucial to maintain unique names for each InstanceTemplate to avoid conflicts.</p>
<p>The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the Component.
Any remaining replicas will be generated using the default template and will follow the default naming rules.</p>
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
<p>Specifies the names of instances to be transitioned to offline status.</p>
<p>Marking an instance as offline results in the following:</p>
<ol>
<li>The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
future reuse or data recovery, but it is no longer actively used.</li>
<li>The ordinal number assigned to this instance is preserved, ensuring it remains unique
and avoiding conflicts with new instances.</li>
</ol>
<p>Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
ordinal consistency within the cluster.
Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines RuntimeClassName for all Pods managed by this component.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>monitorEnabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines whether the metrics exporter needs to be published to the service endpoint.
If set to true, the metrics exporter will be published to the service endpoint,
the service will be injected with the following annotations:
- &ldquo;monitor.kubeblocks.io/path&rdquo;
- &ldquo;monitor.kubeblocks.io/port&rdquo;
- &ldquo;monitor.kubeblocks.io/scheme&rdquo;</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition
</h3>
<div>
<p>ComponentDefinition is the Schema for the ComponentDefinition API</p>
</div>
<table>
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
<p>Specifies the name of the component provider, typically the vendor or developer name.
It identifies the entity responsible for creating and maintaining the component.</p>
<p>When specifying the provider name, consider the following guidelines:</p>
<ul>
<li>Keep the name concise and relevant to the component.</li>
<li>Use a consistent naming convention across components from the same provider.</li>
<li>Avoid using trademarked or copyrighted names without proper permission.</li>
</ul>
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
<p>Provides a brief and concise explanation of the component&rsquo;s purpose, functionality, and any relevant details.
It serves as a quick reference for users to understand the component&rsquo;s role and characteristics.</p>
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
<p>Defines the type of well-known service protocol that the component provides.
It specifies the standard or widely recognized protocol used by the component to offer its services.</p>
<p>The <code>serviceKind</code> field allows users to quickly identify the type of service provided by the component
based on common protocols or service types. This information helps in understanding the compatibility,
interoperability, and usage of the component within a system.</p>
<p>Some examples of well-known service protocols include:</p>
<ul>
<li>&ldquo;MySQL&rdquo;: Indicates that the component provides a MySQL database service.</li>
<li>&ldquo;PostgreSQL&rdquo;: Indicates that the component offers a PostgreSQL database service.</li>
<li>&ldquo;Redis&rdquo;: Signifies that the component functions as a Redis key-value store.</li>
<li>&ldquo;ETCD&rdquo;: Denotes that the component serves as an ETCD distributed key-value store.</li>
</ul>
<p>The <code>serviceKind</code> value is case-insensitive, allowing for flexibility in specifying the protocol name.</p>
<p>When specifying the <code>serviceKind</code>, consider the following guidelines:</p>
<ul>
<li>Use well-established and widely recognized protocol names or service types.</li>
<li>Ensure that the <code>serviceKind</code> accurately represents the primary service offered by the component.</li>
<li>If the component provides multiple services, choose the most prominent or commonly used protocol.</li>
<li>Limit the <code>serviceKind</code> to a maximum of 32 characters for conciseness and readability.</li>
</ul>
<p>Note: The <code>serviceKind</code> field is optional and can be left empty if the component does not fit into a well-known
service category or if the protocol is not widely recognized. It is primarily used to convey information about
the component&rsquo;s service type to users and facilitate discovery and integration.</p>
<p>The <code>serviceKind</code> field is immutable and cannot be updated.</p>
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
<p>Specifies the version of the service provided by the component.
It follows the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).</p>
<p>The Semantic Versioning specification defines a version number format of X.Y.Z (MAJOR.MINOR.PATCH), where:</p>
<ul>
<li>X represents the major version and indicates incompatible API changes.</li>
<li>Y represents the minor version and indicates added functionality in a backward-compatible manner.</li>
<li>Z represents the patch version and indicates backward-compatible bug fixes.</li>
</ul>
<p>Additional labels for pre-release and build metadata are available as extensions to the X.Y.Z format:</p>
<ul>
<li>Use pre-release labels (e.g., -alpha, -beta) for versions that are not yet stable or ready for production use.</li>
<li>Use build metadata (e.g., +build.1) for additional version information if needed.</li>
</ul>
<p>Examples of valid ServiceVersion values:</p>
<ul>
<li>&ldquo;1.0.0&rdquo;</li>
<li>&ldquo;2.3.1&rdquo;</li>
<li>&ldquo;3.0.0-alpha.1&rdquo;</li>
<li>&ldquo;4.5.2+build.1&rdquo;</li>
</ul>
<p>The <code>serviceVersion</code> field is immutable and cannot be updated.</p>
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
<p>Defines the component&rsquo;s container configuration that serves as a template for instantiated Components.
It includes the following elements:</p>
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
<p>The runtime field is intended to define information that remains consistent across all instantiated components
and serves as a template for their configuration.
Dynamic settings such as CPU and memory resource limits, as well as scheduling settings (affinity,
toleration, priority), may vary for each instantiated component and are specified separately in the
<code>cluster.spec.componentSpecs</code> field.</p>
<p>However, there can be exceptions. In some cases, individual component instances may need to override
certain aspects of the runtime configuration to customize their behavior.
For example, they may specify a different container image or add/modify environment variables.
Such instance-specific overrides can be specified in the <code>cluster.spec.componentSpecs[*].instances</code> field.</p>
<p>This field is immutable and cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainerSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSpec">
[]SidecarContainerSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>builtinMonitorContainer</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BuiltinMonitorContainerRef">
BuiltinMonitorContainerRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the built-in metrics exporter container.</p>
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
<p>Represents user-defined variables that can be used as environment variables for Pods and Actions,
or to render templates of config and script.
These variables are placed in front of the environment variables declared in the Pod if used as
environment variables.</p>
<p>The value of a var can be populated from the following sources:</p>
<ul>
<li>ConfigMap: Allows you to select a ConfigMap and a specific key within that ConfigMap to extract the value from.</li>
<li>Secret: Allows you to select a Secret and a specific key within that Secret to extract the value from.</li>
<li>Pod: Retrieves values (including ports) from a selected Pod.</li>
<li>Service: Retrieves values (including address, port, NodePort) from a selected Service.
The purpose of ServiceVar is to obtain the address of a ComponentService.</li>
<li>Credential: Retrieves values (including account name, account password) from a SystemAccount variable.</li>
<li>ServiceRef: Retrieves values (including address, port, account name, account password) from a selected
ServiceRefDeclaration.
The purpose of ServiceRefVar is to obtain the specific address that a ServiceRef is bound to
(e.g., a ClusterService of another Cluster).</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the volumes used by the Component and some static attributes of the volumes.
After defining the volumes here, user can reference them in the
<code>cluster.spec.componentSpecs[*].volumeClaimTemplates</code> field to configure dynamic properties such as
volume capacity and storage class.</p>
<p>This field allows you to specify the following:</p>
<ul>
<li>Snapshot behavior: Determines whether a snapshot of the volume should be taken when performing
a snapshot backup of the component.</li>
<li>Disk high watermark: Sets the high watermark for the volume&rsquo;s disk usage.
When the disk usage reaches the specified threshold, it triggers an alert or action.</li>
</ul>
<p>By configuring these volume behaviors, you can control how the volumes are managed and monitored within the component.</p>
<p>This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>hostNetwork</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HostNetwork">
HostNetwork
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the host network configuration for the component.</p>
<p>When <code>hostNetwork</code> option is enabled, the Pod shares the host&rsquo;s network namespace and can directly access
the host&rsquo;s network interfaces.
This means that if multiple Pods need to use the same port, they cannot run on the same host simultaneously
due to port conflicts.</p>
<p>The DNSPolicy field in the Pod spec determines how containers within the Pod perform DNS resolution.
When using hostNetwork, the operator will set the DNSPolicy to &lsquo;ClusterFirstWithHostNet&rsquo;.
With this policy, DNS queries will first go through the K8s cluster&rsquo;s DNS service.
If the query fails, it will fall back to the host&rsquo;s DNS settings.</p>
<p>If set, the DNS policy will be automatically set to &ldquo;ClusterFirstWithHostNet&rdquo;.</p>
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
<p>Defines the Services used to access the Component.</p>
<p>By default, a headless service will be automatically created with the name pattern
<code>&#123;cluster.name&#125;-&#123;component.name&#125;-headless</code>.
This headless service is used for internal communication within the cluster.</p>
<p>In addition to the default headless service, this field allows you to customize a list of services
that expose the Component&rsquo;s endpoints to other Components within the same cluster.
Each service in the ComponentService can have its own set of ports, type, and other service-related configurations.</p>
<p>When a Component needs to access a ComponentService provided by another Component within the same cluster,
it can declare a variable in the <code>componentDefinition.spec.vars</code> section and bind it to the exposed address
using the <code>serviceVarRef</code> field.</p>
<p>This field is immutable.</p>
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
<p>Defines the configuration file templates and volume mount parameters used by the Component.
This field contains a list of templateRef that will be rendered into Component instances&rsquo; configuration files
based on the provided templates.
The rendered configuration files will be mounted into the component&rsquo;s containers
using the specified volume mount parameters.</p>
<p>This field is immutable.</p>
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
<p>Defines the types of logs generated by instances of the Component and their corresponding file paths.
These logs can be collected for further analysis and monitoring.</p>
<p>The <code>logConfigs</code> field is an optional list of <code>LogConfig</code> objects, where each object represents
a specific log type and its configuration.
It allows you to specify multiple log types and their respective file paths for the Component instances.</p>
<p>Examples:</p>
<pre><code class="language-yaml"> logConfigs:
- filePathPattern: /data/mysql/log/mysqld-error.log
name: error
- filePathPattern: /data/mysql/log/mysqld.log
name: general
- filePathPattern: /data/mysql/log/mysqld-slowquery.log
name: slow
</code></pre>
<p>This field is immutable.</p>
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
<p>Specifies groups of scripts (each group of scripts provided as a single ConfigMap) to be mounted as volumes and
can be invoked in the container&rsquo;s startup command or action&rsquo;s command.</p>
<p>The <code>scripts</code> field is an optional array that allows you to define a set of scripts to be used by the component.
Each element in the <code>scripts</code> array is defined as a <code>ComponentTemplateSpec</code> object, which contains the following:</p>
<ul>
<li>A ConfigMap object that holds a set of scripts.</li>
<li>A mount point where the scripts will be mounted inside the container.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the namespaced policy rules required by the Component.</p>
<p>The <code>policyRules</code> field is an optional array of <code>rbacv1.PolicyRule</code> objects that define the policy rules
needed by the Component to operate within a namespace.
These policy rules determine the permissions and actions the Component is allowed to perform on
Kubernetes resources within the namespace.</p>
<p>The purpose of this field is to automatically generate the necessary RBAC roles
for the Component based on the specified policy rules.
This ensures that the Pods in the Component has the required permissions to function properly within the namespace.</p>
<p>Note: This field is currently non-functional and is reserved for future implementation.</p>
<p>This field is immutable.</p>
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
<p>Specifies static labels that will be patched to all Kubernetes resources created for the component.</p>
<p>Note: If a label key in the <code>labels</code> field conflicts with any system labels or user-specified labels,
it will be silently ignored to avoid overriding critical labels.</p>
<p>This field is immutable.</p>
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
<p>Specifies static annotations that will be patched to all Kubernetes resources created for the component.</p>
<p>Note: If an annotation key in the <code>annotations</code> field conflicts with any system annotations
or user-specified annotations, it will be silently ignored to avoid overriding critical annotations.</p>
<p>This field is immutable.</p>
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
<p>Defines the upper limit of the number of replicas supported by the Component.</p>
<p>It defines the maximum number of replicas that can be created for the component.
This field allows you to set a limit on the scalability of the component, preventing it from exceeding a certain number of replicas.</p>
<p>This field is immutable.</p>
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
<p>An array of <code>SystemAccount</code> objects that define the system accounts needed
for the management operations of the Component.</p>
<p>Each <code>SystemAccount</code> object in the array contains the following fields:</p>
<ul>
<li>Account name.</li>
<li>The SQL statement template used to create the system account.</li>
<li>The source of the password for the system account. It can be generated based on certain rules or copied from a secret.</li>
</ul>
<p>Common scenarios of system accounts include:</p>
<ul>
<li>Accounts required to initialize the system</li>
<li>Performing backup tasks</li>
<li>Monitoring</li>
<li>Health checks</li>
<li>Replica replication</li>
<li>Other system-level operations</li>
</ul>
<p>System accounts are distinct from user accounts, although both are database accounts.</p>
<ul>
<li>System accounts are typically created by the operator during cluster creation and initialization.
They usually have higher privileges and are used exclusively by the operator and for system management tasks.
System accounts are managed by the KubeBlocks operator using a declarative API, and their lifecycle is fully controlled by the operator.</li>
<li>User accounts, on the other hand, are managed through ops operations and their lifecycle is controlled by users or administrators.
User account permissions should follow the principle of least privilege, granting only the necessary access rights to complete their required tasks.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Specifies the strategy for updating the component instance when it has multiple replicas.
It determines whether multiple replicas can be updated concurrently.</p>
<p>The available strategies are:</p>
<ul>
<li><code>Serial</code>: The replicas are updated one at a time in a serial manner.
The operator waits for each replica to be updated and ready before proceeding to the next one.
This ensures that only one replica is unavailable at a time during the update process.</li>
<li><code>Parallel</code>: The replicas are updated in parallel, with the operator updating all replicas concurrently.
This strategy provides the fastest update time but may lead to a period of reduced availability or
capacity during the update process.</li>
<li><code>BestEffortParallel</code>: The replicas are updated in parallel, with the operator making a best-effort attempt
to update as many replicas as possible concurrently while maintaining the component&rsquo;s availability.
Unlike the <code>Parallel</code> strategy, the <code>BestEffortParallel</code> strategy aims to ensure that a minimum number
of replicas remain available during the update process to maintain the component&rsquo;s quorum and functionality.
For example, consider a component with 5 replicas. To maintain the component&rsquo;s availability and quorum,
the operator may allow a maximum of 2 replicas to be simultaneously updated. This ensures that at least
3 replicas (a quorum) remain available and functional during the update process.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Enumerate all the possible roles a replica of the Component can have.
Each replica can have zero or more roles assigned to it.</p>
<p>KubeBlocks operator determines the roles of each replica by invoking the <code>lifecycleActions.roleProbe</code> method.
This method returns a list of roles for each replica, and the returned roles must be defined in the <code>roles</code> field.</p>
<p>The roles assigned to a replica can influence various aspects of the Component&rsquo;s behavior, such as:</p>
<ul>
<li>Service selection: The Component&rsquo;s provided services can be selected based on the roles of the replicas.
For example, a service may only target replicas with a specific role.</li>
<li>Update order: The roles can determine the order in which replicas are updated during a Component update.
For instance, replicas with a &ldquo;follower&rdquo; role can be updated first, while the replica with the &ldquo;leader&rdquo;
role is updated last. This helps minimize the number of leader changes during the update process.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the strategy for electing the component&rsquo;s active role.</p>
<p>This field has been deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
<p>This field is immutable.</p>
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
<p>Defines a set of Actions for customizing the behavior of a Component.</p>
<p>Each Action defines a customizable hook or procedure, designed to be invoked at predetermined points
within the lifecycle of a Component instance.</p>
<p>Available Action triggers include:</p>
<ul>
<li><code>postProvision</code>: Defines the hook to be executed after the creation of a Component,
with <code>preCondition</code> specifying when the action should be fired relative to the Component&rsquo;s lifecycle stages:
<code>Immediately</code>, <code>RuntimeReady</code>, <code>ComponentReady</code>, and <code>ClusterReady</code>.</li>
<li><code>preTerminate</code>: Defines the hook to be executed before terminating a Component.</li>
<li><code>roleProbe</code>: Defines the procedure which is invoked regularly to assess the role of replicas.</li>
<li><code>switchover</code>: Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
such as during planned maintenance or upgrades on the current leader node.</li>
<li><code>memberJoin</code>: Defines the procedure to add a new replica to the replication group.</li>
<li><code>memberLeave</code>: Defines the method to remove a replica from the replication group.</li>
<li><code>readOnly</code>: Defines the procedure to switch a replica into the read-only state.</li>
<li><code>readWrite</code>: transition a replica from the read-only state back to the read-write state.</li>
<li><code>dataDump</code>: Defines the procedure to export the data from a replica.</li>
<li><code>dataLoad</code>: Defines the procedure to import data into a replica.</li>
<li><code>reconfigure</code>: Defines the procedure that update a replica with new configuration.</li>
<li><code>accountProvision</code>: Defines the procedure to generate a new database account.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Enumerates external service dependencies for the current Component.</p>
<p>This can include services hosted on other Clusters as well as services deployed outside of the K8s environment.
It is essential for defining cross-service interactions and ensuring that the Component can properly access and
integrate with these specified services.</p>
<p>This field is immutable.</p>
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
<p>Specifies the minimum duration in seconds that a new pod should remain in the ready
state without any of its containers crashing, before the pod is deemed available for use.
This helps ensure that the pod is stable and capable of serving requests without immediate failures.</p>
<p>A default value of 0 means the pod is considered available as soon as it enters the ready state.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVersion">ComponentVersion
</h3>
<div>
<p>ComponentVersion is the Schema for the componentversions API</p>
</div>
<table>
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
<td><code>ComponentVersion</code></td>
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
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionSpec">
ComponentVersionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>compatibilityRules</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionCompatibilityRule">
[]ComponentVersionCompatibilityRule
</a>
</em>
</td>
<td>
<p>CompatibilityRules defines compatibility rules between sets of component definitions and releases.</p>
</td>
</tr>
<tr>
<td>
<code>releases</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionRelease">
[]ComponentVersionRelease
</a>
</em>
</td>
<td>
<p>Releases represents different releases of component instances within this ComponentVersion.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionStatus">
ComponentVersionStatus
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
<code>preConditions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the actions for the operation.
if set, it will check the condition before the component run this operation.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodTemplates</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TargetPodTemplate">
[]TargetPodTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the targetPodTemplate to be referenced by the action.</p>
</td>
</tr>
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
<em>(Optional)</em>
<p>Specifies the types of componentDefinitions supported by the operation.
It can reference certain variables of the componentDefinition.
If set, any component not meeting these conditions will be intercepted.</p>
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
<code>actions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsAction">
[]OpsAction
</a>
</em>
</td>
<td>
<p>The actions to be executed in the opsRequest are performed sequentially.</p>
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
<code>force</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates if pre-checks should be bypassed, allowing the opsRequest to execute immediately. If set to true, pre-checks are skipped except for &lsquo;Start&rsquo; type.
Particularly useful when concurrent execution of VerticalScaling and HorizontalScaling opsRequests is required,
achievable through the use of the Force flag.</p>
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
<p>Specifies the type or nature of the service. Should represent a well-known application cluster type, such as &#123;mysql, redis, mongodb&#125;.
This field is case-insensitive and supports abbreviations for some well-known databases.
For instance, both <code>zk</code> and <code>zookeeper</code> will be recognized as a ZooKeeper cluster, and <code>pg</code>, <code>postgres</code>, <code>postgresql</code> will all be recognized as a PostgreSQL cluster.</p>
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
<p>Represents the version of the service reference.</p>
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
<p>Represents the endpoint of the service connection credential.</p>
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
<p>Represents the authentication details of the service connection credential.</p>
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
<p>Represents the port of the service connection credential.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConsensusMember">ConsensusMember</a>)
</p>
<div>
<p>AccessMode defines the modes of access granted to the SVC.
The modes can be <code>None</code>, <code>Readonly</code>, or <code>ReadWrite</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;None&#34;</p></td>
<td><p>None implies no access.</p>
</td>
</tr><tr><td><p>&#34;ReadWrite&#34;</p></td>
<td><p>ReadWrite permits both read and write operations.</p>
</td>
</tr><tr><td><p>&#34;Readonly&#34;</p></td>
<td><p>Readonly allows only read operations.</p>
</td>
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
<p>Action defines a customizable hook or procedure tailored for different database engines,
designed to be invoked at predetermined points within the lifecycle of a Component instance.
It provides a modular and extensible way to customize a Component&rsquo;s behavior through the execution of defined actions.</p>
<p>Available Action triggers include:</p>
<ul>
<li><code>postProvision</code>: Defines the hook to be executed after the creation of a Component,
with <code>preCondition</code> specifying when the action should be fired relative to the Component&rsquo;s lifecycle stages:
<code>Immediately</code>, <code>RuntimeReady</code>, <code>ComponentReady</code>, and <code>ClusterReady</code>.</li>
<li><code>preTerminate</code>: Defines the hook to be executed before terminating a Component.</li>
<li><code>roleProbe</code>: Defines the procedure which is invoked regularly to assess the role of replicas.</li>
<li><code>switchover</code>: Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
such as during planned maintenance or upgrades on the current leader node.</li>
<li><code>memberJoin</code>: Defines the procedure to add a new replica to the replication group.</li>
<li><code>memberLeave</code>: Defines the method to remove a replica from the replication group.</li>
<li><code>readOnly</code>: Defines the procedure to switch a replica into the read-only state.</li>
<li><code>readWrite</code>: Defines the procedure to transition a replica from the read-only state back to the read-write state.</li>
<li><code>dataDump</code>: Defines the procedure to export the data from a replica.</li>
<li><code>dataLoad</code>: Defines the procedure to import data into a replica.</li>
<li><code>reconfigure</code>: Defines the procedure that update a replica with new configuration.</li>
<li><code>accountProvision</code>: Defines the procedure to generate a new database account.</li>
</ul>
<p>Actions can be executed in different ways:</p>
<ul>
<li>ExecAction: Executes a command inside a container.
which may run as a K8s job or be executed inside the Lorry sidecar container, depending on the implementation.
Future implementations will standardize execution within Lorry.
A set of predefined environment variables are available and can be leveraged within the <code>exec.command</code>
to access context information such as details about pods, components, the overall cluster state,
or database connection credentials.
These variables provide a dynamic and context-aware mechanism for script execution.</li>
<li>HTTPAction: Performs an HTTP request.
HTTPAction is to be implemented in future version.</li>
<li>GRPCAction: In future version, Actions will support initiating gRPC calls.
This allows developers to implement Actions using plugins written in programming language like Go,
providing greater flexibility and extensibility.</li>
</ul>
<p>An action is considered successful on returning 0, or HTTP 200 for status HTTP(s) Actions.
Any other return value or HTTP status codes indicate failure,
and the action may be retried based on the configured retry policy.</p>
<ul>
<li>If an action exceeds the specified timeout duration, it will be terminated, and the action is considered failed.</li>
<li>If an action produces any data as output, it should be written to stdout,
or included in the HTTP response payload for HTTP(s) actions.</li>
<li>If an action encounters any errors, error messages should be written to stderr,
or detailed in the HTTP response with the appropriate non-200 status code.</li>
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
<p>Specifies the container image to be used for running the Action.</p>
<p>When specified, a dedicated container will be created using this image to execute the Action.
This field is mutually exclusive with the <code>container</code> field; only one of them should be provided.</p>
<p>This field cannot be updated.</p>
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
<p>Defines the command to run.</p>
<p>This field cannot be updated.</p>
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
<p>Specifies the HTTP request to perform.</p>
<p>This field cannot be updated.</p>
<p>Note: HTTPAction is to be implemented in future version.</p>
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
<p>Represents a list of environment variables that will be injected into the container.
These variables enable the container to adapt its behavior based on the environment it&rsquo;s running in.</p>
<p>This field cannot be updated.</p>
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
<p>Defines the criteria used to select the target Pod(s) for executing the Action.
This is useful when there is no default target replica identified.
It allows for precise control over which Pod(s) the Action should run in.</p>
<p>This field cannot be updated.</p>
<p>Note: This field is reserved for future use and is not currently active.</p>
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
<p>Used in conjunction with the <code>targetPodSelector</code> field to refine the selection of target pod(s) for Action execution.
The impact of this field depends on the <code>targetPodSelector</code> value:</p>
<ul>
<li>When <code>targetPodSelector</code> is set to <code>Any</code> or <code>All</code>, this field will be ignored.</li>
<li>When <code>targetPodSelector</code> is set to <code>Role</code>, only those replicas whose role matches the <code>matchingKey</code>
will be selected for the Action.</li>
</ul>
<p>This field cannot be updated.</p>
<p>Note: This field is reserved for future use and is not currently active.</p>
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
<p>Defines the name of the container within the target Pod where the action will be executed.</p>
<p>This name must correspond to one of the containers defined in <code>componentDefinition.spec.runtime</code>.
If this field is not specified, the default behavior is to use the first container listed in
<code>componentDefinition.spec.runtime</code>.</p>
<p>This field cannot be updated.</p>
<p>Note: This field is reserved for future use and is not currently active.</p>
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
<p>Specifies the maximum duration in seconds that the Action is allowed to run.</p>
<p>If the Action does not complete within this time frame, it will be terminated.</p>
<p>This field cannot be updated.</p>
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
<p>Defines the strategy to be taken when retrying the Action after a failure.</p>
<p>It specifies the conditions under which the Action should be retried and the limits to apply,
such as the maximum number of retries and backoff strategy.</p>
<p>This field cannot be updated.</p>
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
<p>Specifies the state that the cluster must reach before the Action is executed.
Currently, this is only applicable to the <code>postProvision</code> action.</p>
<p>The conditions are as follows:</p>
<ul>
<li><code>Immediately</code>: Executed right after the Component object is created.
The readiness of the Component and its resources is not guaranteed at this stage.
The Component&rsquo;s state can not be marked as ready until the Action completes successfully.</li>
<li><code>RuntimeReady</code>: The Action is triggered after the Component object has been created and all associated
runtime resources (e.g. Pods) are in a ready state.
The Component&rsquo;s state can not be marked as ready until the Action completes successfully.</li>
<li><code>ComponentReady</code>: The Action is triggered after the Component itself is in a ready state.
This process does not affect the readiness state of the Component or the Cluster.</li>
<li><code>ClusterReady</code>: The Action is executed after the Cluster is in a ready state.
This execution does not alter the Component or the Cluster&rsquo;s state of readiness.</li>
</ul>
<p>This field cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ActionTask">ActionTask
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProgressStatusDetail">ProgressStatusDetail</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the name of the task workload.</p>
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
<p>Defines the namespace where the task workload is deployed.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ActionTaskStatus">
ActionTaskStatus
</a>
</em>
</td>
<td>
<p>Indicates the current status of the task.</p>
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
<p>The name of the target pod for the task.</p>
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
<p>The number of retry attempts for this task.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ActionTaskStatus">ActionTaskStatus
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ActionTask">ActionTask</a>)
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
<p>Specifies the anti-affinity level of Pods within a Component.
It determines how pods should be spread across nodes to improve availability and performance.
It can have the following values: <code>Preferred</code> and <code>Required</code>.
The default value is <code>Preferred</code>.</p>
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
<p>Represents the key of node labels used to define the topology domain for Pod anti-affinity
and Pod spread constraints.</p>
<p>In K8s, a topology domain is a set of nodes that have the same value for a specific label key.
Nodes with labels containing any of the specified TopologyKeys and identical values are considered
to be in the same topology domain.</p>
<p>Note: The concept of topology in the context of K8s TopologyKeys is different from the concept of
topology in the ClusterDefinition.</p>
<p>When a Pod has anti-affinity or spread constraints specified, Kubernetes will attempt to schedule the
Pod on nodes with different values for the specified TopologyKeys.
This ensures that Pods are spread across different topology domains, promoting high availability and
reducing the impact of node failures.</p>
<p>Some well-known label keys, such as <code>kubernetes.io/hostname</code> and <code>topology.kubernetes.io/zone</code>,
are often used as TopologyKey.
These keys represent the hostname and zone of a node, respectively.
By including these keys in the TopologyKeys list, Pods will be spread across nodes with
different hostnames or zones.</p>
<p>In addition to the well-known keys, users can also specify custom label keys as TopologyKeys.
This allows for more flexible and custom topology definitions based on the specific needs
of the application or environment.</p>
<p>The TopologyKeys field is a slice of strings, where each string represents a label key.
The order of the keys in the slice does not matter.</p>
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
<p>Indicates the node labels that must be present on nodes for pods to be scheduled on them.
It is a map where the keys are the label keys and the values are the corresponding label values.
Pods will only be scheduled on nodes that have all the specified labels with the corresponding values.</p>
<p>For example, if NodeLabels is set to &#123;&ldquo;nodeType&rdquo;: &ldquo;ssd&rdquo;, &ldquo;environment&rdquo;: &ldquo;production&rdquo;&#125;,
pods will only be scheduled on nodes that have both the &ldquo;nodeType&rdquo; label with value &ldquo;ssd&rdquo;
and the &ldquo;environment&rdquo; label with value &ldquo;production&rdquo;.</p>
<p>This field allows users to control Pod placement based on specific node labels.
It can be used to ensure that Pods are scheduled on nodes with certain characteristics,
such as specific hardware (e.g., SSD), environment (e.g., production, staging),
or any other custom labels assigned to nodes.</p>
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
<p>Determines the level of resource isolation between Pods.
It can have the following values: <code>SharedNode</code> and <code>DedicatedNode</code>.</p>
<ul>
<li>SharedNode: Allow that multiple Pods may share the same node, which is the default behavior of K8s.</li>
<li>DedicatedNode: Each Pod runs on a dedicated node, ensuring that no two Pods share the same node.
In other words, if a Pod is already running on a node, no other Pods will be scheduled on that node.
Which provides a higher level of isolation and resource guarantee for Pods.</li>
</ul>
<p>The default value is <code>SharedNode</code>.</p>
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
<p>AvailabilityPolicyType defines the type of availability policy to be applied for cluster affinity, influencing how
resources are distributed across zones or nodes for high availability and resilience.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;node&#34;</p></td>
<td><p>AvailabilityPolicyNode specifies that resources should be distributed across different nodes within the same zone.
This policy aims to provide resilience against node failures, ensuring that the failure of a single node does not
impact the overall service availability.</p>
</td>
</tr><tr><td><p>&#34;none&#34;</p></td>
<td><p>AvailabilityPolicyNone specifies that no specific availability policy is applied.
Resources may not be explicitly distributed for high availability, potentially concentrating them in a single
zone or node based on other scheduling decisions.</p>
</td>
</tr><tr><td><p>&#34;zone&#34;</p></td>
<td><p>AvailabilityPolicyZone specifies that resources should be distributed across different availability zones.
This policy aims to ensure high availability and protect against zone failures, spreading the resources to reduce
the risk of simultaneous downtime.</p>
</td>
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
<p>Specifies the name of dataprotection.BackupMethod.</p>
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
<p>If set, specifies the method for selecting the replica to be backed up using the criteria defined here.
If this field is not set, the selection method specified in <code>backupPolicy.target</code> is used.</p>
<p>This field provides a way to override the global <code>backupPolicy.target</code> setting for specific BackupMethod.</p>
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
<p>Specifies a mapping of an environment variable key to the appropriate version of the tool image
required for backups, as determined by ClusterVersion and ComponentDefinition.
The environment variable is then injected into the container executing the backup task.</p>
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
<p>BackupPolicy is the template corresponding to a specified ComponentDefinition
or to a group of ComponentDefinitions that are different versions of definitions of the same component.</p>
</div>
<table>
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
<p>Specifies the name of ClusterComponentDefinition defined in the ClusterDefinition.
Must comply with the IANA Service Naming rule.</p>
<p>Deprecated since v0.9, should use <code>componentDefs</code> instead.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies a list of names of ComponentDefinitions that the specified ClusterDefinition references.
They should be different versions of definitions of the same component,
thus allowing them to share a single BackupPolicy.
Each name must adhere to the IANA Service Naming rule.</p>
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
<p>Defines the selection criteria of instance to be backed up, and the connection credential to be used
during the backup process.</p>
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
<p>Defines the execution plans for backup tasks, specifying when and how backups should occur,
and the retention period of backup files.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.BackupPolicyTemplate">BackupPolicyTemplate
</h3>
<div>
<p>BackupPolicyTemplate should be provided by addon developers and is linked to a ClusterDefinition
and its associated ComponentDefinitions.
It is responsible for generating BackupPolicies for Components that require backup operations,
also determining the suitable backup methods and strategies.
This template is automatically selected based on the specified ClusterDefinition and ComponentDefinitions
when a Cluster is created.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.BackupPolicyTemplateSpec">
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
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of a ClusterDefinition.</p>
<p>This is an immutable attribute that cannot be changed after creation.</p>
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
<p>Represents an array of BackupPolicy templates, with each template corresponding to a specified ComponentDefinition
or to a group of ComponentDefinitions that are different versions of definitions of the same component.</p>
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
<p>Specifies a unique identifier for the BackupPolicyTemplate.</p>
<p>This identifier will be used as the suffix of the name of automatically generated BackupPolicy.
This prevents unintended overwriting of BackupPolicies due to name conflicts when multiple BackupPolicyTemplates
are present.
For instance, using &ldquo;backup-policy&rdquo; for regular backups and &ldquo;backup-policy-hscale&rdquo; for horizontal-scale ops
can differentiate the policies.</p>
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
<p>Populated by the system, it represents the current information about the BackupPolicyTemplate.</p>
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
<code>clusterDefinitionRef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name of a ClusterDefinition.</p>
<p>This is an immutable attribute that cannot be changed after creation.</p>
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
<p>Represents an array of BackupPolicy templates, with each template corresponding to a specified ComponentDefinition
or to a group of ComponentDefinitions that are different versions of definitions of the same component.</p>
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
<p>Specifies a unique identifier for the BackupPolicyTemplate.</p>
<p>This identifier will be used as the suffix of the name of automatically generated BackupPolicy.
This prevents unintended overwriting of BackupPolicies due to name conflicts when multiple BackupPolicyTemplates
are present.
For instance, using &ldquo;backup-policy&rdquo; for regular backups and &ldquo;backup-policy-hscale&rdquo; for horizontal-scale ops
can differentiate the policies.</p>
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
<p>BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate.</p>
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
<p>BuiltinActionHandlerType defines build-in action handlers provided by Lorry, including:</p>
<ul>
<li><code>mysql</code></li>
<li><code>wesql</code></li>
<li><code>oceanbase</code></li>
<li><code>redis</code></li>
<li><code>mongodb</code></li>
<li><code>etcd</code></li>
<li><code>postgresql</code></li>
<li><code>official-postgresql</code></li>
<li><code>apecloud-postgresql</code></li>
<li><code>polardbx</code></li>
<li><code>custom</code></li>
<li><code>unknown</code></li>
</ul>
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
<h3 id="apps.kubeblocks.io/v1alpha1.BuiltinMonitorContainerRef">BuiltinMonitorContainerRef
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
<p>Specifies the name of the built-in metrics exporter container.</p>
</td>
</tr>
<tr>
<td>
<code>PrometheusScrapeConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PrometheusScrapeConfig">
PrometheusScrapeConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>PrometheusScrapeConfig</code> are embedded into this type.)
</p>
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
<p>Specifies whether automated backup is enabled for the Cluster.</p>
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
<p>Determines the duration to retain backups. Backups older than this period are automatically removed.</p>
<p>For example, RetentionPeriod of <code>30d</code> will keep only the backups of last 30 days.
Sample duration format:</p>
<ul>
<li>years: 	2y</li>
<li>months: 	6mo</li>
<li>days: 		30d</li>
<li>hours: 	12h</li>
<li>minutes: 	30m</li>
</ul>
<p>You can also combine the above durations. For example: 30d12h30m.
Default value is 7d.</p>
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
<p>Specifies the backup method to use, as defined in backupPolicy.</p>
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
<p>The cron expression for the schedule. The timezone is in UTC. See <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
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
<p>Specifies the maximum time in minutes that the system will wait to start a missed backup job.
If the scheduled backup time is missed for any reason, the backup job must start within this deadline.
Values must be between 0 (immediate execution) and 1440 (one day).</p>
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
<p>Specifies the name of the backupRepo. If not set, the default backupRepo will be used.</p>
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
<p>Specifies whether to enable point-in-time recovery.</p>
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
<p>ClusterComponentDefinition defines a Component within a ClusterDefinition but is deprecated and
has been replaced by ComponentDefinition.</p>
<p>Deprecated: Use ComponentDefinition instead. This type is deprecated as of version 0.8.</p>
</div>
<table>
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
<p>This name could be used as default name of <code>cluster.spec.componentSpecs.name</code>, and needs to conform with same
validation rules as <code>cluster.spec.componentSpecs.name</code>, currently complying with IANA Service Naming rule.
This name will apply to cluster objects as the value of label &ldquo;apps.kubeblocks.io/component-name&rdquo;.</p>
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
<p>Description of the component definition.</p>
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
<p>Defines the type of the workload.</p>
<ul>
<li><code>Stateless</code> describes stateless applications.</li>
<li><code>Stateful</code> describes common stateful applications.</li>
<li><code>Consensus</code> describes applications based on consensus protocols, such as raft and paxos.</li>
<li><code>Replication</code> describes applications based on the primary-secondary data replication protocol.</li>
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
<p>Defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql).</p>
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
<p>Defines the template of configurations.</p>
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
<p>Defines the template of scripts.</p>
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
<p>Settings for health checks.</p>
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
<p>Specify the logging files which can be observed and configured by cluster users.</p>
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
<p>Defines the pod spec template of component.</p>
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
<p>Defines the service spec.</p>
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
<p>Defines spec for <code>Stateless</code> workloads.</p>
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
<p>Defines spec for <code>Stateful</code> workloads.</p>
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
<p>Defines spec for <code>Consensus</code> workloads. It&rsquo;s required if the workload type is <code>Consensus</code>.</p>
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
<p>Defines spec for <code>Replication</code> workloads.</p>
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
<p>Defines workload spec of this component.
From KB 0.7.0, RSM(InstanceSetSpec) will be the underlying CR which powers all kinds of workload in KB.
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
<p>Defines the behavior of horizontal scale.</p>
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
<p>Defines system accounts needed to manage the component, and the statement to create them.</p>
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
<p>Used to describe the purpose of the volumes mapping the name of the VolumeMounts in the PodSpec.Container field,
such as data volume, log volume, etc. When backing up the volume, the volume can be correctly backed up according
to the volumeType.</p>
<p>For example:</p>
<ul>
<li><code>name: data, type: data</code> means that the volume named <code>data</code> is used to store <code>data</code>.</li>
<li><code>name: binlog, type: log</code> means that the volume named <code>binlog</code> is used to store <code>log</code>.</li>
</ul>
<p>NOTE: When volumeTypes is not defined, the backup function will not be supported, even if a persistent volume has
been specified.</p>
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
<p>Used for custom label tags which you want to add to the component resources.</p>
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
<p>Defines command to do switchover.
In particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under
the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.</p>
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
<p>Defines the command to be executed when the component is ready, and the command will only be executed once after
the component becomes ready.</p>
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
<p>Defines settings to do volume protect.</p>
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
<p>Used to inject values from other components into the current component. Values will be saved and updated in a
configmap and mounted to the current component.</p>
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
<p>Used to declare the service reference of the current component.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainerSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSpec">
[]SidecarContainerSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>builtinMonitorContainer</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BuiltinMonitorContainerRef">
BuiltinMonitorContainerRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the built-in metrics exporter container.</p>
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
<p>ClusterComponentPhase defines the phase of a cluster component as represented in cluster.status.components.phase field.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Abnormal&#34;</p></td>
<td><p>AbnormalClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
The component is functioning, but it is in a fragile state.</p>
</td>
</tr><tr><td><p>&#34;Creating&#34;</p></td>
<td><p>CreatingClusterCompPhase indicates the component is being created.</p>
</td>
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td><p>DeletingClusterCompPhase indicates the component is currently being deleted.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>FailedClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
The component is not functioning.</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>RunningClusterCompPhase indicates the component has more than zero replicas, and all pods are up-to-date and
in a &lsquo;Running&rsquo; state.</p>
</td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td><p>StoppedClusterCompPhase indicates the component has zero replicas, and all pods have been deleted.</p>
</td>
</tr><tr><td><p>&#34;Stopping&#34;</p></td>
<td><p>StoppingClusterCompPhase indicates the component has zero replicas, and there are pods that are terminating.</p>
</td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
<td><p>UpdatingClusterCompPhase indicates the component has more than zero replicas, and there are no failed pods,
it is currently being updated.</p>
</td>
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
<p>References the component service name defined in the ComponentDefinition.Spec.Services[x].Name.</p>
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
<p>Determines how the Service is exposed. Valid options are <code>ClusterIP</code>, <code>NodePort</code>, and <code>LoadBalancer</code>.</p>
<ul>
<li><code>ClusterIP</code> allocates a cluster-internal IP address for load-balancing to endpoints. Endpoints are determined
by the selector or if that is not specified, they are determined by manual construction of an Endpoints object
or EndpointSlice objects. If clusterIP is &ldquo;None&rdquo;, no virtual IP is allocated and the endpoints are published
as a set of endpoints rather than a virtual IP.</li>
<li><code>NodePort</code> builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.</li>
<li><code>LoadBalancer</code> builds on NodePort and creates an external load-balancer (if supported in the current cloud)
which routes to the same endpoints as the clusterIP.</li>
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
<p>If ServiceType is LoadBalancer, cloud provider related parameters can be put here.
More info: <a href="https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer">https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer</a>.</p>
</td>
</tr>
<tr>
<td>
<code>podService</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether to generate individual services for each pod.
If set to true, a separate service will be created for each pod in the cluster.</p>
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
<p>ClusterComponentSpec defines the specifications for a Component in a Cluster.</p>
</div>
<table>
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
<p>Specifies the name of the Component.
This name is also part of the Service DNS name and must comply with the IANA service naming rule.
When ClusterComponentSpec is referenced as a template, the name is optional. Otherwise, it is required.</p>
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
<p>References a ClusterComponentDefinition defined in the <code>clusterDefinition.spec.componentDef</code> field.
Must comply with the IANA service naming rule.</p>
<p>Deprecated since v0.9,
because defining components in <code>clusterDefinition.spec.componentDef</code> field has been deprecated.
This field is replaced by the <code>componentDef</code> field, use <code>componentDef</code> instead.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>References the name of a ComponentDefinition.
The ComponentDefinition specifies the behavior and characteristics of the Component.
If both <code>componentDefRef</code> and <code>componentDef</code> are provided,
the <code>componentDef</code> will take precedence over <code>componentDefRef</code>.</p>
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
<p>ServiceVersion specifies the version of the service expected to be provisioned by this component.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).
If no version is specified, the latest available version will be used.</p>
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
<p>Defines a list of ServiceRef for a Component, allowing it to connect and interact with other services.
These services can be external or managed by the same KubeBlocks operator, categorized as follows:</p>
<ol>
<li><p>External Services:</p>
<ul>
<li>Not managed by KubeBlocks. These could be services outside KubeBlocks or non-Kubernetes services.</li>
<li>Connection requires a ServiceDescriptor providing details for service binding.</li>
</ul></li>
<li><p>KubeBlocks Services:</p>
<ul>
<li>Managed within the same KubeBlocks environment.</li>
<li>Service binding is achieved by specifying cluster names in the service references,
with configurations handled by the KubeBlocks operator.</li>
</ul></li>
</ol>
<p>ServiceRef maintains cluster-level semantic consistency; references with the same <code>serviceRef.name</code>
within the same cluster are treated as identical.
Only bindings to the same cluster or ServiceDescriptor are allowed within a cluster.</p>
<p>Example:</p>
<pre><code class="language-yaml">serviceRefs:
- name: &quot;redis-sentinel&quot;
serviceDescriptor:
name: &quot;external-redis-sentinel&quot;
- name: &quot;postgres-cluster&quot;
cluster:
name: &quot;my-postgres-cluster&quot;
</code></pre>
<p>The example above includes references to an external Redis Sentinel service and a PostgreSQL cluster managed by KubeBlocks.</p>
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
<p>Specifies which types of logs should be collected for the Cluster.
The log types are defined in the <code>componentDefinition.spec.logConfigs</code> field with the LogConfig entries.</p>
<p>The elements in the <code>enabledLogs</code> array correspond to the names of the LogConfig entries.
For example, if the <code>componentDefinition.spec.logConfigs</code> defines LogConfig entries with
names &ldquo;slow_query_log&rdquo; and &ldquo;error_log&rdquo;,
you can enable the collection of these logs by including their names in the <code>enabledLogs</code> array:
enabledLogs: [&ldquo;slow_query_log&rdquo;, &ldquo;error_log&rdquo;]</p>
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
<p>Each component supports running multiple replicas to provide high availability and persistence.
This field can be used to specify the desired number of replicas.</p>
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
<p>Specifies a group of affinity scheduling rules for the Component.
It allows users to control how the Component&rsquo;s Pods are scheduled onto nodes in the cluster.</p>
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
<p>Allows the Component to be scheduled onto nodes with matching taints.
It is an array of tolerations that are attached to the Component&rsquo;s Pods.</p>
<p>Each toleration consists of a <code>key</code>, <code>value</code>, <code>effect</code>, and <code>operator</code>.
The <code>key</code>, <code>value</code>, and <code>effect</code> define the taint that the toleration matches.
The <code>operator</code> specifies how the toleration matches the taint.</p>
<p>If a node has a taint that matches a toleration, the Component&rsquo;s pods can be scheduled onto that node.
This allows the Component&rsquo;s Pods to run on nodes that have been tainted to prevent regular Pods from being scheduled.</p>
</td>
</tr>
<tr>
<td>
<code>schedulingPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulingPolicy">
SchedulingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling policy for the component.</p>
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
<p>Specifies the resources required by the Component.
It allows defining the CPU, memory requirements and limits for the Component&rsquo;s containers.</p>
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
<p>Specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component.
Each template specifies the desired characteristics of a persistent volume, such as storage class,
size, and access modes.
These templates are used to dynamically provision persistent volumes for the Component when it is deployed.</p>
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
<p>Services overrides services defined in referenced ComponentDefinition and expose endpoints that can be accessed
by clients.</p>
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
<p>Defines the strategy for switchover and failover when workloadType is Replication.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>A boolean flag that indicates whether the component should use Transport Layer Security (TLS)
for secure communication.
When set to true, the component will be configured to use TLS encryption for its network connections.
This ensures that the data transmitted between the component and its clients or other components is encrypted
and protected from unauthorized access.
If TLS is enabled, the component may require additional configuration, such as specifying TLS certificates and keys,
to properly set up the secure communication channel.</p>
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
<p>Specifies the configuration for the TLS certificates issuer.
It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
Required when TLS is enabled.</p>
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
<p>Specifies the name of the ServiceAccount required by the running Component.
This ServiceAccount is used to grant necessary permissions for the Component&rsquo;s Pods to interact
with other Kubernetes resources, such as modifying pod labels or sending events.</p>
<p>Defaults:
If not specified, KubeBlocks automatically assigns a default ServiceAccount named &ldquo;kb-&#123;cluster.name&#125;&rdquo;,
bound to a default role defined during KubeBlocks installation.</p>
<p>Future Changes:
Future versions might change the default ServiceAccount creation strategy to one per Component,
potentially revising the naming to &ldquo;kb-&#123;cluster.name&#125;-&#123;component.name&#125;&rdquo;.</p>
<p>Users can override the automatic ServiceAccount assignment by explicitly setting the name of
an existed ServiceAccount in this field.</p>
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
<p>Defines the update strategy for the component.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Allows users to specify custom ConfigMaps and Secrets to be mounted as volumes
in the Cluster&rsquo;s Pods.
This is useful in scenarios where users need to provide additional resources to the Cluster, such as:</p>
<ul>
<li>Mounting custom scripts or configuration files during Cluster startup.</li>
<li>Mounting Secrets as volumes to provide sensitive information, like S3 AK/SK, to the Cluster.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allows for the customization of configuration values for each instance within a component.
An Instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
While instances typically share a common configuration as defined in the ClusterComponentSpec,
they can require unique settings in various scenarios:</p>
<p>For example:
- A database component might require different resource allocations for primary and secondary instances,
with primaries needing more resources.
- During a rolling upgrade, a component may first update the image for one or a few instances,
and then update the remaining instances after verifying that the updated instances are functioning correctly.</p>
<p>InstanceTemplate allows for specifying these unique configurations per instance.
Each instance&rsquo;s name is constructed using the pattern: $(component.name)-$(template.name)-$(ordinal),
starting with an ordinal of 0.
It is crucial to maintain unique names for each InstanceTemplate to avoid conflicts.</p>
<p>The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the Component.
Any remaining replicas will be generated using the default template and will follow the default naming rules.</p>
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
<p>Specifies the names of instances to be transitioned to offline status.</p>
<p>Marking an instance as offline results in the following:</p>
<ol>
<li>The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
future reuse or data recovery, but it is no longer actively used.</li>
<li>The ordinal number assigned to this instance is preserved, ensuring it remains unique
and avoiding conflicts with new instances.</li>
</ol>
<p>Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
ordinal consistency within the cluster.
Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>monitorEnabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines whether the metrics exporter needs to be published to the service endpoint.
If set to true, the metrics exporter will be published to the service endpoint,
the service will be injected with the following annotations:
- &ldquo;monitor.kubeblocks.io/path&rdquo;
- &ldquo;monitor.kubeblocks.io/port&rdquo;
- &ldquo;monitor.kubeblocks.io/scheme&rdquo;</p>
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
<p>ClusterComponentStatus records Component status.</p>
</div>
<table>
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
<p>Specifies the current state of the Component.</p>
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
<p>Records detailed information about the Component in its current phase.
The keys are either podName, deployName, or statefulSetName, formatted as &lsquo;ObjectKind/Name&rsquo;.</p>
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
<p>Checks if all pods of the Component are ready.</p>
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
<p>Indicates the time when all Component Pods became ready.
This is the readiness time of the last Component Pod.</p>
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
<p>Represents the status of the members.</p>
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
<p>Deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Specifies a reference to one of the cluster component definition names in the ClusterDefinition API (spec.componentDefs.name).</p>
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
<p>Defines a configuration extension mechanism to handle configuration differences between versions.
The configTemplateRefs field, in conjunction with the configTemplateRefs in the ClusterDefinition, determines
the final configuration file.</p>
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
<p>Defines the image for the component to connect to databases or engines.
This overrides the <code>image</code> and <code>env</code> attributes defined in clusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.
To clear default environment settings, set systemAccountSpec.cmdExecutorConfig.env to an empty list.</p>
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
<p>Defines the context for container images for component versions.
This value replaces the values in clusterDefinition.spec.componentDefs.podSpec.[initContainers | containers].</p>
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
<p>Defines the images for the component to perform a switchover.
This overrides the image and env attributes defined in clusterDefinition.spec.componentDefs.SwitchoverSpec.CommandExecutorEnvItem.</p>
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
<p>Refers to the name of a volumeMount defined in either:</p>
<ul>
<li><code>componentDefinition.spec.runtime.containers[*].volumeMounts</code></li>
<li><code>clusterDefinition.spec.componentDefs[*].podSpec.containers[*].volumeMounts</code> (deprecated)</li>
</ul>
<p>The value of <code>name</code> must match the <code>name</code> field of a volumeMount specified in the corresponding <code>volumeMounts</code> array.</p>
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
<p>Defines the desired characteristics of a PersistentVolumeClaim that will be created for the volume
with the mount name specified in the <code>name</code> field.</p>
<p>When a Pod is created for this ClusterComponent, a new PVC will be created based on the specification
defined in the <code>spec</code> field. The PVC will be associated with the volume mount specified by the <code>name</code> field.</p>
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
<p>Contains the desired access modes the volume should have.
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
<p>Represents the minimum resources the volume should have.
If the RecoverVolumeExpansionFailure feature is enabled, users are allowed to specify resource requirements that
are lower than the previous value but must still be higher than the capacity recorded in the status field of the claim.
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
<p>The name of the StorageClass required by the claim.
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
<p>Defines what type of volume is required by the claim, either Block or Filesystem.</p>
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
<p>ClusterDefinitionProbe is deprecated since v0.8.</p>
</div>
<table>
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
<p>Commands used to execute for probe.</p>
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
<p>ClusterDefinitionProbeCMDs is deprecated since v0.8.</p>
</div>
<table>
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
<p>Defines write checks that are executed on the probe sidecar.</p>
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
<p>Defines read checks that are executed on the probe sidecar.</p>
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
<p>ClusterDefinitionProbes is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the probe used for checking the running status of the component.</p>
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
<p>Specifies the probe used for checking the status of the component.</p>
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
<p>Specifies the probe used for checking the role of the component.</p>
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
<p>Defines the timeout (in seconds) for the role probe after all pods of the component are ready.
The system will check if the application is available in the pod.
If pods exceed the InitializationTimeoutSeconds time without a role label, this component will enter the
Failed/Abnormal phase.</p>
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
<p>ClusterDefinitionSpec defines the desired state of ClusterDefinition.</p>
</div>
<table>
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
<p>Specifies the well-known database type, such as mysql, redis, or mongodb.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<em>(Optional)</em>
<p>Provides the definitions for the cluster components.</p>
<p>Deprecated since v0.9.
Components should now be individually defined using ComponentDefinition and
collectively referenced via <code>topology.components</code>.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Connection credential template used for creating a connection credential secret for cluster objects.</p>
<p>Built-in objects are:</p>
<ul>
<li><code>$(RANDOM_PASSWD)</code> random 8 characters.</li>
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
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>topologies</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTopology">
[]ClusterTopology
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Topologies defines all possible topologies within the cluster.</p>
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
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the most recent generation observed for this ClusterDefinition.</p>
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
<p>Specifies the current phase of the ClusterDefinition. Valid values are <code>empty</code>, <code>Available</code>, <code>Unavailable</code>.
When <code>Available</code>, the ClusterDefinition is ready and can be referenced by related objects.</p>
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
<tr>
<td>
<code>topologies</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Topologies this ClusterDefinition supported.</p>
</td>
</tr>
<tr>
<td>
<code>serviceRefs</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The service references declared by this ClusterDefinition.</p>
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
<p>ClusterNetwork is deprecated since v0.9.</p>
</div>
<table>
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
<p>Indicates whether the host network can be accessed. By default, this is set to false.</p>
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
<p>Indicates whether the network is accessible to the public. By default, this is set to false.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterObjectReference">ClusterObjectReference
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CredentialVarSelector">CredentialVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.PodVarSelector">PodVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceRefVarSelector">ServiceRefVarSelector</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceVarSelector">ServiceVarSelector</a>)
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
<p>ClusterPhase defines the phase of the Cluster within the .status.phase field.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Abnormal&#34;</p></td>
<td><p>AbnormalClusterPhase represents some components are in <code>Failed</code> or <code>Abnormal</code> phase, indicates that the cluster
is in a fragile state and troubleshooting is required.</p>
</td>
</tr><tr><td><p>&#34;Creating&#34;</p></td>
<td><p>CreatingClusterPhase represents all components are in <code>Creating</code> phase.</p>
</td>
</tr><tr><td><p>&#34;Deleting&#34;</p></td>
<td><p>DeletingClusterPhase indicates the cluster is being deleted.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>FailedClusterPhase represents all components are in <code>Failed</code> phase, indicates that the cluster is unavailable.</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>RunningClusterPhase represents all components are in <code>Running</code> phase, indicates that the cluster is functioning properly.</p>
</td>
</tr><tr><td><p>&#34;Stopped&#34;</p></td>
<td><p>StoppedClusterPhase represents all components are in <code>Stopped</code> phase, indicates that the cluster has stopped and
is not providing any functionality.</p>
</td>
</tr><tr><td><p>&#34;Stopping&#34;</p></td>
<td><p>StoppingClusterPhase represents at least one component is in <code>Stopping</code> phase, indicates that the cluster is in
the process of stopping.</p>
</td>
</tr><tr><td><p>&#34;Updating&#34;</p></td>
<td><p>UpdatingClusterPhase represents all components are in <code>Creating</code>, <code>Running</code> or <code>Updating</code> phase, and at least one
component is in <code>Creating</code> or <code>Updating</code> phase, indicates that the cluster is undergoing an update.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterResources">ClusterResources
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>ClusterResources is deprecated since v0.9.</p>
</div>
<table>
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
<p>Specifies the amount of processing power the cluster needs.
For more information, refer to: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>Specifies the amount of memory the cluster needs.
For more information, refer to: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>ClusterService defines a service that is exposed externally, allowing entities outside the cluster to access it.
For example, external applications, or other Clusters.
And another Cluster managed by the same KubeBlocks operator can resolve the address exposed by a ClusterService
using the <code>serviceRef</code> field.</p>
<p>When a Component needs to access another Cluster&rsquo;s ClusterService using the <code>serviceRef</code> field,
it must also define the service type and version information in the <code>componentDefinition.spec.serviceRefDeclarations</code>
section.</p>
</div>
<table>
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
<p>Extends the ServiceSpec.Selector by allowing the specification of a sharding name, which is defined in
<code>cluster.spec.shardingSpecs[*].name</code>, to be used as a selector for the service.
Note that this and the <code>componentSelector</code> are mutually exclusive and cannot be set simultaneously.</p>
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
<p>Extends the ServiceSpec.Selector by allowing the specification of a component, to be used as a selector for the service.
Note that this and the <code>shardingSelector</code> are mutually exclusive and cannot be set simultaneously.</p>
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
<p>Specifies the name of the ClusterDefinition to use when creating a Cluster.</p>
<p>This field enables users to create a Cluster using a specific ClusterDefinition and topology.
The specified ClusterDefinition determines the components to be used based on its defined topology,
allowing for the utilization of predefined configurations and behaviors.</p>
<p>Advanced users may opt for more precise control by directly referencing specific
ComponentDefinitions for each component within <code>componentSpecs[*].componentDef</code>.
This method offers granular control over the composition of the Cluster.</p>
<p>If this field is not provided, each component must be explicitly defined in <code>componentSpecs[*].componentDef</code>.</p>
<p>Note: Once set, this field cannot be modified; it is immutable.</p>
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
<p>Refers to the ClusterVersion name.</p>
<p>Deprecated since v0.9, use ComponentVersion instead.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>topology</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the ClusterTopology to be used when creating the Cluster.</p>
<p>This field defines which set of components, as outlined in the ClusterDefinition, will be used to
construct the Cluster based on the named topology.
The ClusterDefinition may list multiple topologies under <code>clusterdefinition.spec.topologies[*]</code>,
each tailored to different use cases or environments.</p>
<p>If <code>topology</code> is not specified, the Cluster will default to the topology defined in the ClusterDefinition.</p>
<p>Note: Once set during the Cluster creation, the Topology field cannot be modified.
It establishes the initial composition and structure of the Cluster and is intended for one-time configuration.</p>
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
<p>Specifies the behavior when a Cluster is deleted.
It defines how resources, data, and backups associated with a Cluster are managed during termination.
Choose a policy based on the desired level of resource cleanup and data preservation:</p>
<ul>
<li><code>DoNotTerminate</code>: Prevents deletion of the Cluster. This policy ensures that all resources remain intact.</li>
<li><code>Halt</code>: Deletes Cluster resources like Pods and Services but retains Persistent Volume Claims (PVCs),
allowing for data preservation while stopping other operations.</li>
<li><code>Delete</code>: Extends the <code>Halt</code> policy by also removing PVCs, leading to a thorough cleanup while
removing all persistent data.</li>
<li><code>WipeOut</code>: An aggressive policy that deletes all Cluster resources, including volume snapshots and
backups in external storage.
This results in complete data removal and should be used cautiously, primarily in non-production environments
to avoid irreversible data loss.</li>
</ul>
<p>Warning: Choosing an inappropriate termination policy can result in significant data loss.
The <code>WipeOut</code> policy is particularly risky in production environments due to its irreversible nature.</p>
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
<p>Specifies a list of ShardingSpec objects that configure the sharding topology for components of a Cluster.
Each ShardingSpec corresponds to a group of components organized into shards,
with each shard containing multiple replicas.
Components within a shard are based on a common ClusterComponentSpec template,
ensuring that all components in a shard have identical configurations as per the template.</p>
<p>This field supports dynamic scaling by facilitating the addition or removal of shards based on
the specified number in each ShardingSpec.</p>
<p>Note: <code>shardingSpecs</code> and <code>componentSpecs</code> cannot both be empty; at least one must be defined to configure a cluster.</p>
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
<p>Specifies a list of ClusterComponentSpec objects used to define the individual components that make up a Cluster.
This field allows for detailed configuration of each component within the Cluster.</p>
<p>Note: <code>shardingSpecs</code> and <code>componentSpecs</code> cannot both be empty; at least one must be defined to configure a cluster.</p>
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
<p>Defines the list of services that are exposed by a Cluster.
This field allows selected components, either from <code>componentSpecs</code> or <code>shardingSpecs</code>,
to be exposed as cluster-level services.</p>
<p>Services defined here can be referenced by other clusters using the ServiceRefClusterSelector.</p>
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
<p>Defines a set of node affinity scheduling rules for the Cluster&rsquo;s Pods.
This field helps control the placement of Pods on nodes within the cluster.</p>
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
<p>An array that specifies tolerations attached to the Cluster&rsquo;s Pods,
allowing them to be scheduled onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>schedulingPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulingPolicy">
SchedulingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling policy for the cluster.</p>
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
<p>Specifies the backup configuration of the Cluster.</p>
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
<p>Describes how pods are distributed across node.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Describes the availability policy, including zone, node, and none.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified,
this value will be ignored.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>The configuration of network.</p>
<p>Deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines RuntimeClassName for all Pods managed by this cluster.</p>
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
<p>The most recent generation number that has been observed by the controller.</p>
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
<p>The current phase of the Cluster includes:
<code>Creating</code>, <code>Running</code>, <code>Updating</code>, <code>Stopping</code>, <code>Stopped</code>, <code>Deleting</code>, <code>Failed</code>, <code>Abnormal</code>.</p>
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
<p>Records the current status information of all components within the Cluster.</p>
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
<p>Represents the generation number of the referenced ClusterDefinition.</p>
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
<p>Describes the current state of the cluster API resource, such as warnings.</p>
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
<p>ClusterStorage is deprecated since v0.9.</p>
</div>
<table>
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
<p>Specifies the amount of storage the cluster needs.
For more information, refer to: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>ClusterSwitchPolicy defines the switch policy for a cluster.</p>
<p>Deprecated since v0.9.</p>
</div>
<table>
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
<p>Type specifies the type of switch policy to be applied.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTopology">ClusterTopology
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionSpec">ClusterDefinitionSpec</a>)
</p>
<div>
<p>ClusterTopology represents the definition for a specific cluster topology.</p>
</div>
<table>
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
<p>Name is the unique identifier for the cluster topology.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTopologyComponent">
[]ClusterTopologyComponent
</a>
</em>
</td>
<td>
<p>Components specifies the components in the topology.</p>
</td>
</tr>
<tr>
<td>
<code>orders</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ClusterTopologyOrders">
ClusterTopologyOrders
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the sequence in which components within a cluster topology are
started, stopped, and upgraded.
This ordering is crucial for maintaining the correct dependencies and operational flow across components.</p>
</td>
</tr>
<tr>
<td>
<code>default</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default indicates whether this topology serves as the default configuration.
When set to true, this topology is automatically used unless another is explicitly specified.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTopologyComponent">ClusterTopologyComponent
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterTopology">ClusterTopology</a>)
</p>
<div>
<p>ClusterTopologyComponent defines a Component within a ClusterTopology.</p>
</div>
<table>
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
<p>Defines the unique identifier of the component within the cluster topology.
It follows IANA Service naming rules and is used as part of the Service&rsquo;s DNS name.
The name must start with a lowercase letter, can contain lowercase letters, numbers,
and hyphens, and must end with a lowercase letter or number.</p>
<p>Cannot be updated once set.</p>
</td>
</tr>
<tr>
<td>
<code>compDef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the name or prefix of the ComponentDefinition custom resource(CR) that
defines the Component&rsquo;s characteristics and behavior.</p>
<p>When a prefix is used, the system selects the ComponentDefinition CR with the latest version that matches the prefix.
This approach allows:</p>
<ol>
<li>Precise selection by providing the exact name of a ComponentDefinition CR.</li>
<li>Flexible and automatic selection of the most up-to-date ComponentDefinition CR by specifying a prefix.</li>
</ol>
<p>Once set, this field cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ClusterTopologyOrders">ClusterTopologyOrders
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterTopology">ClusterTopology</a>)
</p>
<div>
<p>ClusterTopologyOrders manages the lifecycle of components within a cluster by defining their provisioning,
terminating, and updating sequences.
It organizes components into stages or groups, where each group indicates a set of components
that can be managed concurrently.
These groups are processed sequentially, allowing precise control based on component dependencies and requirements.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>provision</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the order for creating and initializing components.
This is designed for components that depend on one another. Components without dependencies can be grouped together.</p>
<p>Components that can be provisioned independently or have no dependencies can be listed together in the same stage,
separated by commas.</p>
</td>
</tr>
<tr>
<td>
<code>terminate</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Outlines the order for stopping and deleting components.
This sequence is designed for components that require a graceful shutdown or have interdependencies.</p>
<p>Components that can be terminated independently or have no dependencies can be listed together in the same stage,
separated by commas.</p>
</td>
</tr>
<tr>
<td>
<code>update</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Update determines the order for updating components&rsquo; specifications, such as image upgrades or resource scaling.
This sequence is designed for components that have dependencies or require specific update procedures.</p>
<p>Components that can be updated independently or have no dependencies can be listed together in the same stage,
separated by commas.</p>
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
<p>ClusterVersionSpec defines the desired state of ClusterVersion.</p>
<p>Deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Specifies a reference to the ClusterDefinition.</p>
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
<p>Contains a list of versioning contexts for the components&rsquo; containers.</p>
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
<p>ClusterVersionStatus defines the observed state of ClusterVersion.</p>
<p>Deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>The current phase of the ClusterVersion.</p>
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
<tr>
<td>
<code>observedGeneration</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>The generation number that has been observed by the controller.</p>
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
<p>The generation number of the ClusterDefinition that is currently being referenced.</p>
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
<p>Deprecated since v0.8.</p>
</div>
<table>
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
<p>CommandExecutorEnvItem is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the image used to execute the command.</p>
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
<p>A list of environment variables that will be injected into the command execution context.</p>
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
<p>CommandExecutorItem is deprecated since v0.8.</p>
</div>
<table>
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
<p>The command to be executed.</p>
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
<p>Additional parameters used in the execution of the command.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CompletionProbe">CompletionProbe
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
</p>
<div>
</div>
<table>
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
<p>Defines the number of seconds after which the probe times out.
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
<p>Indicates the frequency (in seconds) at which the probe should be performed.
The default value is 5 seconds, with a minimum value of 1.</p>
</td>
</tr>
<tr>
<td>
<code>matchExpressions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MatchExpressions">
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
<p>Specifies the configuration files within the ConfigMap that support dynamic updates.</p>
<p>A configuration template (provided in the form of a ConfigMap) may contain templates for multiple
configuration files.
Each configuration file corresponds to a key in the ConfigMap.
Some of these configuration files may support dynamic modification and reloading without requiring
a pod restart.</p>
<p>If empty or omitted, all configuration files in the ConfigMap are assumed to support dynamic updates,
and ConfigConstraint applies to all keys.</p>
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
<p>Specifies the secondary rendered config spec for pod-specific customization.</p>
<p>The template is rendered inside the pod (by the &ldquo;config-manager&rdquo; sidecar container) and merged with the main
template&rsquo;s render result to generate the final configuration file.</p>
<p>This field is intended to handle scenarios where different pods within the same Component have
varying configurations. It allows for pod-specific customization of the configuration.</p>
<p>Note: This field will be deprecated in future versions, and the functionality will be moved to
<code>cluster.spec.componentSpecs[*].instances[*]</code>.</p>
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
<p>Specifies the name of the referenced configuration constraints object.</p>
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
<p>Deprecated: AsEnvFrom has been deprecated since 0.9.0 and will be removed in 0.10.0
Specifies the containers to inject the ConfigMap parameters as environment variables.</p>
<p>This is useful when application images accept parameters through environment variables and
generate the final configuration file in the startup script based on these variables.</p>
<p>This field allows users to specify a list of container names, and KubeBlocks will inject the environment
variables converted from the ConfigMap into these designated containers. This provides a flexible way to
pass the configuration items from the ConfigMap to the container without modifying the image.</p>
<p>Note: The field name <code>asEnvFrom</code> may be changed to <code>injectEnvTo</code> in future versions for better clarity.</p>
</td>
</tr>
<tr>
<td>
<code>injectEnvTo</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the containers to inject the ConfigMap parameters as environment variables.</p>
<p>This is useful when application images accept parameters through environment variables and
generate the final configuration file in the startup script based on these variables.</p>
<p>This field allows users to specify a list of container names, and KubeBlocks will inject the environment
variables converted from the ConfigMap into these designated containers. This provides a flexible way to
pass the configuration items from the ConfigMap to the container without modifying the image.</p>
</td>
</tr>
<tr>
<td>
<code>reRenderResourceTypes</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.RerenderResourceType">
[]RerenderResourceType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether the configuration needs to be re-rendered after v-scale or h-scale operations to reflect changes.</p>
<p>In some scenarios, the configuration may need to be updated to reflect the changes in resource allocation
or cluster topology. Examples:</p>
<ul>
<li>Redis: adjust maxmemory after v-scale operation.</li>
<li>MySQL: increase max connections after v-scale operation.</li>
<li>Zookeeper: update zoo.cfg with new node addresses after h-scale operation.</li>
</ul>
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
<p>Deprecated since v0.8.</p>
</div>
<table>
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
<p>The name of the componentDef to be selected.</p>
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
<p>Defines the policy to be followed in case of a failure in finding the component.</p>
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
<p>The values that are to be injected as environment variables into each component.</p>
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
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinition">ComponentDefinition</a>)
</p>
<div>
<p>ComponentDefinitionSpec defines the desired state of ComponentDefinition.</p>
</div>
<table>
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
<p>Specifies the name of the component provider, typically the vendor or developer name.
It identifies the entity responsible for creating and maintaining the component.</p>
<p>When specifying the provider name, consider the following guidelines:</p>
<ul>
<li>Keep the name concise and relevant to the component.</li>
<li>Use a consistent naming convention across components from the same provider.</li>
<li>Avoid using trademarked or copyrighted names without proper permission.</li>
</ul>
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
<p>Provides a brief and concise explanation of the component&rsquo;s purpose, functionality, and any relevant details.
It serves as a quick reference for users to understand the component&rsquo;s role and characteristics.</p>
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
<p>Defines the type of well-known service protocol that the component provides.
It specifies the standard or widely recognized protocol used by the component to offer its services.</p>
<p>The <code>serviceKind</code> field allows users to quickly identify the type of service provided by the component
based on common protocols or service types. This information helps in understanding the compatibility,
interoperability, and usage of the component within a system.</p>
<p>Some examples of well-known service protocols include:</p>
<ul>
<li>&ldquo;MySQL&rdquo;: Indicates that the component provides a MySQL database service.</li>
<li>&ldquo;PostgreSQL&rdquo;: Indicates that the component offers a PostgreSQL database service.</li>
<li>&ldquo;Redis&rdquo;: Signifies that the component functions as a Redis key-value store.</li>
<li>&ldquo;ETCD&rdquo;: Denotes that the component serves as an ETCD distributed key-value store.</li>
</ul>
<p>The <code>serviceKind</code> value is case-insensitive, allowing for flexibility in specifying the protocol name.</p>
<p>When specifying the <code>serviceKind</code>, consider the following guidelines:</p>
<ul>
<li>Use well-established and widely recognized protocol names or service types.</li>
<li>Ensure that the <code>serviceKind</code> accurately represents the primary service offered by the component.</li>
<li>If the component provides multiple services, choose the most prominent or commonly used protocol.</li>
<li>Limit the <code>serviceKind</code> to a maximum of 32 characters for conciseness and readability.</li>
</ul>
<p>Note: The <code>serviceKind</code> field is optional and can be left empty if the component does not fit into a well-known
service category or if the protocol is not widely recognized. It is primarily used to convey information about
the component&rsquo;s service type to users and facilitate discovery and integration.</p>
<p>The <code>serviceKind</code> field is immutable and cannot be updated.</p>
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
<p>Specifies the version of the service provided by the component.
It follows the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).</p>
<p>The Semantic Versioning specification defines a version number format of X.Y.Z (MAJOR.MINOR.PATCH), where:</p>
<ul>
<li>X represents the major version and indicates incompatible API changes.</li>
<li>Y represents the minor version and indicates added functionality in a backward-compatible manner.</li>
<li>Z represents the patch version and indicates backward-compatible bug fixes.</li>
</ul>
<p>Additional labels for pre-release and build metadata are available as extensions to the X.Y.Z format:</p>
<ul>
<li>Use pre-release labels (e.g., -alpha, -beta) for versions that are not yet stable or ready for production use.</li>
<li>Use build metadata (e.g., +build.1) for additional version information if needed.</li>
</ul>
<p>Examples of valid ServiceVersion values:</p>
<ul>
<li>&ldquo;1.0.0&rdquo;</li>
<li>&ldquo;2.3.1&rdquo;</li>
<li>&ldquo;3.0.0-alpha.1&rdquo;</li>
<li>&ldquo;4.5.2+build.1&rdquo;</li>
</ul>
<p>The <code>serviceVersion</code> field is immutable and cannot be updated.</p>
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
<p>Defines the component&rsquo;s container configuration that serves as a template for instantiated Components.
It includes the following elements:</p>
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
<p>The runtime field is intended to define information that remains consistent across all instantiated components
and serves as a template for their configuration.
Dynamic settings such as CPU and memory resource limits, as well as scheduling settings (affinity,
toleration, priority), may vary for each instantiated component and are specified separately in the
<code>cluster.spec.componentSpecs</code> field.</p>
<p>However, there can be exceptions. In some cases, individual component instances may need to override
certain aspects of the runtime configuration to customize their behavior.
For example, they may specify a different container image or add/modify environment variables.
Such instance-specific overrides can be specified in the <code>cluster.spec.componentSpecs[*].instances</code> field.</p>
<p>This field is immutable and cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainerSpecs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSpec">
[]SidecarContainerSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>builtinMonitorContainer</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.BuiltinMonitorContainerRef">
BuiltinMonitorContainerRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the built-in metrics exporter container.</p>
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
<p>Represents user-defined variables that can be used as environment variables for Pods and Actions,
or to render templates of config and script.
These variables are placed in front of the environment variables declared in the Pod if used as
environment variables.</p>
<p>The value of a var can be populated from the following sources:</p>
<ul>
<li>ConfigMap: Allows you to select a ConfigMap and a specific key within that ConfigMap to extract the value from.</li>
<li>Secret: Allows you to select a Secret and a specific key within that Secret to extract the value from.</li>
<li>Pod: Retrieves values (including ports) from a selected Pod.</li>
<li>Service: Retrieves values (including address, port, NodePort) from a selected Service.
The purpose of ServiceVar is to obtain the address of a ComponentService.</li>
<li>Credential: Retrieves values (including account name, account password) from a SystemAccount variable.</li>
<li>ServiceRef: Retrieves values (including address, port, account name, account password) from a selected
ServiceRefDeclaration.
The purpose of ServiceRefVar is to obtain the specific address that a ServiceRef is bound to
(e.g., a ClusterService of another Cluster).</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the volumes used by the Component and some static attributes of the volumes.
After defining the volumes here, user can reference them in the
<code>cluster.spec.componentSpecs[*].volumeClaimTemplates</code> field to configure dynamic properties such as
volume capacity and storage class.</p>
<p>This field allows you to specify the following:</p>
<ul>
<li>Snapshot behavior: Determines whether a snapshot of the volume should be taken when performing
a snapshot backup of the component.</li>
<li>Disk high watermark: Sets the high watermark for the volume&rsquo;s disk usage.
When the disk usage reaches the specified threshold, it triggers an alert or action.</li>
</ul>
<p>By configuring these volume behaviors, you can control how the volumes are managed and monitored within the component.</p>
<p>This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>hostNetwork</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HostNetwork">
HostNetwork
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the host network configuration for the component.</p>
<p>When <code>hostNetwork</code> option is enabled, the Pod shares the host&rsquo;s network namespace and can directly access
the host&rsquo;s network interfaces.
This means that if multiple Pods need to use the same port, they cannot run on the same host simultaneously
due to port conflicts.</p>
<p>The DNSPolicy field in the Pod spec determines how containers within the Pod perform DNS resolution.
When using hostNetwork, the operator will set the DNSPolicy to &lsquo;ClusterFirstWithHostNet&rsquo;.
With this policy, DNS queries will first go through the K8s cluster&rsquo;s DNS service.
If the query fails, it will fall back to the host&rsquo;s DNS settings.</p>
<p>If set, the DNS policy will be automatically set to &ldquo;ClusterFirstWithHostNet&rdquo;.</p>
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
<p>Defines the Services used to access the Component.</p>
<p>By default, a headless service will be automatically created with the name pattern
<code>&#123;cluster.name&#125;-&#123;component.name&#125;-headless</code>.
This headless service is used for internal communication within the cluster.</p>
<p>In addition to the default headless service, this field allows you to customize a list of services
that expose the Component&rsquo;s endpoints to other Components within the same cluster.
Each service in the ComponentService can have its own set of ports, type, and other service-related configurations.</p>
<p>When a Component needs to access a ComponentService provided by another Component within the same cluster,
it can declare a variable in the <code>componentDefinition.spec.vars</code> section and bind it to the exposed address
using the <code>serviceVarRef</code> field.</p>
<p>This field is immutable.</p>
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
<p>Defines the configuration file templates and volume mount parameters used by the Component.
This field contains a list of templateRef that will be rendered into Component instances&rsquo; configuration files
based on the provided templates.
The rendered configuration files will be mounted into the component&rsquo;s containers
using the specified volume mount parameters.</p>
<p>This field is immutable.</p>
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
<p>Defines the types of logs generated by instances of the Component and their corresponding file paths.
These logs can be collected for further analysis and monitoring.</p>
<p>The <code>logConfigs</code> field is an optional list of <code>LogConfig</code> objects, where each object represents
a specific log type and its configuration.
It allows you to specify multiple log types and their respective file paths for the Component instances.</p>
<p>Examples:</p>
<pre><code class="language-yaml"> logConfigs:
- filePathPattern: /data/mysql/log/mysqld-error.log
name: error
- filePathPattern: /data/mysql/log/mysqld.log
name: general
- filePathPattern: /data/mysql/log/mysqld-slowquery.log
name: slow
</code></pre>
<p>This field is immutable.</p>
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
<p>Specifies groups of scripts (each group of scripts provided as a single ConfigMap) to be mounted as volumes and
can be invoked in the container&rsquo;s startup command or action&rsquo;s command.</p>
<p>The <code>scripts</code> field is an optional array that allows you to define a set of scripts to be used by the component.
Each element in the <code>scripts</code> array is defined as a <code>ComponentTemplateSpec</code> object, which contains the following:</p>
<ul>
<li>A ConfigMap object that holds a set of scripts.</li>
<li>A mount point where the scripts will be mounted inside the container.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the namespaced policy rules required by the Component.</p>
<p>The <code>policyRules</code> field is an optional array of <code>rbacv1.PolicyRule</code> objects that define the policy rules
needed by the Component to operate within a namespace.
These policy rules determine the permissions and actions the Component is allowed to perform on
Kubernetes resources within the namespace.</p>
<p>The purpose of this field is to automatically generate the necessary RBAC roles
for the Component based on the specified policy rules.
This ensures that the Pods in the Component has the required permissions to function properly within the namespace.</p>
<p>Note: This field is currently non-functional and is reserved for future implementation.</p>
<p>This field is immutable.</p>
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
<p>Specifies static labels that will be patched to all Kubernetes resources created for the component.</p>
<p>Note: If a label key in the <code>labels</code> field conflicts with any system labels or user-specified labels,
it will be silently ignored to avoid overriding critical labels.</p>
<p>This field is immutable.</p>
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
<p>Specifies static annotations that will be patched to all Kubernetes resources created for the component.</p>
<p>Note: If an annotation key in the <code>annotations</code> field conflicts with any system annotations
or user-specified annotations, it will be silently ignored to avoid overriding critical annotations.</p>
<p>This field is immutable.</p>
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
<p>Defines the upper limit of the number of replicas supported by the Component.</p>
<p>It defines the maximum number of replicas that can be created for the component.
This field allows you to set a limit on the scalability of the component, preventing it from exceeding a certain number of replicas.</p>
<p>This field is immutable.</p>
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
<p>An array of <code>SystemAccount</code> objects that define the system accounts needed
for the management operations of the Component.</p>
<p>Each <code>SystemAccount</code> object in the array contains the following fields:</p>
<ul>
<li>Account name.</li>
<li>The SQL statement template used to create the system account.</li>
<li>The source of the password for the system account. It can be generated based on certain rules or copied from a secret.</li>
</ul>
<p>Common scenarios of system accounts include:</p>
<ul>
<li>Accounts required to initialize the system</li>
<li>Performing backup tasks</li>
<li>Monitoring</li>
<li>Health checks</li>
<li>Replica replication</li>
<li>Other system-level operations</li>
</ul>
<p>System accounts are distinct from user accounts, although both are database accounts.</p>
<ul>
<li>System accounts are typically created by the operator during cluster creation and initialization.
They usually have higher privileges and are used exclusively by the operator and for system management tasks.
System accounts are managed by the KubeBlocks operator using a declarative API, and their lifecycle is fully controlled by the operator.</li>
<li>User accounts, on the other hand, are managed through ops operations and their lifecycle is controlled by users or administrators.
User account permissions should follow the principle of least privilege, granting only the necessary access rights to complete their required tasks.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Specifies the strategy for updating the component instance when it has multiple replicas.
It determines whether multiple replicas can be updated concurrently.</p>
<p>The available strategies are:</p>
<ul>
<li><code>Serial</code>: The replicas are updated one at a time in a serial manner.
The operator waits for each replica to be updated and ready before proceeding to the next one.
This ensures that only one replica is unavailable at a time during the update process.</li>
<li><code>Parallel</code>: The replicas are updated in parallel, with the operator updating all replicas concurrently.
This strategy provides the fastest update time but may lead to a period of reduced availability or
capacity during the update process.</li>
<li><code>BestEffortParallel</code>: The replicas are updated in parallel, with the operator making a best-effort attempt
to update as many replicas as possible concurrently while maintaining the component&rsquo;s availability.
Unlike the <code>Parallel</code> strategy, the <code>BestEffortParallel</code> strategy aims to ensure that a minimum number
of replicas remain available during the update process to maintain the component&rsquo;s quorum and functionality.
For example, consider a component with 5 replicas. To maintain the component&rsquo;s availability and quorum,
the operator may allow a maximum of 2 replicas to be simultaneously updated. This ensures that at least
3 replicas (a quorum) remain available and functional during the update process.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Enumerate all the possible roles a replica of the Component can have.
Each replica can have zero or more roles assigned to it.</p>
<p>KubeBlocks operator determines the roles of each replica by invoking the <code>lifecycleActions.roleProbe</code> method.
This method returns a list of roles for each replica, and the returned roles must be defined in the <code>roles</code> field.</p>
<p>The roles assigned to a replica can influence various aspects of the Component&rsquo;s behavior, such as:</p>
<ul>
<li>Service selection: The Component&rsquo;s provided services can be selected based on the roles of the replicas.
For example, a service may only target replicas with a specific role.</li>
<li>Update order: The roles can determine the order in which replicas are updated during a Component update.
For instance, replicas with a &ldquo;follower&rdquo; role can be updated first, while the replica with the &ldquo;leader&rdquo;
role is updated last. This helps minimize the number of leader changes during the update process.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Defines the strategy for electing the component&rsquo;s active role.</p>
<p>This field has been deprecated since v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
<p>This field is immutable.</p>
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
<p>Defines a set of Actions for customizing the behavior of a Component.</p>
<p>Each Action defines a customizable hook or procedure, designed to be invoked at predetermined points
within the lifecycle of a Component instance.</p>
<p>Available Action triggers include:</p>
<ul>
<li><code>postProvision</code>: Defines the hook to be executed after the creation of a Component,
with <code>preCondition</code> specifying when the action should be fired relative to the Component&rsquo;s lifecycle stages:
<code>Immediately</code>, <code>RuntimeReady</code>, <code>ComponentReady</code>, and <code>ClusterReady</code>.</li>
<li><code>preTerminate</code>: Defines the hook to be executed before terminating a Component.</li>
<li><code>roleProbe</code>: Defines the procedure which is invoked regularly to assess the role of replicas.</li>
<li><code>switchover</code>: Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
such as during planned maintenance or upgrades on the current leader node.</li>
<li><code>memberJoin</code>: Defines the procedure to add a new replica to the replication group.</li>
<li><code>memberLeave</code>: Defines the method to remove a replica from the replication group.</li>
<li><code>readOnly</code>: Defines the procedure to switch a replica into the read-only state.</li>
<li><code>readWrite</code>: transition a replica from the read-only state back to the read-write state.</li>
<li><code>dataDump</code>: Defines the procedure to export the data from a replica.</li>
<li><code>dataLoad</code>: Defines the procedure to import data into a replica.</li>
<li><code>reconfigure</code>: Defines the procedure that update a replica with new configuration.</li>
<li><code>accountProvision</code>: Defines the procedure to generate a new database account.</li>
</ul>
<p>This field is immutable.</p>
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
<p>Enumerates external service dependencies for the current Component.</p>
<p>This can include services hosted on other Clusters as well as services deployed outside of the K8s environment.
It is essential for defining cross-service interactions and ensuring that the Component can properly access and
integrate with these specified services.</p>
<p>This field is immutable.</p>
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
<p>Specifies the minimum duration in seconds that a new pod should remain in the ready
state without any of its containers crashing, before the pod is deemed available for use.
This helps ensure that the pod is stable and capable of serving requests without immediate failures.</p>
<p>A default value of 0 means the pod is considered available as soon as it enters the ready state.</p>
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
<p>Refers to the most recent generation that has been observed for the ComponentDefinition.</p>
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
<p>Represents the current status of the ComponentDefinition. Valid values include `<code>,</code>Available<code>, and</code>Unavailable<code>.
When the status is</code>Available`, the ComponentDefinition is ready and can be utilized by related objects.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>)
</p>
<div>
<p>ComponentLifecycleActions defines a collection of Actions for customizing the behavior of a Component.</p>
</div>
<table>
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
<p>Specifies the hook to be executed after a component&rsquo;s creation.</p>
<p>By setting <code>postProvision.customHandler.preCondition</code>, you can determine the specific lifecycle stage
at which the action should trigger: <code>Immediately</code>, <code>RuntimeReady</code>, <code>ComponentReady</code>, and <code>ClusterReady</code>.
with <code>ComponentReady</code> being the default.</p>
<p>The PostProvision Action is intended to run only once.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li><p>KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster&rsquo;s pod IP addresses (e.g., &ldquo;podIp1,podIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster&rsquo;s pod names (e.g., &ldquo;pod1,pod2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
KB_CLUSTER_POD_NAME_LIST (e.g., &ldquo;hostName1,hostName2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
KB_CLUSTER_POD_NAME_LIST (e.g., &ldquo;hostIp1,hostIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
(e.g., &ldquo;pod1,pod2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;podIp1,podIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;hostName1,hostName2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;hostIp1,hostIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
(e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
(e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
</ul>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Specifies the hook to be executed prior to terminating a component.</p>
<p>The PreTerminate Action is intended to run only once.</p>
<p>This action is executed immediately when a scale-down operation for the Component is initiated.
The actual termination and cleanup of the Component and its associated resources will not proceed
until the PreTerminate action has completed successfully.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li><p>KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster&rsquo;s pod IP addresses (e.g., &ldquo;podIp1,podIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster&rsquo;s pod names (e.g., &ldquo;pod1,pod2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
KB_CLUSTER_POD_NAME_LIST (e.g., &ldquo;hostName1,hostName2&rdquo;).</p></li>
<li><p>KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
KB_CLUSTER_POD_NAME_LIST (e.g., &ldquo;hostIp1,hostIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
(e.g., &ldquo;pod1,pod2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;podIp1,podIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;hostName1,hostName2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., &ldquo;hostIp1,hostIp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
(e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
(e.g., &ldquo;comp1,comp2&rdquo;).</p></li>
<li><p>KB_CLUSTER_COMPONENT_IS_SCALING_IN: Indicates whether the component is currently scaling in.
If this variable is present and set to &ldquo;true&rdquo;, it denotes that the component is undergoing a scale-in operation.
During scale-in, data rebalancing is necessary to maintain cluster integrity.
Contrast this with a cluster deletion scenario where data rebalancing is not required as the entire cluster
is being cleaned up.</p></li>
</ul>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure which is invoked regularly to assess the role of replicas.</p>
<p>This action is periodically triggered by Lorry at the specified interval to determine the role of each replica.
Upon successful execution, the action&rsquo;s output designates the role of the replica,
which should match one of the predefined role names within <code>componentDefinition.spec.roles</code>.
The output is then compared with the previous successful execution result.
If a role change is detected, an event is generated to inform the controller,
which initiates an update of the replica&rsquo;s role.</p>
<p>Defining a RoleProbe Action for a Component is required if roles are defined for the Component.
It ensures replicas are correctly labeled with their respective roles.
Without this, services that rely on roleSelectors might improperly direct traffic to wrong replicas.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_POD_FQDN: The FQDN of the Pod whose role is being assessed.</li>
<li>KB_SERVICE_PORT: The port used by the database service.</li>
<li>KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.</li>
<li>KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.</li>
</ul>
<p>Expected output of this action:
- On Success: The determined role of the replica, which must align with one of the roles specified
in the component definition.
- On Failure: An error message, if applicable, indicating why the action failed.</p>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
during events such as planned maintenance or when performing stop, shutdown, restart, or upgrade operations
involving the current leader node.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).</li>
<li>KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate&rsquo;s pod, which may not be specified (empty).</li>
<li>KB_LEADER_POD_IP: The IP address of the current leader&rsquo;s pod prior to the switchover.</li>
<li>KB_LEADER_POD_NAME: The name of the current leader&rsquo;s pod prior to the switchover.</li>
<li>KB_LEADER_POD_FQDN: The FQDN of the current leader&rsquo;s pod prior to the switchover.</li>
</ul>
<p>The environment variables with the following prefixes are deprecated and will be removed in future releases:</p>
<ul>
<li>KB_REPLICATION_PRIMARY<em>POD</em></li>
<li>KB_CONSENSUS_LEADER<em>POD</em></li>
</ul>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure to add a new replica to the replication group.</p>
<p>This action is initiated after a replica pod becomes ready.</p>
<p>The role of the replica (e.g., primary, secondary) will be determined and assigned as part of the action command
implementation, or automatically by the database kernel or a sidecar utility like Patroni that implements
a consensus algorithm.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_SERVICE_PORT: The port used by the database service.</li>
<li>KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.</li>
<li>KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.</li>
<li>KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.</li>
<li>KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.</li>
<li>KB_NEW_MEMBER_POD_NAME: The pod name of the replica being added to the group.</li>
<li>KB_NEW_MEMBER_POD_IP: The IP address of the replica being added to the group.</li>
</ul>
<p>Expected action output:
- On Failure: An error message detailing the reason for any failure encountered
during the addition of the new member.</p>
<p>For example, to add a new OBServer to an OceanBase Cluster in &lsquo;zone1&rsquo;, the following command may be used:</p>
<pre><code class="language-yaml">command:
- bash
- -c
- |
ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
HOST=$(echo $ADDRESS | cut -d ':' -f 1)
PORT=$(echo $ADDRESS | cut -d ':' -f 2)
CLIENT=&quot;mysql -u $KB_SERVICE_USER -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e&quot;
$CLIENT &quot;ALTER SYSTEM ADD SERVER '$KB_NEW_MEMBER_POD_IP:$KB_SERVICE_PORT' ZONE 'zone1'&quot;
</code></pre>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure to remove a replica from the replication group.</p>
<p>This action is initiated before remove a replica from the group.
The operator will wait for MemberLeave to complete successfully before releasing the replica and cleaning up
related Kubernetes resources.</p>
<p>The process typically includes updating configurations and informing other group members about the removal.
Data migration is generally not part of this action and should be handled separately if needed.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_SERVICE_PORT: The port used by the database service.</li>
<li>KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.</li>
<li>KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.</li>
<li>KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.</li>
<li>KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.</li>
<li>KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.</li>
<li>KB_LEAVE_MEMBER_POD_IP: The IP address of the replica being removed from the group.</li>
</ul>
<p>Expected action output:
- On Failure: An error message, if applicable, indicating why the action failed.</p>
<p>For example, to remove an OBServer from an OceanBase Cluster in &lsquo;zone1&rsquo;, the following command can be executed:</p>
<pre><code class="language-yaml">command:
- bash
- -c
- |
ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
HOST=$(echo $ADDRESS | cut -d ':' -f 1)
PORT=$(echo $ADDRESS | cut -d ':' -f 2)
CLIENT=&quot;mysql -u $KB_SERVICE_USER  -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e&quot;
$CLIENT &quot;ALTER SYSTEM DELETE SERVER '$KB_LEAVE_MEMBER_POD_IP:$KB_SERVICE_PORT' ZONE 'zone1'&quot;
</code></pre>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure to switch a replica into the read-only state.</p>
<p>Use Case:
This action is invoked when the database&rsquo;s volume capacity nears its upper limit and space is about to be exhausted.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_POD_FQDN: The FQDN of the replica pod whose role is being checked.</li>
<li>KB_SERVICE_PORT: The port used by the database service.</li>
<li>KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.</li>
<li>KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.</li>
</ul>
<p>Expected action output:
- On Failure: An error message, if applicable, indicating why the action failed.</p>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure to transition a replica from the read-only state back to the read-write state.</p>
<p>Use Case:
This action is used to bring back a replica that was previously in a read-only state,
which restricted write operations, to its normal operational state where it can handle
both read and write operations.</p>
<p>The container executing this action has access to following environment variables:</p>
<ul>
<li>KB_POD_FQDN: The FQDN of the replica pod whose role is being checked.</li>
<li>KB_SERVICE_PORT: The port used by the database service.</li>
<li>KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.</li>
<li>KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.</li>
</ul>
<p>Expected action output:
- On Failure: An error message, if applicable, indicating why the action failed.</p>
<p>Note: This field is immutable once it has been set.</p>
</td>
</tr>
<tr>
<td>
<code>dataDump</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the procedure for exporting the data from a replica.</p>
<p>Use Case:
This action is intended for initializing a newly created replica with data. It involves exporting data
from an existing replica and importing it into the new, empty replica. This is essential for synchronizing
the state of replicas across the system.</p>
<p>Applicability:
Some database engines or associated sidecar applications (e.g., Patroni) may already provide this functionality.
In such cases, this action may not be required.</p>
<p>The output should be a valid data dump streamed to stdout. It must exclude any irrelevant information to ensure
that only the necessary data is exported for import into the new replica.</p>
<p>Note: This field is immutable once it has been set.</p>
</td>
</tr>
<tr>
<td>
<code>dataLoad</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">
LifecycleActionHandler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the procedure for importing data into a replica.</p>
<p>Use Case:
This action is intended for initializing a newly created replica with data. It involves exporting data
from an existing replica and importing it into the new, empty replica. This is essential for synchronizing
the state of replicas across the system.</p>
<p>Some database engines or associated sidecar applications (e.g., Patroni) may already provide this functionality.
In such cases, this action may not be required.</p>
<p>Data should be received through stdin. If any error occurs during the process,
the action must be able to guarantee idempotence to allow for retries from the beginning.</p>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Defines the procedure that update a replica with new configuration.</p>
<p>Note: This field is immutable once it has been set.</p>
<p>This Action is reserved for future versions.</p>
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
<p>Defines the procedure to generate a new database account.</p>
<p>Use Case:
This action is designed to create system accounts that are utilized for replication, monitoring, backup,
and other administrative tasks.</p>
<p>Note: This field is immutable once it has been set.</p>
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
<p>Deprecated since v0.8.</p>
</div>
<table>
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
<p>The name of the env, it must be a C identifier.</p>
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
<p>The value of the env.</p>
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
<p>The source from which the value of the env.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>)
</p>
<div>
<p>ComponentService defines a service that would be exposed as an inter-component service within a Cluster.
A Service defined in the ComponentService is expected to be accessed by other Components within the same Cluster.</p>
<p>When a Component needs to use a ComponentService provided by another Component within the same Cluster,
it can declare a variable in the <code>componentDefinition.spec.vars</code> section and bind it to the specific exposed address
of the ComponentService using the <code>serviceVarRef</code> field.</p>
</div>
<table>
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
<code>podService</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether to create a corresponding Service for each Pod of the selected Component.
When set to true, a set of Services will be automatically generated for each Pod,
and the <code>roleSelector</code> field will be ignored.</p>
<p>The names of the generated Services will follow the same naming pattern: <code>$(serviceName)-$(podOrdinal)</code>.</p>
<p>The podOrdinal is zero-based, meaning it starts from 0 for the first Pod and increments for each subsequent Pod.
The total number of generated Services will be equal to the number of replicas specified for the Component.</p>
<p>Example usage:</p>
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
<p>In this example, if the Component has 3 replicas, three Services will be generated:
- my-service-0: Points to the first Pod (podOrdinal: 0)
- my-service-1: Points to the second Pod (podOrdinal: 1)
- my-service-2: Points to the third Pod (podOrdinal: 2)</p>
<p>Each generated Service will have the specified spec configuration and will target its respective Pod.</p>
<p>This feature is useful when you need to expose each Pod of a Component individually, allowing external access
to specific instances of the Component.</p>
</td>
</tr>
<tr>
<td>
<code>disableAutoProvision</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the automatic provisioning of the service should be disabled.</p>
<p>If set to true, the service will not be automatically created at the component provisioning.
Instead, you can enable the creation of this service by specifying it explicitly in the cluster API.</p>
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
<p>ComponentSpec defines the desired state of Component.</p>
</div>
<table>
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
<p>Specifies the name of the referenced ComponentDefinition.</p>
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
<p>ServiceVersion specifies the version of the service expected to be provisioned by this component.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).</p>
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
<p>Defines a list of ServiceRef for a Component, allowing it to connect and interact with other services.
These services can be external or managed by the same KubeBlocks operator, categorized as follows:</p>
<ol>
<li><p>External Services:</p>
<ul>
<li>Not managed by KubeBlocks. These could be services outside KubeBlocks or non-Kubernetes services.</li>
<li>Connection requires a ServiceDescriptor providing details for service binding.</li>
</ul></li>
<li><p>KubeBlocks Services:</p>
<ul>
<li>Managed within the same KubeBlocks environment.</li>
<li>Service binding is achieved by specifying cluster names in the service references,
with configurations handled by the KubeBlocks operator.</li>
</ul></li>
</ol>
<p>ServiceRef maintains cluster-level semantic consistency; references with the same <code>serviceRef.name</code>
within the same cluster are treated as identical.
Only bindings to the same cluster or ServiceDescriptor are allowed within a cluster.</p>
<p>Example:</p>
<pre><code class="language-yaml">serviceRefs:
- name: &quot;redis-sentinel&quot;
serviceDescriptor:
name: &quot;external-redis-sentinel&quot;
- name: &quot;postgres-cluster&quot;
cluster:
name: &quot;my-postgres-cluster&quot;
</code></pre>
<p>The example above includes references to an external Redis Sentinel service and a PostgreSQL cluster managed by KubeBlocks.</p>
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
<p>Specifies the resources required by the Component.
It allows defining the CPU, memory requirements and limits for the Component&rsquo;s containers.</p>
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
<p>Specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component.
Each template specifies the desired characteristics of a persistent volume, such as storage class,
size, and access modes.
These templates are used to dynamically provision persistent volumes for the Component when it is deployed.</p>
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
<p>Overrides services defined in referenced ComponentDefinition and exposes endpoints that can be accessed
by clients.</p>
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
<p>Each component supports running multiple replicas to provide high availability and persistence.
This field can be used to specify the desired number of replicas.</p>
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
<p>Reserved field for future use.</p>
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
<p>Specifies which types of logs should be collected for the Cluster.
The log types are defined in the <code>componentDefinition.spec.logConfigs</code> field with the LogConfig entries.</p>
<p>The elements in the <code>enabledLogs</code> array correspond to the names of the LogConfig entries.
For example, if the <code>componentDefinition.spec.logConfigs</code> defines LogConfig entries with
names &ldquo;slow_query_log&rdquo; and &ldquo;error_log&rdquo;,
you can enable the collection of these logs by including their names in the <code>enabledLogs</code> array:
enabledLogs: [&ldquo;slow_query_log&rdquo;, &ldquo;error_log&rdquo;]</p>
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
<p>Specifies the name of the ServiceAccount required by the running Component.
This ServiceAccount is used to grant necessary permissions for the Component&rsquo;s Pods to interact
with other Kubernetes resources, such as modifying pod labels or sending events.</p>
<p>Defaults:
If not specified, KubeBlocks automatically assigns a default ServiceAccount named &ldquo;kb-&#123;cluster.name&#125;&rdquo;,
bound to a default role defined during KubeBlocks installation.</p>
<p>Future Changes:
Future versions might change the default ServiceAccount creation strategy to one per Component,
potentially revising the naming to &ldquo;kb-&#123;cluster.name&#125;-&#123;component.name&#125;&rdquo;.</p>
<p>Users can override the automatic ServiceAccount assignment by explicitly setting the name of
an existed ServiceAccount in this field.</p>
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
<p>Specifies a group of affinity scheduling rules for the Component.
It allows users to control how the Component&rsquo;s Pods are scheduled onto nodes in the cluster.</p>
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
<p>Allows the Component to be scheduled onto nodes with matching taints.
It is an array of tolerations that are attached to the Component&rsquo;s Pods.</p>
<p>Each toleration consists of a <code>key</code>, <code>value</code>, <code>effect</code>, and <code>operator</code>.
The <code>key</code>, <code>value</code>, and <code>effect</code> define the taint that the toleration matches.
The <code>operator</code> specifies how the toleration matches the taint.</p>
<p>If a node has a taint that matches a toleration, the Component&rsquo;s pods can be scheduled onto that node.
This allows the Component&rsquo;s Pods to run on nodes that have been tainted to prevent regular Pods from being scheduled.</p>
</td>
</tr>
<tr>
<td>
<code>schedulingPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SchedulingPolicy">
SchedulingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the scheduling policy for the component.</p>
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
<p>Specifies the TLS configuration for the component, including:</p>
<ul>
<li>A boolean flag that indicates whether the component should use Transport Layer Security (TLS) for secure communication.</li>
<li>An optional field that specifies the configuration for the TLS certificates issuer when TLS is enabled.
It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allows for the customization of configuration values for each instance within a component.
An Instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
While instances typically share a common configuration as defined in the ClusterComponentSpec,
they can require unique settings in various scenarios:</p>
<p>For example:
- A database component might require different resource allocations for primary and secondary instances,
with primaries needing more resources.
- During a rolling upgrade, a component may first update the image for one or a few instances,
and then update the remaining instances after verifying that the updated instances are functioning correctly.</p>
<p>InstanceTemplate allows for specifying these unique configurations per instance.
Each instance&rsquo;s name is constructed using the pattern: $(component.name)-$(template.name)-$(ordinal),
starting with an ordinal of 0.
It is crucial to maintain unique names for each InstanceTemplate to avoid conflicts.</p>
<p>The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the Component.
Any remaining replicas will be generated using the default template and will follow the default naming rules.</p>
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
<p>Specifies the names of instances to be transitioned to offline status.</p>
<p>Marking an instance as offline results in the following:</p>
<ol>
<li>The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
future reuse or data recovery, but it is no longer actively used.</li>
<li>The ordinal number assigned to this instance is preserved, ensuring it remains unique
and avoiding conflicts with new instances.</li>
</ol>
<p>Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
ordinal consistency within the cluster.
Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClassName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines RuntimeClassName for all Pods managed by this component.</p>
</td>
</tr>
<tr>
<td>
<code>sidecars</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the sidecar containers that will be attached to the component&rsquo;s main container.</p>
</td>
</tr>
<tr>
<td>
<code>monitorEnabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Determines whether the metrics exporter needs to be published to the service endpoint.
If set to true, the metrics exporter will be published to the service endpoint,
the service will be injected with the following annotations:
- &ldquo;monitor.kubeblocks.io/path&rdquo;
- &ldquo;monitor.kubeblocks.io/port&rdquo;
- &ldquo;monitor.kubeblocks.io/scheme&rdquo;</p>
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
<p>ComponentStatus represents the observed state of a Component within the cluster.</p>
</div>
<table>
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
<p>Specifies the most recent generation observed for this Component.
This corresponds to the Cluster&rsquo;s generation, which is updated by the API Server upon mutation.</p>
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
<p>Defines the current state of the component API Resource, such as warnings.</p>
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
<p>Indicates the phase of the component. Detailed information for each phase is as follows:</p>
<ul>
<li>Creating: A special <code>Updating</code> phase with previous phase <code>empty</code>(means &ldquo;&rdquo;) or <code>Creating</code>.</li>
<li>Running: Component replicas &gt; 0 and all pod specs are latest with a Running state.</li>
<li>Updating: Component replicas &gt; 0 and no failed pods. The component is being updated.</li>
<li>Abnormal: Component replicas &gt; 0 but some pods have failed. The component is functional but in a fragile state.</li>
<li>Failed: Component replicas &gt; 0 but some pods have failed. The component is no longer functional.</li>
<li>Stopping: Component replicas = 0 and pods are terminating.</li>
<li>Stopped: Component replicas = 0 and all pods have been deleted.</li>
<li>Deleting: The component is being deleted.</li>
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
<p>Records the detailed message of the component in its current phase.
Keys can be podName, deployName, or statefulSetName. The format is <code>ObjectKind/Name</code>.</p>
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
<p>Represents the switchover process for a specified candidate primary or leader instance.
Note that only Action.Exec is currently supported, while Action.HTTP is not.</p>
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
<p>Represents a switchover process that does not involve a specific candidate primary or leader instance.
As with the previous field, only Action.Exec is currently supported, not Action.HTTP.</p>
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
<p>Used to define the selectors for the scriptSpecs that need to be referenced.
If this field is set, the scripts defined under the &lsquo;scripts&rsquo; field can be invoked or referenced within an Action.</p>
<p>This field is deprecated from v0.9.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
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
<p>Specifies the name of the configuration template.</p>
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
<p>Specifies the name of the referenced configuration template ConfigMap object.</p>
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
<p>Specifies the namespace of the referenced configuration template ConfigMap object.
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
<p>Refers to the volume name of PodTemplate. The configuration file produced through the configuration
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
<p>Deprecated: DefaultMode is deprecated since 0.9.0 and will be removed in 0.10.0
for scripts, auto set 0555
for configs, auto set 0444
Refers to the mode bits used to set permissions on created files by default.</p>
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
<p>ComponentValueFrom is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the source to select. It can be one of three types: <code>FieldRef</code>, <code>ServiceRef</code>, <code>HeadlessServiceRef</code>.</p>
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
<p>The jsonpath of the source to select when the Type is <code>FieldRef</code>.
Two objects are registered in the jsonpath: <code>componentDef</code> and <code>components</code>:</p>
<ul>
<li><code>componentDef</code> is the component definition object specified in <code>componentRef.componentDefName</code>.</li>
<li><code>components</code> are the component list objects referring to the component definition object.</li>
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
<p>Defines the format of each headless service address.
Three builtin variables can be used as placeholders: <code>$POD_ORDINAL</code>, <code>$POD_FQDN</code>, <code>$POD_NAME</code></p>
<ul>
<li><code>$POD_ORDINAL</code> represents the ordinal of the pod.</li>
<li><code>$POD_FQDN</code> represents the fully qualified domain name of the pod.</li>
<li><code>$POD_NAME</code> represents the name of the pod.</li>
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
<p>The string used to join the values of headless service addresses.</p>
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
<p>ComponentValueFromType specifies the type of component value from which the data is derived.</p>
<p>Deprecated since v0.8.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;FieldRef&#34;</p></td>
<td><p>FromFieldRef refers to the value of a specific field in the object.</p>
</td>
</tr><tr><td><p>&#34;HeadlessServiceRef&#34;</p></td>
<td><p>FromHeadlessServiceRef refers to a headless service within the same namespace as the object.</p>
</td>
</tr><tr><td><p>&#34;ServiceRef&#34;</p></td>
<td><p>FromServiceRef refers to a service within the same namespace as the object.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVersionCompatibilityRule">ComponentVersionCompatibilityRule
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<div>
<p>ComponentVersionCompatibilityRule defines the compatibility between a set of component definitions and a set of releases.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>compDefs</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>CompDefs specifies names for the component definitions associated with this ComponentVersion.
Each name in the list can represent an exact name, or a name prefix.</p>
<p>For example:</p>
<ul>
<li>&ldquo;mysql-8.0.30-v1alpha1&rdquo;: Matches the exact name &ldquo;mysql-8.0.30-v1alpha1&rdquo;</li>
<li>&ldquo;mysql-8.0.30&rdquo;: Matches all names starting with &ldquo;mysql-8.0.30&rdquo;</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>releases</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Releases is a list of identifiers for the releases.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVersionRelease">ComponentVersionRelease
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<div>
<p>ComponentVersionRelease represents a release of component instances within a ComponentVersion.</p>
</div>
<table>
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
<p>Name is a unique identifier for this release.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>changes</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Changes provides information about the changes made in this release.</p>
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
<p>ServiceVersion defines the version of the well-known service that the component provides.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).
If the release is used, it will serve as the service version for component instances, overriding the one defined in the component definition.
Cannot be updated.</p>
</td>
</tr>
<tr>
<td>
<code>images</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Images define the new images for different containers within the release.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVersionSpec">ComponentVersionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentVersion">ComponentVersion</a>)
</p>
<div>
<p>ComponentVersionSpec defines the desired state of ComponentVersion</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>compatibilityRules</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionCompatibilityRule">
[]ComponentVersionCompatibilityRule
</a>
</em>
</td>
<td>
<p>CompatibilityRules defines compatibility rules between sets of component definitions and releases.</p>
</td>
</tr>
<tr>
<td>
<code>releases</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionRelease">
[]ComponentVersionRelease
</a>
</em>
</td>
<td>
<p>Releases represents different releases of component instances within this ComponentVersion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ComponentVersionStatus">ComponentVersionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentVersion">ComponentVersion</a>)
</p>
<div>
<p>ComponentVersionStatus defines the observed state of ComponentVersion</p>
</div>
<table>
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
<p>ObservedGeneration is the most recent generation observed for this ComponentVersion.</p>
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
<p>Phase valid values are `<code>,</code>Available<code>, 'Unavailable</code>.
Available is ComponentVersion become available, and can be used for co-related objects.</p>
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
<tr>
<td>
<code>serviceVersions</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceVersions represent the supported service versions of this ComponentVersion.</p>
</td>
</tr>
</tbody>
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
<p>Specifies the name of the volume.
It must be a DNS_LABEL and unique within the pod.
More info can be found at: <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names">https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names</a>
Note: This field cannot be updated.</p>
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
<p>Specifies whether the creation of a snapshot of this volume is necessary when performing a backup of the Component.</p>
<p>Note: This field cannot be updated.</p>
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
<p>Sets the critical threshold for volume space utilization as a percentage (0-100).</p>
<p>Exceeding this percentage triggers the system to switch the volume to read-only mode as specified in
<code>componentDefinition.spec.lifecycleActions.readOnly</code>.
This precaution helps prevent space depletion while maintaining read-only access.
If the space utilization later falls below this threshold, the system reverts the volume to read-write mode
as defined in <code>componentDefinition.spec.lifecycleActions.readWrite</code>, restoring full functionality.</p>
<p>Note: This field cannot be updated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ConfigConstraint">ConfigConstraint
</h3>
<div>
<p>ConfigConstraint manages the parameters across multiple configuration files contained in a single configure template.
These configuration files should have the same format (e.g. ini, xml, properties, json).</p>
<p>It provides the following functionalities:</p>
<ol>
<li><strong>Parameter Value Validation</strong>: Validates and ensures compliance of parameter values with defined constraints.</li>
<li><strong>Dynamic Reload on Modification</strong>: Monitors parameter changes and triggers dynamic reloads to apply updates.</li>
<li><strong>Parameter Rendering in Templates</strong>: Injects parameters into templates to generate up-to-date configuration files.</li>
</ol>
</div>
<table>
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
<p>Specifies the dynamic reload action supported by the engine.
When set, the controller executes the method defined here to execute hot parameter updates.</p>
<p>Dynamic reloading is triggered only if both of the following conditions are met:</p>
<ol>
<li>The modified parameters are listed in the <code>dynamicParameters</code> field.
If <code>dynamicParameterSelectedPolicy</code> is set to &ldquo;all&rdquo;, modifications to <code>staticParameters</code>
can also trigger a reload.</li>
<li><code>reloadOptions</code> is set.</li>
</ol>
<p>If <code>reloadOptions</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">reloadOptions:
tplScriptTrigger:
namespace: kb-system
scriptConfigMapRef: mysql-reload-script
sync: true
</code></pre>
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
<p>Indicates whether to consolidate dynamic reload and restart actions into a single restart.</p>
<ul>
<li>If true, updates requiring both actions will result in only a restart, merging the actions.</li>
<li>If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.</li>
</ul>
<p>This flag allows for more efficient handling of configuration changes by potentially eliminating
an unnecessary reload step.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>reloadOptions</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>&ldquo;dynamic&rdquo; (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>&ldquo;all&rdquo;: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>toolsImageSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ReloadToolsImage">
ReloadToolsImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tools container image used by ShellTrigger for dynamic reload.
If the dynamic reload action is triggered by a ShellTrigger, this field is required.
This image must contain all necessary tools for executing the ShellTrigger scripts.</p>
<p>Usually the specified image is referenced by the init container,
which is then responsible for copy the tools from the image to a bin volume.
This ensures that the tools are available to the &lsquo;config-manager&rsquo; sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>downwardAPIOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DownwardAction">
[]DownwardAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of actions to execute specified commands based on Pod labels.</p>
<p>It utilizes the K8s Downward API to mount label information as a volume into the pod.
The &lsquo;config-manager&rsquo; sidecar container watches for changes in the role label and dynamically invoke
registered commands (usually execute some SQL statements) when a change is detected.</p>
<p>It is designed for scenarios where:</p>
<ul>
<li>Replicas with different roles have different configurations, such as Redis primary &amp; secondary replicas.</li>
<li>After a role switch (e.g., from secondary to primary), some changes in configuration are needed
to reflect the new role.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of ScriptConfig Object.</p>
<p>Each ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the dynamic reload
and DownwardAction to perform specific tasks or configurations.</p>
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
<p>Specifies the top-level key in the &lsquo;configurationSchema.cue&rsquo; that organizes the validation rules for parameters.
This key must exist within the CUE script defined in &lsquo;configurationSchema.cue&rsquo;.</p>
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
<p>Defines a list of parameters including their names, default values, descriptions,
types, and constraints (permissible values or the range of valid values).</p>
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
<p>List static parameters.
Modifications to any of these parameters require a restart of the process to take effect.</p>
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
<p>List dynamic parameters.
Modifications to these parameters trigger a configuration reload without requiring a process restart.</p>
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
<p>Lists the parameters that cannot be modified once set.
Attempting to change any of these parameters will be ignored.</p>
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
<p>Used to match labels on the pod to determine whether a dynamic reload should be performed.</p>
<p>In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
The <code>selector</code> allows you to specify label selectors to target the desired pods for the reload process.</p>
<p>If the <code>selector</code> is not specified or is nil, all pods managed by the workload will be considered for the dynamic
reload.</p>
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
Supported formats include <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>, <code>hcl</code>, <code>dotenv</code>, <code>properties</code>, and <code>toml</code>.</p>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
<p>Example:</p>
<pre><code>formatterConfig:
format: ini
iniConfig:
sectionName: mysqld
</code></pre>
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
<p>Specifies the dynamic reload action supported by the engine.
When set, the controller executes the method defined here to execute hot parameter updates.</p>
<p>Dynamic reloading is triggered only if both of the following conditions are met:</p>
<ol>
<li>The modified parameters are listed in the <code>dynamicParameters</code> field.
If <code>dynamicParameterSelectedPolicy</code> is set to &ldquo;all&rdquo;, modifications to <code>staticParameters</code>
can also trigger a reload.</li>
<li><code>reloadOptions</code> is set.</li>
</ol>
<p>If <code>reloadOptions</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">reloadOptions:
tplScriptTrigger:
namespace: kb-system
scriptConfigMapRef: mysql-reload-script
sync: true
</code></pre>
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
<p>Indicates whether to consolidate dynamic reload and restart actions into a single restart.</p>
<ul>
<li>If true, updates requiring both actions will result in only a restart, merging the actions.</li>
<li>If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.</li>
</ul>
<p>This flag allows for more efficient handling of configuration changes by potentially eliminating
an unnecessary reload step.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>reloadOptions</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>&ldquo;dynamic&rdquo; (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>&ldquo;all&rdquo;: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>toolsImageSpec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ReloadToolsImage">
ReloadToolsImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tools container image used by ShellTrigger for dynamic reload.
If the dynamic reload action is triggered by a ShellTrigger, this field is required.
This image must contain all necessary tools for executing the ShellTrigger scripts.</p>
<p>Usually the specified image is referenced by the init container,
which is then responsible for copy the tools from the image to a bin volume.
This ensures that the tools are available to the &lsquo;config-manager&rsquo; sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>downwardAPIOptions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DownwardAction">
[]DownwardAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of actions to execute specified commands based on Pod labels.</p>
<p>It utilizes the K8s Downward API to mount label information as a volume into the pod.
The &lsquo;config-manager&rsquo; sidecar container watches for changes in the role label and dynamically invoke
registered commands (usually execute some SQL statements) when a change is detected.</p>
<p>It is designed for scenarios where:</p>
<ul>
<li>Replicas with different roles have different configurations, such as Redis primary &amp; secondary replicas.</li>
<li>After a role switch (e.g., from secondary to primary), some changes in configuration are needed
to reflect the new role.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of ScriptConfig Object.</p>
<p>Each ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the dynamic reload
and DownwardAction to perform specific tasks or configurations.</p>
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
<p>Specifies the top-level key in the &lsquo;configurationSchema.cue&rsquo; that organizes the validation rules for parameters.
This key must exist within the CUE script defined in &lsquo;configurationSchema.cue&rsquo;.</p>
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
<p>Defines a list of parameters including their names, default values, descriptions,
types, and constraints (permissible values or the range of valid values).</p>
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
<p>List static parameters.
Modifications to any of these parameters require a restart of the process to take effect.</p>
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
<p>List dynamic parameters.
Modifications to these parameters trigger a configuration reload without requiring a process restart.</p>
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
<p>Lists the parameters that cannot be modified once set.
Attempting to change any of these parameters will be ignored.</p>
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
<p>Used to match labels on the pod to determine whether a dynamic reload should be performed.</p>
<p>In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
The <code>selector</code> allows you to specify label selectors to target the desired pods for the reload process.</p>
<p>If the <code>selector</code> is not specified or is nil, all pods managed by the workload will be considered for the dynamic
reload.</p>
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
Supported formats include <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>, <code>hcl</code>, <code>dotenv</code>, <code>properties</code>, and <code>toml</code>.</p>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
<p>Example:</p>
<pre><code>formatterConfig:
format: ini
iniConfig:
sectionName: mysqld
</code></pre>
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
<p>ConfigConstraintStatus represents the observed state of a ConfigConstraint.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintPhase">
ConfigConstraintPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the status of the configuration template.
When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition or ClusterVersion.</p>
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
<p>Provides descriptions for abnormal states.</p>
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
<p>Refers to the most recent generation observed for this ConfigConstraint. This value is updated by the API Server.</p>
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
<p>ConfigMapRef defines a reference to a ConfigMap.</p>
</div>
<table>
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
<p>ConfigMap specifies the ConfigMap to be mounted as a volume.</p>
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
<p>Holds the configuration keys and values. This field is a workaround for issues found in kubebuilder and code-generator.
Refer to <a href="https://github.com/kubernetes-sigs/kubebuilder/issues/528">https://github.com/kubernetes-sigs/kubebuilder/issues/528</a> and <a href="https://github.com/kubernetes/code-generator/issues/50">https://github.com/kubernetes/code-generator/issues/50</a> for more details.</p>
<p>Represents the content of the configuration file.</p>
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
<p>Represents the updated parameters for a single configuration file.</p>
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
<p>Specifies the name of the referenced configuration template ConfigMap object.</p>
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
<p>Specifies the namespace of the referenced configuration template ConfigMap object.
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
<p>Defines the strategy for merging externally imported templates into component templates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Configuration">Configuration
</h3>
<div>
<p>Configuration represents the complete set of configurations for a specific Component of a Cluster.
This includes templates for each configuration file, their corresponding ConfigConstraints, volume mounts,
and other relevant details.</p>
</div>
<table>
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
<p>Specifies the name of the Cluster that this configuration is associated with.</p>
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
<p>Represents the name of the Component that this configuration pertains to.</p>
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
<p>ConfigItemDetails is an array of ConfigurationItemDetail objects.</p>
<p>Each ConfigurationItemDetail corresponds to a configuration template,
which is a ConfigMap that contains multiple configuration files.
Each configuration file is stored as a key-value pair within the ConfigMap.</p>
<p>The ConfigurationItemDetail includes information such as:</p>
<ul>
<li>The configuration template (a ConfigMap)</li>
<li>The corresponding ConfigConstraint (constraints and validation rules for the configuration)</li>
<li>Volume mounts (for mounting the configuration files)</li>
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
<p>ConfigurationItemDetail corresponds to settings of a configuration template (a ConfigMap).</p>
</div>
<table>
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
<p>Defines the unique identifier of the configuration template.</p>
<p>It must be a string of maximum 63 characters, and can only include lowercase alphanumeric characters,
hyphens, and periods.
The name must start and end with an alphanumeric character.</p>
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
<p>Deprecated: No longer used. Please use &lsquo;Payload&rsquo; instead. Previously represented the version of the configuration template.</p>
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
<p>External controllers can trigger a configuration rerender by modifying this field.</p>
<p>Note: Currently, the <code>payload</code> field is opaque and its content is not interpreted by the system.
Modifying this field will cause a rerender, regardless of the specific content of this field.</p>
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
<p>Specifies the name of the configuration template (a ConfigMap), ConfigConstraint, and other miscellaneous options.</p>
<p>The configuration template is a ConfigMap that contains multiple configuration files.
Each configuration file is stored as a key-value pair within the ConfigMap.</p>
<p>ConfigConstraint allows defining constraints and validation rules for configuration parameters.
It ensures that the configuration adheres to certain requirements and limitations.</p>
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
<p>Specifies the user-defined configuration template.</p>
<p>When provided, the <code>importTemplateRef</code> overrides the default configuration template
specified in <code>configSpec.templateRef</code>.
This allows users to customize the configuration template according to their specific requirements.</p>
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
<p>Specifies the user-defined configuration parameters.</p>
<p>When provided, the parameter values in <code>configFileParams</code> override the default configuration parameters.
This allows users to override the default configuration according to their specific needs.</p>
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
<p>Specifies the name of the configuration template. It is a required field and must be a string of maximum 63 characters.
The name should only contain lowercase alphanumeric characters, hyphens, or periods. It should start and end with an alphanumeric character.</p>
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
<p>Indicates the current status of the configuration item. This field is optional.</p>
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
<p>Represents the last completed revision of the configuration item. This field is optional.</p>
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
<p>Represents the updated revision of the configuration item. This field is optional.</p>
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
<p>Provides a description of any abnormal status. This field is optional.</p>
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
<p>Provides detailed information about the execution of the configuration change. This field is optional.</p>
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
<p>ConfigurationSpec defines the desired state of a Configuration resource.</p>
</div>
<table>
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
<p>Specifies the name of the Cluster that this configuration is associated with.</p>
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
<p>Represents the name of the Component that this configuration pertains to.</p>
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
<p>ConfigItemDetails is an array of ConfigurationItemDetail objects.</p>
<p>Each ConfigurationItemDetail corresponds to a configuration template,
which is a ConfigMap that contains multiple configuration files.
Each configuration file is stored as a key-value pair within the ConfigMap.</p>
<p>The ConfigurationItemDetail includes information such as:</p>
<ul>
<li>The configuration template (a ConfigMap)</li>
<li>The corresponding ConfigConstraint (constraints and validation rules for the configuration)</li>
<li>Volume mounts (for mounting the configuration files)</li>
</ul>
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
<p>ConfigurationStatus represents the observed state of a Configuration resource.</p>
</div>
<table>
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
<p>Provides a description of any abnormal status.</p>
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
<p>Represents the latest generation observed for this
ClusterDefinition. It corresponds to the ConfigConstraint&rsquo;s generation, which is
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
<p>Provides detailed status information for opsRequest.</p>
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
<p>Provides the status of each component undergoing reconfiguration.</p>
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
<p>ConnectionCredentialAuth represents the authentication details of the service connection credential.</p>
</div>
<table>
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
<p>Represents the username credential for the service connection.</p>
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
<p>Represents the password credential for the service connection.</p>
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
<p>Represents the key of the password in the connection credential secret.
If not specified, the default key &ldquo;password&rdquo; is used.</p>
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
<p>Represents the key of the username in the connection credential secret.
If not specified, the default key &ldquo;username&rdquo; is used.</p>
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
<p>Defines the key of the host in the connection credential secret.</p>
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
<p>Indicates map key of the port in the connection credential secret.</p>
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
<p>ConsensusMember is deprecated since v0.7.</p>
</div>
<table>
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
<p>Specifies the name of the consensus member.</p>
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
<p>Specifies the services that this member is capable of providing.</p>
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
<p>Indicates the number of Pods that perform this role.
The default is 1 for <code>Leader</code>, 0 for <code>Learner</code>, others for <code>Followers</code>.</p>
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
<p>ConsensusSetSpec is deprecated since v0.7.</p>
</div>
<table>
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
<p>Represents a single leader in the consensus set.</p>
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
<p>Members of the consensus set that have voting rights but are not the leader.</p>
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
<p>Represents a member of the consensus set that does not have voting rights.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ContainerVars">ContainerVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PodVars">PodVars</a>)
</p>
<div>
<p>ContainerVars defines the vars that can be referenced from a Container.</p>
</div>
<table>
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
<p>The name of the container.</p>
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
<p>Container port to reference.</p>
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
<p>CredentialVar defines the value of credential variable.</p>
</div>
<table>
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
<p>Specifies an optional variable. Only one of the following may be specified.
Variable references, denoted by $(VAR_NAME), are expanded using previously defined
environment variables in the container and any service environment variables.
If a variable cannot be resolved, the reference in the input string remains unchanged.</p>
<p>Double $$ are reduced to a single $, enabling the escaping of the $(VAR_NAME) syntax.
For instance, &ldquo;$$(VAR_NAME)&rdquo; will produce the string literal &ldquo;$(VAR_NAME)&rdquo;.
Escaped references will never be expanded, irrespective of the variable&rsquo;s existence.
The default value is &ldquo;&rdquo;.</p>
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
<p>Defines the source for the environment variable&rsquo;s value. This cannot be used if the value is not empty.</p>
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
<p>CredentialVars defines the vars that can be referenced from a Credential (SystemAccount).
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
<p>CustomLabelSpec is deprecated since v0.8.</p>
</div>
<table>
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
<p>The key of the label.</p>
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
<p>The value of the label.</p>
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
<p>The resources that will be patched with the label.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.CustomOpsComponent">CustomOpsComponent
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CustomOpsSpec">CustomOpsSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the unique identifier of the cluster component</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.Parameter">
[]Parameter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the parameters for this operation as declared in the opsDefinition.spec.parametersSchema.</p>
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
<code>serviceAccountName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>parallelism</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
Kubernetes api utils intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the execution concurrency. By default, all incoming Components will be executed simultaneously.
The value can be an absolute number (e.g., 5) or a percentage of desired components (e.g., 10%).
The absolute number is calculated from the percentage by rounding up.
For instance, if the percentage value is 10% and the components length is 1,
the calculated number will be rounded up to 1.</p>
</td>
</tr>
<tr>
<td>
<code>components</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CustomOpsComponent">
[]CustomOpsComponent
</a>
</em>
</td>
<td>
<p>Defines which components need to perform the actions defined by this OpsDefinition.
At least one component is required. The components are identified by their name and can be merged or retained.</p>
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
<p>CustomParametersValidation Defines a list of configuration items with their names, default values, descriptions,
types, and constraints.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cue</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Hold a string that contains a script written in CUE language that defines a list of configuration items.
Each item is detailed with its name, default value, description, type (e.g. string, integer, float),
and constraints (permissible values or the valid range of values).</p>
<p>CUE (Configure, Unify, Execute) is a declarative language designed for defining and validating
complex data configurations.
It is particularly useful in environments like K8s where complex configurations and validation rules are common.</p>
<p>This script functions as a validator for user-provided configurations, ensuring compliance with
the established specifications and constraints.</p>
</td>
</tr>
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
<p>Generated from the &lsquo;cue&rsquo; field and transformed into a JSON format.</p>
</td>
</tr>
</tbody>
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
<p>Specifies the environment variable key in the mapping.</p>
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
<p>Specifies the source used to derive the value of the environment variable,
which typically represents the tool image required for backup operation.</p>
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
<p>Variable references <code>$(VAR_NAME)</code> are expanded using the previously defined variables in the current context.</p>
<p>If a variable cannot be resolved, the reference in the input string will be unchanged.
Double <code>$$</code> are reduced to a single <code>$</code>, which allows for escaping the <code>$(VAR_NAME)</code> syntax: i.e.</p>
<ul>
<li><code>$$(VAR_NAME)</code> will produce the string literal <code>$(VAR_NAME)</code>.</li>
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
<p>ExecAction describes an Action that executes a command inside a container.
Which may run as a K8s job or be executed inside the Lorry sidecar container, depending on the implementation.
Future implementations will standardize execution within Lorry.</p>
</div>
<table>
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
<p>Specifies the command to be executed inside the container.
The working directory for this command is the container&rsquo;s root directory(&lsquo;/&rsquo;).
Commands are executed directly without a shell environment, meaning shell-specific syntax (&lsquo;|&rsquo;, etc.) is not supported.
If the shell is required, it must be explicitly invoked in the command.</p>
<p>A successful execution is indicated by an exit status of 0; any non-zero status signifies a failure.</p>
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
<p>Args represents the arguments that are passed to the <code>command</code> for execution.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentDefRef">ComponentDefRef</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.GVKResource">GVKResource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CustomLabelSpec">CustomLabelSpec</a>)
</p>
<div>
<p>GVKResource is deprecated since v0.8.</p>
</div>
<table>
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
<p>Represents the GVK of a resource, such as &ldquo;v1/Pod&rdquo;, &ldquo;apps/v1/StatefulSet&rdquo;, etc.
When a resource matching this is found by the selector, a custom label will be added if it doesn&rsquo;t already exist,
or updated if it does.</p>
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
<p>A label query used to filter a set of resources.</p>
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
<p>HScaleDataClonePolicyType defines the data clone policy to be used during horizontal scaling.
This policy determines how data is handled when new nodes are added to the cluster.
The policy can be set to <code>None</code>, <code>CloneVolume</code>, or <code>Snapshot</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CloneVolume&#34;</p></td>
<td><p>HScaleDataClonePolicyCloneVolume indicates that data will be cloned from existing volumes during horizontal scaling.</p>
</td>
</tr><tr><td><p>&#34;Snapshot&#34;</p></td>
<td><p>HScaleDataClonePolicyFromSnapshot indicates that data will be cloned from a snapshot during horizontal scaling.</p>
</td>
</tr><tr><td><p>&#34;None&#34;</p></td>
<td><p>HScaleDataClonePolicyNone indicates that no data cloning will occur during horizontal scaling.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HTTPAction">HTTPAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Action">Action</a>)
</p>
<div>
<p>HTTPAction describes an Action that triggers HTTP requests.
HTTPAction is to be implemented in future version.</p>
</div>
<table>
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
<p>Specifies the endpoint to be requested on the HTTP server.</p>
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
<p>Specifies the target port for the HTTP request.
It can be specified either as a numeric value in the range of 1 to 65535,
or as a named port that meets the IANA_SVC_NAME specification.</p>
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
<p>Indicates the server&rsquo;s domain name or IP address. Defaults to the Pod&rsquo;s IP.
Prefer setting the &ldquo;Host&rdquo; header in httpHeaders when needed.</p>
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
<p>Designates the protocol used to make the request, such as HTTP or HTTPS.
If not specified, HTTP is used by default.</p>
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
<p>Represents the type of HTTP request to be made, such as &ldquo;GET,&rdquo; &ldquo;POST,&rdquo; &ldquo;PUT,&rdquo; etc.
If not specified, &ldquo;GET&rdquo; is the default method.</p>
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
<p>Allows for the inclusion of custom headers in the request.
HTTP permits the use of repeated headers.</p>
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
<p>HorizontalScalePolicy is deprecated since v0.8.</p>
</div>
<table>
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
<p>Determines the data synchronization method when a component scales out.
The policy can be one of the following: &#123;None, CloneVolume&#125;. The default policy is <code>None</code>.</p>
<ul>
<li><code>None</code>: This is the default policy. It creates an empty volume without data cloning.</li>
<li><code>CloneVolume</code>: This policy clones data to newly scaled pods. It first tries to use a volume snapshot.
If volume snapshot is not enabled, it will attempt to use a backup tool. If neither method works, it will report an error.</li>
<li><code>Snapshot</code>: This policy is deprecated and is an alias for CloneVolume.</li>
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
<p>Refers to the backup policy template.</p>
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
<p>Specifies the volumeMount of the container to backup.
This only works if Type is not None. If not specified, the first volumeMount will be selected.</p>
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
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies instances to be added and/or deleted for the workloads.
Name and Replicas should be provided. Other fields will simply be ignored.
The Replicas will be overridden if an existing InstanceTemplate is matched by Name.
Or the InstanceTemplate will be added as a new one.</p>
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
<p>Specifies instances to be scaled in with dedicated names in the list.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HostNetwork">HostNetwork
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
<code>containerPorts</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.HostNetworkContainerPort">
[]HostNetworkContainerPort
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The list of container ports that are required by the component.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.HostNetworkContainerPort">HostNetworkContainerPort
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.HostNetwork">HostNetwork</a>)
</p>
<div>
</div>
<table>
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
<p>Container specifies the target container within the pod.</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Ports are named container ports within the specified container.
These container ports must be defined in the container for proper port allocation.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.InstanceTemplate">InstanceTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentSpec">ComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.HorizontalScaling">HorizontalScaling</a>, <a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">LastComponentConfiguration</a>)
</p>
<div>
<p>InstanceTemplate allows customization of individual replica configurations within a Component,
without altering the base component template defined in ClusterComponentSpec.
It enables the application of distinct settings to specific instances (replicas),
providing flexibility while maintaining a common configuration baseline.</p>
</div>
<table>
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
<p>Name specifies the unique name of the instance Pod created using this InstanceTemplate.
This name is constructed by concatenating the component&rsquo;s name, the template&rsquo;s name, and the instance&rsquo;s ordinal
using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
The specified name overrides any default naming conventions or patterns.</p>
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
<p>Specifies the number of instances (Pods) to create from this InstanceTemplate.
This field allows setting how many replicated instances of the component,
with the specific overrides in the InstanceTemplate, are created.
The default value is 1. A value of 0 disables instance creation.</p>
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
<p>Specifies a map of key-value pairs to be merged into the Pod&rsquo;s existing annotations.
Existing keys will have their values overwritten, while new keys will be added to the annotations.</p>
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
<p>Specifies a map of key-value pairs that will be merged into the Pod&rsquo;s existing labels.
Values for existing keys will be overwritten, and new keys will be added.</p>
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
<p>Specifies an override for the first container&rsquo;s image in the pod.</p>
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
<p>Specifies the name of the node where the Pod should be scheduled.
If set, the Pod will be directly assigned to the specified node, bypassing the Kubernetes scheduler.
This is useful for controlling Pod placement on specific nodes.</p>
<p>Important considerations:
- <code>nodeName</code> bypasses default scheduling constraints (e.g., resource requirements, node selectors, affinity rules).
- It is the user&rsquo;s responsibility to ensure the node is suitable for the Pod.
- If the node is unavailable, the Pod will remain in &ldquo;Pending&rdquo; state until the node is available or the Pod is deleted.</p>
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
<p>Defines NodeSelector to override.</p>
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
<p>Tolerations specifies a list of tolerations to be applied to the Pod, allowing it to tolerate node taints.
This field can be used to add new tolerations or override existing ones.</p>
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
<p>Specifies an override for the resource requirements of the first container in the Pod.
This field allows for customizing resource allocation (CPU, memory, etc.) for the container.</p>
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
<p>Defines Env to override.
Add new or override existing envs.</p>
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
<p>Defines Volumes to override.
Add new or override existing volumes.</p>
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
<p>Defines VolumeMounts to override.
Add new or override existing volume mounts of the first container in the pod.</p>
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
<p>Defines VolumeClaimTemplates to override.
Add new or override existing volume claim templates.</p>
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
<p>Issuer defines the TLS certificates issuer for the cluster.</p>
</div>
<table>
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
<p>The issuer for TLS certificates.
It only allows two enum values: <code>KubeBlocks</code> and <code>UserProvided</code>.</p>
<ul>
<li><code>KubeBlocks</code> indicates that the self-signed TLS certificates generated by the KubeBlocks Operator will be used.</li>
<li><code>UserProvided</code> means that the user is responsible for providing their own CA, Cert, and Key.
In this case, the user-provided CA certificate, server certificate, and private key will be used
for TLS communication.</li>
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
<p>SecretRef is the reference to the secret that contains user-provided certificates.
It is required when the issuer is set to <code>UserProvided</code>.</p>
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
<p>IssuerName defines the name of the TLS certificates issuer.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;KubeBlocks&#34;</p></td>
<td><p>IssuerKubeBlocks represents certificates that are signed by the KubeBlocks Operator.</p>
</td>
</tr><tr><td><p>&#34;UserProvided&#34;</p></td>
<td><p>IssuerUserProvided indicates that the user has provided their own CA-signed certificates.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.JSONPatchOperation">JSONPatchOperation
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
</p>
<div>
</div>
<table>
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
<p>Represents the type of JSON patch operation. It supports the following values: &lsquo;add&rsquo;, &lsquo;remove&rsquo;, &lsquo;replace&rsquo;.</p>
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
<p>Represents the json patch path.</p>
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
<p>Represents the value to be used in the JSON patch operation.</p>
</td>
</tr>
</tbody>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.LastConfiguration">LastConfiguration</a>, <a href="#apps.kubeblocks.io/v1alpha1.OverrideBy">OverrideBy</a>)
</p>
<div>
</div>
<table>
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
<tr>
<td>
<code>instances</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.InstanceTemplate">
InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the last instances of the component.</p>
</td>
</tr>
<tr>
<td>
<code>offlineInstances</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the last offline instances of the component.</p>
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
<p>LegacyRenderedTemplateSpec describes the configuration extension for the lazy rendered template.
Deprecated: LegacyRenderedTemplateSpec has been deprecated since 0.9.0 and will be removed in 0.10.0</p>
</div>
<table>
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
<p>Extends the configuration template.</p>
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
<p>LetterCase defines the available cases to be used in password generation.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;LowerCases&#34;</p></td>
<td><p>LowerCases represents the use of lower case letters only.</p>
</td>
</tr><tr><td><p>&#34;MixedCases&#34;</p></td>
<td><p>MixedCases represents the use of a mix of both lower and upper case letters.</p>
</td>
</tr><tr><td><p>&#34;UpperCases&#34;</p></td>
<td><p>UpperCases represents the use of upper case letters only.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.LifecycleActionHandler">LifecycleActionHandler
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentLifecycleActions">ComponentLifecycleActions</a>, <a href="#apps.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe</a>)
</p>
<div>
<p>LifecycleActionHandler describes the implementation of a specific lifecycle action.</p>
<p>Each action is deemed successful if it returns an exit code of 0 for command executions,
or an HTTP 200 status for HTTP(s) actions.
Any other exit code or HTTP status is considered an indication of failure.</p>
</div>
<table>
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
<p>Specifies the name of the predefined action handler to be invoked for lifecycle actions.</p>
<p>Lorry, as a sidecar agent co-located with the database container in the same Pod,
includes a suite of built-in action implementations that are tailored to different database engines.
These are known as &ldquo;builtin&rdquo; handlers, includes: <code>mysql</code>, <code>redis</code>, <code>mongodb</code>, <code>etcd</code>,
<code>postgresql</code>, <code>official-postgresql</code>, <code>apecloud-postgresql</code>, <code>wesql</code>, <code>oceanbase</code>, <code>polardbx</code>.</p>
<p>If the <code>builtinHandler</code> field is specified, it instructs Lorry to utilize its internal built-in action handler
to execute the specified lifecycle actions.</p>
<p>The <code>builtinHandler</code> field is of type <code>BuiltinActionHandlerType</code>,
which represents the name of the built-in handler.
The <code>builtinHandler</code> specified within the same <code>ComponentLifecycleActions</code> should be consistent across all
actions.
This means that if you specify a built-in handler for one action, you should use the same handler
for all other actions throughout the entire <code>ComponentLifecycleActions</code> collection.</p>
<p>If you need to define lifecycle actions for database engines not covered by the existing built-in support,
or when the pre-existing built-in handlers do not meet your specific needs,
you can use the <code>customHandler</code> field to define your own action implementation.</p>
<p>Deprecation Notice:</p>
<ul>
<li>In the future, the <code>builtinHandler</code> field will be deprecated in favor of using the <code>customHandler</code> field
for configuring all lifecycle actions.</li>
<li>Instead of using a name to indicate the built-in action implementations in Lorry,
the recommended approach will be to explicitly invoke the desired action implementation through
a gRPC interface exposed by the sidecar agent.</li>
<li>Developers will have the flexibility to either use the built-in action implementations provided by Lorry
or develop their own sidecar agent to implement custom actions and expose them via gRPC interfaces.</li>
<li>This change will allow for greater customization and extensibility of lifecycle actions,
as developers can create their own &ldquo;builtin&rdquo; implementations tailored to their specific requirements.</li>
</ul>
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
<p>Specifies a user-defined hook or procedure that is called to perform the specific lifecycle action.
It offers a flexible and expandable approach for customizing the behavior of a Component by leveraging
tailored actions.</p>
<p>An Action can be implemented as either an ExecAction or an HTTPAction, with future versions planning
to support GRPCAction,
thereby accommodating unique logic for different database systems within the Action&rsquo;s framework.</p>
<p>In future iterations, all built-in handlers are expected to transition to GRPCAction.
This change means that Lorry or other sidecar agents will expose the implementation of actions
through a GRPC interface for external invocation.
Then the controller will interact with these actions via GRPCAction calls.</p>
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
<p>Specifies a descriptive label for the log type, such as &lsquo;slow&rsquo; for a MySQL slow log file.
It provides a clear identification of the log&rsquo;s purpose and content.</p>
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
<p>Specifies the paths or patterns identifying where the log files are stored.
This field allows the system to locate and manage log files effectively.</p>
<p>Examples:</p>
<ul>
<li>/home/postgres/pgdata/pgroot/data/log/postgresql-*</li>
<li>/data/mysql/log/mysqld-error.log</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.MatchExpressions">MatchExpressions
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CompletionProbe">CompletionProbe</a>)
</p>
<div>
</div>
<table>
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
<p>Defines a failure condition for an action using a Go template expression.
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
<p>Defines a success condition for an action using a Go template expression.
Should evaluate to either <code>true</code> or <code>false</code>.
The current resource object is parsed into the Go template.
for example, using &lsquo;&#123;&#123; eq .spec.replicas 1 &#125;&#125;&rsquo;</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.MonitorKind">MonitorKind
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.MonitorSource">MonitorSource</a>)
</p>
<div>
<p>MonitorKind defines the kind of monitor.</p>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.MonitorSource">MonitorSource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSource">SidecarContainerSource</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MonitorKind">
MonitorKind
</a>
</em>
</td>
<td>
<p>Defines the kind of monitor, such as metrics or logs.</p>
</td>
</tr>
<tr>
<td>
<code>scrapeConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PrometheusScrapeConfig">
PrometheusScrapeConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the scrape configuration for the prometheus.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ContainerVars">ContainerVars</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceVars">ServiceVars</a>)
</p>
<div>
</div>
<table>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsAction">OpsAction
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
<p>action name.</p>
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
<p>failurePolicy is the failure policy of the action. valid values Fail and Ignore.
- Fail: if the action failed, the opsRequest will be failed.
- Ignore: opsRequest will ignore the failure if the action is failed.</p>
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
<p>Refers to the parameter of the ParametersSchema.
The parameter will be used in the action.
If it is a &lsquo;workload&rsquo; and &lsquo;exec&rsquo; Action, they will be injected into the corresponding environment variable.
If it is a &lsquo;resourceModifier&rsquo; Action, parameter can be referenced using $() in completionProbe.matchExpressions and JsonPatches[*].Value.</p>
</td>
</tr>
<tr>
<td>
<code>workload</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsWorkloadAction">
OpsWorkloadAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the workload action and a corresponding workload will be created to execute this action.</p>
</td>
</tr>
<tr>
<td>
<code>exec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsExecAction">
OpsExecAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the exec action. This will call the kubectl exec interface.</p>
</td>
</tr>
<tr>
<td>
<code>resourceModifier</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsResourceModifierAction">
OpsResourceModifierAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the resource modifier to update the custom resource.</p>
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
<code>preConditions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreCondition">
[]PreCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the preconditions that must be met to run the actions for the operation.
if set, it will check the condition before the component run this operation.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodTemplates</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.TargetPodTemplate">
[]TargetPodTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the targetPodTemplate to be referenced by the action.</p>
</td>
</tr>
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
<em>(Optional)</em>
<p>Specifies the types of componentDefinitions supported by the operation.
It can reference certain variables of the componentDefinition.
If set, any component not meeting these conditions will be intercepted.</p>
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
<code>actions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OpsAction">
[]OpsAction
</a>
</em>
</td>
<td>
<p>The actions to be executed in the opsRequest are performed sequentially.</p>
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.TargetPodTemplate">TargetPodTemplate</a>)
</p>
<div>
</div>
<table>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsExecAction">OpsExecAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>targetPodTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<p>Refers to the spec.targetPodTemplates. Defines the target pods that need to execute exec actions.</p>
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
<p>Specifies the number of retries before marking the action as failed.</p>
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
<p>The command to execute.</p>
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
<p>The name of the container in the target pod to execute the command.
If not set, the first container is used.</p>
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
<code>preCheck</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PreCheckResult">
PreCheckResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the outcome of the preConditions check for the opsRequest. This result is crucial for determining the next steps in the operation.</p>
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
<code>overrideBy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.OverrideBy">
OverrideBy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Describes the configuration covered by the latest OpsRequest of the same kind.
when reconciling, this information will be used as a benchmark rather than the &lsquo;spec&rsquo;, such as &lsquo;Spec.HorizontalScaling&rsquo;.</p>
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
<code>force</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates if pre-checks should be bypassed, allowing the opsRequest to execute immediately. If set to true, pre-checks are skipped except for &lsquo;Start&rsquo; type.
Particularly useful when concurrent execution of VerticalScaling and HorizontalScaling opsRequests is required,
achievable through the use of the Force flag.</p>
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
<code>extras</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>A collection of additional key-value pairs that provide supplementary information for the opsRequest.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
</p>
<div>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.TypedObjectRef">
TypedObjectRef
</a>
</em>
</td>
<td>
<p>Refers to the Kubernetes objects that are required to be updated.</p>
</td>
</tr>
<tr>
<td>
<code>jsonPatches</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.JSONPatchOperation">
[]JSONPatchOperation
</a>
</em>
</td>
<td>
<p>Defines the set of patches that are used to perform updates on the resource object.</p>
</td>
</tr>
<tr>
<td>
<code>completionProbe</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.CompletionProbe">
CompletionProbe
</a>
</em>
</td>
<td>
<p>Provides a method to check if the action has been completed.</p>
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
<code>envRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.EnvVarRef">
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
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the JSONPath of the target pod. This is used to specify the exact location of the data within the JSON structure of the pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsWorkloadAction">OpsWorkloadAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsAction">OpsAction</a>)
</p>
<div>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.OpsWorkloadType">
OpsWorkloadType
</a>
</em>
</td>
<td>
<p>Defines the workload type of the action. Valid values include &ldquo;Job&rdquo; and &ldquo;Pod&rdquo;.
&ldquo;Job&rdquo; creates a job to execute the action.
&ldquo;Pod&rdquo; creates a pod to execute the action. Note that unlike jobs, if a pod is manually deleted, it will not consume backoffLimit times.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<p>Refers to the spec.targetPodTemplates.
This field defines the target pod for the current action.</p>
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
<p>Specifies the number of retries before marking the action as failed.</p>
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
<p>Represents the pod spec of the workload.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.OpsWorkloadType">OpsWorkloadType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsWorkloadAction">OpsWorkloadAction</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.OverrideBy">OverrideBy
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
<code>opsName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the opsRequest name.</p>
</td>
</tr>
<tr>
<td>
<code>LastComponentConfiguration</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.LastComponentConfiguration">
LastComponentConfiguration
</a>
</em>
</td>
<td>
<p>
(Members of <code>LastComponentConfiguration</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Parameter">Parameter
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.CustomOpsComponent">CustomOpsComponent</a>)
</p>
<div>
</div>
<table>
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
<p>The length of the password.</p>
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
<p>The number of digits in the password.</p>
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
<p>The number of symbols in the password.</p>
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
<p>The case of the letters in the password.</p>
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
<p>Seed to generate the account&rsquo;s password.
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
<p>Holds the payload data. This field is optional and can contain any type of data.
Not included in the JSON representation of the object.</p>
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
<p>Contains the desired access modes the volume should have.
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
<p>Represents the minimum resources the volume should have.
If the RecoverVolumeExpansionFailure feature is enabled, users are allowed to specify resource requirements that
are lower than the previous value but must still be higher than the capacity recorded in the status field of the claim.
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
<p>The name of the StorageClass required by the claim.
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
<p>Defines what type of volume is required by the claim, either Block or Filesystem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.Phase">Phase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterDefinitionStatus">ClusterDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ClusterVersionStatus">ClusterVersionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionStatus">ComponentDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentVersionStatus">ComponentVersionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.OpsDefinitionStatus">OpsDefinitionStatus</a>, <a href="#apps.kubeblocks.io/v1alpha1.ServiceDescriptorStatus">ServiceDescriptorStatus</a>)
</p>
<div>
<p>Phase represents the current status of the ClusterDefinition and ClusterVersion CR.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.PodAntiAffinity">PodAntiAffinity
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.Affinity">Affinity</a>)
</p>
<div>
<p>PodAntiAffinity defines the pod anti-affinity strategy.</p>
<p>This strategy determines how pods are scheduled in relation to other pods, with the aim of either spreading pods
across nodes (Preferred) or ensuring that certain pods do not share a node (Required).</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Preferred&#34;</p></td>
<td><p>Preferred indicates that the scheduler will try to enforce the anti-affinity rules, but it will not guarantee it.</p>
</td>
</tr><tr><td><p>&#34;Required&#34;</p></td>
<td><p>Required indicates that the scheduler must enforce the anti-affinity rules and will not schedule the pods unless
the rules are met.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PodAvailabilityPolicy">PodAvailabilityPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>)
</p>
<div>
<p>PodAvailabilityPolicy pod availability strategy.</p>
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
</tr><tr><td><p>&#34;None&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;UnAvailable&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PodSelectionPolicy">PodSelectionPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PodSelector">PodSelector</a>)
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
<h3 id="apps.kubeblocks.io/v1alpha1.PodSelector">PodSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.TargetPodTemplate">TargetPodTemplate</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the role of the target pod.</p>
</td>
</tr>
<tr>
<td>
<code>selectionPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodSelectionPolicy">
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
<tr>
<td>
<code>availability</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodAvailabilityPolicy">
PodAvailabilityPolicy
</a>
</em>
</td>
<td>
<p>Indicates the desired availability status of the pods to be selected.
valid values:
- &lsquo;Available&rsquo;: selects only available pods and terminates the action if none are found.
- &lsquo;PreferredAvailable&rsquo;: prioritizes the selection of available pods。
- &lsquo;None&rsquo;: there are no requirements for the availability of pods.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PodVarSelector">PodVarSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.VarSource">VarSource</a>)
</p>
<div>
<p>PodVarSelector selects a var from a Pod.</p>
</div>
<table>
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
<p>The pod to select from.</p>
</td>
</tr>
<tr>
<td>
<code>PodVars</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodVars">
PodVars
</a>
</em>
</td>
<td>
<p>
(Members of <code>PodVars</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PodVars">PodVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PodVarSelector">PodVarSelector</a>)
</p>
<div>
<p>PodVars defines the vars that can be referenced from a Pod.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1alpha1.ContainerVars">
ContainerVars
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
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
<p>PostStartAction is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the  post-start command to be executed.</p>
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
<p>Used to select the script that need to be referenced.
When defined, the scripts defined in scriptSpecs can be referenced within the CmdExecutorConfig.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PreCheckResult">PreCheckResult
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
<code>pass</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Indicates whether the preCheck operation was successful or not.</p>
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
<p>Provides additional details about the preCheck operation in a human-readable format.</p>
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
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.PreConditionExec">PreConditionExec
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
<em>(Optional)</em>
<p>Represents the unique key of the object.
either objectKey or actionName.</p>
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
<p>Refer to the action name of the OpsDefinition.spec.actions[*].name.
either objectKey or actionName.</p>
</td>
</tr>
<tr>
<td>
<code>actionTasks</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ActionTask">
[]ActionTask
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the tasks associated with an action. such as Jobs/Pods that executes action.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.PrometheusProtocol">PrometheusProtocol
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.PrometheusScrapeConfig">PrometheusScrapeConfig</a>)
</p>
<div>
<p>PrometheusProtocol defines the protocol of prometheus scrape metrics.</p>
</div>
<h3 id="apps.kubeblocks.io/v1alpha1.PrometheusScrapeConfig">PrometheusScrapeConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.BuiltinMonitorContainerRef">BuiltinMonitorContainerRef</a>, <a href="#apps.kubeblocks.io/v1alpha1.MonitorSource">MonitorSource</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metricsPath</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the http/https url path to scrape for metrics.
If empty, Prometheus uses the default value (e.g. <code>/metrics</code>).</p>
</td>
</tr>
<tr>
<td>
<code>metricsPort</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the port name to scrape for metrics.</p>
</td>
</tr>
<tr>
<td>
<code>protocol</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PrometheusProtocol">
PrometheusProtocol
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the schema to use for scraping.
<code>http</code> and <code>https</code> are the expected values unless you rewrite the <code>__scheme__</code> label via relabeling.
If empty, Prometheus uses the default value <code>http</code>.</p>
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
<p>ProtectedVolume is deprecated since v0.9, replaced with ComponentVolume.HighWatermark.</p>
</div>
<table>
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
<p>The Name of the volume to protect.</p>
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
<p>Defines the high watermark threshold for the volume, it will override the component level threshold.
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
<p>Deprecated since v0.9.</p>
</div>
<table>
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
<p>Specifies the method to provision an account.</p>
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
<p>Defines the scope within which the account is provisioned.</p>
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
<p>The statement to provision an account.</p>
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
<p>The external secret to refer.</p>
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
<p>ProvisionScope defines the scope of provision within a component.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;AllPods&#34;</p></td>
<td><p>AllPods indicates that accounts will be created for all pods within the component.</p>
</td>
</tr><tr><td><p>&#34;AnyPods&#34;</p></td>
<td><p>AnyPods indicates that accounts will be created only on a single pod within the component.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ProvisionSecretRef">ProvisionSecretRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ProvisionPolicy">ProvisionPolicy</a>, <a href="#apps.kubeblocks.io/v1alpha1.SystemAccount">SystemAccount</a>)
</p>
<div>
<p>ProvisionSecretRef represents the reference to a secret.</p>
</div>
<table>
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
<p>The unique identifier of the secret.</p>
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
<p>The namespace where the secret is located.</p>
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
<p>Deprecated since v0.9.</p>
</div>
<table>
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
<p>Specifies the statement required to create a new account with the necessary privileges.</p>
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
<p>Defines the statement required to update the password of an existing account.</p>
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
<p>Defines the statement required to delete an existing account.
Typically used in conjunction with the creation statement to delete an account before recreating it.
For example, one might use a <code>drop user if exists</code> statement followed by a <code>create user</code> statement to ensure a fresh account.</p>
<p>Deprecated: This field is deprecated and the update statement should be used instead.</p>
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
<p>RSMSpec is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies a list of roles defined within the system.</p>
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
<p>Defines the method used to probe a role.</p>
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
<p>Indicates the actions required for dynamic membership reconfiguration.</p>
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
<p>Describes the strategy for updating Members (Pods).</p>
<ul>
<li><code>Serial</code>: Updates Members sequentially to ensure minimum component downtime.</li>
<li><code>BestEffortParallel</code>: Updates Members in parallel to ensure minimum component write downtime.</li>
<li><code>Parallel</code>: Forces parallel updates.</li>
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
<p>Represents the policy applied during the most recent execution.</p>
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
<p>Represents the outcome of the most recent execution.</p>
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
<p>Represents the current revision of the configuration item.</p>
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
<p>Represents the number of pods where configuration changes were successfully applied.</p>
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
<p>Represents the total number of pods that require execution of configuration changes.</p>
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
<p>Represents the error message generated when the execution of configuration changes fails.</p>
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
<p>ReloadOptions defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.</p>
<p>Only one of the mechanisms can be specified at a time.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.UnixSignalTrigger">
UnixSignalTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to trigger a reload by sending a specific Unix signal to the process.</p>
</td>
</tr>
<tr>
<td>
<code>shellTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ShellTrigger">
ShellTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allows to execute a custom shell script to reload the process.</p>
</td>
</tr>
<tr>
<td>
<code>tplScriptTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.TPLScriptTrigger">
TPLScriptTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enables reloading process using a Go template script.</p>
</td>
</tr>
<tr>
<td>
<code>autoTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.AutoTrigger">
AutoTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Automatically perform the reload when specified conditions are met.</p>
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
<p>Defines the role&rsquo;s identifier. It is used to set the &ldquo;apps.kubeblocks.io/role&rdquo; label value
on the corresponding object.</p>
<p>This field is immutable once set.</p>
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
<p>Indicates whether a replica assigned this role is capable of providing services.</p>
<p>This field is immutable once set.</p>
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
<p>Determines if a replica in this role has the authority to perform write operations.
A writable replica can modify data, handle update operations.</p>
<p>This field is immutable once set.</p>
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
<p>Specifies whether a replica with this role has voting rights.
In distributed systems, this typically means the replica can participate in consensus decisions,
configuration changes, or other processes that require a quorum.</p>
<p>This field is immutable once set.</p>
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
<p>ReplicasLimit defines the valid range of number of replicas supported.</p>
</div>
<table>
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
<h3 id="apps.kubeblocks.io/v1alpha1.ReplicationSetSpec">ReplicationSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
</p>
<div>
<p>ReplicationSetSpec is deprecated since v0.7.</p>
</div>
<table>
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
<h3 id="apps.kubeblocks.io/v1alpha1.RerenderResourceType">RerenderResourceType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ComponentConfigSpec">ComponentConfigSpec</a>)
</p>
<div>
<p>RerenderResourceType defines the resource requirements for a component.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;hscale&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;vscale&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ResourceMeta">ResourceMeta
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigMapRef">ConfigMapRef</a>, <a href="#apps.kubeblocks.io/v1alpha1.SecretRef">SecretRef</a>)
</p>
<div>
<p>ResourceMeta encapsulates metadata and configuration for referencing ConfigMaps and Secrets as volumes.</p>
</div>
<table>
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
<p>Name is the name of the referenced ConfigMap or Secret object. It must conform to DNS label standards.</p>
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
<p>MountPoint is the filesystem path where the volume will be mounted.</p>
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
<p>SubPath specifies a path within the volume from which to mount.</p>
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
<p>AsVolumeFrom lists the names of containers in which the volume should be mounted.</p>
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
<p>Defines the maximum number of retry attempts that should be made for a given Action.
This value is set to 0 by default, indicating that no retries will be made.</p>
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
<p>Indicates the duration of time to wait between each retry attempt.
This value is set to 0 by default, indicating that there will be no delay between retry attempts.</p>
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
<p>Deprecated since v0.9</p>
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
<p>Specifies the number of seconds to wait after the container has started before the RoleProbe
begins to detect the container&rsquo;s role.</p>
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
<p>Specifies the frequency at which the probe is conducted. This value is expressed in seconds.
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
<p>Defines the backup method name that is defined in backupPolicy.</p>
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
<p>Represents the cron expression for schedule, with the timezone set in UTC.
Refer to <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a> for more details.</p>
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
<p>Determines the duration for which the backup should be retained.
The controller will remove all backups that are older than the RetentionPeriod.
For instance, a RetentionPeriod of <code>30d</code> will retain only the backups from the last 30 days.
Sample duration format:</p>
<ul>
<li>years: 	2y</li>
<li>months: 	6mo</li>
<li>days: 		30d</li>
<li>hours: 	12h</li>
<li>minutes: 	30m</li>
</ul>
<p>These durations can also be combined, for example: 30d12h30m.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SchedulingPolicy">SchedulingPolicy
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
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSelector is a selector which must be true for the pod to fit on a node.
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
<p>NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
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
<p>If specified, the cluster&rsquo;s scheduling constraints.</p>
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
<p>Attached to tolerate any taint that matches the triple <code>key,value,effect</code> using the matching operator <code>operator</code>.</p>
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
<p>TopologySpreadConstraints describes how a group of pods ought to spread across topology
domains. Scheduler will schedule pods in a way which abides by the constraints.
All topologySpreadConstraints are ANDed.</p>
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
<p>Represents the name of the ScriptSpec referent.</p>
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
<p>SecretRef defines a reference to a Secret.</p>
</div>
<table>
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
<p>Secret specifies the secret to be mounted as a volume.</p>
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
<p>Extends the above <code>serviceSpec.selector</code> by allowing you to specify defined role as selector for the service.
When <code>roleSelector</code> is set, it adds a label selector &ldquo;kubeblocks.io/role: &#123;roleSelector&#125;&rdquo;
to the <code>serviceSpec.selector</code>.
Example usage:</p>
<pre><code>  roleSelector: &quot;leader&quot;
</code></pre>
<p>In this example, setting <code>roleSelector</code> to &ldquo;leader&rdquo; will add a label selector
&ldquo;kubeblocks.io/role: leader&rdquo; to the <code>serviceSpec.selector</code>.
This means that the service will select and route traffic to Pods with the label
&ldquo;kubeblocks.io/role&rdquo; set to &ldquo;leader&rdquo;.</p>
<p>Note that if <code>generatePodOrdinalService</code> sets to true, RoleSelector will be ignored.
The <code>generatePodOrdinalService</code> flag takes precedence over <code>roleSelector</code> and generates a service for each Pod.</p>
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
<p>Specifies the type or nature of the service. Should represent a well-known application cluster type, such as &#123;mysql, redis, mongodb&#125;.
This field is case-insensitive and supports abbreviations for some well-known databases.
For instance, both <code>zk</code> and <code>zookeeper</code> will be recognized as a ZooKeeper cluster, and <code>pg</code>, <code>postgres</code>, <code>postgresql</code> will all be recognized as a PostgreSQL cluster.</p>
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
<p>Represents the version of the service reference.</p>
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
<p>Represents the endpoint of the service connection credential.</p>
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
<p>Represents the authentication details of the service connection credential.</p>
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
<p>Represents the port of the service connection credential.</p>
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
<p>Indicates the current lifecycle phase of the ServiceDescriptor. This can be either &lsquo;Available&rsquo; or &lsquo;Unavailable&rsquo;.</p>
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
<p>Provides a human-readable explanation detailing the reason for the current phase of the ServiceConnectionCredential.</p>
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
<p>Represents the generation number that has been processed by the controller.</p>
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
<p>ServicePort is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the identifier of the service reference declaration.
It corresponds to the serviceRefDeclaration name defined in either:</p>
<ul>
<li><code>componentDefinition.spec.serviceRefDeclarations[*].name</code></li>
<li><code>clusterDefinition.spec.componentDefs[*].serviceRefDeclarations[*].name</code> (deprecated)</li>
</ul>
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
<p>Specifies the namespace of the referenced Cluster or the namespace of the referenced ServiceDescriptor object.
If not provided, the referenced Cluster and ServiceDescriptor will be searched in the namespace of the current
Cluster by default.</p>
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
<p>Specifies the name of the KubeBlocks Cluster being referenced.
This is used when services from another KubeBlocks Cluster are consumed.</p>
<p>By default, the referenced KubeBlocks Cluster&rsquo;s <code>clusterDefinition.spec.connectionCredential</code>
will be utilized to bind to the current Component. This credential should include:
<code>endpoint</code>, <code>port</code>, <code>username</code>, and <code>password</code>.</p>
<p>Note:</p>
<ul>
<li>The <code>ServiceKind</code> and <code>ServiceVersion</code> specified in the service reference within the
ClusterDefinition are not validated when using this approach.</li>
<li>If both <code>cluster</code> and <code>serviceDescriptor</code> are present, <code>cluster</code> will take precedence.</li>
</ul>
<p>Deprecated since v0.9 since <code>clusterDefinition.spec.connectionCredential</code> is deprecated,
use <code>clusterRef</code> instead.
This field is maintained for backward compatibility and its use is discouraged.
Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.</p>
</td>
</tr>
<tr>
<td>
<code>clusterServiceSelector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefClusterSelector">
ServiceRefClusterSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterRef is used to reference a service provided by another KubeBlocks Cluster.
It specifies the ClusterService and the account credentials needed for access.</p>
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
<p>Specifies the name of the ServiceDescriptor object that describes the service provided by external sources.</p>
<p>When referencing a service provided by external sources, a ServiceDescriptor object is required to establish
the service binding.
The <code>serviceDescriptor.spec.serviceKind</code> and <code>serviceDescriptor.spec.serviceVersion</code> should match the serviceKind
and serviceVersion declared in the definition.</p>
<p>If both <code>cluster</code> and <code>serviceDescriptor</code> are specified, the <code>cluster</code> takes precedence.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefClusterSelector">ServiceRefClusterSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceRef">ServiceRef</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cluster</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the KubeBlocks Cluster being referenced.</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefServiceSelector">
ServiceRefServiceSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identifies a ClusterService from the list of services defined in <code>cluster.spec.services</code> of the referenced Cluster.</p>
</td>
</tr>
<tr>
<td>
<code>credential</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ServiceRefCredentialSelector">
ServiceRefCredentialSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the SystemAccount to authenticate and establish a connection with the referenced Cluster.
The SystemAccount should be defined in <code>componentDefinition.spec.systemAccounts</code>
of the Component providing the service in the referenced Cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefCredentialSelector">ServiceRefCredentialSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceRefClusterSelector">ServiceRefClusterSelector</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the component where the credential resides in.</p>
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
<p>The name of the credential (SystemAccount) to reference.</p>
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
<p>ServiceRefDeclaration represents a reference to a service that can be either provided by a KubeBlocks Cluster
or an external service.
It acts as a placeholder for the actual service reference, which is determined later when a Cluster is created.</p>
<p>The purpose of ServiceRefDeclaration is to declare a service dependency without specifying the concrete details
of the service.
It allows for flexibility and abstraction in defining service references within a Component.
By using ServiceRefDeclaration, you can define service dependencies in a declarative manner, enabling loose coupling
and easier management of service references across different components and clusters.</p>
<p>Upon Cluster creation, the ServiceRefDeclaration is bound to an actual service through the ServiceRef field,
effectively resolving and connecting to the specified service.</p>
</div>
<table>
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
<p>Specifies the name of the ServiceRefDeclaration.</p>
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
<p>Defines a list of constraints and requirements for services that can be bound to this ServiceRefDeclaration
upon Cluster creation.
Each ServiceRefDeclarationSpec defines a ServiceKind and ServiceVersion,
outlining the acceptable service types and versions that are compatible.</p>
<p>This flexibility allows a ServiceRefDeclaration to be fulfilled by any one of the provided specs.
For example, if it requires an OLTP database, specs for both MySQL and PostgreSQL are listed,
either MySQL or PostgreSQL services can be used when binding.</p>
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
<p>Specifies the type or nature of the service. This should be a well-known application cluster type, such as
&#123;mysql, redis, mongodb&#125;.
The field is case-insensitive and supports abbreviations for some well-known databases.
For instance, both <code>zk</code> and <code>zookeeper</code> are considered as a ZooKeeper cluster, while <code>pg</code>, <code>postgres</code>, <code>postgresql</code>
are all recognized as a PostgreSQL cluster.</p>
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
<p>Defines the service version of the service reference. This is a regular expression that matches a version number pattern.
For instance, <code>^8.0.8$</code>, <code>8.0.\d&#123;1,2&#125;$</code>, <code>^[v\-]*?(\d&#123;1,2&#125;\.)&#123;0,3&#125;\d&#123;1,2&#125;$</code> are all valid patterns.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceRefServiceSelector">ServiceRefServiceSelector
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceRefClusterSelector">ServiceRefClusterSelector</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The name of the component where the service resides in.</p>
<p>It is required when referencing a component service.</p>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the service to reference.</p>
<p>Leave it empty to reference the default service. Set it to &ldquo;headless&rdquo; to reference the default headless service.
If the referenced service is a pod-service, there will be multiple service objects matched,
and the resolved value will be presented in the following format: service1.name,service2.name&hellip;</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The port name of the service to reference.</p>
<p>If there is a non-zero node-port exist for the matched service port, the node-port will be selected first.
If the referenced service is a pod-service, there will be multiple service objects matched,
and the resolved value will be presented in the following format: service1.name:port1,service2.name:port2&hellip;</p>
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
<p>ServiceRefVars defines the vars that can be referenced from a ServiceRef.</p>
</div>
<table>
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
<p>ServiceSpec is deprecated since v0.8.</p>
</div>
<table>
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
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.ServiceVars">ServiceVars
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ServiceVarSelector">ServiceVarSelector</a>)
</p>
<div>
<p>ServiceVars defines the vars that can be referenced from a Service.</p>
</div>
<table>
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
<p>Port references a port or node-port defined in the service.</p>
<p>If the referenced service is a pod-service, there will be multiple service objects matched,
and the value will be presented in the following format: service1.name:port1,service2.name:port2&hellip;</p>
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
<p>ShardingSpec defines how KubeBlocks manage dynamic provisioned shards.
A typical design pattern for distributed databases is to distribute data across multiple shards,
with each shard consisting of multiple replicas.
Therefore, KubeBlocks supports representing a shard with a Component and dynamically instantiating Components
using a template when shards are added.
When shards are removed, the corresponding Components are also deleted.</p>
</div>
<table>
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
<p>Represents the common parent part of all shard names.
This identifier is included as part of the Service DNS name and must comply with IANA service naming rules.
It is used to generate the names of underlying Components following the pattern <code>$(shardingSpec.name)-$(ShardID)</code>.
ShardID is a random string that is appended to the Name to generate unique identifiers for each shard.
For example, if the sharding specification name is &ldquo;my-shard&rdquo; and the ShardID is &ldquo;abc&rdquo;, the resulting component name
would be &ldquo;my-shard-abc&rdquo;.</p>
<p>Note that the name defined in component template(<code>shardingSpec.template.name</code>) will be disregarded
when generating the component names of the shards. The <code>shardingSpec.name</code> field takes precedence.</p>
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
<p>The template for generating Components for shards, where each shard consists of one Component.
This field is of type ClusterComponentSpec, which encapsulates all the required details and
definitions for creating and managing the Components.
KubeBlocks uses this template to generate a set of identical Components or shards.
All the generated Components will have the same specifications and definitions as specified in the <code>template</code> field.</p>
<p>This allows for the creation of multiple Components with consistent configurations,
enabling sharding and distribution of workloads across Components.</p>
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
<p>Specifies the desired number of shards.
Users can declare the desired number of shards through this field.
KubeBlocks dynamically creates and deletes Components based on the difference
between the desired and actual number of shards.
KubeBlocks provides lifecycle management for sharding, including:</p>
<ul>
<li>Executing the postProvision Action defined in the ComponentDefinition when the number of shards increases.
This allows for custom actions to be performed after a new shard is provisioned.</li>
<li>Executing the preTerminate Action defined in the ComponentDefinition when the number of shards decreases.
This enables custom cleanup or data migration tasks to be executed before a shard is terminated.
Resources and data associated with the corresponding Component will also be deleted.</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SidecarContainerSource">SidecarContainerSource
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSpec">SidecarContainerSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>monitor</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.MonitorSource">
MonitorSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the function or purpose of the container, such as the monitor type sidecar.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.SidecarContainerSpec">SidecarContainerSpec
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
<code>Container</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#container-v1-core">
Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>
(Members of <code>Container</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>SidecarContainerSource</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.SidecarContainerSource">
SidecarContainerSource
</a>
</em>
</td>
<td>
<p>
(Members of <code>SidecarContainerSource</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Define the function or purpose of the container, such as the monitor type sidecar.
In order to allow prometheus to scrape metrics from the sidecar container, the schema, port, and url will be injected into the annotation of the service.</p>
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
<p>StatefulSetSpec is deprecated since v0.7.</p>
</div>
<table>
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
<p>Specifies the strategy for updating Pods.
For workloadType=<code>Consensus</code>, the update strategy can be one of the following:</p>
<ul>
<li><code>Serial</code>: Updates Members sequentially to minimize component downtime.</li>
<li><code>BestEffortParallel</code>: Updates Members in parallel to minimize component write downtime. Majority remains online
at all times.</li>
<li><code>Parallel</code>: Forces parallel updates.</li>
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
<p>Controls the creation of pods during initial scale up, replacement of pods on nodes, and scaling down.</p>
<ul>
<li><code>OrderedReady</code>: Creates pods in increasing order (pod-0, then pod-1, etc). The controller waits until each pod
is ready before continuing. Pods are removed in reverse order when scaling down.</li>
<li><code>Parallel</code>: Creates pods in parallel to match the desired scale without waiting. All pods are deleted at once
when scaling down.</li>
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
<p>Specifies the low-level StatefulSetUpdateStrategy to be used when updating Pods in the StatefulSet upon a
revision to the Template.
<code>UpdateStrategy</code> will be ignored if this is provided.</p>
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
<p>StatelessSetSpec is deprecated since v0.7.</p>
</div>
<table>
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
<p>Specifies the deployment strategy that will be used to replace existing pods with new ones.</p>
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
<p>SwitchPolicyType defines the types of switch policies that can be applied to a cluster.</p>
<p>Currently, only the Noop policy is supported. Support for MaximumAvailability and MaximumDataProtection policies is
planned for future releases.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;MaximumAvailability&#34;</p></td>
<td><p>MaximumAvailability represents a switch policy that aims for maximum availability. This policy will switch if the
primary is active and the synchronization delay is 0 according to the user-defined lagProbe data delay detection
logic. If the primary is down, it will switch immediately.
This policy is intended for future support.</p>
</td>
</tr><tr><td><p>&#34;MaximumDataProtection&#34;</p></td>
<td><p>MaximumDataProtection represents a switch policy focused on maximum data protection. This policy will only switch
if the primary is active and the synchronization delay is 0, based on the user-defined lagProbe data lag detection
logic. If the primary is down, it will switch only if it can be confirmed that the primary and secondary data are
consistent. Otherwise, it will not switch.
This policy is planned for future implementation.</p>
</td>
</tr><tr><td><p>&#34;Noop&#34;</p></td>
<td><p>Noop indicates that KubeBlocks will not perform any high-availability switching for the components. Users are
required to implement their own HA solution or integrate an existing open-source HA solution.</p>
</td>
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
<p>SwitchoverAction is deprecated since v0.8.</p>
</div>
<table>
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
<p>Specifies the switchover command.</p>
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
<p>Used to select the script that need to be referenced.
When defined, the scripts defined in scriptSpecs can be referenced within the SwitchoverAction.CmdExecutorConfig.</p>
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
<p>SwitchoverShortSpec represents a condensed version of the SwitchoverSpec.</p>
<p>Deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Represents the configuration for the command executor.</p>
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
<p>SwitchoverSpec is deprecated since v0.8.</p>
</div>
<table>
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
<p>Represents the action of switching over to a specified candidate primary or leader instance.</p>
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
<p>Represents the action of switching over without specifying a candidate primary or leader instance.</p>
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
<p>Specifies the unique identifier for the account. This name is used by other entities to reference the account.</p>
<p>This field is immutable once set.</p>
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
<p>Indicates if this account is the unique system initialization account (e.g., MySQL root).
Only one system initialization account is permitted.</p>
<p>This field is immutable once set.</p>
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
<p>Defines the statement used to create the account with the necessary privileges.</p>
<p>This field is immutable once set.</p>
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
<p>Specifies the policy for generating the account&rsquo;s password.</p>
<p>This field is immutable once set.</p>
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
<p>Refers to the secret from which data will be copied to create the new account.</p>
<p>This field is immutable once set.</p>
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
<p>Deprecated since v0.9.</p>
</div>
<table>
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
<p>The unique identifier of a system account.</p>
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
<p>Outlines the strategy for creating the account.</p>
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
<p>SystemAccountShortSpec represents a condensed version of the SystemAccountSpec.</p>
<p>Deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Configures the method for obtaining the client SDK and executing statements.</p>
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
<p>Deprecated since v0.8, be replaced by <code>componentDefinition.spec.systemAccounts</code> and
<code>componentDefinition.spec.lifecycleActions.accountProvision</code>.</p>
</div>
<table>
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
<p>Configures how to obtain the client SDK and execute statements.</p>
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
<p>Defines the pattern used to generate passwords for system accounts.</p>
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
<p>Defines the configuration settings for system accounts.</p>
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
<p>A boolean flag that indicates whether the component should use Transport Layer Security (TLS)
for secure communication.
When set to true, the component will be configured to use TLS encryption for its network connections.
This ensures that the data transmitted between the component and its clients or other components is encrypted
and protected from unauthorized access.
If TLS is enabled, the component may require additional configuration,
such as specifying TLS certificates and keys, to properly set up the secure communication channel.</p>
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
<p>Specifies the configuration for the TLS certificates issuer.
It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
Required when TLS is enabled.</p>
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
<p>Name of the Secret that contains user-provided certificates.</p>
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
<p>Key of CA cert in Secret</p>
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
<p>Key of Cert in Secret</p>
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
<p>If <code>backupPolicy.componentDefRef</code> (a legacy and deprecated API) is set, the secret defined in
<code>clusterDefinition.spec.ConnectionCredential</code> is used instead.</p>
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
<code>connectionCredentialKey</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.ConnectionCredentialKey">
ConnectionCredentialKey
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the keys of the connection credential secret defined in <code>clusterDefinition.spec.ConnectionCredential</code>.
It will be ignored when the <code>account</code> is set.</p>
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
<p>TargetPodSelector defines how to select pod(s) to execute an Action.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.TargetPodTemplate">TargetPodTemplate
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
<p>Represents the template name.</p>
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
<em>(Optional)</em>
<p>Defines the environment variables that need to be referenced from the target component pod, and will be injected into the pod&rsquo;s containers.</p>
</td>
</tr>
<tr>
<td>
<code>podSelector</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodSelector">
PodSelector
</a>
</em>
</td>
<td>
<p>Used to identify the target pod.</p>
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
<p>Specifies the mount points for the volumes defined in the <code>Volumes</code> section for the action pod.</p>
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
<p>TenancyType defines the type of tenancy for cluster tenant resources.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;DedicatedNode&#34;</p></td>
<td><p>DedicatedNode means each pod runs on their own dedicated node.</p>
</td>
</tr><tr><td><p>&#34;SharedNode&#34;</p></td>
<td><p>SharedNode means multiple pods may share the same node.</p>
</td>
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
<h3 id="apps.kubeblocks.io/v1alpha1.TypedObjectRef">TypedObjectRef
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.OpsResourceModifierAction">OpsResourceModifierAction</a>)
</p>
<div>
</div>
<table>
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
<p>Defines the group for the resource being referenced.
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
<h3 id="apps.kubeblocks.io/v1alpha1.UpdateStrategy">UpdateStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentSpec">ClusterComponentSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.ComponentDefinitionSpec">ComponentDefinitionSpec</a>, <a href="#apps.kubeblocks.io/v1alpha1.StatefulSetSpec">StatefulSetSpec</a>)
</p>
<div>
<p>UpdateStrategy defines the update strategy for cluster components. This strategy determines how updates are applied
across the cluster.
The available strategies are <code>Serial</code>, <code>BestEffortParallel</code>, and <code>Parallel</code>.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;BestEffortParallel&#34;</p></td>
<td><p>BestEffortParallelStrategy indicates that the replicas are updated in parallel, with the operator making
a best-effort attempt to update as many replicas as possible concurrently
while maintaining the component&rsquo;s availability.
Unlike the <code>Parallel</code> strategy, the <code>BestEffortParallel</code> strategy aims to ensure that a minimum number
of replicas remain available during the update process to maintain the component&rsquo;s quorum and functionality.</p>
<p>For example, consider a component with 5 replicas. To maintain the component&rsquo;s availability and quorum,
the operator may allow a maximum of 2 replicas to be simultaneously updated. This ensures that at least
3 replicas (a quorum) remain available and functional during the update process.</p>
<p>The <code>BestEffortParallel</code> strategy strikes a balance between update speed and component availability.</p>
</td>
</tr><tr><td><p>&#34;Parallel&#34;</p></td>
<td><p>ParallelStrategy indicates that updates are applied simultaneously to all Pods of a Component.
The replicas are updated in parallel, with the operator updating all replicas concurrently.
This strategy provides the fastest update time but may lead to a period of reduced availability or
capacity during the update process.</p>
</td>
</tr><tr><td><p>&#34;Serial&#34;</p></td>
<td><p>SerialStrategy indicates that updates are applied one at a time in a sequential manner.
The operator waits for each replica to be updated and ready before proceeding to the next one.
This ensures that only one replica is unavailable at a time during the update process.</p>
</td>
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
<p>UserResourceRefs defines references to user-defined secrets and config maps.</p>
</div>
<table>
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
<p>SecretRefs defines the user-defined secrets.</p>
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
<p>ConfigMapRefs defines the user-defined config maps.</p>
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
<p>Determine the appropriate version of the backup tool image from ClusterVersion.</p>
<p>Deprecated since v0.9, since ClusterVersion is deprecated.</p>
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
<p>Determine the appropriate version of the backup tool image from ComponentDefinition.</p>
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
<p>Represents an array of names of ClusterVersion or ComponentDefinition that can be mapped to
the appropriate version of the backup tool image.</p>
<p>This mapping allows different versions of component images to correspond to specific versions of backup tool images.</p>
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
<p>Specifies the appropriate version of the backup tool image.</p>
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
<code>podVarRef</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1alpha1.PodVarSelector">
PodVarSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a defined var of a Pod.</p>
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
<h3 id="apps.kubeblocks.io/v1alpha1.VersionsContext">VersionsContext
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentVersion">ClusterComponentVersion</a>)
</p>
<div>
<p>VersionsContext is deprecated since v0.9.
This struct is maintained for backward compatibility and its use is discouraged.</p>
</div>
<table>
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
<p>Provides override values for ClusterDefinition.spec.componentDefs.podSpec.initContainers.
Typically used in scenarios such as updating application container images.</p>
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
<p>Provides override values for ClusterDefinition.spec.componentDefs.podSpec.containers.
Typically used in scenarios such as updating application container images.</p>
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
<p>VolumeProtectionSpec is deprecated since v0.9, replaced with ComponentVolume.HighWatermark.</p>
</div>
<table>
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
<p>The Volumes to be protected.</p>
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
<p>VolumeType defines the type of volume, specifically distinguishing between volumes used for backup data and those used for logs.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;data&#34;</p></td>
<td><p>VolumeTypeData indicates a volume designated for storing backup data. This type of volume is optimized for the
storage and retrieval of data backups, ensuring data persistence and reliability.</p>
</td>
</tr><tr><td><p>&#34;log&#34;</p></td>
<td><p>VolumeTypeLog indicates a volume designated for storing logs. This type of volume is optimized for log data,
facilitating efficient log storage, retrieval, and management.</p>
</td>
</tr></tbody>
</table>
<h3 id="apps.kubeblocks.io/v1alpha1.VolumeTypeSpec">VolumeTypeSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentDefinition">ClusterComponentDefinition</a>)
</p>
<div>
<p>VolumeTypeSpec is deprecated since v0.9, replaced with ComponentVolume.</p>
</div>
<table>
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
<p>Corresponds to the name of the VolumeMounts field in PodSpec.Container.</p>
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
<p>Type of data the volume will persistent.</p>
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
<p>WorkloadType defines the type of workload for the components of the ClusterDefinition.
It can be one of the following: <code>Stateless</code>, <code>Stateful</code>, <code>Consensus</code>, or <code>Replication</code>.</p>
<p>Deprecated since v0.8.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Consensus&#34;</p></td>
<td><p>Consensus represents a workload type involving distributed consensus algorithms for coordinated decision-making.</p>
</td>
</tr><tr><td><p>&#34;Replication&#34;</p></td>
<td><p>Replication represents a workload type that involves replication, typically used for achieving high availability
and fault tolerance.</p>
</td>
</tr><tr><td><p>&#34;Stateful&#34;</p></td>
<td><p>Stateful represents a workload type where components maintain state, and each instance has a unique identity.</p>
</td>
</tr><tr><td><p>&#34;Stateless&#34;</p></td>
<td><p>Stateless represents a workload type where components do not maintain state, and instances are interchangeable.</p>
</td>
</tr></tbody>
</table>
<hr/>
<h2 id="apps.kubeblocks.io/v1beta1">apps.kubeblocks.io/v1beta1</h2>
<div>
</div>
Resource Types:
<ul></ul>
<h3 id="apps.kubeblocks.io/v1beta1.AutoTrigger">AutoTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>, <a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">DynamicReloadAction</a>)
</p>
<div>
<p>AutoTrigger automatically perform the reload when specified conditions are met.</p>
</div>
<table>
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
<p>The name of the process.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.CfgFileFormat">CfgFileFormat
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">FormatterConfig</a>)
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
<h3 id="apps.kubeblocks.io/v1beta1.ConfigConstraint">ConfigConstraint
</h3>
<div>
<p>ConfigConstraint manages the parameters across multiple configuration files contained in a single configure template.
These configuration files should have the same format (e.g. ini, xml, properties, json).</p>
<p>It provides the following functionalities:</p>
<ol>
<li><strong>Parameter Value Validation</strong>: Validates and ensures compliance of parameter values with defined constraints.</li>
<li><strong>Dynamic Reload on Modification</strong>: Monitors parameter changes and triggers dynamic reloads to apply updates.</li>
<li><strong>Parameter Rendering in Templates</strong>: Injects parameters into templates to generate up-to-date configuration files.</li>
</ol>
</div>
<table>
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
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">
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
<code>dynamicReloadAction</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">
DynamicReloadAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the dynamic reload (dynamic reconfiguration) actions supported by the engine.
When set, the controller executes the scripts defined in these actions to handle dynamic parameter updates.</p>
<p>Dynamic reloading is triggered only if both of the following conditions are met:</p>
<ol>
<li>The modified parameters are listed in the <code>dynamicParameters</code> field.
If <code>dynamicParameterSelectedPolicy</code> is set to &ldquo;all&rdquo;, modifications to <code>staticParameters</code>
can also trigger a reload.</li>
<li><code>dynamicReloadAction</code> is set.</li>
</ol>
<p>If <code>dynamicReloadAction</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">reloadOptions:
tplScriptTrigger:
namespace: kb-system
scriptConfigMapRef: mysql-reload-script
sync: true
</code></pre>
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
<p>Indicates whether to consolidate dynamic reload and restart actions into a single restart.</p>
<ul>
<li>If true, updates requiring both actions will result in only a restart, merging the actions.</li>
<li>If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.</li>
</ul>
<p>This flag allows for more efficient handling of configuration changes by potentially eliminating
an unnecessary reload step.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>dynamicReloadAction</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>&ldquo;dynamic&rdquo; (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>&ldquo;all&rdquo;: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>reloadToolsImage</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ReloadToolsImage">
ReloadToolsImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tools container image used by ShellTrigger for dynamic reload.
If the dynamic reload action is triggered by a ShellTrigger, this field is required.
This image must contain all necessary tools for executing the ShellTrigger scripts.</p>
<p>Usually the specified image is referenced by the init container,
which is then responsible for copy the tools from the image to a bin volume.
This ensures that the tools are available to the &lsquo;config-manager&rsquo; sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>downwardActions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DownwardAction">
[]DownwardAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of actions to execute specified commands based on Pod labels.</p>
<p>It utilizes the K8s Downward API to mount label information as a volume into the pod.
The &lsquo;config-manager&rsquo; sidecar container watches for changes in the role label and dynamically invoke
registered commands (usually execute some SQL statements) when a change is detected.</p>
<p>It is designed for scenarios where:</p>
<ul>
<li>Replicas with different roles have different configurations, such as Redis primary &amp; secondary replicas.</li>
<li>After a role switch (e.g., from secondary to primary), some changes in configuration are needed
to reflect the new role.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of ScriptConfig Object.</p>
<p>Each ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the dynamic reload
and DownwardAction to perform specific tasks or configurations.</p>
</td>
</tr>
<tr>
<td>
<code>configSchemaTopLevelKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the top-level key in the &lsquo;configSchema.cue&rsquo; that organizes the validation rules for parameters.
This key must exist within the CUE script defined in &lsquo;configSchema.cue&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>configSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ConfigSchema">
ConfigSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines a list of parameters including their names, default values, descriptions,
types, and constraints (permissible values or the range of valid values).</p>
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
<p>List static parameters.
Modifications to any of these parameters require a restart of the process to take effect.</p>
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
<p>List dynamic parameters.
Modifications to these parameters trigger a configuration reload without requiring a process restart.</p>
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
<p>Lists the parameters that cannot be modified once set.
Attempting to change any of these parameters will be ignored.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicReloadSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to match labels on the pod to determine whether a dynamic reload should be performed.</p>
<p>In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
The <code>dynamicReloadSelector</code> allows you to specify label selectors to target the desired pods for the reload process.</p>
<p>If the <code>dynamicReloadSelector</code> is not specified or is nil, all pods managed by the workload will be considered for the dynamic
reload.</p>
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
Supported formats include <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>, <code>hcl</code>, <code>dotenv</code>, <code>properties</code>, and <code>toml</code>.</p>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
<p>Example:</p>
<pre><code>formatterConfig:
format: ini
iniConfig:
sectionName: mysqld
</code></pre>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintStatus">
ConfigConstraintStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ConfigConstraintPhase">ConfigConstraintPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintStatus">ConfigConstraintStatus</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintStatus">ConfigConstraintStatus</a>)
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
<h3 id="apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.ConfigConstraint">ConfigConstraint</a>)
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
<code>dynamicReloadAction</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">
DynamicReloadAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the dynamic reload (dynamic reconfiguration) actions supported by the engine.
When set, the controller executes the scripts defined in these actions to handle dynamic parameter updates.</p>
<p>Dynamic reloading is triggered only if both of the following conditions are met:</p>
<ol>
<li>The modified parameters are listed in the <code>dynamicParameters</code> field.
If <code>dynamicParameterSelectedPolicy</code> is set to &ldquo;all&rdquo;, modifications to <code>staticParameters</code>
can also trigger a reload.</li>
<li><code>dynamicReloadAction</code> is set.</li>
</ol>
<p>If <code>dynamicReloadAction</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">reloadOptions:
tplScriptTrigger:
namespace: kb-system
scriptConfigMapRef: mysql-reload-script
sync: true
</code></pre>
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
<p>Indicates whether to consolidate dynamic reload and restart actions into a single restart.</p>
<ul>
<li>If true, updates requiring both actions will result in only a restart, merging the actions.</li>
<li>If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.</li>
</ul>
<p>This flag allows for more efficient handling of configuration changes by potentially eliminating
an unnecessary reload step.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicParameterSelectedPolicy</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DynamicParameterSelectedPolicy">
DynamicParameterSelectedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>dynamicReloadAction</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>&ldquo;dynamic&rdquo; (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>&ldquo;all&rdquo;: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>reloadToolsImage</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ReloadToolsImage">
ReloadToolsImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tools container image used by ShellTrigger for dynamic reload.
If the dynamic reload action is triggered by a ShellTrigger, this field is required.
This image must contain all necessary tools for executing the ShellTrigger scripts.</p>
<p>Usually the specified image is referenced by the init container,
which is then responsible for copy the tools from the image to a bin volume.
This ensures that the tools are available to the &lsquo;config-manager&rsquo; sidecar.</p>
</td>
</tr>
<tr>
<td>
<code>downwardActions</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.DownwardAction">
[]DownwardAction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of actions to execute specified commands based on Pod labels.</p>
<p>It utilizes the K8s Downward API to mount label information as a volume into the pod.
The &lsquo;config-manager&rsquo; sidecar container watches for changes in the role label and dynamically invoke
registered commands (usually execute some SQL statements) when a change is detected.</p>
<p>It is designed for scenarios where:</p>
<ul>
<li>Replicas with different roles have different configurations, such as Redis primary &amp; secondary replicas.</li>
<li>After a role switch (e.g., from secondary to primary), some changes in configuration are needed
to reflect the new role.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>scriptConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ScriptConfig">
[]ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of ScriptConfig Object.</p>
<p>Each ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the dynamic reload
and DownwardAction to perform specific tasks or configurations.</p>
</td>
</tr>
<tr>
<td>
<code>configSchemaTopLevelKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the top-level key in the &lsquo;configSchema.cue&rsquo; that organizes the validation rules for parameters.
This key must exist within the CUE script defined in &lsquo;configSchema.cue&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>configSchema</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ConfigSchema">
ConfigSchema
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines a list of parameters including their names, default values, descriptions,
types, and constraints (permissible values or the range of valid values).</p>
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
<p>List static parameters.
Modifications to any of these parameters require a restart of the process to take effect.</p>
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
<p>List dynamic parameters.
Modifications to these parameters trigger a configuration reload without requiring a process restart.</p>
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
<p>Lists the parameters that cannot be modified once set.
Attempting to change any of these parameters will be ignored.</p>
</td>
</tr>
<tr>
<td>
<code>dynamicReloadSelector</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to match labels on the pod to determine whether a dynamic reload should be performed.</p>
<p>In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
The <code>dynamicReloadSelector</code> allows you to specify label selectors to target the desired pods for the reload process.</p>
<p>If the <code>dynamicReloadSelector</code> is not specified or is nil, all pods managed by the workload will be considered for the dynamic
reload.</p>
</td>
</tr>
<tr>
<td>
<code>formatterConfig</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">
FormatterConfig
</a>
</em>
</td>
<td>
<p>Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
Supported formats include <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>, <code>hcl</code>, <code>dotenv</code>, <code>properties</code>, and <code>toml</code>.</p>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
<p>Example:</p>
<pre><code>formatterConfig:
format: ini
iniConfig:
sectionName: mysqld
</code></pre>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ConfigConstraintStatus">ConfigConstraintStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.ConfigConstraint">ConfigConstraint</a>)
</p>
<div>
<p>ConfigConstraintStatus represents the observed state of a ConfigConstraint.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintPhase">
ConfigConstraintPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the status of the configuration template.
When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition or ClusterVersion.</p>
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
<p>Provides descriptions for abnormal states.</p>
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
<p>Refers to the most recent generation observed for this ConfigConstraint. This value is updated by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ConfigSchema">ConfigSchema
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>ConfigSchema Defines a list of configuration items with their names, default values, descriptions,
types, and constraints.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cue</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Hold a string that contains a script written in CUE language that defines a list of configuration items.
Each item is detailed with its name, default value, description, type (e.g. string, integer, float),
and constraints (permissible values or the valid range of values).</p>
<p>CUE (Configure, Unify, Execute) is a declarative language designed for defining and validating
complex data configurations.
It is particularly useful in environments like K8s where complex configurations and validation rules are common.</p>
<p>This script functions as a validator for user-provided configurations, ensuring compliance with
the established specifications and constraints.</p>
</td>
</tr>
<tr>
<td>
<code>schemaInJSON</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#jsonschemaprops-v1-apiextensions-k8s-io">
Kubernetes api extensions v1.JSONSchemaProps
</a>
</em>
</td>
<td>
<p>Generated from the &lsquo;cue&rsquo; field and transformed into a JSON format.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.DownwardAction">DownwardAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>DownwardAction defines an action that triggers specific commands in response to changes in Pod labels.
For example, a command might be executed when the &lsquo;role&rsquo; label of the Pod is updated.</p>
</div>
<table>
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
<p>Specifies the name of the field. It must be a string of maximum length 63.
The name should match the regex pattern <code>^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$</code>.</p>
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
<p>Specifies the mount point of the Downward API volume.</p>
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
<p>Represents a list of files under the Downward API volume.</p>
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
<p>Specifies the command to be triggered when changes are detected in Downward API volume files.
It relies on the inotify mechanism in the config-manager sidecar to monitor file changes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.DynamicParameterSelectedPolicy">DynamicParameterSelectedPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
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
<h3 id="apps.kubeblocks.io/v1beta1.DynamicReloadAction">DynamicReloadAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>DynamicReloadAction defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.</p>
<p>Only one of the mechanisms can be specified at a time.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.UnixSignalTrigger">
UnixSignalTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to trigger a reload by sending a specific Unix signal to the process.</p>
</td>
</tr>
<tr>
<td>
<code>shellTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ShellTrigger">
ShellTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allows to execute a custom shell script to reload the process.</p>
</td>
</tr>
<tr>
<td>
<code>tplScriptTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.TPLScriptTrigger">
TPLScriptTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enables reloading process using a Go template script.</p>
</td>
</tr>
<tr>
<td>
<code>autoTrigger</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.AutoTrigger">
AutoTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Automatically perform the reload when specified conditions are met.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.DynamicReloadType">DynamicReloadType
(<code>string</code> alias)</h3>
<div>
<p>DynamicReloadType defines reload method.</p>
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
<h3 id="apps.kubeblocks.io/v1beta1.FormatterAction">FormatterAction
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.FormatterConfig">FormatterConfig</a>)
</p>
<div>
<p>FormatterAction configures format-specific options for different configuration file format.
Note: Only one of its members should be specified at any given time.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.IniConfig">
IniConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Holds options specific to the &lsquo;ini&rsquo; file format.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.FormatterConfig">FormatterConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>FormatterConfig specifies the format of the configuration file and any associated parameters
that are specific to the chosen format.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>FormatterAction</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.FormatterAction">
FormatterAction
</a>
</em>
</td>
<td>
<p>
(Members of <code>FormatterAction</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
</td>
</tr>
<tr>
<td>
<code>format</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.CfgFileFormat">
CfgFileFormat
</a>
</em>
</td>
<td>
<p>The config file format. Valid values are <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>,
<code>hcl</code>, <code>dotenv</code>, <code>properties</code> and <code>toml</code>. Each format has its own characteristics and use cases.</p>
<ul>
<li>ini: is a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: <a href="https://en.wikipedia.org/wiki/INI_file">https://en.wikipedia.org/wiki/INI_file</a></li>
<li>xml: refers to wiki: <a href="https://en.wikipedia.org/wiki/XML">https://en.wikipedia.org/wiki/XML</a></li>
<li>yaml: supports for complex data types and structures.</li>
<li>json: refers to wiki: <a href="https://en.wikipedia.org/wiki/JSON">https://en.wikipedia.org/wiki/JSON</a></li>
<li>hcl: The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: <a href="https://www.linode.com/docs/guides/introduction-to-hcl/">https://www.linode.com/docs/guides/introduction-to-hcl/</a></li>
<li>dotenv: is a plain text file with simple key–value pairs, reference wiki: <a href="https://en.wikipedia.org/wiki/Configuration_file#MS-DOS">https://en.wikipedia.org/wiki/Configuration_file#MS-DOS</a></li>
<li>properties: a file extension mainly used in Java, reference wiki: <a href="https://en.wikipedia.org/wiki/.properties">https://en.wikipedia.org/wiki/.properties</a></li>
<li>toml: refers to wiki: <a href="https://en.wikipedia.org/wiki/TOML">https://en.wikipedia.org/wiki/TOML</a></li>
<li>props-plus: a file extension mainly used in Java, supports CamelCase(e.g: brokerMaxConnectionsPerIp)</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.IniConfig">IniConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.FormatterAction">FormatterAction</a>)
</p>
<div>
<p>IniConfig holds options specific to the &lsquo;ini&rsquo; file format.</p>
</div>
<table>
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
<p>A string that describes the name of the ini section.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ReloadToolsImage">ReloadToolsImage
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>)
</p>
<div>
<p>ReloadToolsImage specifies the tools container image used by ShellTrigger for dynamic reload.</p>
</div>
<table>
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
<p>Specifies the directory path in the container where the tools-related files are to be copied.
This field is typically used with an emptyDir volume to ensure a temporary, empty directory is provided at pod creation.</p>
</td>
</tr>
<tr>
<td>
<code>toolConfigs</code><br/>
<em>
<a href="#apps.kubeblocks.io/v1beta1.ToolConfig">
[]ToolConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a list of settings of init containers that prepare tools for dynamic reload.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ScriptConfig">ScriptConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.ConfigConstraintSpec">ConfigConstraintSpec</a>, <a href="#apps.kubeblocks.io/v1beta1.TPLScriptTrigger">TPLScriptTrigger</a>)
</p>
<div>
<p>ScriptConfig specifies the ConfigMap that contains the script to be executed for reload.</p>
</div>
<table>
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
<p>Specifies the reference to the ConfigMap that contains the script to be executed for reload.</p>
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
<p>Specifies the namespace where the referenced tpl script ConfigMap in.
If left empty, by default in the &ldquo;default&rdquo; namespace.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ShellTrigger">ShellTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>, <a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">DynamicReloadAction</a>)
</p>
<div>
<p>ShellTrigger allows to execute a custom shell script to reload the process.</p>
</div>
<table>
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
<p>Specifies the command to execute in order to reload the process. It should be a valid shell command.</p>
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
<p>Determines whether parameter updates should be synchronized with the config manager.
Specifies the controller&rsquo;s reload strategy:</p>
<ul>
<li>If set to &lsquo;True&rsquo;, the controller executes the reload action in synchronous mode,
pausing execution until the reload completes.</li>
<li>If set to &lsquo;False&rsquo;, the controller executes the reload action in asynchronous mode,
updating the ConfigMap without waiting for the reload process to finish.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>batchReload</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies whether to process dynamic parameter updates individually or collectively in a batch:</p>
<ul>
<li>Set to &lsquo;True&rsquo; to execute all parameter changes in one batch reload action.</li>
<li>Set to &lsquo;False&rsquo; to execute a reload action for each individual parameter change.
The default behavior, if not specified, is &lsquo;False&rsquo;.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>batchParametersTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>BatchParametersTemplate provides an optional Go template string to format the batch input data
passed into the STDIN of the script when <code>batchReload</code> is set to &lsquo;True&rsquo;.
The template uses the updated parameters&rsquo; key-value pairs, accessible via the &lsquo;$&rsquo; variable.
This allows for custom formatting of the input data.
Example template:</p>
<pre><code class="language-yaml">batchParametersTemplate: |-
&#123;&#123;- range $pKey, $pValue := $ &#125;&#125;
&#123;&#123; printf &quot;%s:%s&quot; $pKey $pValue &#125;&#125;
&#123;&#123;- end &#125;&#125;
</code></pre>
<p>This example generates batch input data in a key:value format, sorted by keys.</p>
<pre><code>key1:value1
key2:value2
key3:value3
</code></pre>
<p>If not specified, the default format is key=value, sorted by keys, for each updated parameter.</p>
<pre><code>key1=value1
key2=value2
key3=value3
</code></pre>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.SignalType">SignalType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.UnixSignalTrigger">UnixSignalTrigger</a>)
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
<h3 id="apps.kubeblocks.io/v1beta1.TPLScriptTrigger">TPLScriptTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>, <a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">DynamicReloadAction</a>)
</p>
<div>
<p>TPLScriptTrigger Enables reloading process using a Go template script.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.ScriptConfig">
ScriptConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>ScriptConfig</code> are embedded into this type.)
</p>
<p>Specifies the ConfigMap that contains the script to be executed for reload.</p>
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
<p>Determines whether parameter updates should be synchronized with the config manager.
Specifies the controller&rsquo;s reload strategy:</p>
<ul>
<li>If set to &lsquo;True&rsquo;, the controller executes the reload action in synchronous mode,
pausing execution until the reload completes.</li>
<li>If set to &lsquo;False&rsquo;, the controller executes the reload action in asynchronous mode,
updating the ConfigMap without waiting for the reload process to finish.</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.ToolConfig">ToolConfig
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1beta1.ReloadToolsImage">ReloadToolsImage</a>)
</p>
<div>
<p>ToolConfig specifies the settings of an init container that prepare tools for dynamic reload.</p>
</div>
<table>
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
<p>Specifies the name of the init container.</p>
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
<p>Specifies the tool container image.</p>
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
<p>Specifies the command to be executed by the init container.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="apps.kubeblocks.io/v1beta1.UnixSignalTrigger">UnixSignalTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ReloadOptions">ReloadOptions</a>, <a href="#apps.kubeblocks.io/v1beta1.DynamicReloadAction">DynamicReloadAction</a>)
</p>
<div>
<p>UnixSignalTrigger is used to trigger a reload by sending a specific Unix signal to the process.</p>
</div>
<table>
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
<a href="#apps.kubeblocks.io/v1beta1.SignalType">
SignalType
</a>
</em>
</td>
<td>
<p>Specifies a valid Unix signal to be sent.
For a comprehensive list of all Unix signals, see: ../../pkg/configuration/configmap/handler.go:allUnixSignals</p>
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
<p>Identifies the name of the process to which the Unix signal will be sent.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<h2 id="workloads.kubeblocks.io/v1alpha1">workloads.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#workloads.kubeblocks.io/v1alpha1.InstanceSet">InstanceSet</a>
</li></ul>
<h3 id="workloads.kubeblocks.io/v1alpha1.InstanceSet">InstanceSet
</h3>
<div>
<p>InstanceSet is the Schema for the instancesets API.</p>
</div>
<table>
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
<td><code>InstanceSet</code></td>
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
<p>Contains the metadata for the particular object, such as name, namespace, labels, and annotations.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">
InstanceSetSpec
</a>
</em>
</td>
<td>
<p>Defines the desired state of the state machine. It includes the configuration details for the state machine.</p>
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
<p>Specifies the desired number of replicas of the given Template.
These replicas are instantiations of the same Template, with each having a consistent identity.
Defaults to 1 if unspecified.</p>
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
<p>Defines the minimum number of seconds a newly created pod should be ready
without any of its container crashing to be considered available.
Defaults to 0, meaning the pod will be considered available as soon as it is ready.</p>
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
<p>Represents a label query over pods that should match the desired replica count indicated by the <code>replica</code> field.
It must match the labels defined in the pod template.
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
<p>Refers to the name of the service that governs this StatefulSet.
This service must exist before the StatefulSet and is responsible for
the network identity of the set. Pods get DNS/hostnames that follow a specific pattern.</p>
<p>Note: This field will be removed in future version.</p>
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
<p>Defines the behavior of a service spec.
Provides read-write service.
<a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
<p>Note: This field will be removed in future version.</p>
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
<p>Defines Alternative Services selector pattern specifier.</p>
<p>Note: This field will be removed in future version.</p>
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
<code>instances</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides values in default Template.</p>
<p>Instance is the fundamental unit managed by KubeBlocks.
It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
An InstanceSet manages instances with a total count of Replicas,
and by default, all these instances are generated from the same template.
The InstanceTemplate provides a way to override values in the default template,
allowing the InstanceSet to manage instances from different templates.</p>
<p>The naming convention for instances (pods) based on the InstanceSet Name, InstanceTemplate Name, and ordinal.
The constructed instance name follows the pattern: $(instance_set.name)-$(template.name)-$(ordinal).
By default, the ordinal starts from 0 for each InstanceTemplate.
It is important to ensure that the Name of each InstanceTemplate is unique.</p>
<p>The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the InstanceSet.
Any remaining replicas will be generated using the default template and will follow the default naming rules.</p>
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
<p>Specifies the names of instances to be transitioned to offline status.</p>
<p>Marking an instance as offline results in the following:</p>
<ol>
<li>The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
future reuse or data recovery, but it is no longer actively used.</li>
<li>The ordinal number assigned to this instance is preserved, ensuring it remains unique
and avoiding conflicts with new instances.</li>
</ol>
<p>Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
ordinal consistency within the cluster.
Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.</p>
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
<p>Specifies a list of PersistentVolumeClaim templates that define the storage requirements for each replica.
Each template specifies the desired characteristics of a persistent volume, such as storage class,
size, and access modes.
These templates are used to dynamically provision persistent volumes for replicas upon their creation.
The final name of each PVC is generated by appending the pod&rsquo;s identifier to the name specified in volumeClaimTemplates[*].name.</p>
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
<p>Controls how pods are created during initial scale up,
when replacing pods on nodes, or when scaling down.</p>
<p>The default policy is <code>OrderedReady</code>, where pods are created in increasing order and the controller waits until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.
The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</p>
<p>Note: This field will be removed in future version.</p>
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
<p>Indicates the StatefulSetUpdateStrategy that will be
employed to update Pods in the InstanceSet when a revision is made to
Template.
UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil</p>
<p>Note: This field will be removed in future version.</p>
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
<p>A list of roles defined in the system.</p>
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
<p>Provides method to probe role.</p>
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
<p>Provides actions to do membership dynamic reconfiguration.</p>
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
<p>Members(Pods) update strategy.</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.</li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.</li>
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
<p>Indicates that the InstanceSet is paused, meaning the reconciliation of this InstanceSet object will be paused.</p>
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
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetStatus">
InstanceSetStatus
</a>
</em>
</td>
<td>
<p>Represents the current information about the state machine. This data may be out of date.</p>
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
<p>Refers to the utility image that contains the command which can be utilized to retrieve or process role information.</p>
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
<p>A set of instructions that will be executed within the Container to retrieve or process role information. This field is required.</p>
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
<p>Additional parameters used to perform specific statements. This field is optional.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.Credential">Credential
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Defines the user&rsquo;s name for the credential.
The corresponding environment variable will be KB_ITS_USERNAME.</p>
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
<p>Represents the user&rsquo;s password for the credential.
The corresponding environment variable will be KB_ITS_PASSWORD.</p>
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
<p>Specifies the value of the environment variable. This field is optional and defaults to an empty string.
The value can include variable references in the format $(VAR_NAME) which will be expanded using previously defined environment variables in the container and any service environment variables.</p>
<p>If a variable cannot be resolved, the reference in the input string will remain unchanged.
Double $$ can be used to escape the $(VAR_NAME) syntax, resulting in a single $ and producing the string literal &ldquo;$(VAR_NAME)&rdquo;.
Escaped references will not be expanded, regardless of whether the variable exists or not.</p>
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
<p>Defines the source for the environment variable&rsquo;s value. This field is optional and cannot be used if the &lsquo;Value&rsquo; field is not empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.InstanceSet">InstanceSet</a>)
</p>
<div>
<p>InstanceSetSpec defines the desired state of InstanceSet</p>
</div>
<table>
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
<p>Specifies the desired number of replicas of the given Template.
These replicas are instantiations of the same Template, with each having a consistent identity.
Defaults to 1 if unspecified.</p>
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
<p>Defines the minimum number of seconds a newly created pod should be ready
without any of its container crashing to be considered available.
Defaults to 0, meaning the pod will be considered available as soon as it is ready.</p>
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
<p>Represents a label query over pods that should match the desired replica count indicated by the <code>replica</code> field.
It must match the labels defined in the pod template.
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
<p>Refers to the name of the service that governs this StatefulSet.
This service must exist before the StatefulSet and is responsible for
the network identity of the set. Pods get DNS/hostnames that follow a specific pattern.</p>
<p>Note: This field will be removed in future version.</p>
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
<p>Defines the behavior of a service spec.
Provides read-write service.
<a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
<p>Note: This field will be removed in future version.</p>
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
<p>Defines Alternative Services selector pattern specifier.</p>
<p>Note: This field will be removed in future version.</p>
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
<code>instances</code><br/>
<em>
<a href="#workloads.kubeblocks.io/v1alpha1.InstanceTemplate">
[]InstanceTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Overrides values in default Template.</p>
<p>Instance is the fundamental unit managed by KubeBlocks.
It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
An InstanceSet manages instances with a total count of Replicas,
and by default, all these instances are generated from the same template.
The InstanceTemplate provides a way to override values in the default template,
allowing the InstanceSet to manage instances from different templates.</p>
<p>The naming convention for instances (pods) based on the InstanceSet Name, InstanceTemplate Name, and ordinal.
The constructed instance name follows the pattern: $(instance_set.name)-$(template.name)-$(ordinal).
By default, the ordinal starts from 0 for each InstanceTemplate.
It is important to ensure that the Name of each InstanceTemplate is unique.</p>
<p>The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the InstanceSet.
Any remaining replicas will be generated using the default template and will follow the default naming rules.</p>
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
<p>Specifies the names of instances to be transitioned to offline status.</p>
<p>Marking an instance as offline results in the following:</p>
<ol>
<li>The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
future reuse or data recovery, but it is no longer actively used.</li>
<li>The ordinal number assigned to this instance is preserved, ensuring it remains unique
and avoiding conflicts with new instances.</li>
</ol>
<p>Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
ordinal consistency within the cluster.
Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.</p>
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
<p>Specifies a list of PersistentVolumeClaim templates that define the storage requirements for each replica.
Each template specifies the desired characteristics of a persistent volume, such as storage class,
size, and access modes.
These templates are used to dynamically provision persistent volumes for replicas upon their creation.
The final name of each PVC is generated by appending the pod&rsquo;s identifier to the name specified in volumeClaimTemplates[*].name.</p>
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
<p>Controls how pods are created during initial scale up,
when replacing pods on nodes, or when scaling down.</p>
<p>The default policy is <code>OrderedReady</code>, where pods are created in increasing order and the controller waits until each pod is ready before
continuing. When scaling down, the pods are removed in the opposite order.
The alternative policy is <code>Parallel</code> which will create pods in parallel
to match the desired scale without waiting, and on scale down will delete
all pods at once.</p>
<p>Note: This field will be removed in future version.</p>
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
<p>Indicates the StatefulSetUpdateStrategy that will be
employed to update Pods in the InstanceSet when a revision is made to
Template.
UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil</p>
<p>Note: This field will be removed in future version.</p>
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
<p>A list of roles defined in the system.</p>
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
<p>Provides method to probe role.</p>
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
<p>Provides actions to do membership dynamic reconfiguration.</p>
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
<p>Members(Pods) update strategy.</p>
<ul>
<li>serial: update Members one by one that guarantee minimum component unavailable time.</li>
<li>bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.</li>
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
<p>Indicates that the InstanceSet is paused, meaning the reconciliation of this InstanceSet object will be paused.</p>
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
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.InstanceSetStatus">InstanceSetStatus
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.InstanceSet">InstanceSet</a>)
</p>
<div>
<p>InstanceSetStatus defines the observed state of InstanceSet</p>
</div>
<table>
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
<p>Defines the initial number of pods (members) when the cluster is first initialized.
This value is set to spec.Replicas at the time of object creation and remains constant thereafter.</p>
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
<p>Represents the number of pods (members) that have already reached the MembersStatus during the cluster initialization stage.
This value remains constant once it equals InitReplicas.</p>
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
<p>When not empty, indicates the version of the InstanceSet used to generate the underlying workload.</p>
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
<p>Provides the status of each member in the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>currentRevisions</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>currentRevisions, if not empty, indicates the old version of the InstanceSet used to generate the underlying workload.
key is the pod name, value is the revision.</p>
</td>
</tr>
<tr>
<td>
<code>updateRevisions</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>updateRevisions, if not empty, indicates the new version of the InstanceSet used to generate the underlying workload.
key is the pod name, value is the revision.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.InstanceTemplate">InstanceTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>)
</p>
<div>
<p>InstanceTemplate allows customization of individual replica configurations within a Component,
without altering the base component template defined in ClusterComponentSpec.
It enables the application of distinct settings to specific instances (replicas),
providing flexibility while maintaining a common configuration baseline.</p>
</div>
<table>
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
<p>Name specifies the unique name of the instance Pod created using this InstanceTemplate.
This name is constructed by concatenating the component&rsquo;s name, the template&rsquo;s name, and the instance&rsquo;s ordinal
using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
The specified name overrides any default naming conventions or patterns.</p>
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
<p>Specifies the number of instances (Pods) to create from this InstanceTemplate.
This field allows setting how many replicated instances of the component,
with the specific overrides in the InstanceTemplate, are created.
The default value is 1. A value of 0 disables instance creation.</p>
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
<p>Specifies a map of key-value pairs to be merged into the Pod&rsquo;s existing annotations.
Existing keys will have their values overwritten, while new keys will be added to the annotations.</p>
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
<p>Specifies a map of key-value pairs that will be merged into the Pod&rsquo;s existing labels.
Values for existing keys will be overwritten, and new keys will be added.</p>
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
<p>Specifies an override for the first container&rsquo;s image in the pod.</p>
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
<p>Specifies the name of the node where the Pod should be scheduled.
If set, the Pod will be directly assigned to the specified node, bypassing the Kubernetes scheduler.
This is useful for controlling Pod placement on specific nodes.</p>
<p>Important considerations:
- <code>nodeName</code> bypasses default scheduling constraints (e.g., resource requirements, node selectors, affinity rules).
- It is the user&rsquo;s responsibility to ensure the node is suitable for the Pod.
- If the node is unavailable, the Pod will remain in &ldquo;Pending&rdquo; state until the node is available or the Pod is deleted.</p>
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
<p>Defines NodeSelector to override.</p>
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
<p>Tolerations specifies a list of tolerations to be applied to the Pod, allowing it to tolerate node taints.
This field can be used to add new tolerations or override existing ones.</p>
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
<p>Specifies an override for the resource requirements of the first container in the Pod.
This field allows for customizing resource allocation (CPU, memory, etc.) for the container.</p>
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
<p>Defines Env to override.
Add new or override existing envs.</p>
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
<p>Defines Volumes to override.
Add new or override existing volumes.</p>
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
<p>Defines VolumeMounts to override.
Add new or override existing volume mounts of the first container in the pod.</p>
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
<p>Defines VolumeClaimTemplates to override.
Add new or override existing volume claim templates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.MemberStatus">MemberStatus
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.ClusterComponentStatus">ClusterComponentStatus</a>, <a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetStatus">InstanceSetStatus</a>)
</p>
<div>
</div>
<table>
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
<p>Represents the name of the pod.</p>
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
<em>(Optional)</em>
<p>Defines the role of the replica in the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>ready</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whether the corresponding Pod is in ready condition.</p>
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
<p>Indicates whether it is required for the InstanceSet to have at least one primary instance ready.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.MemberUpdateStrategy">MemberUpdateStrategy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>)
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
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the environment variables that can be used in all following Actions:
- KB_ITS_USERNAME: Represents the username part of the credential
- KB_ITS_PASSWORD: Represents the password part of the credential
- KB_ITS_LEADER_HOST: Represents the leader host
- KB_ITS_TARGET_HOST: Represents the target host
- KB_ITS_SERVICE_PORT: Represents the service port</p>
<p>Defines the action to perform a switchover.
If the Image is not configured, the latest <a href="https://busybox.net/">BusyBox</a> image will be used.</p>
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
<p>Defines the action to add a member.
If the Image is not configured, the Image from the previous non-nil action will be used.</p>
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
<p>Defines the action to remove a member.
If the Image is not configured, the Image from the previous non-nil action will be used.</p>
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
<p>Defines the action to trigger the new member to start log syncing.
If the Image is not configured, the Image from the previous non-nil action will be used.</p>
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
<p>Defines the action to inform the cluster that the new member can join voting now.
If the Image is not configured, the Image from the previous non-nil action will be used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.ReplicaRole">ReplicaRole
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.MemberStatus">MemberStatus</a>)
</p>
<div>
</div>
<table>
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
<p>Defines the role name of the replica.</p>
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
<p>Specifies the service capabilities of this member.</p>
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
<p>Indicates if this member has voting rights.</p>
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
<p>Determines if this member is the leader.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="workloads.kubeblocks.io/v1alpha1.RoleProbe">RoleProbe
</h3>
<p>
(<em>Appears on:</em><a href="#apps.kubeblocks.io/v1alpha1.RSMSpec">RSMSpec</a>, <a href="#workloads.kubeblocks.io/v1alpha1.InstanceSetSpec">InstanceSetSpec</a>)
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
<p>Specifies the builtin handler name to use to probe the role of the main container.
Available handlers include: mysql, postgres, mongodb, redis, etcd, kafka.
Use CustomHandler to define a custom role probe function if none of the built-in handlers meet the requirement.</p>
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
<p>Defines a custom method for role probing.
If the BuiltinHandler meets the requirement, use it instead.
Actions defined here are executed in series.
Upon completion of all actions, the final output should be a single string representing the role name defined in spec.Roles.
The latest <a href="https://busybox.net/">BusyBox</a> image will be used if Image is not configured.
Environment variables can be used in Command:
- v_KB_ITS_LAST<em>STDOUT: stdout from the last action, watch for &lsquo;v</em>&rsquo; prefix
- KB_ITS_USERNAME: username part of the credential
- KB_ITS_PASSWORD: password part of the credential</p>
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
<p>Specifies the number of seconds to wait after the container has started before initiating role probing.</p>
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
<p>Specifies the number of seconds after which the probe times out.</p>
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
<p>Specifies the frequency (in seconds) of probe execution.</p>
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
<p>Specifies the minimum number of consecutive successes for the probe to be considered successful after having failed.</p>
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
<p>Specifies the minimum number of consecutive failures for the probe to be considered failed after having succeeded.</p>
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
<p>Specifies the method for updating the pod role label.</p>
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
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
