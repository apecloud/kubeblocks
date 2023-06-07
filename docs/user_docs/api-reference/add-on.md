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
<p>Addon is the Schema for the add-ons API.</p><br />
</div>
<table>
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
<p>Addon description.</p><br />
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
<p>Add-on type. The valid value is helm.</p><br />
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
<p>Helm installation spec. It&rsquo;s processed only when type=helm.</p><br />
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
<p>Default installation parameters.</p><br />
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
<p>Installation parameters.</p><br />
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
<p>Addon installable spec. It provides selector and auto-install settings.</p><br />
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
<p>Plugin installation spec.</p><br />
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
<p>Addon installs parameters selectors by default. If multiple selectors are provided,<br />all selectors must evaluate to true.</p><br />
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
<p>Name of the item.</p><br />
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
<p>enabled can be set if there are no specific installation attributes to be set.</p><br />
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
<p>Installs spec. for extra items.</p><br />
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
<p>Replicas value.</p><br />
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
<p>Persistent Volume Enabled value.</p><br />
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
<p>Storage class name.</p><br />
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
<p>Tolerations JSON array string value.</p><br />
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
<p>Resource requirements.</p><br />
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
<p>AddonPhase defines addon phases.</p><br />
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
<p>AddonSelectorKey are selector requirement key types.</p><br />
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
<p>AddonSpec defines the desired state of an add-on.</p><br />
</div>
<table>
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
<p>Addon description.</p><br />
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
<p>Add-on type. The valid value is helm.</p><br />
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
<p>Helm installation spec. It&rsquo;s processed only when type=helm.</p><br />
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
<p>Default installation parameters.</p><br />
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
<p>Installation parameters.</p><br />
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
<p>Addon installable spec. It provides selector and auto-install settings.</p><br />
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
<p>Plugin installation spec.</p><br />
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
<p>AddonStatus defines the observed state of an add-on.</p><br />
</div>
<table>
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
<p>Add-on installation phases. Valid values are Disabled, Enabled, Failed, Enabling, Disabling.</p><br />
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
<p>Describes the current state of add-on API installation conditions.</p><br />
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
<p>observedGeneration is the most recent generation observed for this<br />add-on. It corresponds to the add-on&rsquo;s generation, which is<br />updated on mutation by the API Server.</p><br />
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
<p>AddonType defines the addon types.</p><br />
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
<p>Name of the plugin.</p><br />
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
<p>The index repository of the plugin.</p><br />
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
<p>The description of the plugin.</p><br />
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
<p>Object name of the referent.</p><br />
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
<p>The key to select.</p><br />
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
<p>Selects a key of a ConfigMap item list. The value of ConfigMap can be<br />a JSON or YAML string content. Use a key name with &ldquo;.json&rdquo; or &ldquo;.yaml&rdquo; or &ldquo;.yml&rdquo;<br />extension name to specify a content type.</p><br />
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
<p>Selects a key of a Secrets item list. The value of Secrets can be<br />a JSON or YAML string content. Use a key name with &ldquo;.json&rdquo; or &ldquo;.yaml&rdquo; or &ldquo;.yml&rdquo;<br />extension name to specify a content type.</p><br />
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
<p>Helm install set values. It can specify multiple or separate values with commas(key1=val1,key2=val2).</p><br />
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
<p>Helm install set JSON values. It can specify multiple or separate values with commas(key1=jsonval1,key2=jsonval2).</p><br />
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
<p>tolerations sets the toleration mapping key.</p><br />
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
</div>
<table>
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
<p>A Helm Chart location URL.</p><br />
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
<p>installOptions defines Helm release installation options.</p><br />
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
<p>HelmInstallValues defines Helm release installation set values.</p><br />
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
<p>valuesMapping defines add-on normalized resources parameters mapped to Helm values&rsquo; keys.</p><br />
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
<p>replicaCount sets the replicaCount value mapping key.</p><br />
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
<p>persistentVolumeEnabled sets the persistent volume enabled mapping key.</p><br />
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
<p>storageClass sets the storageClass mapping key.</p><br />
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
<p>Helm value mapping items for extra items.</p><br />
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
<p>Name of the item.</p><br />
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
<p>valueMap define the &ldquo;key&rdquo; mapping values. Valid keys are replicaCount,<br />persistentVolumeEnabled, and storageClass. Enum values explained:<br /><code>&quot;replicaCount&quot;</code> sets the replicaCount value mapping key.<br /><code>&quot;persistentVolumeEnabled&quot;</code> sets the persistent volume enabled mapping key.<br /><code>&quot;storageClass&quot;</code> sets the storageClass mapping key.</p><br />
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
<p>jsonMap defines the &ldquo;key&rdquo; mapping values. The valid key is tolerations.<br />Enum values explained:<br /><code>&quot;tolerations&quot;</code> sets the toleration mapping key.</p><br />
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
<p>resources sets resources related mapping keys.</p><br />
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
<p>Add-on installable selectors. If multiple selectors are provided,<br />all selectors must evaluate to true.</p><br />
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
<p>autoInstall defines an add-on should be installed automatically.</p><br />
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
<p>LineSelectorOperator defines line selector operators.</p><br />
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
<p>storage sets the storage size value mapping key.</p><br />
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
<p>cpu sets CPU requests and limits mapping keys.</p><br />
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
<p>memory sets Memory requests and limits mapping keys.</p><br />
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
<p>Requests value mapping key.</p><br />
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
<p>Limits value mapping key.</p><br />
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
<p>Limits describes the maximum amount of compute resources allowed.<br />More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a>.</p><br />
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
<p>Requests describes the minimum amount of compute resources required.<br />If Requests is omitted for a container, it defaults to Limits if that is explicitly specified;<br />otherwise, it defaults to an implementation-defined value.<br />More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a>.</p><br />
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
<p>The selector key. Valid values are KubeVersion, KubeGitVersion.<br />&ldquo;KubeVersion&rdquo; the semver expression of Kubernetes versions, i.e., v1.24.<br />&ldquo;KubeGitVersion&rdquo; may contain distro. info., i.e., v1.24.4+eks.</p><br />
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
<p>Represents a key&rsquo;s relationship to a set of values.<br />Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.</p><br /><br /><p>Possible enum values:<br /><code>&quot;Contains&quot;</code> line contains a string.<br /><code>&quot;DoesNotContain&quot;</code> line does not contain a string.<br /><code>&quot;MatchRegex&quot;</code> line contains a match to the regular expression.<br /><code>&quot;DoesNotMatchRegex&quot;</code> line does not contain a match to the regular expression.</p><br />
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
<p>An array of string values. It serves as an &ldquo;OR&rdquo; expression to the operator.</p><br />
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
