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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var clusterCreateExample = templates.Examples(`
	# Create a cluster with cluster definition apecloud-mysql and cluster version ac-mysql-8.0.30
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --cluster-version ac-mysql-8.0.30

	# --cluster-definition is required, if --cluster-version is not specified, will use the most recently created version
	kbcli cluster create mycluster --cluster-definition apecloud-mysql

	# OOutput resource information in YAML format, but do not create resources.
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run=client -o yaml

	# Output resource information in YAML format, the information will be sent to the server, but the resource will not be actually created.
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run=server -o yaml
	
	# Create a cluster and set termination policy DoNotTerminate that will prevent the cluster from being deleted
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy DoNotTerminate

	# In scenarios where you want to delete resources such as statements, deployments, services, pdb, but keep PVCs
	# when deleting the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Halt

	# In scenarios where you want to delete resource such as statements, deployments, services, pdb, and including
	# PVCs when deleting the cluster, use termination policy Delete
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Delete

	# In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
	# the cluster, use termination policy WipeOut
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy WipeOut

	# Create a cluster and set cpu to 1 core, memory to 1Gi, storage size to 20Gi and replicas to 3
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set cpu=1,memory=1Gi,storage=20Gi,replicas=3

	# Create a cluster and set the class to general-1c1g, valid classes can be found by executing the command "kbcli class list --cluster-definition=<cluster-definition-name>"
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set class=general-1c1g

	# Create a cluster with replicationSet workloadType and set switchPolicy to Noop
	kbcli cluster create mycluster --cluster-definition postgresql --set switchPolicy=Noop

	# Create a cluster and use a URL to set cluster resource
	kbcli cluster create mycluster --cluster-definition apecloud-mysql --set-file https://kubeblocks.io/yamls/apecloud-mysql.yaml

	# Create a cluster and load cluster resource set from stdin
	cat << EOF | kbcli cluster create mycluster --cluster-definition apecloud-mysql --set-file -
	- name: my-test ...

	# Create a cluster forced to scatter by node
	kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname --pod-anti-affinity Required

	# Create a cluster in specific labels nodes
	kbcli cluster create --cluster-definition apecloud-mysql --node-labels '"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'

	# Create a Cluster with two tolerations 
	kbcli cluster create --cluster-definition apecloud-mysql --tolerations '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'

    # Create a cluster, with each pod runs on their own dedicated node
    kbcli cluster create --cluster-definition apecloud-mysql --tenancy=DedicatedNode
`)

const (
	CueTemplateName = "cluster_template.cue"
	monitorKey      = "monitor"
)

type setKey string

const (
	keyType         setKey = "type"
	keyCPU          setKey = "cpu"
	keyClass        setKey = "class"
	keyMemory       setKey = "memory"
	keyReplicas     setKey = "replicas"
	keyStorage      setKey = "storage"
	keySwitchPolicy setKey = "switchPolicy"
	keyUnknown      setKey = "unknown"
)

type envSet struct {
	name       string
	defaultVal string
}

var setKeyEnvMap = map[setKey]envSet{
	keyCPU:      {"CLUSTER_DEFAULT_CPU", "1000m"},
	keyMemory:   {"CLUSTER_DEFAULT_MEMORY", "1Gi"},
	keyStorage:  {"CLUSTER_DEFAULT_STORAGE_SIZE", "20Gi"},
	keyReplicas: {"CLUSTER_DEFAULT_REPLICAS", "1"},
}

// UpdatableFlags is the flags that cat be updated by update command
type UpdatableFlags struct {
	// Options for cluster termination policy
	TerminationPolicy string `json:"terminationPolicy"`

	// Add-on switches for cluster observability
	Monitor       bool `json:"monitor"`
	EnableAllLogs bool `json:"enableAllLogs"`

	// Configuration and options for cluster affinity and tolerations
	PodAntiAffinity string `json:"podAntiAffinity"`
	// TopologyKeys if TopologyKeys is nil, add omitempty json tag, because CueLang can not covert null to list.
	TopologyKeys   []string          `json:"topologyKeys,omitempty"`
	NodeLabels     map[string]string `json:"nodeLabels,omitempty"`
	Tenancy        string            `json:"tenancy"`
	TolerationsRaw []string          `json:"-"`
}

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string                   `json:"clusterDefRef"`
	ClusterVersionRef string                   `json:"clusterVersionRef"`
	Tolerations       []interface{}            `json:"tolerations,omitempty"`
	ComponentSpecs    []map[string]interface{} `json:"componentSpecs"`
	Annotations       map[string]string        `json:"annotations,omitempty"`
	SetFile           string                   `json:"-"`
	Values            []string                 `json:"-"`

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

func getRestoreFromBackupAnnotation(backup *dataprotectionv1alpha1.Backup, compSpecsCount int, firstCompName string) (string, error) {
	componentName := backup.Labels[constant.KBAppComponentLabelKey]
	if len(componentName) == 0 {
		if compSpecsCount != 1 {
			return "", fmt.Errorf("unable to obtain the name of the component to be recovered, please ensure that Backup.status.componentName exists")
		}
		componentName = firstCompName
	}
	restoreFromBackupAnnotation := fmt.Sprintf(`{"%s":"%s"}`, componentName, backup.Name)
	return restoreFromBackupAnnotation, nil
}

func setBackup(o *CreateOptions, components []map[string]interface{}) error {
	backupName := o.Backup
	if len(backupName) == 0 || len(components) == 0 {
		return nil
	}
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, backupName); err != nil {
		return err
	}
	if backup.Status.Phase != dataprotectionv1alpha1.BackupCompleted {
		return fmt.Errorf(`backup "%s" is not completed`, backup.Name)
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, len(components), components[0]["name"].(string))
	if err != nil {
		return err
	}
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
	o.Annotations[constant.RestoreFromBackUpAnnotationKey] = restoreAnnotation
	return nil
}

func (o *CreateOptions) Validate() error {
	if o.ClusterDefRef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one, run \"kbcli clusterdefinition list\" to show all cluster definition")
	}

	if o.TerminationPolicy == "" {
		return fmt.Errorf("a valid termination policy is needed, use --termination-policy to specify one of: DoNotTerminate, Halt, Delete, WipeOut")
	}

	if o.ClusterVersionRef == "" {
		version, err := cluster.GetLatestVersion(o.Dynamic, o.ClusterDefRef)
		if err != nil {
			return err
		}
		o.ClusterVersionRef = version
		printer.Warning(o.Out, "cluster version is not specified, use the recently created ClusterVersion %s\n", o.ClusterVersionRef)
	}

	if len(o.Values) > 0 && len(o.SetFile) > 0 {
		return fmt.Errorf("does not support --set and --set-file being specified at the same time")
	}

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		name, err := generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("failed to generate a random cluster name")
		}
		o.Name = name
	}
	if len(o.Name) > 16 {
		return fmt.Errorf("cluster name should be less than 16 characters")
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
	o.ComponentSpecs = components

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
	components := o.ComponentSpecs
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
		cd, err := cluster.GetClusterDefByName(o.Dynamic, o.ClusterDefRef)
		if err != nil {
			return nil, err
		}

		compSets, err := buildCompSetsMap(o.Values, cd)
		if err != nil {
			return nil, err
		}

		componentObjs, err := buildClusterComp(cd, compSets)
		if err != nil {
			return nil, err
		}
		for _, compObj := range componentObjs {
			comp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(compObj)
			if err != nil {
				return nil, err
			}
			components = append(components, comp)
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
	o := &CreateOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: streams,
		}}

	inputs := create.Inputs{
		Use:             "create [NAME]",
		Short:           "Create a cluster.",
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
			cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cd list\" to show all available cluster definitions")
			cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Specify cluster version, run \"kbcli cv list\" to show all available cluster versions, use the latest version if not specified")
			cmd.Flags().StringVarP(&o.SetFile, "set-file", "f", "", "Use yaml file, URL, or stdin to set the cluster resource")
			cmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "Set the cluster resource including cpu, memory, replicas and storage, or you can just specify the class, each set corresponds to a component.(e.g. --set cpu=1,memory=1Gi,replicas=3,storage=20Gi or --set class=general-1c1g)")
			cmd.Flags().StringVar(&o.Backup, "backup", "", "Set a source backup to restore data")
			cmd.Flags().String("dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"
			// add updatable flags
			o.UpdatableFlags.addFlags(cmd)

			// add print flags
			printer.AddOutputFlagForCreate(cmd, &o.Format)

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

	var formatsWithDesc = map[string]string{
		"JSON": "Output result in JSON format",
		"YAML": "Output result in YAML format",
	}
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range formatsWithDesc {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

// PreCreate before commit yaml to k8s, make changes on Unstructured yaml
func (o *CreateOptions) PreCreate(obj *unstructured.Unstructured) error {
	if !o.EnableAllLogs {
		// EnableAllLogs is false, nothing will change
		return nil
	}
	c := &appsv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}
	// get cluster definition from k8s
	cd, err := cluster.GetClusterDefByName(o.Dynamic, c.Spec.ClusterDefRef)
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
func setEnableAllLogs(c *appsv1alpha1.Cluster, cd *appsv1alpha1.ClusterDefinition) {
	for idx, comCluster := range c.Spec.ComponentSpecs {
		for _, com := range cd.Spec.ComponentDefs {
			if !strings.EqualFold(comCluster.ComponentDefRef, com.Name) {
				continue
			}
			typeList := make([]string, 0, len(com.LogConfigs))
			for _, logConf := range com.LogConfigs {
				typeList = append(typeList, logConf.Name)
			}
			c.Spec.ComponentSpecs[idx].EnabledLogs = typeList
		}
	}
}

func buildClusterComp(cd *appsv1alpha1.ClusterDefinition, setsMap map[string]map[setKey]string) ([]*appsv1alpha1.ClusterComponentSpec, error) {
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

	buildSwitchPolicy := func(c *appsv1alpha1.ClusterComponentDefinition, compObj *appsv1alpha1.ClusterComponentSpec, sets map[setKey]string) error {
		if c.WorkloadType != appsv1alpha1.Replication {
			return nil
		}
		var switchPolicyType appsv1alpha1.SwitchPolicyType
		switch getVal(keySwitchPolicy, sets) {
		case "Noop", "":
			switchPolicyType = appsv1alpha1.Noop
		case "MaximumAvailability":
			switchPolicyType = appsv1alpha1.MaximumAvailability
		case "MaximumPerformance":
			switchPolicyType = appsv1alpha1.MaximumDataProtection
		default:
			return fmt.Errorf("switchPolicy is illegal, only support Noop, MaximumAvailability, MaximumPerformance")
		}
		compObj.SwitchPolicy = &appsv1alpha1.ClusterSwitchPolicy{
			Type: switchPolicyType,
		}
		return nil
	}

	var comps []*appsv1alpha1.ClusterComponentSpec
	for _, c := range cd.Spec.ComponentDefs {
		sets := map[setKey]string{}
		if setsMap != nil {
			sets = setsMap[c.Name]
		}

		// get replicas
		setReplicas, err := strconv.Atoi(getVal(keyReplicas, sets))
		if err != nil {
			return nil, fmt.Errorf("repicas is illegal " + err.Error())
		}
		replicas := int32(setReplicas)

		compObj := &appsv1alpha1.ClusterComponentSpec{
			Name:            c.Name,
			ComponentDefRef: c.Name,
			Replicas:        replicas,
		}

		// class has higher priority than other resource related parameters
		className := getVal(keyClass, sets)
		if className != "" {
			compObj.ClassDefRef = &appsv1alpha1.ClassDefRef{Class: className}
		} else {
			resourceList := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(getVal(keyCPU, sets)),
				corev1.ResourceMemory: resource.MustParse(getVal(keyMemory, sets)),
			}
			compObj.Resources = corev1.ResourceRequirements{
				Requests: resourceList,
				Limits:   resourceList,
			}
			compObj.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{{
				Name: "data",
				Spec: appsv1alpha1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(getVal(keyStorage, sets)),
						},
					},
				},
			}}
		}
		if err := buildSwitchPolicy(&c, compObj, sets); err != nil {
			return nil, err
		}
		comps = append(comps, compObj)
	}
	return comps, nil
}

// buildCompSetsMap builds the map between component definition name and its set values, if the name is not
// specified in the set, use the cluster definition default component name.
func buildCompSetsMap(values []string, cd *appsv1alpha1.ClusterDefinition) (map[string]map[setKey]string, error) {
	allSets := map[string]map[setKey]string{}
	keys := []string{string(keyCPU), string(keyType), string(keyStorage), string(keyMemory), string(keyReplicas), string(keyClass), string(keySwitchPolicy)}
	parseKey := func(key string) setKey {
		for _, k := range keys {
			if strings.EqualFold(k, key) {
				return setKey(k)
			}
		}
		return keyUnknown
	}
	buildSetMap := func(sets []string) (map[setKey]string, error) {
		res := map[setKey]string{}
		for _, set := range sets {
			kv := strings.Split(set, "=")
			if len(kv) != 2 {
				return nil, fmt.Errorf("unknown set format \"%s\", should be like key1=value1", set)
			}

			// only record the supported key
			k := parseKey(kv[0])
			if k == keyUnknown {
				return nil, fmt.Errorf("unknown set key \"%s\", should be one of [%s]", kv[0], strings.Join(keys, ","))
			}
			res[k] = kv[1]
		}
		return res, nil
	}

	// each value corresponds to a component
	for _, value := range values {
		sets, err := buildSetMap(strings.Split(value, ","))
		if err != nil {
			return nil, err
		}
		if len(sets) == 0 {
			continue
		}

		// get the component definition name
		compDefName := sets[keyType]

		// type is not specified by user, use the default component definition name, now only
		// support cluster definition with one component
		if len(compDefName) == 0 {
			name, err := cluster.GetDefaultCompName(cd)
			if err != nil {
				return nil, err
			}
			compDefName = name
		}

		// if already set by other value, later values override earlier values
		if old, ok := allSets[compDefName]; ok {
			for k, v := range sets {
				old[k] = v
			}
			sets = old
		}
		allSets[compDefName] = sets
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
	cmd.Flags().StringVar(&f.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type, one of: (Preferred, Required)")
	cmd.Flags().BoolVar(&f.Monitor, "monitor", true, "Set monitor enabled and inject metrics exporter")
	cmd.Flags().BoolVar(&f.EnableAllLogs, "enable-all-logs", true, "Enable advanced application all log extraction, and true will ignore enabledLogs of component level")
	cmd.Flags().StringVar(&f.TerminationPolicy, "termination-policy", "Delete", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
	cmd.Flags().StringArrayVar(&f.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	cmd.Flags().StringToStringVar(&f.NodeLabels, "node-labels", nil, "Node label selector")
	cmd.Flags().StringSliceVar(&f.TolerationsRaw, "tolerations", nil, `Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'`)
	cmd.Flags().StringVar(&f.Tenancy, "tenancy", "SharedNode", "Tenancy options, one of: (SharedNode, DedicatedNode)")

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
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"pod-anti-affinity",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"Preferred\ttry to spread pods of the cluster by the specified topology-keys",
				"Required\tmust spread pods of the cluster by the specified topology-keys",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"tenancy",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"SharedNode\tpods of the cluster may share the same node",
				"DedicatedNode\teach pod of the cluster will run on their own dedicated node",
			}, cobra.ShellCompDirectiveNoFileComp
		}))
}
