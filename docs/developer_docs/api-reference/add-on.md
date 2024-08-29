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
<a href="#extensions.kubeblocks.io%2fv1alpha1">extensions.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="extensions.kubeblocks.io/v1alpha1">extensions.kubeblocks.io/v1alpha1</h2>
<div>
</div>
Resource Types:
<ul><li>
<a href="#extensions.kubeblocks.io/v1alpha1.Addon">Addon</a>
</li></ul>
<h3 id="extensions.kubeblocks.io/v1alpha1.Addon">Addon
</h3>
<div>
<p>Addon is the Schema for the add-ons API.</p>
</div>
<table>
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
<code>extensions.kubeblocks.io/v1alpha1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>Addon</code></td>
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
<a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">
AddonSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the description of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonType">
AddonType
</a>
</em>
</td>
<td>
<p>Defines the type of the add-on. The only valid value is &lsquo;helm&rsquo;.</p>
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
<p>Indicates the version of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the provider of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>helm</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">
HelmTypeInstallSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the Helm installation specifications. This is only processed
when the type is set to &lsquo;helm&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>defaultInstallValues</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonDefaultInstallSpecItem">
[]AddonDefaultInstallSpecItem
</a>
</em>
</td>
<td>
<p>Specifies the default installation parameters.</p>
</td>
</tr>
<tr>
<td>
<code>install</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">
AddonInstallSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the installation parameters.</p>
</td>
</tr>
<tr>
<td>
<code>installable</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.InstallableSpec">
InstallableSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the installable specifications of the add-on. This includes
the selector and auto-install settings.</p>
</td>
</tr>
<tr>
<td>
<code>cliPlugins</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.CliPlugin">
[]CliPlugin
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the CLI plugin installation specifications.</p>
</td>
</tr>
<tr>
<td>
<code>addonDependencies</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonDependency">
[]AddonDependency
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify all addons that this addon depends on.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonStatus">
AddonStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonDefaultInstallSpecItem">AddonDefaultInstallSpecItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>AddonInstallSpec</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">
AddonInstallSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>AddonInstallSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>selectors</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.SelectorRequirement">
[]SelectorRequirement
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates the default selectors for add-on installations. If multiple selectors are provided,
all selectors must evaluate to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonDependency">AddonDependency
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
</div>
<table>
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
<p>The name of the dependent addon.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>All matching versions of the dependent addon. If empty, defaults to the same version as the current addon.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonInstallExtraItem">AddonInstallExtraItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">AddonInstallSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>AddonInstallSpecItem</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpecItem">
AddonInstallSpecItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>AddonInstallSpecItem</code> are embedded into this type.)
</p>
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
<p>Specifies the name of the item.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">AddonInstallSpec
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonDefaultInstallSpecItem">AddonDefaultInstallSpecItem</a>, <a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>AddonInstallSpecItem</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpecItem">
AddonInstallSpecItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>AddonInstallSpecItem</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>enabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Can be set to true if there are no specific installation attributes to be set.</p>
</td>
</tr>
<tr>
<td>
<code>extras</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallExtraItem">
[]AddonInstallExtraItem
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the installation specifications for extra items.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonInstallSpecItem">AddonInstallSpecItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallExtraItem">AddonInstallExtraItem</a>, <a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">AddonInstallSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the number of replicas.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeEnabled</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the Persistent Volume is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>storageClass</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the name of the storage class.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the tolerations in a JSON array string format.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.ResourceRequirements">
ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the resource requirements.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonPhase">AddonPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonStatus">AddonStatus</a>)
</p>
<div>
<p>AddonPhase defines addon phases.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Disabled&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Disabling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Enabled&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Enabling&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonSelectorKey">AddonSelectorKey
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.SelectorRequirement">SelectorRequirement</a>)
</p>
<div>
<p>AddonSelectorKey are selector requirement key types.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;KubeGitVersion&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;KubeProvider&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;KubeVersion&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.Addon">Addon</a>)
</p>
<div>
<p>AddonSpec defines the desired state of an add-on.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the description of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonType">
AddonType
</a>
</em>
</td>
<td>
<p>Defines the type of the add-on. The only valid value is &lsquo;helm&rsquo;.</p>
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
<p>Indicates the version of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the provider of the add-on.</p>
</td>
</tr>
<tr>
<td>
<code>helm</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">
HelmTypeInstallSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the Helm installation specifications. This is only processed
when the type is set to &lsquo;helm&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>defaultInstallValues</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonDefaultInstallSpecItem">
[]AddonDefaultInstallSpecItem
</a>
</em>
</td>
<td>
<p>Specifies the default installation parameters.</p>
</td>
</tr>
<tr>
<td>
<code>install</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpec">
AddonInstallSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the installation parameters.</p>
</td>
</tr>
<tr>
<td>
<code>installable</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.InstallableSpec">
InstallableSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents the installable specifications of the add-on. This includes
the selector and auto-install settings.</p>
</td>
</tr>
<tr>
<td>
<code>cliPlugins</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.CliPlugin">
[]CliPlugin
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the CLI plugin installation specifications.</p>
</td>
</tr>
<tr>
<td>
<code>addonDependencies</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.AddonDependency">
[]AddonDependency
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specify all addons that this addon depends on.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonStatus">AddonStatus
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.Addon">Addon</a>)
</p>
<div>
<p>AddonStatus defines the observed state of an add-on.</p>
</div>
<table>
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
<a href="#extensions.kubeblocks.io/v1alpha1.AddonPhase">
AddonPhase
</a>
</em>
</td>
<td>
<p>Defines the current installation phase of the add-on. It can take one of
the following values: <code>Disabled</code>, <code>Enabled</code>, <code>Failed</code>, <code>Enabling</code>, <code>Disabling</code>.</p>
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
<p>Provides a detailed description of the current state of add-on API installation.</p>
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
<p>Represents the most recent generation observed for this add-on. It corresponds
to the add-on&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.AddonType">AddonType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
<p>AddonType defines the addon types.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Helm&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.CliPlugin">CliPlugin
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
</div>
<table>
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
<p>Specifies the name of the plugin.</p>
</td>
</tr>
<tr>
<td>
<code>indexRepository</code><br/>
<em>
string
</em>
</td>
<td>
<p>Defines the index repository of the plugin.</p>
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
<p>Provides a brief description of the plugin.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.DataObjectKeySelector">DataObjectKeySelector
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmInstallValues">HelmInstallValues</a>)
</p>
<div>
</div>
<table>
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
<p>Defines the name of the object being referred to.</p>
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
<p>Specifies the key to be selected.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmInstallOptions">HelmInstallOptions
(<code>map[string]string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">HelmTypeInstallSpec</a>)
</p>
<div>
</div>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmInstallValues">HelmInstallValues
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">HelmTypeInstallSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>urls</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the URL location of the values file.</p>
</td>
</tr>
<tr>
<td>
<code>configMapRefs</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.DataObjectKeySelector">
[]DataObjectKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a key from a ConfigMap item list. The value can be
a JSON or YAML string content. Use a key name with &ldquo;.json&rdquo;, &ldquo;.yaml&rdquo;, or &ldquo;.yml&rdquo;
extension to specify a content type.</p>
</td>
</tr>
<tr>
<td>
<code>secretRefs</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.DataObjectKeySelector">
[]DataObjectKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Selects a key from a Secrets item list. The value can be
a JSON or YAML string content. Use a key name with &ldquo;.json&rdquo;, &ldquo;.yaml&rdquo;, or &ldquo;.yml&rdquo;
extension to specify a content type.</p>
</td>
</tr>
<tr>
<td>
<code>setValues</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Values set during Helm installation. Multiple or separate values can be specified with commas (key1=val1,key2=val2).</p>
</td>
</tr>
<tr>
<td>
<code>setJSONValues</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>JSON values set during Helm installation. Multiple or separate values can be specified with commas (key1=jsonval1,key2=jsonval2).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmJSONValueMapType">HelmJSONValueMapType
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">HelmValuesMappingItem</a>)
</p>
<div>
</div>
<table>
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
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the toleration mapping key.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">HelmTypeInstallSpec
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
<p>HelmTypeInstallSpec defines the Helm installation spec.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>chartLocationURL</code><br/>
<em>
string
</em>
</td>
<td>
<p>Specifies the URL location of the Helm Chart.</p>
</td>
</tr>
<tr>
<td>
<code>installOptions</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmInstallOptions">
HelmInstallOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the options for Helm release installation.</p>
</td>
</tr>
<tr>
<td>
<code>installValues</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmInstallValues">
HelmInstallValues
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the set values for Helm release installation.</p>
</td>
</tr>
<tr>
<td>
<code>valuesMapping</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMapping">
HelmValuesMapping
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the mapping of add-on normalized resources parameters to Helm values&rsquo; keys.</p>
</td>
</tr>
<tr>
<td>
<code>chartsImage</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the image of Helm charts.</p>
</td>
</tr>
<tr>
<td>
<code>chartsPathInImage</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the path of Helm charts in the image. This path is used to copy
Helm charts from the image to the shared volume. The default path is &ldquo;/charts&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmValueMapType">HelmValueMapType
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">HelmValuesMappingItem</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicaCount</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the key for setting the replica count in the Helm values map.</p>
</td>
</tr>
<tr>
<td>
<code>persistentVolumeEnabled</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether the persistent volume is enabled in the Helm values map.</p>
</td>
</tr>
<tr>
<td>
<code>storageClass</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the key for setting the storage class in the Helm values map.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmValuesMapping">HelmValuesMapping
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmTypeInstallSpec">HelmTypeInstallSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>HelmValuesMappingItem</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">
HelmValuesMappingItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>HelmValuesMappingItem</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>extras</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingExtraItem">
[]HelmValuesMappingExtraItem
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Helm value mapping items for extra items.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmValuesMappingExtraItem">HelmValuesMappingExtraItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMapping">HelmValuesMapping</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>HelmValuesMappingItem</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">
HelmValuesMappingItem
</a>
</em>
</td>
<td>
<p>
(Members of <code>HelmValuesMappingItem</code> are embedded into this type.)
</p>
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
<p>Name of the item.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">HelmValuesMappingItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMapping">HelmValuesMapping</a>, <a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingExtraItem">HelmValuesMappingExtraItem</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>valueMap</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmValueMapType">
HelmValueMapType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the &ldquo;key&rdquo; mapping values. Valid keys include <code>replicaCount</code>,
<code>persistentVolumeEnabled</code>, and <code>storageClass</code>.
Enum values explained:</p>
<ul>
<li><code>replicaCount</code> sets the replicaCount value mapping key.</li>
<li><code>persistentVolumeEnabled</code> sets the persistent volume enabled mapping key.</li>
<li><code>storageClass</code> sets the storageClass mapping key.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>jsonMap</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.HelmJSONValueMapType">
HelmJSONValueMapType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defines the &ldquo;key&rdquo; mapping values. The valid key is tolerations.
Enum values explained:</p>
<ul>
<li><code>tolerations</code> sets the toleration mapping key.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.ResourceMappingItem">
ResourceMappingItem
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Sets resources related mapping keys.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.InstallableSpec">InstallableSpec
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonSpec">AddonSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>selectors</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.SelectorRequirement">
[]SelectorRequirement
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the selectors for add-on installation. If multiple selectors are provided,
they must all evaluate to true for the add-on to be installed.</p>
</td>
</tr>
<tr>
<td>
<code>autoInstall</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Indicates whether an add-on should be installed automatically.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.LineSelectorOperator">LineSelectorOperator
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.SelectorRequirement">SelectorRequirement</a>)
</p>
<div>
<p>LineSelectorOperator defines line selector operators.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Contains&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;DoesNotContain&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;DoesNotMatchRegex&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;MatchRegex&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.ResourceMappingItem">ResourceMappingItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.HelmValuesMappingItem">HelmValuesMappingItem</a>)
</p>
<div>
</div>
<table>
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
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the key used for mapping the storage size value.</p>
</td>
</tr>
<tr>
<td>
<code>cpu</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.ResourceReqLimItem">
ResourceReqLimItem
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the key used for mapping both CPU requests and limits.</p>
</td>
</tr>
<tr>
<td>
<code>memory</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.ResourceReqLimItem">
ResourceReqLimItem
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the key used for mapping both Memory requests and limits.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.ResourceReqLimItem">ResourceReqLimItem
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.ResourceMappingItem">ResourceMappingItem</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>requests</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the mapping key for the request value.</p>
</td>
</tr>
<tr>
<td>
<code>limits</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specifies the mapping key for the limit value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.ResourceRequirements">ResourceRequirements
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonInstallSpecItem">AddonInstallSpecItem</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>limits</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits describes the maximum amount of compute resources allowed.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a>.</p>
</td>
</tr>
<tr>
<td>
<code>requests</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requests describes the minimum amount of compute resources required.
If Requests is omitted for a container, it defaults to Limits if that is explicitly specified;
otherwise, it defaults to an implementation-defined value.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="extensions.kubeblocks.io/v1alpha1.SelectorRequirement">SelectorRequirement
</h3>
<p>
(<em>Appears on:</em><a href="#extensions.kubeblocks.io/v1alpha1.AddonDefaultInstallSpecItem">AddonDefaultInstallSpecItem</a>, <a href="#extensions.kubeblocks.io/v1alpha1.InstallableSpec">InstallableSpec</a>)
</p>
<div>
</div>
<table>
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
<a href="#extensions.kubeblocks.io/v1alpha1.AddonSelectorKey">
AddonSelectorKey
</a>
</em>
</td>
<td>
<p>The selector key. Valid values are KubeVersion, KubeGitVersion and KubeProvider.</p>
<ul>
<li><code>KubeVersion</code> the semver expression of Kubernetes versions, i.e., v1.24.</li>
<li><code>KubeGitVersion</code> may contain distro. info., i.e., v1.24.4+eks.</li>
<li><code>KubeProvider</code> the Kubernetes provider, i.e., aws, gcp, azure, huaweiCloud, tencentCloud etc.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>operator</code><br/>
<em>
<a href="#extensions.kubeblocks.io/v1alpha1.LineSelectorOperator">
LineSelectorOperator
</a>
</em>
</td>
<td>
<p>Represents a key&rsquo;s relationship to a set of values.
Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.</p>
<p>Possible enum values:</p>
<ul>
<li><code>Contains</code> line contains a string.</li>
<li><code>DoesNotContain</code> line does not contain a string.</li>
<li><code>MatchRegex</code> line contains a match to the regular expression.</li>
<li><code>DoesNotMatchRegex</code> line does not contain a match to the regular expression.</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>values</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Represents an array of string values. This serves as an &ldquo;OR&rdquo; expression to the operator.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>