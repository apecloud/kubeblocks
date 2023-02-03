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

	# --cluster-definition is required, if --cluster-version is not specified, will use the latest cluster version
	kbcli cluster create mycluster --cluster-definition=my-cluster-def

	# Create a cluster using file my.yaml and termination policy DoNotDelete that will prevent
	# the cluster from being deleted
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=DoNotDelete

	# In scenarios where you want to delete resources such as sts, deploy, svc, pdb, but keep pvcs when deleting
	# the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=Halt

	# In scenarios where you want to delete resource such as sts, deploy, svc, pdb, and including pvcs when
	# deleting the cluster, use termination policy Delete
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=Delete

	# In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
	# the cluster, use termination policy WipeOut
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=WipeOut

	# In scenarios where you want to load components data from website URL
	# the cluster, use termination policy Halt
	kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=https://kubeblocks.io/yamls/wesql_single.yaml --termination-policy=Halt

	# In scenarios where you want to load components data from stdin
	# the cluster, use termination policy Halt
	cat << EOF | kbcli cluster create mycluster --cluster-definition=my-cluster-def --termination-policy=Halt --set -
	- name: wesql-test... (omission from stdin)

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

	Sets           string   `json:"-"`
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
		fmt.Fprintf(o.Out, "Cluster version is not specified, use latest ClusterVersion %s\n", o.ClusterVersionRef)
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
	var (
		componentByte []byte
		err           error
		components    = o.Components
	)

	if len(o.Sets) > 0 {
		if componentByte, err = MultipleSourceComponents(o.Sets, o.IOStreams.In); err != nil {
			return err
		}
		if componentByte, err = yaml.YAMLToJSON(componentByte); err != nil {
			return err
		}
		if err = json.Unmarshal(componentByte, &components); err != nil {
			return err
		}
	} else if len(components) == 0 {
		if components, err = buildClusterComp(o.Client, o.ClusterDefRef); err != nil {
			return err
		}
	}
	setMonitor(o.Monitor, components)
	if err = setBackup(o, components); err != nil {
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
			cmd.Flags().StringVar(&o.Sets, "set", "", "Use yaml file, URL, or stdin to set the cluster parameters")
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

func buildClusterComp(dynamic dynamic.Interface, clusterDef string) ([]map[string]interface{}, error) {
	cd, err := cluster.GetClusterDefByName(dynamic, clusterDef)
	if err != nil {
		return nil, err
	}

	defaultStorageSize := viper.GetString("KBCLI_CLUSTER_DEFAULT_STORAGE_SIZE")
	if len(defaultStorageSize) == 0 {
		defaultStorageSize = "10Gi"
	}
	var comps []map[string]interface{}
	for _, c := range cd.Spec.Components {
		// if cluster definition component default replicas greater than 0, build a cluster component
		// by cluster definition component.
		r := c.DefaultReplicas
		if r <= 0 {
			continue
		}
		compObj := &dbaasv1alpha1.ClusterComponent{
			Name:     c.TypeName,
			Type:     c.TypeName,
			Replicas: &r,
			VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
				Name: "data",
				Spec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(defaultStorageSize),
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
		// check whether the cluster exists
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
