<p>Packages:</p>
<ul>
<li>
<a href="#extensions.kubeblocks.io%2fv1alpha1">extensions.kubeblocks.io/v1alpha1</a>
</li>
</ul>
<h2 id="extensions.kubeblocks.io/v1alpha1">extensions.kubeblocks.io/v1alpha1</h2>
Resource Types:
<ul><li>
<a href="#extensions.kubeblocks.io/v1alpha1.Addon">Addon</a>
</li></ul>
<h3 id="extensions.kubeblocks.io/v1alpha1.Addon">Addon
</h3>
<div>
<p>Addon is the Schema for the addons API</p>
</div>
<table>
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
<p>Addon description.</p>
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
<p>Addon type, valid value is helm.</p>
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
<p>Helm installation spec., it&rsquo;s only being processed if type=helm.</p>
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
<p>Default installation parameters.</p>
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
<p>Installation parameters.</p>
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
<p>Addon installable spec., provide selector and auto-install settings.</p>
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
<p>Plugin installation spec.</p>
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
<p>Addon default install parameters selectors. If multiple selectors are provided
that all selectors must evaluate to true.</p>
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
<p>Name of the item.</p>
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
<p>enabled can be set if there are no specific installation attributes to be set.</p>
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
<p>Install spec. for extra items.</p>
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
<p>Replicas value.</p>
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
<p>Persistent Volume Enabled value.</p>
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
<p>Storage class name.</p>
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
<p>Tolerations JSON array string value.</p>
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
<p>Resource requirements.</p>
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
<p>AddonSpec defines the desired state of Addon</p>
</div>
<table>
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
<p>Addon description.</p>
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
<p>Addon type, valid value is helm.</p>
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
<p>Helm installation spec., it&rsquo;s only being processed if type=helm.</p>
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
<p>Default installation parameters.</p>
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
<p>Installation parameters.</p>
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
<p>Addon installable spec., provide selector and auto-install settings.</p>
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
<p>Plugin installation spec.</p>
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
<p>AddonStatus defines the observed state of Addon</p>
</div>
<table>
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
<p>Addon installation phases. Valid values are Disabled, Enabled, Failed, Enabling, Disabling.</p>
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
<p>Describe current state of Addon API installation conditions.</p>
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
Addon. It corresponds to the Addon&rsquo;s generation, which is
updated on mutation by the API Server.</p>
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
<p>Name of the plugin.</p>
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
<p>The index repository of the plugin.</p>
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
<p>The description of the plugin.</p>
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
<p>Object name of the referent.</p>
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
<p>The key to select.</p>
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
<p>Selects a key of a ConfigMap item list, the value of ConfigMap can be
a JSON or YAML string content, use key name with &ldquo;.json&rdquo; or &ldquo;.yaml&rdquo; or &ldquo;.yml&rdquo;
extension name to specify content type.</p>
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
<p>Selects a key of a Secrets item list, the value of Secrets can be
a JSON or YAML string content, use key name with &ldquo;.json&rdquo; or &ldquo;.yaml&rdquo; or &ldquo;.yml&rdquo;
extension name to specify content type.</p>
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
<p>Helm install set values, can specify multiple or separate values with commas(key1=val1,key2=val2).</p>
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
<p>Helm install set JSON values, can specify multiple or separate values with commas(key1=jsonval1,key2=jsonval2).</p>
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
<p>tolerations sets toleration mapping key.</p>
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
<p>A Helm Chart location URL.</p>
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
<p>installOptions defines Helm release install options.</p>
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
<p>HelmInstallValues defines Helm release install set values.</p>
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
<p>valuesMapping defines addon normalized resources parameters mapped to Helm values&rsquo; keys.</p>
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
<p>replicaCount sets replicaCount value mapping key.</p>
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
<p>persistentVolumeEnabled persistent volume enabled mapping key.</p>
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
<p>storageClass sets storageClass mapping key.</p>
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
<p>valueMap define the &ldquo;key&rdquo; mapping values, valid keys are replicaCount,
persistentVolumeEnabled, and storageClass. Enum values explained:
<code>&quot;replicaCount&quot;</code> sets replicaCount value mapping key
<code>&quot;persistentVolumeEnabled&quot;</code> sets persistent volume enabled mapping key
<code>&quot;storageClass&quot;</code> sets storageClass mapping key</p>
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
<p>jsonMap define the &ldquo;key&rdquo; mapping values, valid keys are tolerations.
Enum values explained:
<code>&quot;tolerations&quot;</code> sets toleration mapping key</p>
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
<p>resources sets resources related mapping keys.</p>
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
<p>Addon installable selectors. If multiple selectors are provided
that all selectors must evaluate to true.</p>
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
<p>autoInstall defines an addon should auto installed</p>
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
<p>storage sets storage size value mapping key.</p>
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
<p>cpu sets CPU requests and limits mapping keys.</p>
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
<p>memory sets Memory requests and limits mapping keys.</p>
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
<p>Requests value mapping key.</p>
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
<p>Limits value mapping key.</p>
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
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
otherwise to an implementation-defined value.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/">https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</a></p>
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
<p>The selector key, valid values are KubeVersion, KubeGitVersion.
&ldquo;KubeVersion&rdquo; the semver expression of Kubernetes versions, i.e., v1.24.
&ldquo;KubeGitVersion&rdquo; may contain distro. info., i.e., v1.24.4+eks.</p>
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
<p>Possible enum values:
<code>&quot;Contains&quot;</code> line contains string
<code>&quot;DoesNotContain&quot;</code> line does not contain string
<code>&quot;MatchRegex&quot;</code> line contains a match to the regular expression
<code>&quot;DoesNotMatchRegex&quot;</code> line does not contain a match to the regular expression</p>
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
<p>An array of string values. Server as &ldquo;OR&rdquo; expression to the operator.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
