---
title: Parameters API Reference
description: Parameters API Reference
keywords: [parameters, api]
sidebar_position: 3
sidebar_label: Parameters
---
<br />
<p>Packages:</p>
<ul>
<li>
<a href="#parameters.kubeblocks.io%2fv1alpha1">parameters.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="parameters.kubeblocks.io/v1alpha1">parameters.kubeblocks.io/v1alpha1</h2>
Resource Types:
<ul><li>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameter">ComponentParameter</a>
</li><li>
<a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRenderer">ParamConfigRenderer</a>
</li><li>
<a href="#parameters.kubeblocks.io/v1alpha1.Parameter">Parameter</a>
</li><li>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinition">ParametersDefinition</a>
</li></ul>
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentParameter">ComponentParameter
</h3>
<div>
<p>ComponentParameter is the Schema for the componentparameters API</p>
</div>
<table>
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
<code>parameters.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ComponentParameter</code></td>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameterSpec">
ComponentParameterSpec
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
<em>(Optional)</em>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">
[]ConfigTemplateItemDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigItemDetails is an array of ConfigTemplateItemDetail objects.</p>
<p>Each ConfigTemplateItemDetail corresponds to a configuration template,
which is a ConfigMap that contains multiple configuration files.
Each configuration file is stored as a key-value pair within the ConfigMap.</p>
<p>The ConfigTemplateItemDetail includes information such as:</p>
<ul>
<li>The configuration template (a ConfigMap)</li>
<li>The corresponding ConfigConstraint (constraints and validation rules for the configuration)</li>
<li>Volume mounts (for mounting the configuration files)</li>
</ul>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameterStatus">
ComponentParameterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParamConfigRenderer">ParamConfigRenderer
</h3>
<div>
<p>ParamConfigRenderer is the Schema for the paramconfigrenderers API</p>
</div>
<table>
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
<code>parameters.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ParamConfigRenderer</code></td>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRendererSpec">
ParamConfigRendererSpec
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
<code>componentDef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the ComponentDefinition custom resource (CR) that defines the Component&rsquo;s characteristics and behavior.</p>
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
<p>ServiceVersion specifies the version of the Service expected to be provisioned by this Component.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).
If no version is specified, the latest available version will be used.</p>
</td>
</tr>
<tr>
<td>
<code>parametersDefs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the ParametersDefinition custom resource (CR) that defines the Component parameter&rsquo;s schema and behavior.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentConfigDescription">
[]ComponentConfigDescription
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration files.</p>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRendererStatus">
ParamConfigRendererStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.Parameter">Parameter
</h3>
<div>
<p>Parameter is the Schema for the parameters API</p>
</div>
<table>
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
<code>parameters.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Parameter</code></td>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterSpec">
ParameterSpec
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
<code>componentParameters</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParametersSpec">
[]ComponentParametersSpec
</a>
</em>
</td>
<td>
<p>Lists ComponentParametersSpec objects, each specifying a Component and its parameters and template updates.</p>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterStatus">
ParameterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersDefinition">ParametersDefinition
</h3>
<div>
<p>ParametersDefinition is the Schema for the parametersdefinitions API</p>
</div>
<table>
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
<code>parameters.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>ParametersDefinition</code></td>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">
ParametersDefinitionSpec
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
<code>fileName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the config file name in the config template.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
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
<code>reloadAction</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">
ReloadAction
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
<li><code>reloadAction</code> is set.</li>
</ol>
<p>If <code>reloadAction</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">dynamicReloadAction:
 tplScriptTrigger:
   namespace: kb-system
   scriptConfigMapRef: mysql-reload-script
   sync: true
</code></pre>
</td>
</tr>
<tr>
<td>
<code>downwardAPIChangeTriggeredActions</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.DownwardAPIChangeTriggeredAction">
[]DownwardAPIChangeTriggeredAction
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
<code>deletedPolicy</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterDeletedPolicy">
ParameterDeletedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the policy when parameter be removed.</p>
</td>
</tr>
<tr>
<td>
<code>mergeReloadAndRestart</code><br/>
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
<code>reloadStaticParamsBeforeRestart</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>reloadAction</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>false (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>true: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
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
</tbody>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionStatus">
ParametersDefinitionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.AutoTrigger">AutoTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">ReloadAction</a>)
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
<h3 id="parameters.kubeblocks.io/v1alpha1.CfgFileFormat">CfgFileFormat
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.FileFormatConfig">FileFormatConfig</a>)
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
</tr><tr><td><p>&#34;props-ultra&#34;</p></td>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentConfigDescription">ComponentConfigDescription
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRendererSpec">ParamConfigRendererSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the config file name in the config template.</p>
</td>
</tr>
<tr>
<td>
<code>templateName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the referenced componentTemplateSpec.</p>
</td>
</tr>
<tr>
<td>
<code>fileFormatConfig</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.FileFormatConfig">
FileFormatConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
Supported formats include <code>ini</code>, <code>xml</code>, <code>yaml</code>, <code>json</code>, <code>hcl</code>, <code>dotenv</code>, <code>properties</code>, and <code>toml</code>.</p>
<p>Each format may have its own set of parameters that can be configured.
For instance, when using the <code>ini</code> format, you can specify the section name.</p>
<p>Example:</p>
<pre><code>fileFormatConfig:
 format: ini
 iniConfig:
   sectionName: mysqld
</code></pre>
</td>
</tr>
<tr>
<td>
<code>reRenderResourceTypes</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.RerenderResourceType">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentParameterSpec">ComponentParameterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameter">ComponentParameter</a>)
</p>
<div>
<p>ComponentParameterSpec defines the desired state of ComponentConfiguration</p>
</div>
<table>
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
<em>(Optional)</em>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">
[]ConfigTemplateItemDetail
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigItemDetails is an array of ConfigTemplateItemDetail objects.</p>
<p>Each ConfigTemplateItemDetail corresponds to a configuration template,
which is a ConfigMap that contains multiple configuration files.
Each configuration file is stored as a key-value pair within the ConfigMap.</p>
<p>The ConfigTemplateItemDetail includes information such as:</p>
<ul>
<li>The configuration template (a ConfigMap)</li>
<li>The corresponding ConfigConstraint (constraints and validation rules for the configuration)</li>
<li>Volume mounts (for mounting the configuration files)</li>
</ul>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentParameterStatus">ComponentParameterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameter">ComponentParameter</a>)
</p>
<div>
<p>ComponentParameterStatus defines the observed state of ComponentConfiguration</p>
</div>
<table>
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
<code>phase</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterPhase">
ParameterPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the current status of the configuration item.</p>
<p>Possible values include &ldquo;Creating&rdquo;, &ldquo;Init&rdquo;, &ldquo;Running&rdquo;, &ldquo;Pending&rdquo;, &ldquo;Merged&rdquo;, &ldquo;MergeFailed&rdquo;, &ldquo;FailedAndPause&rdquo;,
&ldquo;Upgrading&rdquo;, &ldquo;Deleting&rdquo;, &ldquo;FailedAndRetry&rdquo;, &ldquo;Finished&rdquo;.</p>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetailStatus">
[]ConfigTemplateItemDetailStatus
</a>
</em>
</td>
<td>
<p>Provides the status of each component undergoing reconfiguration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentParameters">ComponentParameters
(<code>map[string]*string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParametersSpec">ComponentParametersSpec</a>)
</p>
<div>
</div>
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentParametersSpec">ComponentParametersSpec
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParameterSpec">ParameterSpec</a>)
</p>
<div>
</div>
<table>
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
<code>parameters</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameters">
ComponentParameters
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the user-defined configuration template or parameters.</p>
</td>
</tr>
<tr>
<td>
<code>userConfigTemplates</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateExtension">
map[string]github.com/apecloud/kubeblocks/apis/parameters/v1alpha1.ConfigTemplateExtension
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
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ComponentReconfiguringStatus">ComponentReconfiguringStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParameterStatus">ParameterStatus</a>)
</p>
<div>
</div>
<table>
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
<code>phase</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterPhase">
ParameterPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the current status of the configuration item.</p>
<p>Possible values include &ldquo;Creating&rdquo;, &ldquo;Init&rdquo;, &ldquo;Running&rdquo;, &ldquo;Pending&rdquo;, &ldquo;Merged&rdquo;, &ldquo;MergeFailed&rdquo;, &ldquo;FailedAndPause&rdquo;,
&ldquo;Upgrading&rdquo;, &ldquo;Deleting&rdquo;, &ldquo;FailedAndRetry&rdquo;, &ldquo;Finished&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>parameterStatus</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ReconfiguringStatus">
[]ReconfiguringStatus
</a>
</em>
</td>
<td>
<p>Describes the status of the component reconfiguring.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ConfigTemplateExtension">ConfigTemplateExtension
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParametersSpec">ComponentParametersSpec</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">ConfigTemplateItemDetail</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus</a>)
</p>
<div>
</div>
<table>
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
<a href="#parameters.kubeblocks.io/v1alpha1.MergedPolicy">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">ConfigTemplateItemDetail
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameterSpec">ComponentParameterSpec</a>)
</p>
<div>
<p>ConfigTemplateItemDetail corresponds to settings of a configuration template (a ConfigMap).</p>
</div>
<table>
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
<code>payload</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.Payload">
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
github.com/apecloud/kubeblocks/apis/apps/v1.ComponentFileTemplate
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
<code>userConfigTemplates</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateExtension">
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersInFile">
map[string]github.com/apecloud/kubeblocks/apis/parameters/v1alpha1.ParametersInFile
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetailStatus">ConfigTemplateItemDetailStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameterStatus">ComponentParameterStatus</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus</a>)
</p>
<div>
</div>
<table>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterPhase">
ParameterPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the current status of the configuration item.</p>
<p>Possible values include &ldquo;Creating&rdquo;, &ldquo;Init&rdquo;, &ldquo;Running&rdquo;, &ldquo;Pending&rdquo;, &ldquo;Merged&rdquo;, &ldquo;MergeFailed&rdquo;, &ldquo;FailedAndPause&rdquo;,
&ldquo;Upgrading&rdquo;, &ldquo;Deleting&rdquo;, &ldquo;FailedAndRetry&rdquo;, &ldquo;Finished&rdquo;.</p>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ReconcileDetail">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.DownwardAPIChangeTriggeredAction">DownwardAPIChangeTriggeredAction
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">ParametersDefinitionSpec</a>)
</p>
<div>
<p>DownwardAPIChangeTriggeredAction defines an action that triggers specific commands in response to changes in Pod labels.
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
<tr>
<td>
<code>scriptConfig</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ScriptConfig">
ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the DownwardAction to perform specific tasks or configurations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.DynamicParameterSelectedPolicy">DynamicParameterSelectedPolicy
(<code>string</code> alias)</h3>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.DynamicReloadType">DynamicReloadType
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
<h3 id="parameters.kubeblocks.io/v1alpha1.FileFormatConfig">FileFormatConfig
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentConfigDescription">ComponentConfigDescription</a>)
</p>
<div>
<p>FileFormatConfig specifies the format of the configuration file and any associated parameters
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
<a href="#parameters.kubeblocks.io/v1alpha1.FormatterAction">
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
<a href="#parameters.kubeblocks.io/v1alpha1.CfgFileFormat">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.FormatterAction">FormatterAction
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.FileFormatConfig">FileFormatConfig</a>)
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
<a href="#parameters.kubeblocks.io/v1alpha1.IniConfig">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ImageMapping">ImageMapping
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ToolConfig">ToolConfig</a>)
</p>
<div>
</div>
<table>
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
<p>ServiceVersions is a list of service versions that this mapping applies to.</p>
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
<p>Image is the container image addresses to use for the matched service versions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.IniConfig">IniConfig
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.FormatterAction">FormatterAction</a>)
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
<h3 id="parameters.kubeblocks.io/v1alpha1.MergedPolicy">MergedPolicy
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateExtension">ConfigTemplateExtension</a>)
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ParamConfigRendererSpec">ParamConfigRendererSpec
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRenderer">ParamConfigRenderer</a>)
</p>
<div>
<p>ParamConfigRendererSpec defines the desired state of ParamConfigRenderer</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>componentDef</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the ComponentDefinition custom resource (CR) that defines the Component&rsquo;s characteristics and behavior.</p>
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
<p>ServiceVersion specifies the version of the Service expected to be provisioned by this Component.
The version should follow the syntax and semantics of the &ldquo;Semantic Versioning&rdquo; specification (<a href="http://semver.org/">http://semver.org/</a>).
If no version is specified, the latest available version will be used.</p>
</td>
</tr>
<tr>
<td>
<code>parametersDefs</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the ParametersDefinition custom resource (CR) that defines the Component parameter&rsquo;s schema and behavior.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentConfigDescription">
[]ComponentConfigDescription
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the configuration files.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParamConfigRendererStatus">ParamConfigRendererStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRenderer">ParamConfigRenderer</a>)
</p>
<div>
<p>ParamConfigRendererStatus defines the observed state of ParamConfigRenderer</p>
</div>
<table>
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
<p>The most recent generation number of the ParamsDesc object that has been observed by the controller.</p>
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
<code>phase</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersDescPhase">
ParametersDescPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the status of the configuration template.
When set to PDAvailablePhase, the ParamsDesc can be referenced by ComponentDefinition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParameterDeletedMethod">ParameterDeletedMethod
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParameterDeletedPolicy">ParameterDeletedPolicy</a>)
</p>
<div>
<p>ParameterDeletedMethod defines how to handle parameter remove</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;RestoreToDefault&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Reset&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParameterDeletedPolicy">ParameterDeletedPolicy
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">ParametersDefinitionSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>deletedMethod</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterDeletedMethod">
ParameterDeletedMethod
</a>
</em>
</td>
<td>
<p>Specifies the method to handle the deletion of a parameter.
If set to &ldquo;RestoreToDefault&rdquo;, the parameter will be restored to its default value,
which requires engine support, such as pg.
If set to &ldquo;Reset&rdquo;, the parameter will be re-rendered through the configuration template.</p>
</td>
</tr>
<tr>
<td>
<code>defaultValue</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the value to use if DeletedMethod is RestoreToDefault.
Example: pg
SET configuration_parameter TO DEFAULT;</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParameterPhase">ParameterPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentParameterStatus">ComponentParameterStatus</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ComponentReconfiguringStatus">ComponentReconfiguringStatus</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetailStatus">ConfigTemplateItemDetailStatus</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ParameterStatus">ParameterStatus</a>)
</p>
<div>
<p>ParameterPhase defines the Configuration FSM phase</p>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ParameterSpec">ParameterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.Parameter">Parameter</a>)
</p>
<div>
<p>ParameterSpec defines the desired state of Parameter</p>
</div>
<table>
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
<code>componentParameters</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentParametersSpec">
[]ComponentParametersSpec
</a>
</em>
</td>
<td>
<p>Lists ComponentParametersSpec objects, each specifying a Component and its parameters and template updates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParameterStatus">ParameterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.Parameter">Parameter</a>)
</p>
<div>
<p>ParameterStatus defines the observed state of Parameter</p>
</div>
<table>
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
<code>phase</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterPhase">
ParameterPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the current status of the configuration item.</p>
<p>Possible values include &ldquo;Creating&rdquo;, &ldquo;Init&rdquo;, &ldquo;Running&rdquo;, &ldquo;Pending&rdquo;, &ldquo;Merged&rdquo;, &ldquo;MergeFailed&rdquo;, &ldquo;FailedAndPause&rdquo;,
&ldquo;Upgrading&rdquo;, &ldquo;Deleting&rdquo;, &ldquo;FailedAndRetry&rdquo;, &ldquo;Finished&rdquo;.</p>
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
<code>componentReconfiguringStatus</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ComponentReconfiguringStatus">
[]ComponentReconfiguringStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Records the status of a reconfiguring operation if <code>opsRequest.spec.type</code> equals to &ldquo;Reconfiguring&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">ParametersDefinitionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinition">ParametersDefinition</a>)
</p>
<div>
<p>ParametersDefinitionSpec defines the desired state of ParametersDefinition</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>fileName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the config file name in the config template.</p>
</td>
</tr>
<tr>
<td>
<code>parametersSchema</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersSchema">
ParametersSchema
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
<code>reloadAction</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">
ReloadAction
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
<li><code>reloadAction</code> is set.</li>
</ol>
<p>If <code>reloadAction</code> is not set or the modified parameters are not listed in <code>dynamicParameters</code>,
dynamic reloading will not be triggered.</p>
<p>Example:</p>
<pre><code class="language-yaml">dynamicReloadAction:
 tplScriptTrigger:
   namespace: kb-system
   scriptConfigMapRef: mysql-reload-script
   sync: true
</code></pre>
</td>
</tr>
<tr>
<td>
<code>downwardAPIChangeTriggeredActions</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.DownwardAPIChangeTriggeredAction">
[]DownwardAPIChangeTriggeredAction
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
<code>deletedPolicy</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParameterDeletedPolicy">
ParameterDeletedPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the policy when parameter be removed.</p>
</td>
</tr>
<tr>
<td>
<code>mergeReloadAndRestart</code><br/>
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
<code>reloadStaticParamsBeforeRestart</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configures whether the dynamic reload specified in <code>reloadAction</code> applies only to dynamic parameters or
to all parameters (including static parameters).</p>
<ul>
<li>false (default): Only modifications to the dynamic parameters listed in <code>dynamicParameters</code>
will trigger a dynamic reload.</li>
<li>true: Modifications to both dynamic parameters listed in <code>dynamicParameters</code> and static parameters
listed in <code>staticParameters</code> will trigger a dynamic reload.
The &ldquo;all&rdquo; option is for certain engines that require static parameters to be set
via SQL statements before they can take effect on restart.</li>
</ul>
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
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersDefinitionStatus">ParametersDefinitionStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinition">ParametersDefinition</a>)
</p>
<div>
<p>ParametersDefinitionStatus defines the observed state of ParametersDefinition</p>
</div>
<table>
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
<p>The most recent generation number of the ParamsDesc object that has been observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersDescPhase">
ParametersDescPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the status of the configuration template.
When set to PDAvailablePhase, the ParamsDesc can be referenced by ComponentDefinition.</p>
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
<p>Represents a list of detailed status of the ParametersDescription object.</p>
<p>This field is crucial for administrators and developers to monitor and respond to changes within the ParametersDescription.
It provides a history of state transitions and a snapshot of the current state that can be used for
automated logic or direct inspection.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersDescPhase">ParametersDescPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParamConfigRendererStatus">ParamConfigRendererStatus</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionStatus">ParametersDefinitionStatus</a>)
</p>
<div>
<p>ParametersDescPhase defines the ParametersDescription CR .status.phase</p>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersInFile">ParametersInFile
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">ConfigTemplateItemDetail</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus</a>)
</p>
<div>
</div>
<table>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ParametersSchema">ParametersSchema
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">ParametersDefinitionSpec</a>)
</p>
<div>
<p>ParametersSchema Defines a list of configuration items with their names, default values, descriptions,
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
<code>topLevelKey</code><br/>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.Payload">Payload
(<code>map[string]encoding/json.RawMessage</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetail">ConfigTemplateItemDetail</a>)
</p>
<div>
<p>Payload holds the payload data. This field is optional and can contain any type of data.
Not included in the JSON representation of the object.</p>
</div>
<h3 id="parameters.kubeblocks.io/v1alpha1.ReconcileDetail">ReconcileDetail
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetailStatus">ConfigTemplateItemDetailStatus</a>)
</p>
<div>
</div>
<table>
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ReconfiguringStatus">ReconfiguringStatus
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentReconfiguringStatus">ComponentReconfiguringStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ConfigTemplateItemDetailStatus</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateItemDetailStatus">
ConfigTemplateItemDetailStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ConfigTemplateItemDetailStatus</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>updatedParameters</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ParametersInFile">
map[string]github.com/apecloud/kubeblocks/apis/parameters/v1alpha1.ParametersInFile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Contains the updated parameters.</p>
</td>
</tr>
<tr>
<td>
<code>userConfigTemplates</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ConfigTemplateExtension">
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
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ReloadAction">ReloadAction
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ParametersDefinitionSpec">ParametersDefinitionSpec</a>)
</p>
<div>
<p>ReloadAction defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.</p>
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
<a href="#parameters.kubeblocks.io/v1alpha1.UnixSignalTrigger">
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
<a href="#parameters.kubeblocks.io/v1alpha1.ShellTrigger">
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
<a href="#parameters.kubeblocks.io/v1alpha1.TPLScriptTrigger">
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
<a href="#parameters.kubeblocks.io/v1alpha1.AutoTrigger">
AutoTrigger
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Automatically perform the reload when specified conditions are met.</p>
</td>
</tr>
<tr>
<td>
<code>targetPodSelector</code><br/>
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
The <code>reloadedPodSelector</code> allows you to specify label selectors to target the desired pods for the reload process.</p>
<p>If the <code>reloadedPodSelector</code> is not specified or is nil, all pods managed by the workload will be considered for the dynamic
reload.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ReloadPolicy">ReloadPolicy
(<code>string</code> alias)</h3>
<div>
<p>ReloadPolicy defines the policy of reconfiguring.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;asyncReload&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;dynamicReloadBeginRestart&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;none&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;restartContainer&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;restart&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;rolling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;syncReload&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.RerenderResourceType">RerenderResourceType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ComponentConfigDescription">ComponentConfigDescription</a>)
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
</tr><tr><td><p>&#34;tls&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;vscale&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;shardingHScale&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ScriptConfig">ScriptConfig
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.DownwardAPIChangeTriggeredAction">DownwardAPIChangeTriggeredAction</a>, <a href="#parameters.kubeblocks.io/v1alpha1.ShellTrigger">ShellTrigger</a>, <a href="#parameters.kubeblocks.io/v1alpha1.TPLScriptTrigger">TPLScriptTrigger</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the reference to the ConfigMap containing the scripts.</p>
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
<p>Specifies the namespace for the ConfigMap.
If not specified, it defaults to the &ldquo;default&rdquo; namespace.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ShellTrigger">ShellTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">ReloadAction</a>)
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
<p>Determines the synchronization mode of parameter updates with &ldquo;config-manager&rdquo;.</p>
<ul>
<li>&lsquo;True&rsquo;: Executes reload actions synchronously, pausing until completion.</li>
<li>&lsquo;False&rsquo;: Executes reload actions asynchronously, without waiting for completion.</li>
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
<p>Controls whether parameter updates are processed individually or collectively in a batch:</p>
<ul>
<li>&lsquo;True&rsquo;: Processes all changes in one batch reload.</li>
<li>&lsquo;False&rsquo;: Processes each change individually.</li>
</ul>
<p>Defaults to &lsquo;False&rsquo; if unspecified.</p>
</td>
</tr>
<tr>
<td>
<code>batchParamsFormatterTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies a Go template string for formatting batch input data.
It&rsquo;s used when <code>batchReload</code> is &lsquo;True&rsquo; to format data passed into STDIN of the script.
The template accesses key-value pairs of updated parameters via the &lsquo;$&rsquo; variable.
This allows for custom formatting of the input data.</p>
<p>Example template:</p>
<pre><code class="language-yaml">batchParamsFormatterTemplate: |-
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
<tr>
<td>
<code>toolsSetup</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ToolsSetup">
ToolsSetup
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
<code>scriptConfig</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ScriptConfig">
ScriptConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
The scripts are mounted as volumes and can be referenced and executed by the dynamic reload.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.SignalType">SignalType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.UnixSignalTrigger">UnixSignalTrigger</a>)
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
<h3 id="parameters.kubeblocks.io/v1alpha1.TPLScriptTrigger">TPLScriptTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">ReloadAction</a>)
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
<a href="#parameters.kubeblocks.io/v1alpha1.ScriptConfig">
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
<p>Determines whether parameter updates should be synchronized with the &ldquo;config-manager&rdquo;.
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
<h3 id="parameters.kubeblocks.io/v1alpha1.ToolConfig">ToolConfig
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ToolsSetup">ToolsSetup</a>)
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
<code>asContainerImage</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the tool image should be used as the container image for a sidecar.
This is useful for large tool images, such as those for C++ tools, which may depend on
numerous libraries (e.g., *.so files).</p>
<p>If enabled, the tool image is deployed as a sidecar container image.</p>
<p>Examples:</p>
<pre><code class="language-yaml"> toolsSetup::
   mountPoint: /kb_tools
   toolConfigs:
     - name: kb-tools
       asContainerImage: true
       image:  apecloud/oceanbase:4.2.0.0-100010032023083021
</code></pre>
<p>generated containers:</p>
<pre><code class="language-yaml">initContainers:
 - name: install-config-manager-tool
   image: apecloud/kubeblocks-tools:$&#123;version&#125;
   command:
   - cp
   - /bin/config_render
   - /opt/tools
   volumemounts:
   - name: kb-tools
     mountpath: /opt/tools
containers:
 - name: config-manager
   image: apecloud/oceanbase:4.2.0.0-100010032023083021
   imagePullPolicy: IfNotPresent
	  command:
   - /opt/tools/reloader
   - --log-level
   - info
   - --operator-update-enable
   - --tcp
   - &quot;9901&quot;
   - --config
   - /opt/config-manager/config-manager.yaml
   volumemounts:
   - name: kb-tools
     mountpath: /opt/tools
</code></pre>
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
<code>imageMappings</code><br/>
<em>
<a href="#parameters.kubeblocks.io/v1alpha1.ImageMapping">
[]ImageMapping
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines image mapping for different service versions.</p>
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
<p>Specifies the command to be executed by the init container.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="parameters.kubeblocks.io/v1alpha1.ToolsSetup">ToolsSetup
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ShellTrigger">ShellTrigger</a>)
</p>
<div>
<p>ToolsSetup prepares the tools for dynamic reloads used in ShellTrigger from a specified container image.</p>
<p>Example:</p>
<pre><code class="language-yaml">
toolsSetup:
	 mountPoint: /kb_tools
	 toolConfigs:
	   - name: kb-tools
	     command:
	       - cp
	       - /bin/ob-tools
	       - /kb_tools/obtools
	     image: docker.io/apecloud/obtools
</code></pre>
<p>This example copies the &ldquo;/bin/ob-tools&rdquo; binary from the image to &ldquo;/kb_tools/obtools&rdquo;.</p>
</div>
<table>
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
<a href="#parameters.kubeblocks.io/v1alpha1.ToolConfig">
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
<h3 id="parameters.kubeblocks.io/v1alpha1.UnixSignalTrigger">UnixSignalTrigger
</h3>
<p>
(<em>Appears on:</em><a href="#parameters.kubeblocks.io/v1alpha1.ReloadAction">ReloadAction</a>)
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
<a href="#parameters.kubeblocks.io/v1alpha1.SignalType">
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
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>