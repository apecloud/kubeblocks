/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var clusterCreateExample = templates.Examples(`
	# Create a cluster using cluster definition my-cluster-def and cluster version my-version
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --cluster-version=my-version

	# --cluster-definition is required, if --cluster-version is not specified, will use the most recently created version
	kbcli cluster create mycluster --cluster-definition=my-cluster-def

	# Create a cluster and set termination policy DoNotDelete that will prevent the cluster from being deleted
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --termination-policy=DoNotDelete

	# In scenarios where you want to delete resources such as statements, deployments, services, pdb, but keep PVCs
	# when deleting the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --termination-policy=Halt

	# In scenarios where you want to delete resource such as statements, deployments, services, pdb, and including
	# PVCs when deleting the cluster, use termination policy Delete
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --termination-policy=Delete

	# In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
	# the cluster, use termination policy WipeOut
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --termination-policy=WipeOut

	# Create a cluster and set cpu to 1000m, memory to 1Gi, storage size to 10Gi and replicas to 2
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=cpu=1000m,memory=1Gi,storage=10Gi,replicas=2

	# Create a cluster and use a URL to set cluster resource
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set-file=https://kubeblocks.io/yamls/my.yaml

	# Create a cluster and load cluster resource set from stdin
	cat << EOF | kbcli cluster create mycluster --cluster-definition=my-cluster-def --set-file -
	- name: my-test ...

	# Create a cluster forced to scatter by node
	kbcli cluster create --cluster-definition=my-cluster-def --topology-keys=kubernetes.io/hostname --pod-anti-affinity=Required

	# Create a cluster in specific labels nodes
	kbcli cluster create --cluster-definition=my-cluster-def --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'

	# Create a Cluster with two tolerations 
	kbcli cluster create --cluster-definition=my-cluster-def --tolerations='"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
`)

const (
	CueTemplateName = "cluster_template.cue"
	monitorKey      = "monitor"
)

type setKey string

const (
	keyType     setKey = "type"
	keyCPU      setKey = "cpu"
	keyMemory   setKey = "memory"
	keyReplicas setKey = "replicas"
	keyStorage  setKey = "storage"
	keyUnknown  setKey = "unknown"
)

type envSet struct {
	name       string
	defaultVal string
}

var setKeyEnvMap = map[setKey]envSet{
	keyCPU:      {"CLUSTER_DEFAULT_CPU", "1000m"},
	keyMemory:   {"CLUSTER_DEFAULT_MEMORY", "1Gi"},
	keyStorage:  {"CLUSTER_DEFAULT_STORAGE_SIZE", "10Gi"},
	keyReplicas: {"CLUSTER_DEFAULT_REPLICAS", "1"},
}

// UpdatableFlags is the flags that cat be updated by update command
type UpdatableFlags struct {
	TerminationPolicy string `json:"terminationPolicy"`
	PodAntiAffinity   string `json:"podAntiAffinity"`
	Monitor           bool   `json:"monitor"`
	EnableAllLogs     bool   `json:"enableAllLogs"`

	// TopologyKeys if TopologyKeys is nil, add omitempty json tag.
	// because CueLang can not covert null to list.
	TopologyKeys   []string          `json:"topologyKeys,omitempty"`
	NodeLabels     map[string]string `json:"nodeLabels,omitempty"`
	TolerationsRaw []string          `json:"-"`
}

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string                   `json:"clusterDefRef"`
	ClusterVersionRef string                   `json:"clusterVersionRef"`
	Tolerations       []interface{}            `json:"tolerations,omitempty"`
	Components        []map[string]interface{} `json:"components"`

	SetFile        string   `json:"-"`
	Values         []string `json:"-"`
	TolerationsRaw []string `json:"-"`

	// backup name to restore in creation
	Backup string `json:"backup,omitempty"`
	UpdatableFlags
	create.BaseOptions
}

func setMonitor(monitor bool, components []map[string]interface{}) {
	if len(components) == 0 {
		return
	}
	for _, component := range components {
		component[monitorKey] = monitor
	}
}

func setBackup(o *CreateOptions, components []map[string]interface{}) error {
	backup := o.Backup
	if len(backup) == 0 || len(components) == 0 {
		return nil
	}

	gvr := schema.GroupVersionResource{Group: types.DPGroup, Version: types.DPVersion, Resource: types.ResourceBackups}
	backupObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), backup, metav1.GetOptions{})
	if err != nil {
		return err
	}
	backupType, _, _ := unstructured.NestedString(backupObj.Object, "spec", "backupType")
	if backupType != "snapshot" {
		return fmt.Errorf("only support snapshot backup, specified backup type is '%v'", backupType)
	}

	dataSource := make(map[string]interface{}, 0)
	_ = unstructured.SetNestedField(dataSource, backup, "name")
	_ = unstructured.SetNestedField(dataSource, "VolumeSnapshot", "kind")
	_ = unstructured.SetNestedField(dataSource, "snapshot.storage.k8s.io", "apiGroup")

	for _, component := range components {
		templates := component["volumeClaimTemplates"].([]interface{})
		for _, t := range templates {
			templateMap := t.(map[string]interface{})
			_ = unstructured.SetNestedField(templateMap, dataSource, "spec", "dataSource")
		}
	}
	return nil
}

func (o *CreateOptions) Validate() error {
	if o.ClusterDefRef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one, run \"kbcli cluster-definition list\" to show all cluster definition")
	}

	if o.TerminationPolicy == "" {
		return fmt.Errorf("a valid termination policy is needed, use --termination-policy to specify one of: DoNotTerminate, Halt, Delete, WipeOut")
	}

	if o.ClusterVersionRef == "" {
		version, err := cluster.GetLatestVersion(o.Client, o.ClusterDefRef)
		if err != nil {
			return err
		}
		o.ClusterVersionRef = version
		fmt.Fprintf(o.Out, "Cluster version is not specified, use the recently created ClusterVersion %s\n", o.ClusterVersionRef)
	}

	if len(o.Values) > 0 && len(o.SetFile) > 0 {
		return fmt.Errorf("does not support --set and --set-file being specified at the same time")
	}

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		name, err := generateClusterName(o.Client, o.Namespace)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("failed to generate a random cluster name")
		}
		o.Name = name
	}
	return nil
}

func (o *CreateOptions) Complete() error {
	components, err := o.buildComponents()
	if err != nil {
		return err
	}

	setMonitor(o.Monitor, components)
	if err := setBackup(o, components); err != nil {
		return err
	}
	o.Components = components

	// TolerationsRaw looks like `["key=engineType,value=mongo,operator=Equal,effect=NoSchedule"]` after parsing by cmd
	tolerations := buildTolerations(o.TolerationsRaw)
	if len(tolerations) > 0 {
		o.Tolerations = tolerations
	}
	return nil
}

// buildComponents build components from file or set values
func (o *CreateOptions) buildComponents() ([]map[string]interface{}, error) {
	var (
		componentByte []byte
		err           error
	)

	// build components from file
	components := o.Components
	if len(o.SetFile) > 0 {
		if componentByte, err = MultipleSourceComponents(o.SetFile, o.IOStreams.In); err != nil {
			return nil, err
		}
		if componentByte, err = yaml.YAMLToJSON(componentByte); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(componentByte, &components); err != nil {
			return nil, err
		}
		return components, nil
	}

	// build components from set values or environment variables
	if len(components) == 0 {
		cd, err := cluster.GetClusterDefByName(o.Client, o.ClusterDefRef)
		if err != nil {
			return nil, err
		}

		compSets, err := buildCompSetsMap(o.Values, cd)
		if err != nil {
			return nil, err
		}

		if components, err = buildClusterComp(cd, compSets); err != nil {
			return nil, err
		}
	}
	return components, nil
}

// MultipleSourceComponents get component data from multiple source, such as stdin, URI and local file
func MultipleSourceComponents(fileName string, in io.Reader) ([]byte, error) {
	var data io.Reader
	switch {
	case fileName == "-":
		data = in
	case strings.Index(fileName, "http://") == 0 || strings.Index(fileName, "https://") == 0:
		resp, err := http.Get(fileName)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data = resp.Body
	default:
		f, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data = f
	}
	return io.ReadAll(data)
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "create [NAME]",
		Short:           "Create a cluster",
		Example:         clusterCreateExample,
		CueTemplateName: CueTemplateName,
		ResourceName:    types.ResourceClusters,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cluster-definition list\" to show all available cluster definition")
			cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Specify cluster version, run \"kbcli cluster-version list\" to show all available cluster version, use the latest version if not specified")
			cmd.Flags().StringVarP(&o.SetFile, "set-file", "f", "", "Use yaml file, URL, or stdin to set the cluster resource")
			cmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "Set the cluster resource including cpu, memory, replicas and storage, each set corresponds to a component.(e.g. --set cpu=1000m,memory=1Gi,replicas=3,storage=10Gi)")
			cmd.Flags().StringVar(&o.Backup, "backup", "", "Set a source backup to restore data")

			// add updatable flags
			o.UpdatableFlags.addFlags(cmd)

			// set required flag
			util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))

			// register flag completion func
			registerFlagCompletionFunc(cmd, f)
		},
	}

	return create.BuildCommand(inputs)
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-version",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterVersionGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

// PreCreate before commit yaml to k8s, make changes on Unstructured yaml
func (o *CreateOptions) PreCreate(obj *unstructured.Unstructured) error {
	if !o.EnableAllLogs {
		// EnableAllLogs is false, nothing will change
		return nil
	}
	c := &dbaasv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}
	// get cluster definition from k8s
	cd, err := cluster.GetClusterDefByName(o.Client, c.Spec.ClusterDefRef)
	if err != nil {
		return err
	}
	setEnableAllLogs(c, cd)
	data, e := runtime.DefaultUnstructuredConverter.ToUnstructured(c)
	if e != nil {
		return e
	}
	obj.SetUnstructuredContent(data)
	return nil
}

// setEnableAllLog set enable all logs, and ignore enabledLogs of component level.
func setEnableAllLogs(c *dbaasv1alpha1.Cluster, cd *dbaasv1alpha1.ClusterDefinition) {
	for idx, comCluster := range c.Spec.Components {
		for _, com := range cd.Spec.Components {
			if !strings.EqualFold(comCluster.Type, com.TypeName) {
				continue
			}
			typeList := make([]string, 0, len(com.LogConfigs))
			for _, logConf := range com.LogConfigs {
				typeList = append(typeList, logConf.Name)
			}
			c.Spec.Components[idx].EnabledLogs = typeList
		}
	}
}

func buildClusterComp(cd *dbaasv1alpha1.ClusterDefinition, setsMap map[string]map[setKey]string) ([]map[string]interface{}, error) {
	getVal := func(key setKey, sets map[setKey]string) string {
		// get value from set values
		if sets != nil {
			if v := sets[key]; len(v) > 0 {
				return v
			}
		}

		// get value from environment variables
		env := setKeyEnvMap[key]
		val := viper.GetString(env.name)
		if len(val) == 0 {
			val = env.defaultVal
		}
		return val
	}

	var comps []map[string]interface{}
	for _, c := range cd.Spec.Components {
		// if cluster definition component default replicas greater than 0, build a cluster component
		// by cluster definition component.
		replicas := c.DefaultReplicas
		if replicas <= 0 {
			continue
		}

		sets := map[setKey]string{}
		if setsMap != nil {
			sets = setsMap[c.TypeName]
		}

		// get replicas
		setReplicas, err := strconv.Atoi(getVal(keyReplicas, sets))
		if err != nil {
			return nil, fmt.Errorf("repicas is illegal " + err.Error())
		}
		replicas = int32(setReplicas)

		resourceList := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(getVal(keyCPU, sets)),
			corev1.ResourceMemory: resource.MustParse(getVal(keyMemory, sets)),
		}
		compObj := &dbaasv1alpha1.ClusterComponent{
			Name:     c.TypeName,
			Type:     c.TypeName,
			Replicas: &replicas,
			Resources: corev1.ResourceRequirements{
				Requests: resourceList,
				Limits:   resourceList,
			},
			VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
				Name: "data",
				Spec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(getVal(keyStorage, sets)),
						},
					},
				},
			}},
		}
		comp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(compObj)
		if err != nil {
			return nil, err
		}
		comps = append(comps, comp)
	}
	return comps, nil
}

// buildCompSetsMap builds the map between component type name and its set values, if the type name is not
// specified in the set, use the cluster definition default component type name.
func buildCompSetsMap(values []string, cd *dbaasv1alpha1.ClusterDefinition) (map[string]map[setKey]string, error) {
	allSets := map[string]map[setKey]string{}
	parseKey := func(key string) setKey {
		keys := []setKey{keyCPU, keyType, keyStorage, keyMemory, keyReplicas}
		for _, k := range keys {
			if k == setKey(key) {
				return setKey(key)
			}
		}
		return keyUnknown
	}
	buildSetMap := func(sets []string) map[setKey]string {
		res := map[setKey]string{}
		for _, set := range sets {
			kv := strings.Split(set, "=")
			if len(kv) != 2 {
				continue
			}

			// only record the supported key
			k := parseKey(kv[0])
			if k == keyUnknown {
				continue
			}
			res[setKey(kv[0])] = kv[1]
		}
		return res
	}

	// each value corresponds to a component
	for _, value := range values {
		sets := buildSetMap(strings.Split(value, ","))
		if len(sets) == 0 {
			continue
		}

		// get the component type name
		typeName := sets[keyType]

		// type is not specified by user, use the default component type name, now only
		// support cluster definition with one component
		if len(typeName) == 0 {
			name, err := cluster.GetDefaultCompTypeName(cd)
			if err != nil {
				return nil, err
			}
			typeName = name
		}

		// if already set by other value, later values override earlier values
		if old, ok := allSets[typeName]; ok {
			for k, v := range sets {
				old[k] = v
			}
			sets = old
		}
		allSets[typeName] = sets
	}
	return allSets, nil
}

func buildTolerations(raw []string) []interface{} {
	tolerations := make([]interface{}, 0)
	for _, tolerationRaw := range raw {
		toleration := map[string]interface{}{}
		for _, entries := range strings.Split(tolerationRaw, ",") {
			parts := strings.SplitN(entries, "=", 2)
			toleration[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
		tolerations = append(tolerations, toleration)
	}
	return tolerations
}

// generateClusterName generate a random cluster name that does not exist
func generateClusterName(dynamic dynamic.Interface, namespace string) (string, error) {
	var name string
	// retry 10 times
	for i := 0; i < 10; i++ {
		name = cluster.GenerateName()
		// check whether the cluster exists, if not found, return it
		_, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func (f *UpdatableFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type")
	cmd.Flags().BoolVar(&f.Monitor, "monitor", true, "Set monitor enabled and inject metrics exporter")
	cmd.Flags().BoolVar(&f.EnableAllLogs, "enable-all-logs", true, "Enable advanced application all log extraction, and true will ignore enabledLogs of component level")
	cmd.Flags().StringVar(&f.TerminationPolicy, "termination-policy", "Delete", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
	cmd.Flags().StringArrayVar(&f.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	cmd.Flags().StringToStringVar(&f.NodeLabels, "node-labels", nil, "Node label selector")
	cmd.Flags().StringSliceVar(&f.TolerationsRaw, "tolerations", nil, `Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'`)

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"termination-policy",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"DoNotTerminate\tblock delete operation",
				"Halt\tdelete workload resources such as statefulset, deployment workloads but keep PVCs",
				"Delete\tbased on Halt and deletes PVCs",
				"WipeOut\tbased on Delete and wipe out all volume snapshots and snapshot data from backup storage location",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
}
